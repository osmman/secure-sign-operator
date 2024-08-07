//go:build integration

package e2e

import (
	"context"
	"encoding/json"
	"github.com/securesign/operator/api"
	ctlog "github.com/securesign/operator/internal/controller/ctlog/actions"
	fulcio "github.com/securesign/operator/internal/controller/fulcio/actions"
	rekor "github.com/securesign/operator/internal/controller/rekor/actions"
	tuf "github.com/securesign/operator/internal/controller/tuf/actions"
	appsv1 "k8s.io/api/apps/v1"
	"os"
	"time"

	"github.com/securesign/operator/internal/controller/common/utils"
	"github.com/securesign/operator/internal/controller/fulcio/actions"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/securesign/operator/api/v1alpha1"
	"github.com/securesign/operator/internal/controller/constants"
	"github.com/securesign/operator/test/e2e/support"
	"github.com/securesign/operator/test/e2e/support/tas"
	clients "github.com/securesign/operator/test/e2e/support/tas/cli"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeCli "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Securesign hot update", Ordered, func() {
	SetDefaultEventuallyTimeout(time.Duration(5) * time.Minute)
	cli, _ := CreateClient()
	ctx := context.TODO()

	var targetImageName string
	var namespace *v1.Namespace
	var securesign *v1alpha1.Securesign

	getDeploymentGeneration := func(nn types.NamespacedName) int64 {
		deployment := appsv1.Deployment{}
		if err := cli.Get(ctx, nn, &deployment); err != nil {
			return -1
		}
		return deployment.Status.ObservedGeneration
	}

	AfterEach(func() {
		if CurrentSpecReport().Failed() {
			if val, present := os.LookupEnv("CI"); present && val == "true" {
				support.DumpNamespace(ctx, cli, namespace.Name)
			}
		}
	})

	BeforeAll(func() {
		namespace = support.CreateTestNamespace(ctx, cli)
		DeferCleanup(func() {
			cli.Delete(ctx, namespace)
		})

		securesign = &v1alpha1.Securesign{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace.Name,
				Name:      "test",
				Annotations: map[string]string{
					"rhtas.redhat.com/metrics": "false",
				},
			},
			Spec: v1alpha1.SecuresignSpec{
				Rekor: v1alpha1.RekorSpec{
					ExternalAccess: api.ExternalAccess{
						Enabled: true,
					},
					RekorSearchUI: v1alpha1.RekorSearchUI{
						Enabled: utils.Pointer(false),
					},
				},
				Fulcio: v1alpha1.FulcioSpec{
					ExternalAccess: api.ExternalAccess{
						Enabled: true,
					},
					Config: v1alpha1.FulcioConfig{
						OIDCIssuers: []v1alpha1.OIDCIssuer{
							{
								ClientID:  support.OidcClientID(),
								IssuerURL: support.OidcIssuerUrl(),
								Issuer:    support.OidcIssuerUrl(),
								Type:      "email",
							},
						}},
					Certificate: v1alpha1.FulcioCert{
						OrganizationName:  "MyOrg",
						OrganizationEmail: "my@email.org",
						CommonName:        "fulcio",
					},
				},
				Ctlog: v1alpha1.CTlogSpec{},
				Tuf: v1alpha1.TufSpec{
					ExternalAccess: api.ExternalAccess{
						Enabled: true,
					},
				},
				Trillian: v1alpha1.TrillianSpec{Db: v1alpha1.TrillianDB{
					Create: utils.Pointer(true),
				}},
			},
		}
	})

	BeforeAll(func() {
		targetImageName = support.PrepareImage(ctx)
	})

	Describe("Install with autogenerated certificates", func() {
		BeforeAll(func() {
			Expect(cli.Create(ctx, securesign)).To(Succeed())
		})

		It("All other components are running", func() {
			tas.VerifySecuresign(ctx, cli, namespace.Name, securesign.Name)
			tas.VerifyTrillian(ctx, cli, namespace.Name, securesign.Name, true)
			tas.VerifyCTLog(ctx, cli, namespace.Name, securesign.Name)
			tas.VerifyTuf(ctx, cli, namespace.Name, securesign.Name)
			tas.VerifyRekor(ctx, cli, namespace.Name, securesign.Name)
			tas.VerifyFulcio(ctx, cli, namespace.Name, securesign.Name)
		})

	})
	Describe("Inject Fulcio CA", func() {
		It("Pods are restarted after update", func() {
			By("Storing current deployment observed generations")
			tufGeneration := getDeploymentGeneration(types.NamespacedName{Namespace: namespace.Name, Name: tuf.DeploymentName})
			Expect(tufGeneration).Should(BeNumerically(">", 0))
			ctlogGeneration := getDeploymentGeneration(types.NamespacedName{Namespace: namespace.Name, Name: ctlog.DeploymentName})
			Expect(ctlogGeneration).Should(BeNumerically(">", 0))
			fulcioGeneration := getDeploymentGeneration(types.NamespacedName{Namespace: namespace.Name, Name: fulcio.DeploymentName})
			Expect(fulcioGeneration).Should(BeNumerically(">", 0))

			Expect(cli.Get(ctx, runtimeCli.ObjectKeyFromObject(securesign), securesign)).To(Succeed())
			securesign.Spec.Fulcio.Certificate = v1alpha1.FulcioCert{
				PrivateKeyRef: &api.SecretKeySelector{
					LocalObjectReference: api.LocalObjectReference{
						Name: "my-fulcio-secret",
					},
					Key: "private",
				},
				PrivateKeyPasswordRef: &api.SecretKeySelector{
					LocalObjectReference: api.LocalObjectReference{
						Name: "my-fulcio-secret",
					},
					Key: "password",
				},
				CARef: &api.SecretKeySelector{
					LocalObjectReference: api.LocalObjectReference{
						Name: "my-fulcio-secret",
					},
					Key: "cert",
				},
			}

			Expect(cli.Update(ctx, securesign)).To(Succeed())
			Eventually(func() string {
				fulcio := tas.GetFulcio(ctx, cli, namespace.Name, securesign.Name)()
				return meta.FindStatusCondition(fulcio.Status.Conditions, constants.Ready).Reason
			}).Should(Equal(constants.Pending))

			Expect(cli.Create(ctx, initFulcioSecret(namespace.Name, "my-fulcio-secret")))

			Eventually(func() int64 {
				return getDeploymentGeneration(types.NamespacedName{Namespace: namespace.Name, Name: tuf.DeploymentName})
			}).Should(BeNumerically(">", tufGeneration))

			Eventually(func() int64 {
				return getDeploymentGeneration(types.NamespacedName{Namespace: namespace.Name, Name: ctlog.DeploymentName})
			}).Should(BeNumerically(">", ctlogGeneration))

			Eventually(func() int64 {
				return getDeploymentGeneration(types.NamespacedName{Namespace: namespace.Name, Name: fulcio.DeploymentName})
			}).Should(BeNumerically(">", fulcioGeneration))

			tas.VerifyTuf(ctx, cli, namespace.Name, securesign.Name)
			tas.VerifyFulcio(ctx, cli, namespace.Name, securesign.Name)
		})

		It("Verify new configuration", func() {
			var fulcioPod *v1.Pod
			Eventually(func(g Gomega) {
				fulcioPod = tas.GetFulcioServerPod(ctx, cli, namespace.Name)()
				g.Expect(fulcioPod).ToNot(BeNil())
			}).Should(Succeed())
			Expect(fulcioPod.Spec.Volumes).To(ContainElements(And(
				WithTransform(func(v v1.Volume) string { return v.Name }, Equal("fulcio-cert")),
				WithTransform(func(v v1.Volume) string { return v.VolumeSource.Projected.Sources[0].Secret.Name }, Equal("my-fulcio-secret")))))

		})

	})

	Describe("Fulcio Config update", func() {
		It("Pods are restarted after update", func() {
			By("Storing current deployment observed generations")
			fulcioGeneration := getDeploymentGeneration(types.NamespacedName{Namespace: namespace.Name, Name: fulcio.DeploymentName})
			Expect(fulcioGeneration).Should(BeNumerically(">", 0))

			Expect(cli.Get(ctx, runtimeCli.ObjectKeyFromObject(securesign), securesign)).To(Succeed())
			securesign.Spec.Fulcio.Config.OIDCIssuers = []v1alpha1.OIDCIssuer{
				{
					ClientID:  support.OidcClientID(),
					IssuerURL: support.OidcIssuerUrl(),
					Issuer:    support.OidcIssuerUrl(),
					Type:      "email",
				},
				{
					ClientID:  "fake",
					IssuerURL: "fake",
					Issuer:    "fake",
					Type:      "email",
				},
			}
			Expect(cli.Update(ctx, securesign)).To(Succeed())

			Eventually(func() int64 {
				return getDeploymentGeneration(types.NamespacedName{Namespace: namespace.Name, Name: fulcio.DeploymentName})
			}).Should(BeNumerically(">", fulcioGeneration))

			tas.VerifyFulcio(ctx, cli, namespace.Name, securesign.Name)
		})
		It("Verify new configuration", func() {
			var fulcio *v1alpha1.Fulcio
			var fulcioPod *v1.Pod
			Eventually(func(g Gomega) {
				fulcio = tas.GetFulcio(ctx, cli, namespace.Name, securesign.Name)()
				g.Expect(fulcio).NotTo(BeNil())
				fulcioPod = tas.GetFulcioServerPod(ctx, cli, namespace.Name)()
				g.Expect(fulcioPod).NotTo(BeNil())
			}).Should(Succeed())

			Expect(fulcioPod.Spec.Volumes[0].VolumeSource.ConfigMap.Name).To(Equal(fulcio.Status.ServerConfigRef.Name))

			cm := &v1.ConfigMap{}
			Expect(cli.Get(ctx, types.NamespacedName{Namespace: namespace.Name, Name: fulcio.Status.ServerConfigRef.Name}, cm)).To(Succeed())
			config := &actions.FulcioMapConfig{}
			Expect(json.Unmarshal([]byte(cm.Data["config.json"]), config)).To(Succeed())
			Expect(config.OIDCIssuers).To(HaveKey("fake"))
		})
	})

	Describe("Inject Rekor signer", func() {
		It("Pods are restarted after update", func() {
			By("Storing current deployment observed generations")
			tufGeneration := getDeploymentGeneration(types.NamespacedName{Namespace: namespace.Name, Name: tuf.DeploymentName})
			Expect(tufGeneration).Should(BeNumerically(">", 0))
			rekorGeneration := getDeploymentGeneration(types.NamespacedName{Namespace: namespace.Name, Name: rekor.ServerComponentName})
			Expect(rekorGeneration).Should(BeNumerically(">", 0))

			Expect(cli.Get(ctx, runtimeCli.ObjectKeyFromObject(securesign), securesign)).To(Succeed())
			securesign.Spec.Rekor.Signer = v1alpha1.RekorSigner{
				KMS: "secret",
				KeyRef: &api.SecretKeySelector{
					LocalObjectReference: api.LocalObjectReference{
						Name: "my-rekor-secret",
					},
					Key: "private",
				},
			}

			Expect(cli.Update(ctx, securesign)).To(Succeed())
			Eventually(func() string {
				rekor := tas.GetRekor(ctx, cli, namespace.Name, securesign.Name)()
				return meta.FindStatusCondition(rekor.Status.Conditions, constants.Ready).Reason
			}).Should(Equal(constants.Pending))

			Expect(cli.Create(ctx, initRekorSecret(namespace.Name, "my-rekor-secret")))

			Eventually(func() int64 {
				return getDeploymentGeneration(types.NamespacedName{Namespace: namespace.Name, Name: tuf.DeploymentName})
			}).Should(BeNumerically(">", tufGeneration))

			Eventually(func() int64 {
				return getDeploymentGeneration(types.NamespacedName{Namespace: namespace.Name, Name: rekor.ServerComponentName})
			}).Should(BeNumerically(">", rekorGeneration))

			tas.VerifyTuf(ctx, cli, namespace.Name, securesign.Name)
			tas.VerifyRekor(ctx, cli, namespace.Name, securesign.Name)
		})
		It("Verify new configuration", func() {
			var rekor *v1.Pod
			Eventually(func(g Gomega) {
				rekor = tas.GetRekorServerPod(ctx, cli, namespace.Name)()
				g.Expect(rekor).NotTo(BeNil())
			}).Should(Succeed())
			Expect(rekor.Spec.Volumes).To(ContainElements(And(
				WithTransform(func(v v1.Volume) string { return v.Name }, Equal("rekor-private-key-volume")),
				WithTransform(func(v v1.Volume) string { return v.VolumeSource.Secret.SecretName }, Equal("my-rekor-secret")))))

		})
	})

	Describe("Inject CTL secret", func() {
		It("Pods are restarted after update", func() {
			By("Storing current deployment observed generations")
			tufGeneration := getDeploymentGeneration(types.NamespacedName{Namespace: namespace.Name, Name: tuf.DeploymentName})
			Expect(tufGeneration).Should(BeNumerically(">", 0))
			ctlogGeneration := getDeploymentGeneration(types.NamespacedName{Namespace: namespace.Name, Name: ctlog.DeploymentName})
			Expect(ctlogGeneration).Should(BeNumerically(">", 0))

			Expect(cli.Get(ctx, runtimeCli.ObjectKeyFromObject(securesign), securesign)).To(Succeed())
			securesign.Spec.Ctlog.PrivateKeyRef = &api.SecretKeySelector{
				LocalObjectReference: api.LocalObjectReference{
					Name: "my-ctlog-secret",
				},
				Key: "private",
			}
			securesign.Spec.Ctlog.PublicKeyRef = &api.SecretKeySelector{
				LocalObjectReference: api.LocalObjectReference{
					Name: "my-ctlog-secret",
				},
				Key: "public",
			}

			Expect(cli.Update(ctx, securesign)).To(Succeed())
			Eventually(func() string {
				ctl := tas.GetCTLog(ctx, cli, namespace.Name, securesign.Name)()
				return meta.FindStatusCondition(ctl.Status.Conditions, constants.Ready).Reason
			}).Should(Equal(constants.Creating))
			Expect(cli.Create(ctx, initCTSecret(namespace.Name, "my-ctlog-secret")))

			Eventually(func() int64 {
				return getDeploymentGeneration(types.NamespacedName{Namespace: namespace.Name, Name: tuf.DeploymentName})
			}).Should(BeNumerically(">", tufGeneration))

			Eventually(func() int64 {
				return getDeploymentGeneration(types.NamespacedName{Namespace: namespace.Name, Name: ctlog.DeploymentName})
			}).Should(BeNumerically(">", ctlogGeneration))

			tas.VerifyTuf(ctx, cli, namespace.Name, securesign.Name)
			tas.VerifyCTLog(ctx, cli, namespace.Name, securesign.Name)
		})
		It("Verify new configuration", func() {
			var ctl *v1alpha1.CTlog
			var ctlPod *v1.Pod
			Eventually(func(g Gomega) {
				ctl = tas.GetCTLog(ctx, cli, namespace.Name, securesign.Name)()
				g.Expect(ctl).NotTo(BeNil())
				ctlPod = tas.GetCTLogServerPod(ctx, cli, namespace.Name)()
				g.Expect(ctlPod).NotTo(BeNil())
			}).Should(Succeed())

			Expect(ctlPod.Spec.Volumes).To(ContainElements(And(
				WithTransform(func(v v1.Volume) string { return v.Name }, Equal("keys")),
				WithTransform(func(v v1.Volume) string { return v.VolumeSource.Secret.SecretName }, Equal(ctl.Status.ServerConfigRef.Name)))))

			existing := &v1.Secret{}
			expected := &v1.Secret{}
			Expect(cli.Get(ctx, types.NamespacedName{Namespace: namespace.Name, Name: ctl.Status.ServerConfigRef.Name}, existing)).To(Succeed())
			Expect(cli.Get(ctx, types.NamespacedName{Namespace: namespace.Name, Name: "my-ctlog-secret"}, expected)).To(Succeed())

			Expect(existing.Data["public"]).To(Equal(existing.Data["public"]))
		})
	})

	It("Use cosign cli", func() {
		fulcio := tas.GetFulcio(ctx, cli, namespace.Name, securesign.Name)()
		Expect(fulcio).ToNot(BeNil())

		rekor := tas.GetRekor(ctx, cli, namespace.Name, securesign.Name)()
		Expect(rekor).ToNot(BeNil())

		tuf := tas.GetTuf(ctx, cli, namespace.Name, securesign.Name)()
		Expect(tuf).ToNot(BeNil())

		oidcToken, err := support.OidcToken(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(oidcToken).ToNot(BeEmpty())

		// sleep for a while to be sure everything has settled down
		time.Sleep(time.Duration(10) * time.Second)

		Expect(clients.Execute("cosign", "initialize", "--mirror="+tuf.Status.Url, "--root="+tuf.Status.Url+"/root.json")).To(Succeed())

		Expect(clients.Execute(
			"cosign", "sign", "-y",
			"--fulcio-url="+fulcio.Status.Url,
			"--rekor-url="+rekor.Status.Url,
			"--oidc-issuer="+support.OidcIssuerUrl(),
			"--oidc-client-id="+support.OidcClientID(),
			"--identity-token="+oidcToken,
			targetImageName,
		)).To(Succeed())

		Expect(clients.Execute(
			"cosign", "verify",
			"--rekor-url="+rekor.Status.Url,
			"--certificate-identity-regexp", ".*@redhat",
			"--certificate-oidc-issuer-regexp", ".*keycloak.*",
			targetImageName,
		)).To(Succeed())
	})
})

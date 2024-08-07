package server

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/securesign/operator/api"

	"github.com/securesign/operator/api/v1alpha1"
	"github.com/securesign/operator/internal/controller/common/action"
	k8sutils "github.com/securesign/operator/internal/controller/common/utils/kubernetes"
	"github.com/securesign/operator/internal/controller/constants"
	"github.com/securesign/operator/internal/controller/rekor/actions"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const secretNameFormat = "rekor-signer-%s-"

func NewGenerateSignerAction() action.Action[*v1alpha1.Rekor] {
	return &generateSigner{}
}

type generateSigner struct {
	action.BaseAction
}

func (g generateSigner) Name() string {
	return "generate-signer"
}

func (g generateSigner) CanHandle(_ context.Context, instance *v1alpha1.Rekor) bool {
	c := meta.FindStatusCondition(instance.Status.Conditions, constants.Ready)
	if c == nil {
		return false
	}
	if c.Reason != constants.Pending && c.Reason != constants.Ready {
		return false
	}

	return instance.Status.Signer.KeyRef == nil || !equality.Semantic.DeepDerivative(instance.Spec.Signer, instance.Status.Signer)

}

func (g generateSigner) Handle(ctx context.Context, instance *v1alpha1.Rekor) *action.Result {
	if instance.Spec.Signer.KMS != "secret" && instance.Spec.Signer.KMS != "" {
		instance.Status.Signer = instance.Spec.Signer
		// force recreation of public key ref
		instance.Status.PublicKeyRef = nil
		// skip signer resolution and move to creating
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:   constants.Ready,
			Status: metav1.ConditionFalse,
			Reason: constants.Creating,
		})
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:    actions.SignerCondition,
			Status:  metav1.ConditionTrue,
			Reason:  constants.Ready,
			Message: "Not using Secret resource",
		})
		return g.StatusUpdate(ctx, instance)
	}

	// Return to pending state because Signer spec changed
	if meta.FindStatusCondition(instance.Status.Conditions, constants.Ready).Reason != constants.Pending {
		// force recreation of public key ref
		instance.Status.PublicKeyRef = nil
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:   constants.Ready,
			Status: metav1.ConditionFalse,
			Reason: constants.Pending,
		},
		)
		return g.StatusUpdate(ctx, instance)
	}
	var (
		err error
	)

	certConfig, err := g.CreateRekorKey(instance)
	if err != nil {
		if !meta.IsStatusConditionFalse(instance.Status.Conditions, actions.SignerCondition) {
			meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
				Type:    actions.SignerCondition,
				Status:  metav1.ConditionFalse,
				Reason:  constants.Failure,
				Message: err.Error(),
			})
			meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
				Type:    actions.ServerCondition,
				Status:  metav1.ConditionFalse,
				Reason:  constants.Pending,
				Message: "resolving keys",
			})
			meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
				Type:    constants.Ready,
				Status:  metav1.ConditionFalse,
				Reason:  constants.Pending,
				Message: "resolving keys",
			})
			return g.StatusUpdate(ctx, instance)
		}
		// swallow error and retry
		return g.Requeue()
	}

	labels := constants.LabelsFor(actions.ServerComponentName, actions.ServerDeploymentName, instance.Name)

	data := make(map[string][]byte)
	if certConfig.RekorKey != nil {
		data["private"] = certConfig.RekorKey
	}
	if certConfig.RekorKeyPassword != nil {
		data["password"] = certConfig.RekorKeyPassword
	}
	if certConfig.RekorPubKey != nil {
		data["public"] = certConfig.RekorPubKey
	}
	secret := k8sutils.CreateImmutableSecret(fmt.Sprintf(secretNameFormat, instance.Name), instance.Namespace,
		data, labels)
	if _, err = g.Ensure(ctx, secret); err != nil {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:    actions.ServerCondition,
			Status:  metav1.ConditionFalse,
			Reason:  constants.Failure,
			Message: err.Error(),
		})
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:    constants.Ready,
			Status:  metav1.ConditionFalse,
			Reason:  constants.Failure,
			Message: err.Error(),
		})
		return g.FailedWithStatusUpdate(ctx, fmt.Errorf("could not create secret: %w", err), instance)
	}
	g.Recorder.Eventf(instance, v1.EventTypeNormal, "SignerKeyCreated", "Signer private key created: %s", secret.Name)

	instance.Status.Signer = instance.Spec.Signer
	if instance.Spec.Signer.KeyRef == nil {
		instance.Status.Signer.KeyRef = &api.SecretKeySelector{
			Key: "private",
			LocalObjectReference: api.LocalObjectReference{
				Name: secret.Name,
			},
		}
	}
	if _, ok := secret.Data["password"]; instance.Spec.Signer.PasswordRef == nil && ok {
		instance.Status.Signer.PasswordRef = &api.SecretKeySelector{
			Key: "password",
			LocalObjectReference: api.LocalObjectReference{
				Name: secret.Name,
			},
		}
	} else {
		instance.Status.Signer.PasswordRef = instance.Spec.Signer.PasswordRef
	}
	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:   actions.ServerCondition,
		Status: metav1.ConditionFalse,
		Reason: constants.Creating,
	})
	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:   actions.SignerCondition,
		Status: metav1.ConditionTrue,
		Reason: constants.Ready,
	})
	return g.StatusUpdate(ctx, instance)
}

type RekorCertConfig struct {
	RekorKey         []byte
	RekorPubKey      []byte
	RekorKeyPassword []byte
}

func (g generateSigner) CreateRekorKey(instance *v1alpha1.Rekor) (*RekorCertConfig, error) {
	var err error
	if instance.Spec.Signer.KeyRef != nil {
		config := &RekorCertConfig{}
		config.RekorKey, err = k8sutils.GetSecretData(g.Client, instance.Namespace, instance.Spec.Signer.KeyRef)
		if err != nil {
			return nil, err
		}
		if instance.Spec.Signer.PasswordRef != nil {
			config.RekorKeyPassword, err = k8sutils.GetSecretData(g.Client, instance.Namespace, instance.Spec.Signer.PasswordRef)
			if err != nil {
				return nil, err
			}
		}

		return config, nil
	}

	key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return nil, err
	}

	mKey, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}

	mPubKey, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		return nil, err
	}

	var pemRekorKey bytes.Buffer
	err = pem.Encode(&pemRekorKey, &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: mKey,
	})
	if err != nil {
		return nil, err
	}

	var pemPubKey bytes.Buffer
	err = pem.Encode(&pemPubKey, &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: mPubKey,
	})
	if err != nil {
		return nil, err
	}

	return &RekorCertConfig{
		RekorKey:    pemRekorKey.Bytes(),
		RekorPubKey: pemPubKey.Bytes(),
	}, nil
}

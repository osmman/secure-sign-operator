package logserver

import (
	"context"
	"fmt"

	"github.com/securesign/operator/internal/controller/common/action"
	"github.com/securesign/operator/internal/controller/common/utils/kubernetes"
	"github.com/securesign/operator/internal/controller/common/utils/kubernetes/ensure"
	"github.com/securesign/operator/internal/controller/constants"
	"github.com/securesign/operator/internal/controller/labels"
	"github.com/securesign/operator/internal/controller/trillian/actions"
	trillianUtils "github.com/securesign/operator/internal/controller/trillian/utils"
	"golang.org/x/exp/maps"
	apps "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	rhtasv1alpha1 "github.com/securesign/operator/api/v1alpha1"
)

func NewDeployAction() action.Action[*rhtasv1alpha1.Trillian] {
	return &deployAction{}
}

type deployAction struct {
	action.BaseAction
}

func (i deployAction) Name() string {
	return "deploy"
}

func (i deployAction) CanHandle(_ context.Context, instance *rhtasv1alpha1.Trillian) bool {
	c := meta.FindStatusCondition(instance.Status.Conditions, constants.Ready)
	return c.Reason == constants.Creating || c.Reason == constants.Ready
}

func (i deployAction) Handle(ctx context.Context, instance *rhtasv1alpha1.Trillian) *action.Result {
	var (
		err    error
		result controllerutil.OperationResult
	)

	labels := labels.For(actions.LogServerComponentName, actions.LogserverDeploymentName, instance.Name)
	insCopy := instance.DeepCopy()

	if insCopy.Spec.TrustedCA == nil {
		insCopy.Spec.TrustedCA = ensure.TrustedCAAnnotationToReference(instance.Annotations)
	}

	if result, err = kubernetes.CreateOrUpdate(ctx, i.Client,
		&apps.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      actions.LogserverDeploymentName,
				Namespace: instance.Namespace,
			},
		},
		trillianUtils.EnsureServerDeployment(insCopy, constants.TrillianServerImage, actions.LogserverDeploymentName, actions.RBACName, labels),
		ensure.ControllerReference[*apps.Deployment](insCopy, i.Client),
		ensure.Labels[*apps.Deployment](maps.Keys(labels), labels),
		ensure.Proxy(),
		ensure.TrustedCA(insCopy.Spec.TrustedCA),
		ensure.Optional(trillianUtils.UseTLS(insCopy), i.withTlsDB(ctx, insCopy)),
	); err != nil {
		return i.Error(ctx, fmt.Errorf("could not create Trillian server: %w", err), instance, metav1.Condition{
			Type:    actions.ServerCondition,
			Status:  metav1.ConditionFalse,
			Reason:  constants.Failure,
			Message: err.Error(),
		})
	}

	if result != controllerutil.OperationResultNone {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:    actions.ServerCondition,
			Status:  metav1.ConditionFalse,
			Reason:  constants.Creating,
			Message: "Deployment created",
		})
		return i.StatusUpdate(ctx, instance)
	} else {
		return i.Continue()
	}
}

func (i deployAction) withTlsDB(ctx context.Context, instance *rhtasv1alpha1.Trillian) func(deployment *apps.Deployment) error {
	return func(dp *apps.Deployment) error {
		caPath, err := trillianUtils.CAPath(ctx, i.Client, instance)
		if err != nil {
			return fmt.Errorf("failed to get CA path: %w", err)
		}

		c := kubernetes.FindContainerByNameOrCreate(&dp.Spec.Template.Spec, actions.LogserverDeploymentName)
		c.Args = append(c.Args, "--mysql_tls_ca", caPath)

		mysqlServerName := "$(MYSQL_HOSTNAME)." + instance.Namespace + ".svc"
		if !*instance.Spec.Db.Create {
			mysqlServerName = "$(MYSQL_HOSTNAME)"
		}
		c.Args = append(c.Args, "--mysql_server_name", mysqlServerName)
		return nil
	}

}

package ui

import (
	actionK8s "github.com/securesign/operator/internal/controller/common/action/kubernetes"
	"github.com/securesign/operator/internal/controller/common/utils"
	"github.com/securesign/operator/internal/controller/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/intstr"

	rhtasv1alpha1 "github.com/securesign/operator/api/v1alpha1"
	"github.com/securesign/operator/internal/controller/common/action"
	"github.com/securesign/operator/internal/controller/rekor/actions"
)

func NewServiceAction() action.Action[*rhtasv1alpha1.Rekor] {
	return actionK8s.NewServiceAction[*rhtasv1alpha1.Rekor](
		actionK8s.Component{
			Name:     actions.UIComponentName,
			Instance: actions.SearchUiDeploymentName,
		},
		actions.UICondition,
		actionK8s.CanHandle(func(instance *rhtasv1alpha1.Rekor) bool {
			c := meta.FindStatusCondition(instance.Status.Conditions, actions.UICondition)
			if c == nil {
				return false
			}
			return (c.Reason == constants.Creating || c.Reason == constants.Ready) && utils.IsEnabled(instance.Spec.RekorSearchUI.Enabled)
		}),
		actionK8s.MutateFn(func(expected *corev1.Service, instance *rhtasv1alpha1.Rekor) error {
			ports := []corev1.ServicePort{
				{
					Name:       actions.SearchUiDeploymentPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       actions.SearchUiDeploymentPort,
					TargetPort: intstr.FromInt32(actions.SearchUiDeploymentPort),
				},
			}

			expected.Spec.Ports = ports
			return nil
		}),
	)
}

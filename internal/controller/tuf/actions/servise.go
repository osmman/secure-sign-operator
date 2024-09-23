package actions

import (
	actionK8s "github.com/securesign/operator/internal/controller/common/action/kubernetes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	rhtasv1alpha1 "github.com/securesign/operator/api/v1alpha1"
	"github.com/securesign/operator/internal/controller/common/action"
	"github.com/securesign/operator/internal/controller/constants"
)

func NewServiceAction() action.Action[*rhtasv1alpha1.Tuf] {
	return actionK8s.NewServiceAction[*rhtasv1alpha1.Tuf](
		actionK8s.Component{
			Name:     ComponentName,
			Instance: DeploymentName,
		},
		constants.Ready,
		actionK8s.MutateFn(func(expected *corev1.Service, instance *rhtasv1alpha1.Tuf) error {
			ports := []corev1.ServicePort{
				{
					Name:       PortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       Port,
					TargetPort: intstr.FromInt32(Port),
				},
			}

			expected.Spec.Ports = ports
			return nil
		}))
}

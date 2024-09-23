package redis

import (
	actionK8s "github.com/securesign/operator/internal/controller/common/action/kubernetes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	rhtasv1alpha1 "github.com/securesign/operator/api/v1alpha1"
	"github.com/securesign/operator/internal/controller/common/action"
	"github.com/securesign/operator/internal/controller/rekor/actions"
)

func NewServiceAction() action.Action[*rhtasv1alpha1.Rekor] {
	return actionK8s.NewServiceAction[*rhtasv1alpha1.Rekor](
		actionK8s.Component{
			Name:     actions.RedisComponentName,
			Instance: actions.RedisDeploymentName,
		},
		actions.RedisCondition,
		actionK8s.MutateFn(func(expected *corev1.Service, instance *rhtasv1alpha1.Rekor) error {
			ports := []corev1.ServicePort{
				{
					Name:       actions.RedisDeploymentPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       actions.RedisDeploymentPort,
					TargetPort: intstr.FromInt32(actions.RedisDeploymentPort),
				},
			}

			expected.Spec.Ports = ports
			return nil
		}),
	)
}

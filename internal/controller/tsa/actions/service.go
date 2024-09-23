package actions

import (
	rhtasv1alpha1 "github.com/securesign/operator/api/v1alpha1"
	"github.com/securesign/operator/internal/controller/common/action"
	actionK8s "github.com/securesign/operator/internal/controller/common/action/kubernetes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func NewServiceAction() action.Action[*rhtasv1alpha1.TimestampAuthority] {
	return actionK8s.NewServiceAction[*rhtasv1alpha1.TimestampAuthority](
		actionK8s.Component{
			Name:     ComponentName,
			Instance: DeploymentName,
		},
		TSAServerCondition,
		actionK8s.MutateFn(func(expected *corev1.Service, instance *rhtasv1alpha1.TimestampAuthority) error {
			ports := []corev1.ServicePort{
				{
					Name:       ServerPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       ServerPort,
					TargetPort: intstr.FromInt32(ServerPort),
				},
			}

			if instance.Spec.Monitoring.Enabled {
				ports = append(ports, corev1.ServicePort{
					Name:       MetricsPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       MetricsPort,
					TargetPort: intstr.FromInt32(MetricsPort),
				})
			}

			expected.Spec.Ports = ports
			return nil
		}))
}

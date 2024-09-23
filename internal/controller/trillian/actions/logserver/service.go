package logserver

import (
	actionK8s "github.com/securesign/operator/internal/controller/common/action/kubernetes"

	rhtasv1alpha1 "github.com/securesign/operator/api/v1alpha1"
	"github.com/securesign/operator/internal/controller/common/action"
	"github.com/securesign/operator/internal/controller/trillian/actions"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func NewServiceAction() action.Action[*rhtasv1alpha1.Trillian] {
	return actionK8s.NewServiceAction[*rhtasv1alpha1.Trillian](
		actionK8s.Component{
			Name:     actions.LogServerComponentName,
			Instance: actions.LogserverDeploymentName,
		}, actions.ServerCondition,
		actionK8s.MutateFn(func(expected *corev1.Service, instance *rhtasv1alpha1.Trillian) error {
			ports := []corev1.ServicePort{
				{
					Name:       actions.ServerPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       actions.ServerPort,
					TargetPort: intstr.FromInt32(actions.ServerPort),
				},
			}

			if instance.Spec.Monitoring.Enabled {
				ports = append(ports, corev1.ServicePort{
					Name:       actions.MetricsPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       int32(actions.MetricsPort),
					TargetPort: intstr.FromInt32(actions.MetricsPort),
				})
			}

			expected.Spec.Ports = ports
			return nil
		}))
}

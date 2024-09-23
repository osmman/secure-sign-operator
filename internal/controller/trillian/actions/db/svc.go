package db

import (
	"fmt"

	actionK8s "github.com/securesign/operator/internal/controller/common/action/kubernetes"
	"github.com/securesign/operator/internal/controller/common/utils"
	"github.com/securesign/operator/internal/controller/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/intstr"

	rhtasv1alpha1 "github.com/securesign/operator/api/v1alpha1"
	"github.com/securesign/operator/internal/controller/annotations"
	"github.com/securesign/operator/internal/controller/common/action"
	k8sutils "github.com/securesign/operator/internal/controller/common/utils/kubernetes"
	"github.com/securesign/operator/internal/controller/trillian/actions"
)

func NewServiceAction() action.Action[*rhtasv1alpha1.Trillian] {
	return actionK8s.NewServiceAction[*rhtasv1alpha1.Trillian](
		actionK8s.Component{
			Name:     actions.DbComponentName,
			Instance: actions.DbDeploymentName,
		},
		actions.DbCondition,
		actionK8s.MutateFn(func(expected *corev1.Service, instance *rhtasv1alpha1.Trillian) error {
			ports := []corev1.ServicePort{
				{
					Name:       actions.DbPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       actions.DbPort,
					TargetPort: intstr.FromInt32(actions.DbPort),
				},
			}

			expected.Spec.Ports = ports
			return nil
		}),
		actionK8s.MutateFn(func(expected *corev1.Service, instance *rhtasv1alpha1.Trillian) error {
			//TLS: Annotate service
			if k8sutils.IsOpenShift() && instance.Spec.Db.TLS.CertRef == nil {
				if expected.Annotations == nil {
					expected.Annotations = make(map[string]string)
				}
				expected.Annotations[annotations.TLS] = fmt.Sprintf(k8sutils.TlsSecretNameMask, instance.GetName())
			}
			return nil
		}),
		actionK8s.CanHandle(func(instance *rhtasv1alpha1.Trillian) bool {
			c := meta.FindStatusCondition(instance.Status.Conditions, actions.DbCondition)
			if c == nil {
				return false
			}
			return (c.Reason == constants.Creating || c.Reason == constants.Ready) && utils.OptionalBool(instance.Spec.Db.Create)
		}))
}

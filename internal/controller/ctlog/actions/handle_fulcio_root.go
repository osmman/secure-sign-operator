package actions

import (
	"context"
	"github.com/securesign/operator/api"
	"slices"

	"github.com/securesign/operator/api/v1alpha2"
	"github.com/securesign/operator/internal/controller/common/action"
	k8sutils "github.com/securesign/operator/internal/controller/common/utils/kubernetes"
	"github.com/securesign/operator/internal/controller/constants"
	"github.com/securesign/operator/internal/controller/fulcio/actions"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewHandleFulcioCertAction() action.Action[*v1alpha2.CTlog] {
	return &handleFulcioCert{}
}

type handleFulcioCert struct {
	action.BaseAction
}

func (g handleFulcioCert) Name() string {
	return "handle-fulcio-cert"
}

func (g handleFulcioCert) CanHandle(ctx context.Context, instance *v1alpha2.CTlog) bool {
	c := meta.FindStatusCondition(instance.GetConditions(), constants.Ready)
	if c.Reason != constants.Creating && c.Reason != constants.Ready {
		return false
	}

	// if is not read and missing RootsPEMFile inject Fulcio certificate and status is not set

	instance.Spec.LogConfig[0].RootsPemFile = nil // search all Fulcio certs and filter these which contains latest



	if len(instance.Status.RootCertificates) == 0 {
		return true
	}

	if !equality.Semantic.DeepDerivative(instance.Spec.RootCertificates, instance.Status.RootCertificates) {
		return true
	}

	if len(instance.Spec.RootCertificates) == 0 {
		// test if autodiscovery find new secret
		if scr, _ := k8sutils.FindSecret(ctx, g.Client, instance.Namespace, actions.FulcioCALabel); scr != nil {
			return !slices.Contains(instance.Status.RootCertificates, api.SecretKeySelector{
				LocalObjectReference: api.LocalObjectReference{Name: scr.Name},
				Key:                  scr.Labels[actions.FulcioCALabel],
			})
		}
	}

	return false
}

func (g handleFulcioCert) Handle(ctx context.Context, instance *v1alpha2.CTlog) *action.Result {

	if meta.FindStatusCondition(instance.Status.Conditions, constants.Ready).Reason != constants.Creating {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:   constants.Ready,
			Status: metav1.ConditionFalse,
			Reason: constants.Creating,
		},
		)
		return g.StatusUpdate(ctx, instance)
	}

	if len(instance.Spec.RootCertificates) == 0 {
		scr, err := k8sutils.FindSecret(ctx, g.Client, instance.Namespace, actions.FulcioCALabel)
		if err != nil {
			if !k8sErrors.IsNotFound(err) {
				return g.Failed(err)
			}

			meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
				Type:    CertCondition,
				Status:  metav1.ConditionFalse,
				Reason:  constants.Failure,
				Message: "Cert not found",
			})
			g.StatusUpdate(ctx, instance)
			return g.Requeue()
		}
		instance.Status.RootCertificates = []api.SecretKeySelector{
			{
				LocalObjectReference: api.LocalObjectReference{
					Name: scr.Name,
				},
				Key: scr.Labels[actions.FulcioCALabel],
			},
		}
	} else {
		instance.Status.RootCertificates = instance.Spec.RootCertificates
	}

	// invalidate server config
	if instance.Status.ServerConfigRef != nil {
		if err := g.Client.Delete(ctx, &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      instance.Status.ServerConfigRef.Name,
				Namespace: instance.Namespace,
			},
		}); err != nil {
			if !k8sErrors.IsNotFound(err) {
				return g.Failed(err)
			}
		}
		instance.Status.ServerConfigRef = nil
	}

	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:   CertCondition,
		Status: metav1.ConditionTrue,
		Reason: "Resolved",
	},
	)
	return g.StatusUpdate(ctx, instance)
}

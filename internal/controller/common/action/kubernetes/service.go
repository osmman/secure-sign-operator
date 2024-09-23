package kubernetes

import (
	"context"
	"fmt"

	"github.com/securesign/operator/internal/apis"
	"github.com/securesign/operator/internal/controller/annotations"
	"github.com/securesign/operator/internal/controller/common/action"
	"github.com/securesign/operator/internal/controller/common/utils/kubernetes"
	"github.com/securesign/operator/internal/controller/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Component struct {
	Name     string
	Instance string
}

type Option[T apis.ConditionsAwareObject] func(service *createService[T])

func MutateFn[T apis.ConditionsAwareObject](fn mutateFn[T]) Option[T] {
	return func(c *createService[T]) {
		c.mutateFn = append(c.mutateFn, fn)
	}
}

func CanHandle[T apis.ConditionsAwareObject](condition condition[T]) Option[T] {
	return func(c *createService[T]) {
		c.conditionFn = condition
	}
}

type condition[T apis.ConditionsAwareObject] func(instance T) bool

type mutateFn[T apis.ConditionsAwareObject] func(expected *corev1.Service, instance T) error

func NewServiceAction[T apis.ConditionsAwareObject](component Component, condition string, opts ...Option[T]) action.Action[T] {
	a := &createService[T]{
		component:   component,
		condition:   condition,
		conditionFn: nil,
		mutateFn:    []mutateFn[T]{},
	}

	for _, opt := range opts {
		opt(a)
	}

	if a.conditionFn == nil {
		a.conditionFn = func(instance T) bool {
			con := meta.FindStatusCondition(instance.GetConditions(), a.condition)
			if con == nil {
				return false
			}
			return con.Reason == constants.Creating || con.Reason == constants.Ready
		}
	}

	return a
}

type createService[T apis.ConditionsAwareObject] struct {
	action.BaseAction
	component   Component
	condition   string
	conditionFn condition[T]
	mutateFn    []mutateFn[T]
}

func (i createService[T]) Name() string {
	return "create service"
}

func (i createService[T]) CanHandle(_ context.Context, instance T) bool {
	if i.conditionFn != nil {
		return i.conditionFn(instance)
	}
	return false
}

func (i createService[T]) Handle(ctx context.Context, instance T) *action.Result {
	var (
		err     error
		updated bool
	)

	labels := constants.LabelsFor(i.component.Name, i.component.Instance, instance.GetName())

	svc := kubernetes.CreateService(instance.GetNamespace(), i.component.Name, []corev1.ServicePort{}, labels)
	if err = controllerutil.SetControllerReference(instance, svc, i.Client.Scheme()); err != nil {
		return i.Failed(fmt.Errorf("could not set controller reference for Service: %w", err))
	}

	for _, fn := range i.mutateFn {
		if err := fn(svc, instance); err != nil {
			return i.Failed(fmt.Errorf("mutate function failed: %w", err))
		}
	}

	if updated, err = i.Ensure(ctx, svc,
		action.EnsureAnnotations(annotations.TLS),
		action.EnsureLabels(),
		kubernetes.EnsureServiceSpec(),
	); err != nil {
		instance.SetCondition(metav1.Condition{
			Type:    i.condition,
			Status:  metav1.ConditionFalse,
			Reason:  constants.Failure,
			Message: err.Error(),
		})
		return i.FailedWithStatusUpdate(ctx, fmt.Errorf("could not create service: %w", err), instance)
	}

	if updated {
		instance.SetCondition(metav1.Condition{
			Type:    i.condition,
			Status:  metav1.ConditionFalse,
			Reason:  constants.Creating,
			Message: "Service created"},
		)
		return i.StatusUpdate(ctx, instance)
	} else {
		return i.Continue()
	}
}

/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha2

import (
	"fmt"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var ctloglog = logf.Log.WithName("ctlog-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *CTlog) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// Hub marks this type as a conversion hub.
func (r *CTlog) Hub() {}

//+kubebuilder:webhook:path=/mutate-rhtas-redhat-com-v1alpha2-ctlog,mutating=true,failurePolicy=fail,sideEffects=None,groups=rhtas.redhat.com,resources=ctlogs,verbs=create;update,versions=v1alpha2,name=mctlog.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &CTlog{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *CTlog) Default() {
	ctloglog.Info("default", "name", r.Name)

	if r.Spec.Trillian.Address == "" {
		r.Spec.Trillian.Address = fmt.Sprintf("trillian-log-server.%s.svc.cluster.local", r.Namespace)
	}
	if r.Spec.Trillian.Port == nil {
		r.Spec.Trillian.Port =  ptr.To(int32(8091))
	}
	if len(r.Spec.LogConfig) == 0 {
		lc := CTLogConfig{
			Prefix: "trusted-artifact-signer",

		}
		r.Spec.LogConfig = append(r.Spec.LogConfig, lc)
	}
}

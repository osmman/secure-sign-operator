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
	"github.com/securesign/operator/api"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CTlogSpec defines the desired state of CTlog component
// +kubebuilder:validation:XValidation:rule=(!has(self.publicKeyRef) || has(self.privateKeyRef)),message=privateKeyRef cannot be empty
// +kubebuilder:validation:XValidation:rule=(!has(self.privateKeyPasswordRef) || has(self.privateKeyRef)),message=privateKeyRef cannot be empty
type CTlogSpec struct {
	// LogConfig describes the configuration options for a log instance.
	LogConfig []CTLogConfig `json:"log,omitempty"`

	//Enable Service monitors for ctlog
	Monitoring api.MonitoringConfig `json:"monitoring,omitempty"`

	// Trillian service configuration
	//+kubebuilder:default:={port: 8091}
	Trillian api.TrillianService `json:"trillian,omitempty"`
}

// CTLogConfig describes the configuration options for a log instance.
// +kubebuilder:validation:XValidation:rule=(!has(self.public_key_ref) || has(self.private_key_ref)),message=private_key_ref cannot be empty
// +kubebuilder:validation:XValidation:rule=(!has(self.private_key_password_ref) || has(self.private_key_ref)),message=private_key_ref cannot be empty
type CTLogConfig struct {
	// The ID of a Trillian tree that stores the log data. The tree type must be
	// LOG for regular CT logs. For mirror logs it must be either PREORDERED_LOG
	// or LOG, and can change at runtime. CTFE in mirror mode uses only read API
	// which is common for both types.
	LogId *int64 `json:"log_id,omitempty"`
	// prefix is the name of the log. It will come after the global or
	// override handler prefix. For example if the handler prefix is "/logs"
	// and prefix is "vogon" the get-sth handler for this log will be
	// available at "/logs/vogon/ct/v1/get-sth". The prefix cannot be empty
	// and must not include "/" path separator characters.
	// +required
	Prefix string `json:"prefix"`
	// override_handler_prefix if set to a non empty value overrides the global
	// handler prefix for an individual log. For example this field is set to
	// "/otherlogs" then a log with prefix "vogon" will make it's get-sth handler
	// available at "/otherlogs/vogon/ct/v1/get-sth" regardless of what the
	// global prefix is. Can be set to '/' to make the get-sth handler register
	// at "/vogon/ct/v1/get-sth".
	OverrideHandlerPrefix string `json:"override_handler_prefix,omitempty"`
	// Reference to secrets containing root certificates that are acceptable to the
	// log. The certs are served through get-roots endpoint. Optional in mirrors.
	RootsPemFile []api.SecretKeySelector `json:"roots_pem_file,omitempty"`
	// The private key used for signing STHs etc. Not required for mirrors.
	PrivateKeyRef *api.SecretKeySelector `json:"private_key_ref,omitempty"`
	// Password for decrypting the private key.
	// If empty, indicates that the private key is not encrypted.
	//+optional
	PrivateKeyPasswordRef *api.SecretKeySelector `json:"private_key_password_ref,omitempty"`
	// The public key matching the above private key (if both are present). It is
	// used only by mirror logs for verifying the source log's signatures, but can
	// be specified for regular logs as well for the convenience of test tools.
	PublicKeyRef *api.SecretKeySelector `json:"public_key_ref,omitempty"`
	// If reject_expired is true then the certificate validity period will be
	// checked against the current time during the validation of submissions.
	// This will cause expired certificates to be rejected.
	RejectExpired bool `json:"reject_expired,omitempty"`
	// If reject_unexpired is true then CTFE rejects certificates that are either
	// currently valid or not yet valid.
	RejectUnexpired bool `json:"reject_unexpired,omitempty"`
	// If set, ext_key_usages will restrict the set of such usages that the
	// server will accept. By default all are accepted. The values specified
	// must be ones known to the x509 package.
	ExtKeyUsages []string `json:"ext_key_usages,omitempty"`
	// not_after_start defines the start of the range of acceptable NotAfter
	// values, inclusive.
	// Leaving this unset implies no lower bound to the range.
	NotAfterStart *metav1.Time `json:"not_after_start,omitempty"`
	// not_after_limit defines the end of the range of acceptable NotAfter values,
	// exclusive.
	// Leaving this unset implies no upper bound to the range.
	NotAfterLimit *metav1.Time `json:"not_after_limit,omitempty"`
	// accept_only_ca controls whether or not *only* certificates with the CA bit
	// set will be accepted.
	AcceptOnlyCa bool `json:"accept_only_ca,omitempty"`
	// If set, the log is a mirror, i.e. it serves the data of another (source)
	// log. It doesn't handle write requests (add-chain, etc.), so it's not a
	// fully fledged RFC-6962 log, but the tree read requests like get-entries and
	// get-consistency-proof are compatible. A mirror doesn't have the source
	// log's key and can't sign STHs. Consequently, the log operator must ensure
	// to channel source log's STHs into CTFE.
	IsMirror bool `json:"is_mirror,omitempty"`
	// If set, the log serves only read endpoints, and rejects writes through the
	// add-[pre-]chain endpoint.
	IsReadonly bool `json:"is_readonly,omitempty"`
	// The Maximum Merge Delay (MMD) of this log in seconds. See RFC6962 section 3
	// for definition of MMD. If zero, the log does not provide an MMD guarantee
	// (for example, it is a frozen log).
	MaxMergeDelaySec int32 `json:"max_merge_delay_sec,omitempty"`
	// The merge delay that the underlying log implementation is able/targeting to
	// provide. This option is exposed in CTFE metrics, and can be particularly
	// useful to catch when the log is behind but has not yet violated the strict
	// MMD limit.
	// Log operator should decide what exactly EMD means for them. For example, it
	// can be a 99-th percentile of merge delays that they observe, and they can
	// alert on the actual merge delay going above a certain multiple of this EMD.
	ExpectedMergeDelaySec int32 `json:"expected_merge_delay_sec,omitempty"`
	// The STH that this log will serve permanently (if present). Frozen STH must
	// be signed by this log's private key, and will be verified using the public
	// key specified in this config.
	FrozenSth *SignedTreeHead `json:"frozen_sth,omitempty"`
	// A list of X.509 extension OIDs, in dotted string form (e.g. "2.3.4.5")
	// which should cause submissions to be rejected.
	RejectExtensions []string `json:"reject_extensions,omitempty"`
}

// SignedTreeHead represents the structure returned by the get-sth CT method.
// See RFC6962 sections 3.5 and 4.3 for reference.
type SignedTreeHead struct {
	TreeSize          int64  `json:"tree_size,omitempty"`
	Timestamp         int64  `json:"timestamp,omitempty"`
	Sha256RootHash    []byte `json:"sha256_root_hash,omitempty"`
	TreeHeadSignature []byte `json:"tree_head_signature,omitempty"`
}

// CTlogStatus defines the observed state of CTlog component
type CTlogStatus struct {
	LogConfig []CTLogConfig `json:"log,omitempty"`
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:storageversion

// CTlog is the Schema for the ctlogs API
type CTlog struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CTlogSpec   `json:"spec,omitempty"`
	Status CTlogStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CTlogList contains a list of CTlog
type CTlogList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CTlog `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CTlog{}, &CTlogList{})
}

func (i *CTlog) GetConditions() []metav1.Condition {
	return i.Status.Conditions
}

func (i *CTlog) SetCondition(newCondition metav1.Condition) {
	meta.SetStatusCondition(&i.Status.Conditions, newCondition)
}

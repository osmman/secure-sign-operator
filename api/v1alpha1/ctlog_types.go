package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CTlogSpec defines the desired state of CTlog component
type CTlogSpec struct {

	// LogConfig describes the configuration options for a log instance.
	LogConfig []LogConfig `json:"log,omitempty"`

	//Enable Service monitors for ctlog
	Monitoring MonitoringConfig `json:"monitoring,omitempty"`

	// Trillian service configuration
	//+kubebuilder:default:={port: 8091}
	Trillian TrillianService `json:"trillian,omitempty"`
}

// LogConfig describes the configuration options for a log instance.
// +kubebuilder:validation:XValidation:rule=(!has(self.public_key_ref) || has(self.private_key_ref)),message=private_key_ref cannot be empty
// +kubebuilder:validation:XValidation:rule=(!has(self.private_key_password_ref) || has(self.private_key_ref)),message=private_key_ref cannot be empty
type LogConfig struct {
	// The ID of a Trillian tree that stores the log data. The tree type must be
	// LOG for regular CT logs. For mirror logs it must be either PREORDERED_LOG
	// or LOG, and can change at runtime. CTFE in mirror mode uses only read API
	// which is common for both types.
	// +required
	LogId int64 `json:"log_id"`
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
	RootsPemFile []SecretKeySelector  `json:"roots_pem_file,omitempty"`
	// The private key used for signing STHs etc. Not required for mirrors.
	PrivateKeyRef *SecretKeySelector `json:"private_key_ref,omitempty"`
	// Password for decrypting the private key.
	// If empty, indicates that the private key is not encrypted.
	//+optional
	PrivateKeyPasswordRef *SecretKeySelector `json:"private_key_password_ref,omitempty"`
	// The public key matching the above private key (if both are present). It is
	// used only by mirror logs for verifying the source log's signatures, but can
	// be specified for regular logs as well for the convenience of test tools.
	PublicKeyRef *SecretKeySelector `json:"public_key_ref,omitempty"`
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
	NotAfterStart *Timestamp `json:"not_after_start,omitempty"`
	// not_after_limit defines the end of the range of acceptable NotAfter values,
	// exclusive.
	// Leaving this unset implies no upper bound to the range.
	NotAfterLimit *Timestamp `json:"not_after_limit,omitempty"`
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

// A Timestamp represents a point in time independent of any time zone or local
// calendar, encoded as a count of seconds and fractions of seconds at
// nanosecond resolution. The count is relative to an epoch at UTC midnight on
// January 1, 1970, in the proleptic Gregorian calendar which extends the
// Gregorian calendar backwards to year one.
//
// All minutes are 60 seconds long. Leap seconds are "smeared" so that no leap
// second table is needed for interpretation, using a [24-hour linear
// smear](https://developers.google.com/time/smear).
//
// The range is from 0001-01-01T00:00:00Z to 9999-12-31T23:59:59.999999999Z. By
// restricting to that range, we ensure that we can convert to and from [RFC
// 3339](https://www.ietf.org/rfc/rfc3339.txt) date strings.
//
// # Examples
//
// Example 1: Compute Timestamp from POSIX `time()`.
//
//	Timestamp timestamp;
//	timestamp.set_seconds(time(NULL));
//	timestamp.set_nanos(0);
//
// Example 2: Compute Timestamp from POSIX `gettimeofday()`.
//
//	struct timeval tv;
//	gettimeofday(&tv, NULL);
//
//	Timestamp timestamp;
//	timestamp.set_seconds(tv.tv_sec);
//	timestamp.set_nanos(tv.tv_usec * 1000);
//
// Example 3: Compute Timestamp from Win32 `GetSystemTimeAsFileTime()`.
//
//	FILETIME ft;
//	GetSystemTimeAsFileTime(&ft);
//	UINT64 ticks = (((UINT64)ft.dwHighDateTime) << 32) | ft.dwLowDateTime;
//
//	// A Windows tick is 100 nanoseconds. Windows epoch 1601-01-01T00:00:00Z
//	// is 11644473600 seconds before Unix epoch 1970-01-01T00:00:00Z.
//	Timestamp timestamp;
//	timestamp.set_seconds((INT64) ((ticks / 10000000) - 11644473600LL));
//	timestamp.set_nanos((INT32) ((ticks % 10000000) * 100));
//
// Example 4: Compute Timestamp from Java `System.currentTimeMillis()`.
//
//	long millis = System.currentTimeMillis();
//
//	Timestamp timestamp = Timestamp.newBuilder().setSeconds(millis / 1000)
//	    .setNanos((int) ((millis % 1000) * 1000000)).build();
//
// Example 5: Compute Timestamp from Java `Instant.now()`.
//
//	Instant now = Instant.now();
//
//	Timestamp timestamp =
//	    Timestamp.newBuilder().setSeconds(now.getEpochSecond())
//	        .setNanos(now.getNano()).build();
//
// Example 6: Compute Timestamp from current time in Python.
//
//	timestamp = Timestamp()
//	timestamp.GetCurrentTime()
//
// # JSON Mapping
//
// In JSON format, the Timestamp type is encoded as a string in the
// [RFC 3339](https://www.ietf.org/rfc/rfc3339.txt) format. That is, the
// format is "{year}-{month}-{day}T{hour}:{min}:{sec}[.{frac_sec}]Z"
// where {year} is always expressed using four digits while {month}, {day},
// {hour}, {min}, and {sec} are zero-padded to two digits each. The fractional
// seconds, which can go up to 9 digits (i.e. up to 1 nanosecond resolution),
// are optional. The "Z" suffix indicates the timezone ("UTC"); the timezone
// is required. A proto3 JSON serializer should always use UTC (as indicated by
// "Z") when printing the Timestamp type and a proto3 JSON parser should be
// able to accept both UTC and other timezones (as indicated by an offset).
//
// For example, "2017-01-15T01:30:15.01Z" encodes 15.01 seconds past
// 01:30 UTC on January 15, 2017.
//
// In JavaScript, one can convert a Date object to this format using the
// standard
// [toISOString()](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Date/toISOString)
// method. In Python, a standard `datetime.datetime` object can be converted
// to this format using
// [`strftime`](https://docs.python.org/2/library/time.html#time.strftime) with
// the time format spec '%Y-%m-%dT%H:%M:%S.%fZ'. Likewise, in Java, one can use
// the Joda Time's [`ISODateTimeFormat.dateTime()`](
// http://joda-time.sourceforge.net/apidocs/org/joda/time/format/ISODateTimeFormat.html#dateTime()
// ) to obtain a formatter capable of generating timestamps in this format.
type Timestamp struct {
	// Represents seconds of UTC time since Unix epoch
	// 1970-01-01T00:00:00Z. Must be from 0001-01-01T00:00:00Z to
	// 9999-12-31T23:59:59Z inclusive.
	Seconds int64 `json:"seconds,omitempty"`
	// Non-negative fractions of a second at nanosecond resolution. Negative
	// second values with fractions must still have non-negative nanos values
	// that count forward in time. Must be from 0 to 999,999,999
	// inclusive.
	Nanos int32 `json:"nanos,omitempty"`
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
	LogConfig []LogConfig `json:"log,omitempty"`
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`,description="The component status"

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

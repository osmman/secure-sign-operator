package v1alpha1

import (
	"fmt"
	"github.com/securesign/operator/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var ctloglog = logf.Log.WithName("ctlog-resource")

func has[T any](v, b T) T {
	if v != nil {
		return v
	}

	return b
}

func (src *CTlog) ConvertTo(dstRaw conversion.Hub) error {
	ctloglog.Info("convert to v1alpha2")
	dst := dstRaw.(*v1alpha2.CTlog)

	dst.ObjectMeta = src.ObjectMeta

	logConfig := v1alpha2.CTLogConfig {
		LogId: has(src.Status.TreeID, src.Spec.TreeID),
		Prefix: "trusted-artifact-signer",
		PrivateKeyRef: has(src.Status.PrivateKeyRef, src.Spec.PrivateKeyRef),
		PrivateKeyPasswordRef: has(src.Status.PrivateKeyPasswordRef, src.Spec.PrivateKeyPasswordRef),
		PublicKeyRef: has(src.Status.PublicKeyRef, src.Spec.PublicKeyRef),
		RootsPemFile: has(src.Status.RootCertificates, src.Spec.RootCertificates),
	}

	dst.Spec.LogConfig = []v1alpha2.CTLogConfig{logConfig}
	dst.Spec.Monitoring = src.Spec.Monitoring
	dst.Spec.Trillian = src.Spec.Trillian
	return nil
}

func (dst *CTlog) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha2.CTlog)

	dst.ObjectMeta = src.ObjectMeta

	dst.Spec.Trillian = src.Spec.Trillian
	dst.Spec.Monitoring = src.Spec.Monitoring

	switch {
	case len(src.Spec.LogConfig) > 1:
		return fmt.Errorf("unable to conver: too many log configs")
	case len(src.Spec.LogConfig) == 1:
		dst.Spec.TreeID = has(src.Spec.LogConfig[0].LogId, src.Status.LogConfig[0].LogId)
		dst.Spec.PrivateKeyRef = has(src.Spec.LogConfig[0].PrivateKeyRef, src.Spec.LogConfig[0].PrivateKeyRef)
		dst.Spec.PrivateKeyPasswordRef = has(src.Spec.LogConfig[0].PrivateKeyPasswordRef, src.Spec.LogConfig[0].PrivateKeyPasswordRef)
		dst.Spec.PublicKeyRef = has(src.Spec.LogConfig[0].PublicKeyRef, src.Spec.LogConfig[0].PublicKeyRef)
		dst.Spec.RootCertificates = has(src.Spec.LogConfig[0].RootsPemFile, src.Status.LogConfig[0].RootsPemFile)
	}

	return nil
}

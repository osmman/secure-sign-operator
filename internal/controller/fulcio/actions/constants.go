package actions

const (
	DeploymentName     = "fulcio-server"
	ComponentName      = "fulcio"
	MonitoringRoleName = "prometheus-k8s-fulcio"
	ServiceMonitorName = "fulcio-metrics"
	RBACName           = "fulcio"

	CertCondition = "FulcioCertAvailable"

	ServerPortName         = "http"
	ServerPort       int32 = 80
	TargetServerPort int32 = 5555
	GRPCPortName           = "grpc"
	GRPCPort         int32 = 5554
	MetricsPortName        = "metrics"
	MetricsPort      int32 = 2112
)

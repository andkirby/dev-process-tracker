package models

// Additive types for lifecycle support — zero-value defaults preserve backward compatibility

// ServiceStatus represents the persistent status of a managed service.
type ServiceStatus string

const (
	StatusRunning ServiceStatus = "running"
	StatusStopped ServiceStatus = "stopped"
	StatusCrashed ServiceStatus = "crashed"
	StatusUnknown ServiceStatus = "unknown"
)

// ReadinessMode defines how to check if a service is ready.
type ReadinessMode string

const (
	ReadinessProcessOnly ReadinessMode = "process-only"
	ReadinessPortBound   ReadinessMode = "port-bound"
	ReadinessHTTPHealth  ReadinessMode = "http-health"
	ReadinessLogSignal   ReadinessMode = "log-signal"
	ReadinessMultiCheck  ReadinessMode = "multi-check"
)

// ReadinessConfig defines per-service readiness policy.
// Zero-value defaults preserve backward compatibility.
type ReadinessConfig struct {
	Mode       ReadinessMode
	Timeout    int    // seconds
	Endpoint   string // for http-health mode
	LogPattern string // for log-signal mode
}

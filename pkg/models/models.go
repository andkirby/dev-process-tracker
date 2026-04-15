package models

import "time"

// Confidence level for detection heuristics
type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

// Source indicates how a server was detected or started
type Source string

const (
	SourceManual  Source = "manual"
	SourceManaged Source = "managed"
	SourceAgent   Source = "agent"
	SourceUnknown Source = "unknown"
)

// ProcessRecord represents a discovered listening process
type ProcessRecord struct {
	PID         int        `json:"pid"`
	PPID        int        `json:"ppid"`
	User        string     `json:"user"`
	Command     string     `json:"command"`
	Port        int        `json:"port"`
	Protocol    string     `json:"protocol"` // "tcp"
	CWD         string     `json:"cwd"`
	StartTime   *time.Time `json:"start_time,omitempty"`
	ProjectRoot string     `json:"project_root,omitempty"`
	AgentTag    *AgentTag  `json:"agent_tag,omitempty"`
}

// AgentTag identifies servers likely started by AI agents
type AgentTag struct {
	Source     Source     `json:"source"`
	AgentName  string     `json:"agent_name,omitempty"`
	Confidence Confidence `json:"confidence"`
}

// ManagedService represents an explicitly registered server
type ManagedService struct {
	Name      string           `json:"name"`
	CWD       string           `json:"cwd"`
	Command   string           `json:"command"`
	Ports     []int            `json:"ports"`
	LastPID   *int             `json:"last_pid,omitempty"`
	LastStart *time.Time       `json:"last_start,omitempty"`
	LastStop  *time.Time       `json:"last_stop,omitempty"`
	Tags      []string         `json:"tags,omitempty"`
	Readiness *ReadinessConfig `json:"readiness,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// Registry holds all managed services
type Registry struct {
	Services map[string]*ManagedService `json:"services"`
	Version  string                     `json:"version"`
}

// ServerInfo combines discovered and managed server data
type ServerInfo struct {
	ProcessRecord  *ProcessRecord
	ManagedService *ManagedService
	Source         Source
	Status         string // "running", "stopped", etc.
	CrashReason    string
	CrashLogTail   []string
}

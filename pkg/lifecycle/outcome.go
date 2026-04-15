package lifecycle

// Outcome represents the result of a lifecycle command.
type Outcome string

const (
	OutcomeSuccess  Outcome = "success"
	OutcomeNoop     Outcome = "noop"
	OutcomeBlocked  Outcome = "blocked"
	OutcomeFailed   Outcome = "failed"
	OutcomeInvalid  Outcome = "invalid"
	OutcomeNotFound Outcome = "not_found"
)

// Result holds the outcome of a lifecycle operation.
type Result struct {
	Outcome     Outcome
	Message     string
	PID         int
	Diagnostics []string
}

// IsSuccess returns true if the outcome is success.
func (r Result) IsSuccess() bool {
	return r.Outcome == OutcomeSuccess
}

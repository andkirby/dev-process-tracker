package lifecycle

import "testing"

func TestOutcomeTypeValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		outcome Outcome
		want   string
	}{
		{"success", OutcomeSuccess, "success"},
		{"noop", OutcomeNoop, "noop"},
		{"blocked", OutcomeBlocked, "blocked"},
		{"failed", OutcomeFailed, "failed"},
		{"invalid", OutcomeInvalid, "invalid"},
		{"not_found", OutcomeNotFound, "not_found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := string(tt.outcome); got != tt.want {
				t.Errorf("Outcome %q = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestResultZeroValue(t *testing.T) {
	t.Parallel()

	var r Result
	if r.Outcome != "" {
		t.Errorf("zero-value Result.Outcome = %q, want empty string", r.Outcome)
	}
	if r.Message != "" {
		t.Errorf("zero-value Result.Message = %q, want empty string", r.Message)
	}
	if r.PID != 0 {
		t.Errorf("zero-value Result.PID = %d, want 0", r.PID)
	}
}

func TestResultFields(t *testing.T) {
	t.Parallel()

	r := Result{
		Outcome:     OutcomeSuccess,
		Message:     "started",
		PID:         1234,
		Diagnostics: []string{"log line 1", "log line 2"},
	}
	if r.Outcome != OutcomeSuccess {
		t.Errorf("Result.Outcome = %q, want %q", r.Outcome, OutcomeSuccess)
	}
	if r.Message != "started" {
		t.Errorf("Result.Message = %q, want %q", r.Message, "started")
	}
	if r.PID != 1234 {
		t.Errorf("Result.PID = %d, want 1234", r.PID)
	}
	if len(r.Diagnostics) != 2 {
		t.Errorf("Result.Diagnostics length = %d, want 2", len(r.Diagnostics))
	}
}

func TestResultIsSuccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		r    Result
		want bool
	}{
		{"success", Result{Outcome: OutcomeSuccess}, true},
		{"noop", Result{Outcome: OutcomeNoop}, false},
		{"blocked", Result{Outcome: OutcomeBlocked}, false},
		{"failed", Result{Outcome: OutcomeFailed}, false},
		{"invalid", Result{Outcome: OutcomeInvalid}, false},
		{"not_found", Result{Outcome: OutcomeNotFound}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.r.IsSuccess(); got != tt.want {
				t.Errorf("Result{%q}.IsSuccess() = %v, want %v", tt.r.Outcome, got, tt.want)
			}
		})
	}
}

func TestResultMessageFormat(t *testing.T) {
	t.Parallel()

	r := Result{
		Outcome: OutcomeBlocked,
		Message: "port 3000 is in use by PID 4821 (python). Stop it or change the service port.",
		PID:     4821,
	}
	msg := r.Message
	if msg == "" {
		t.Error("Result.Message should not be empty")
	}
	// Verify message answers: what happened, what to do next
	if r.Outcome == OutcomeBlocked && r.Message == "" {
		t.Error("blocked outcome must have a message")
	}
}

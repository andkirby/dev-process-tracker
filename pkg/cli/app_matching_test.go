package cli

import (
	"testing"

	"github.com/devports/devpt/pkg/models"
)

func TestCanMatchByPathRequiresUniqueOwner(t *testing.T) {
	t.Parallel()

	if !canMatchByPath(
		"/workspace/app",
		"/workspace/app",
		"/workspace/app",
		"/workspace/app",
		map[string]int{"/workspace/app": 1},
		map[string]int{"/workspace/app": 1},
	) {
		t.Fatal("expected unique path ownership to match")
	}

	if canMatchByPath(
		"/workspace/app",
		"/workspace/app",
		"/workspace/app",
		"/workspace/app",
		map[string]int{"/workspace/app": 2},
		map[string]int{"/workspace/app": 2},
	) {
		t.Fatal("expected ambiguous path ownership to be rejected")
	}
}

func TestServiceMatchesProcessRequiresStrongerSignalThanPID(t *testing.T) {
	t.Parallel()

	svc := &models.ManagedService{
		Name:  "api",
		CWD:   "/workspace/api",
		Ports: []int{3000},
	}

	if !serviceMatchesProcess(
		svc,
		&models.ProcessRecord{PID: 1234, Port: 3000},
		"/workspace/api",
		"",
		"",
	) {
		t.Fatal("expected declared port to validate the process")
	}

	if !serviceMatchesProcess(
		svc,
		&models.ProcessRecord{PID: 1234, Port: 9999, CWD: "/workspace/api"},
		"/workspace/api",
		"/workspace/api",
		"/workspace/api",
	) {
		t.Fatal("expected matching cwd/project root to validate the process")
	}

	if serviceMatchesProcess(
		svc,
		&models.ProcessRecord{PID: 1234, Port: 9999, CWD: "/tmp/other"},
		"/workspace/api",
		"/tmp/other",
		"/tmp/other",
	) {
		t.Fatal("expected PID-only match without path/port agreement to be rejected")
	}
}

func TestManagedServicePIDReturnsMatchedProcess(t *testing.T) {
	t.Parallel()

	servers := []*models.ServerInfo{
		{
			ProcessRecord: &models.ProcessRecord{PID: 2001},
			ManagedService: &models.ManagedService{
				Name: "api",
			},
		},
		{
			ProcessRecord: &models.ProcessRecord{PID: 2002},
			ManagedService: &models.ManagedService{
				Name: "worker",
			},
		},
	}

	if got := managedServicePID(servers, "worker"); got != 2002 {
		t.Fatalf("managedServicePID(..., worker) = %d, want 2002", got)
	}
	if got := managedServicePID(servers, "missing"); got != 0 {
		t.Fatalf("managedServicePID(..., missing) = %d, want 0", got)
	}
}

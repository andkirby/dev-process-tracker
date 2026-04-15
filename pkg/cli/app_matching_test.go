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

func TestFindManagedProcessForServiceKeepsManagedNonDevProcess(t *testing.T) {
	t.Parallel()

	lastPID := 1234
	svc := &models.ManagedService{
		Name:    "postgres",
		CWD:     "/workspace/db",
		Ports:   []int{5432},
		LastPID: &lastPID,
	}
	processes := []*models.ProcessRecord{
		{
			PID:         1234,
			Port:        5432,
			Command:     "/usr/local/bin/postgres",
			CWD:         "/workspace/db",
			ProjectRoot: "/workspace/db",
		},
	}

	got := findManagedProcessForService(
		svc,
		processes,
		"/workspace/db",
		"/workspace/db",
		map[string]int{"/workspace/db": 1},
		map[string]int{"/workspace/db": 1},
		map[int][]*models.ManagedService{5432: []*models.ManagedService{svc}},
	)
	if got != processes[0] {
		t.Fatalf("expected managed process match, got %#v", got)
	}
}

func TestFindManagedProcessForServiceRejectsPIDOnlyMatch(t *testing.T) {
	t.Parallel()

	lastPID := 4242
	svc := &models.ManagedService{
		Name:    "api",
		CWD:     "/workspace/api",
		Ports:   []int{3000},
		LastPID: &lastPID,
	}
	processes := []*models.ProcessRecord{
		{
			PID:         4242,
			Port:        9999,
			Command:     "/usr/sbin/unrelated",
			CWD:         "/tmp/other",
			ProjectRoot: "/tmp/other",
		},
	}

	got := findManagedProcessForService(
		svc,
		processes,
		"/workspace/api",
		"/workspace/api",
		map[string]int{"/workspace/api": 1, "/tmp/other": 1},
		map[string]int{"/workspace/api": 1, "/tmp/other": 1},
		map[int][]*models.ManagedService{3000: []*models.ManagedService{svc}},
	)
	if got != nil {
		t.Fatalf("expected PID-only candidate to be rejected, got %#v", got)
	}
}



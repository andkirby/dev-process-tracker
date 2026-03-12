package cli

import (
	"testing"

	"github.com/devports/devpt/pkg/models"
)

func TestParseNamePortIdentifier(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantName   string
		wantPort   int
		wantHasPort bool
	}{
		{
			name:       "simple name:port",
			input:      "web-api:3000",
			wantName:   "web-api",
			wantPort:   3000,
			wantHasPort: true,
		},
		{
			name:       "name with colon in it",
			input:      "some:thing:1234",
			wantName:   "some:thing",
			wantPort:   1234,
			wantHasPort: true,
		},
		{
			name:       "name only - no colon",
			input:      "web-api",
			wantName:   "web-api",
			wantPort:   0,
			wantHasPort: false,
		},
		{
			name:       "empty string",
			input:      "",
			wantName:   "",
			wantPort:   0,
			wantHasPort: false,
		},
		{
			name:       "single port number",
			input:      ":8080",
			wantName:   "",
			wantPort:   8080,
			wantHasPort: true,
		},
		{
			name:       "name:port with leading zeros",
			input:      "web-api:0300",
			wantName:   "web-api",
			wantPort:   300,
			wantHasPort: true,
		},
		{
			name:       "invalid port - not a number after colon",
			input:      "web-api:abc",
			wantName:   "web-api:abc",
			wantPort:   0,
			wantHasPort: false,
		},
		{
			name:       "multiple colons but last is not port",
			input:      "some:thing:else",
			wantName:   "some:thing:else",
			wantPort:   0,
			wantHasPort: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotPort, gotHasPort := ParseNamePortIdentifier(tt.input)
			if gotName != tt.wantName {
				t.Errorf("ParseNamePortIdentifier() name = %v, want %v", gotName, tt.wantName)
			}
			if gotPort != tt.wantPort {
				t.Errorf("ParseNamePortIdentifier() port = %v, want %v", gotPort, tt.wantPort)
			}
			if gotHasPort != tt.wantHasPort {
				t.Errorf("ParseNamePortIdentifier() hasPort = %v, want %v", gotHasPort, tt.wantHasPort)
			}
		})
	}
}

func TestLookupServiceWithFallback(t *testing.T) {
	services := []*models.ManagedService{
		{Name: "web-api", Ports: []int{3000, 3001}},
		{Name: "worker", Ports: []int{5000}},
		{Name: "some:thing", Ports: []int{4000}}, // Service with colon in literal name
		{Name: "database", Ports: []int{5432}},
	}

	tests := []struct {
		name            string
		identifier      string
		wantServiceName string
		wantErrors      bool
		errorCount      int
	}{
		{
			name:            "name:port exact match",
			identifier:      "web-api:3000",
			wantServiceName: "web-api",
			wantErrors:      false,
		},
		{
			name:            "name:port second port match",
			identifier:      "web-api:3001",
			wantServiceName: "web-api",
			wantErrors:      false,
		},
		{
			name:            "literal name with colon",
			identifier:      "some:thing",
			wantServiceName: "some:thing",
			wantErrors:      false,
		},
		{
			name:            "name:port with literal name fallback",
			identifier:      "some:thing:4000",
			wantServiceName: "some:thing",
			wantErrors:      false,
		},
		{
			name:            "simple name match",
			identifier:      "worker",
			wantServiceName: "worker",
			wantErrors:      false,
		},
		{
			name:            "name:port not found - both attempts fail",
			identifier:      "foo:5678",
			wantServiceName: "",
			wantErrors:      true,
			errorCount:      2, // name+port attempt + literal name attempt
		},
		{
			name:            "name only not found",
			identifier:      "nonexistent",
			wantServiceName: "",
			wantErrors:      true,
			errorCount:      1,
		},
		{
			name:            "empty identifier",
			identifier:      "",
			wantServiceName: "",
			wantErrors:      true,
			errorCount:      1,
		},
		{
			name:            "name:port with wrong port number",
			identifier:      "web-api:9999",
			wantServiceName: "",
			wantErrors:      true,
			errorCount:      2, // name+port attempt fails + literal name attempt fails (no service named "web-api:9999")
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotService, gotErrors := LookupServiceWithFallback(tt.identifier, services)

			if tt.wantServiceName != "" {
				if gotService == nil {
					t.Errorf("LookupServiceWithFallback() returned nil service, want %q", tt.wantServiceName)
					return
				}
				if gotService.Name != tt.wantServiceName {
					t.Errorf("LookupServiceWithFallback() service = %q, want %q", gotService.Name, tt.wantServiceName)
				}
			} else {
				if gotService != nil {
					t.Errorf("LookupServiceWithFallback() returned service %q, want nil", gotService.Name)
				}
			}

			if tt.wantErrors {
				if len(gotErrors) == 0 {
					t.Errorf("LookupServiceWithFallback() returned no errors, expected %d", tt.errorCount)
				}
				if tt.errorCount > 0 && len(gotErrors) != tt.errorCount {
					t.Errorf("LookupServiceWithFallback() error count = %d, want %d", len(gotErrors), tt.errorCount)
				}
			} else {
				if len(gotErrors) != 0 {
					t.Errorf("LookupServiceWithFallback() returned errors: %v", gotErrors)
				}
			}
		})
	}
}

func TestLookupServiceWithFallback_EmptyServices(t *testing.T) {
	services := []*models.ManagedService{}

	t.Run("empty service list with name:port", func(t *testing.T) {
		gotService, gotErrors := LookupServiceWithFallback("web-api:3000", services)
		if gotService != nil {
			t.Errorf("expected nil service, got %q", gotService.Name)
		}
		if len(gotErrors) != 2 {
			t.Errorf("expected 2 errors, got %d: %v", len(gotErrors), gotErrors)
		}
	})

	t.Run("empty service list with name only", func(t *testing.T) {
		gotService, gotErrors := LookupServiceWithFallback("web-api", services)
		if gotService != nil {
			t.Errorf("expected nil service, got %q", gotService.Name)
		}
		if len(gotErrors) != 1 {
			t.Errorf("expected 1 error, got %d: %v", len(gotErrors), gotErrors)
		}
	})
}

package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/devports/devpt/pkg/models"
)

// Registry manages stored service definitions
type Registry struct {
	filePath string
	data     *models.Registry
	mu       sync.RWMutex
}

// NewRegistry creates a new registry instance
func NewRegistry(filePath string) *Registry {
	return &Registry{
		filePath: filePath,
		data: &models.Registry{
			Services: make(map[string]*models.ManagedService),
			Version:  "1.0",
		},
	}
}

// Load reads the registry from disk
// FilePath returns the registry file path.
func (r *Registry) FilePath() string {
	return r.filePath
}

func (r *Registry) Load() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if file exists
	_, err := os.Stat(r.filePath)
	if os.IsNotExist(err) {
		// File doesn't exist yet, initialize with empty registry
		r.data.Services = make(map[string]*models.ManagedService)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to stat registry file: %w", err)
	}

	// Read file
	content, err := os.ReadFile(r.filePath)
	if err != nil {
		return fmt.Errorf("failed to read registry file: %w", err)
	}

	// Parse JSON
	data := &models.Registry{}
	if err := json.Unmarshal(content, data); err != nil {
		return fmt.Errorf("failed to parse registry: %w", err)
	}

	r.data = data
	return nil
}

// Save writes the registry to disk
func (r *Registry) Save() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Ensure directory exists
	dir := filepath.Dir(r.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create registry directory: %w", err)
	}

	// Marshal to JSON
	content, err := json.MarshalIndent(r.data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	// Write file with mode 0644
	if err := os.WriteFile(r.filePath, content, 0644); err != nil {
		return fmt.Errorf("failed to write registry file: %w", err)
	}

	return nil
}

// AddService registers a new managed service
func (r *Registry) AddService(service *models.ManagedService) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.data.Services[service.Name]; exists {
		return fmt.Errorf("service %q already exists", service.Name)
	}

	now := time.Now()
	service.CreatedAt = now
	service.UpdatedAt = now
	r.data.Services[service.Name] = service

	return r.save()
}

// UpdateService updates an existing managed service
func (r *Registry) UpdateService(service *models.ManagedService) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.data.Services[service.Name]; !exists {
		return fmt.Errorf("service %q not found", service.Name)
	}

	service.UpdatedAt = time.Now()
	r.data.Services[service.Name] = service

	return r.save()
}

// GetService retrieves a service by name
func (r *Registry) GetService(name string) *models.ManagedService {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.data.Services[name]
}

// ListServices returns all managed services
func (r *Registry) ListServices() []*models.ManagedService {
	r.mu.RLock()
	defer r.mu.RUnlock()

	services := make([]*models.ManagedService, 0, len(r.data.Services))
	for _, svc := range r.data.Services {
		services = append(services, svc)
	}
	return services
}

// RemoveService removes a service from the registry
func (r *Registry) RemoveService(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.data.Services[name]; !exists {
		return fmt.Errorf("service %q not found", name)
	}

	delete(r.data.Services, name)
	return r.save()
}

// UpdateServicePID updates the last PID for a service
func (r *Registry) UpdateServicePID(name string, pid int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	svc, exists := r.data.Services[name]
	if !exists {
		return fmt.Errorf("service %q not found", name)
	}

	svc.LastPID = &pid
	now := time.Now()
	svc.LastStart = &now
	svc.LastStop = nil
	svc.UpdatedAt = now

	return r.save()
}

// ClearServicePID marks a managed service as not running.
func (r *Registry) ClearServicePID(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	svc, exists := r.data.Services[name]
	if !exists {
		return fmt.Errorf("service %q not found", name)
	}

	now := time.Now()
	svc.LastPID = nil
	svc.LastStop = &now
	svc.UpdatedAt = now
	return r.save()
}

// save (internal) writes the registry without taking locks
func (r *Registry) save() error {
	dir := filepath.Dir(r.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create registry directory: %w", err)
	}

	content, err := json.MarshalIndent(r.data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	if err := os.WriteFile(r.filePath, content, 0644); err != nil {
		return fmt.Errorf("failed to write registry file: %w", err)
	}

	return nil
}

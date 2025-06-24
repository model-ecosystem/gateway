package compose

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gateway/internal/core"
	"gateway/internal/registry/docker"
	"gateway/pkg/errors"
	"gopkg.in/yaml.v3"
)

// Config represents Docker Compose registry configuration
type Config struct {
	// Compose file settings
	ComposeFile     string   `yaml:"composeFile"`     // Path to docker-compose.yml
	ComposeFiles    []string `yaml:"composeFiles"`    // Multiple compose files
	ProjectName     string   `yaml:"projectName"`     // Compose project name
	EnvironmentFile string   `yaml:"environmentFile"` // .env file path

	// Service discovery settings
	LabelPrefix     string `yaml:"labelPrefix"`     // Label prefix for gateway config
	ServicePrefix   string `yaml:"servicePrefix"`   // Service name prefix to filter
	RefreshInterval int    `yaml:"refreshInterval"` // Service refresh interval in seconds

	// Network settings
	Network         string `yaml:"network"`         // Docker network to use
	UseServiceNames bool   `yaml:"useServiceNames"` // Use service names for internal routing

	// Docker connection (reuse from docker registry)
	DockerHost string `yaml:"dockerHost"` // Docker daemon host
}

// Registry implements service discovery using Docker Compose
type Registry struct {
	config         *Config
	dockerRegistry *docker.Registry
	composeData    map[string]*ComposeService // service name -> compose service
	services       map[string]*core.Service
	mu             sync.RWMutex
	logger         *slog.Logger
	ctx            context.Context
	cancel         context.CancelFunc
	lastModified   map[string]time.Time
}

// ComposeService represents a service definition from docker-compose.yml
type ComposeService struct {
	Name        string
	Image       string                 `yaml:"image"`
	Ports       []string               `yaml:"ports"`
	Labels      map[string]string      `yaml:"labels"`
	Environment map[string]string      `yaml:"environment"`
	Networks    interface{}            `yaml:"networks"` // Can be array or map
	Deploy      *Deploy                `yaml:"deploy"`
	Healthcheck *Healthcheck           `yaml:"healthcheck"`
	External    bool                   // External service (not managed by compose)
}

// Deploy represents deployment configuration
type Deploy struct {
	Replicas int               `yaml:"replicas"`
	Labels   map[string]string `yaml:"labels"`
}

// Healthcheck represents health check configuration
type Healthcheck struct {
	Test        interface{}   `yaml:"test"`
	Interval    time.Duration `yaml:"interval"`
	Timeout     time.Duration `yaml:"timeout"`
	Retries     int           `yaml:"retries"`
	StartPeriod time.Duration `yaml:"start_period"`
}

// ComposeFile represents a docker-compose.yml structure
type ComposeFile struct {
	Version  string                     `yaml:"version"`
	Services map[string]*ComposeService `yaml:"services"`
	Networks map[string]interface{}     `yaml:"networks"`
}

// NewRegistry creates a new Docker Compose registry
func NewRegistry(config *Config, logger *slog.Logger) (*Registry, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Set defaults
	if config.LabelPrefix == "" {
		config.LabelPrefix = "gateway"
	}
	if config.RefreshInterval == 0 {
		config.RefreshInterval = 30
	}
	if config.ComposeFile == "" && len(config.ComposeFiles) == 0 {
		config.ComposeFile = "docker-compose.yml"
	}

	// Create docker registry for container discovery
	dockerConfig := &docker.Config{
		Host:            config.DockerHost,
		LabelPrefix:     config.LabelPrefix,
		Network:         config.Network,
		RefreshInterval: config.RefreshInterval,
	}

	dockerReg, err := docker.NewRegistry(dockerConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker registry: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	r := &Registry{
		config:         config,
		dockerRegistry: dockerReg,
		composeData:    make(map[string]*ComposeService),
		services:       make(map[string]*core.Service),
		logger:         logger.With("component", "compose_registry"),
		ctx:            ctx,
		cancel:         cancel,
		lastModified:   make(map[string]time.Time),
	}

	return r, nil
}

// Start starts the registry
func (r *Registry) Start() error {
	// Load compose files
	if err := r.loadComposeFiles(); err != nil {
		return fmt.Errorf("failed to load compose files: %w", err)
	}

	// Start background refresh
	go r.refreshLoop()

	r.logger.Info("Docker Compose registry started",
		"projectName", r.config.ProjectName,
		"services", len(r.composeData),
	)

	return nil
}

// Stop stops the registry
func (r *Registry) Stop() error {
	r.cancel()

	if r.dockerRegistry != nil {
		r.dockerRegistry.Close()
	}

	r.logger.Info("Docker Compose registry stopped")
	return nil
}

// GetService returns a service by name
func (r *Registry) GetService(name string) (*core.Service, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	service, exists := r.services[name]
	if !exists {
		return nil, errors.NewError(errors.ErrorTypeNotFound, fmt.Sprintf("service %s not found", name))
	}

	// Return copy with only healthy instances
	healthyInstances := make([]*core.ServiceInstance, 0)
	for _, instance := range service.Instances {
		if instance.Healthy {
			healthyInstances = append(healthyInstances, instance)
		}
	}

	if len(healthyInstances) == 0 {
		return nil, errors.NewError(errors.ErrorTypeUnavailable, 
			fmt.Sprintf("no healthy instances available for service %s", name))
	}

	serviceCopy := *service
	serviceCopy.Instances = healthyInstances

	return &serviceCopy, nil
}

// ListServices returns all services
func (r *Registry) ListServices() ([]*core.Service, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	services := make([]*core.Service, 0, len(r.services))
	for _, service := range r.services {
		services = append(services, service)
	}

	return services, nil
}

// loadComposeFiles loads and parses compose files
func (r *Registry) loadComposeFiles() error {
	files := r.config.ComposeFiles
	if len(files) == 0 && r.config.ComposeFile != "" {
		files = []string{r.config.ComposeFile}
	}

	// Load environment file if specified
	envVars := make(map[string]string)
	if r.config.EnvironmentFile != "" {
		if err := r.loadEnvFile(r.config.EnvironmentFile, envVars); err != nil {
			r.logger.Warn("Failed to load environment file",
				"file", r.config.EnvironmentFile,
				"error", err,
			)
		}
	}

	// Merge all compose files
	mergedServices := make(map[string]*ComposeService)

	for _, file := range files {
		composeData, err := r.loadComposeFile(file, envVars)
		if err != nil {
			return fmt.Errorf("failed to load %s: %w", file, err)
		}

		// Merge services
		for name, service := range composeData.Services {
			service.Name = name
			
			// Apply project name prefix if configured
			if r.config.ProjectName != "" {
				service.Name = r.config.ProjectName + "_" + name
			}
			
			// Check service prefix filter
			if r.config.ServicePrefix != "" && !strings.HasPrefix(name, r.config.ServicePrefix) {
				continue
			}
			
			mergedServices[name] = service
		}
	}

	r.mu.Lock()
	r.composeData = mergedServices
	r.mu.Unlock()

	// Update services based on compose data and running containers
	return r.updateServices()
}

// loadComposeFile loads a single compose file
func (r *Registry) loadComposeFile(file string, envVars map[string]string) (*ComposeFile, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	// Substitute environment variables
	content := r.substituteEnvVars(string(data), envVars)

	var compose ComposeFile
	if err := yaml.Unmarshal([]byte(content), &compose); err != nil {
		return nil, fmt.Errorf("failed to parse compose file: %w", err)
	}

	// Store file modification time
	if stat, err := os.Stat(file); err == nil {
		r.lastModified[file] = stat.ModTime()
	}

	return &compose, nil
}

// loadEnvFile loads environment variables from .env file
func (r *Registry) loadEnvFile(file string, envVars map[string]string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Remove quotes if present
			value = strings.Trim(value, `"`)
			value = strings.Trim(value, `'`)
			envVars[key] = value
		}
	}

	return nil
}

// substituteEnvVars substitutes environment variables in content
func (r *Registry) substituteEnvVars(content string, envVars map[string]string) string {
	// Replace ${VAR} and $VAR patterns
	for key, value := range envVars {
		content = strings.ReplaceAll(content, "${"+key+"}", value)
		content = strings.ReplaceAll(content, "$"+key, value)
	}

	// Also check OS environment
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			key, value := parts[0], parts[1]
			if _, exists := envVars[key]; !exists {
				content = strings.ReplaceAll(content, "${"+key+"}", value)
				content = strings.ReplaceAll(content, "$"+key, value)
			}
		}
	}

	return content
}

// updateServices updates service instances based on compose data and running containers
func (r *Registry) updateServices() error {
	// Get running containers from docker registry
	dockerServices, err := r.dockerRegistry.ListServices()
	if err != nil {
		return fmt.Errorf("failed to list docker services: %w", err)
	}

	services := make(map[string]*core.Service)

	r.mu.RLock()
	composeData := r.composeData
	r.mu.RUnlock()

	// Process each compose service
	for name, composeSvc := range composeData {
		// Skip if gateway is not enabled for this service
		if !r.isGatewayEnabled(composeSvc) {
			continue
		}

		service := &core.Service{
			Name:      r.getServiceName(composeSvc),
			Instances: make([]*core.ServiceInstance, 0),
			Metadata: map[string]string{
				"compose_service": name,
				"compose_project": r.config.ProjectName,
			},
		}

		// Find matching containers
		for _, dockerSvc := range dockerServices {
			for _, instance := range dockerSvc.Instances {
				// Match by compose labels
				if r.matchesComposeService(instance, composeSvc) {
					// Override with compose configuration if using service names
					if r.config.UseServiceNames {
						instance.Address = composeSvc.Name
						if port := r.getServicePort(composeSvc); port > 0 {
							instance.Port = port
						}
					}
					service.Instances = append(service.Instances, instance)
				}
			}
		}

		if len(service.Instances) > 0 {
			services[service.Name] = service
		}
	}

	r.mu.Lock()
	r.services = services
	r.mu.Unlock()

	r.logger.Info("Services updated from compose",
		"services", len(services),
		"totalInstances", r.countInstances(services),
	)

	return nil
}

// Helper methods

func (r *Registry) isGatewayEnabled(svc *ComposeService) bool {
	// Check labels
	if val, ok := svc.Labels[r.config.LabelPrefix+".enabled"]; ok {
		return val == "true"
	}
	if val, ok := svc.Labels[r.config.LabelPrefix+".service"]; ok && val != "" {
		return true
	}
	// Check if service exposes ports
	return len(svc.Ports) > 0
}

func (r *Registry) getServiceName(svc *ComposeService) string {
	// Check for explicit service name in labels
	if name, ok := svc.Labels[r.config.LabelPrefix+".service"]; ok {
		return name
	}
	return svc.Name
}

func (r *Registry) getServicePort(svc *ComposeService) int {
	// Check for explicit port in labels
	if portStr, ok := svc.Labels[r.config.LabelPrefix+".port"]; ok {
		if port, err := strconv.Atoi(portStr); err == nil {
			return port
		}
	}

	// Parse first port from ports configuration
	if len(svc.Ports) > 0 {
		// Port format: "8080:80" or "80"
		portSpec := svc.Ports[0]
		parts := strings.Split(portSpec, ":")
		if len(parts) >= 2 {
			// Use container port (second part)
			if port, err := strconv.Atoi(parts[1]); err == nil {
				return port
			}
		} else if len(parts) == 1 {
			// Single port
			if port, err := strconv.Atoi(parts[0]); err == nil {
				return port
			}
		}
	}

	return 0
}

func (r *Registry) matchesComposeService(instance *core.ServiceInstance, composeSvc *ComposeService) bool {
	// Check compose labels in instance metadata
	if instance.Metadata != nil {
		if project, ok := instance.Metadata["com.docker.compose.project"]; ok {
			if r.config.ProjectName != "" && project != r.config.ProjectName {
				return false
			}
		}
		if service, ok := instance.Metadata["com.docker.compose.service"]; ok {
			return service == composeSvc.Name
		}
	}
	return false
}

func (r *Registry) countInstances(services map[string]*core.Service) int {
	count := 0
	for _, svc := range services {
		count += len(svc.Instances)
	}
	return count
}

// refreshLoop periodically refreshes services
func (r *Registry) refreshLoop() {
	ticker := time.NewTicker(time.Duration(r.config.RefreshInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			// Check if compose files have been modified
			if r.filesModified() {
				r.logger.Info("Compose files modified, reloading")
				if err := r.loadComposeFiles(); err != nil {
					r.logger.Error("Failed to reload compose files", "error", err)
				}
			} else {
				// Just update services from running containers
				if err := r.updateServices(); err != nil {
					r.logger.Error("Failed to update services", "error", err)
				}
			}
		}
	}
}

// filesModified checks if compose files have been modified
func (r *Registry) filesModified() bool {
	files := r.config.ComposeFiles
	if len(files) == 0 && r.config.ComposeFile != "" {
		files = []string{r.config.ComposeFile}
	}

	for _, file := range files {
		if stat, err := os.Stat(file); err == nil {
			if lastMod, ok := r.lastModified[file]; ok {
				if stat.ModTime().After(lastMod) {
					return true
				}
			}
		}
	}

	return false
}

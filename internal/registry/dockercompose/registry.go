package dockercompose

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"gateway/internal/core"
	
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// Config represents Docker Compose service discovery configuration
type Config struct {
	// Project name to filter containers (from com.docker.compose.project label)
	ProjectName string `yaml:"projectName"`
	// Service label prefix for configuration
	LabelPrefix string `yaml:"labelPrefix"`
	// Refresh interval for polling Docker
	RefreshInterval time.Duration `yaml:"refreshInterval"`
	// Docker client configuration
	DockerHost string `yaml:"dockerHost"`
	APIVersion string `yaml:"apiVersion"`
}

// Registry implements service discovery for Docker Compose
type Registry struct {
	mu              sync.RWMutex
	config          *Config
	client          *client.Client
	services        map[string][]*core.ServiceInstance
	logger          *slog.Logger
	stopCh          chan struct{}
}

// NewRegistry creates a new Docker Compose registry
func NewRegistry(config *Config, logger *slog.Logger) (*Registry, error) {
	// Set defaults
	if config.LabelPrefix == "" {
		config.LabelPrefix = "gateway"
	}
	if config.RefreshInterval == 0 {
		config.RefreshInterval = 10 * time.Second
	}

	// Create Docker client
	opts := []client.Opt{
		client.WithAPIVersionNegotiation(),
	}
	if config.DockerHost != "" {
		opts = append(opts, client.WithHost(config.DockerHost))
	}
	if config.APIVersion != "" {
		opts = append(opts, client.WithVersion(config.APIVersion))
	}

	dockerClient, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Test Docker connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := dockerClient.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to Docker: %w", err)
	}

	r := &Registry{
		config:   config,
		client:   dockerClient,
		services: make(map[string][]*core.ServiceInstance),
		logger:   logger.With("component", "dockercompose-registry"),
		stopCh:   make(chan struct{}),
	}

	return r, nil
}

// Start starts the registry
func (r *Registry) Start(ctx context.Context) error {
	r.logger.Info("Starting Docker Compose service discovery", 
		"project", r.config.ProjectName,
		"refresh_interval", r.config.RefreshInterval)

	// Initial discovery
	if err := r.refresh(ctx); err != nil {
		return fmt.Errorf("initial service discovery failed: %w", err)
	}

	// Start refresh loop
	go r.refreshLoop(ctx)

	return nil
}

// Stop stops the registry
func (r *Registry) Stop(ctx context.Context) error {
	close(r.stopCh)
	return nil
}

// GetService returns service instances by name
func (r *Registry) GetService(name string) ([]core.ServiceInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instances, exists := r.services[name]
	if !exists {
		return nil, fmt.Errorf("service %s not found", name)
	}

	// Convert pointers to values
	result := make([]core.ServiceInstance, 0, len(instances))
	for _, inst := range instances {
		result = append(result, *inst)
	}

	return result, nil
}

// refreshLoop periodically refreshes services
func (r *Registry) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(r.config.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case <-ticker.C:
			if err := r.refresh(ctx); err != nil {
				r.logger.Error("Failed to refresh services", "error", err)
			}
		}
	}
}

// refresh discovers services from Docker
func (r *Registry) refresh(ctx context.Context) error {
	// Build filters for Docker Compose containers
	filterArgs := filters.NewArgs()
	
	// Filter by compose project if specified
	if r.config.ProjectName != "" {
		filterArgs.Add("label", fmt.Sprintf("com.docker.compose.project=%s", r.config.ProjectName))
	} else {
		// At least filter for compose containers
		filterArgs.Add("label", "com.docker.compose.project")
	}
	
	// Only running containers
	filterArgs.Add("status", "running")

	// List containers
	containers, err := r.client.ContainerList(ctx, container.ListOptions{
		Filters: filterArgs,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	// Build new service map
	newServices := make(map[string][]*core.ServiceInstance)

	for _, container := range containers {
		// Get compose service name
		serviceName := container.Labels["com.docker.compose.service"]
		if serviceName == "" {
			continue
		}

		// Check if this service should be exposed via gateway
		if !r.shouldExposeService(container.Labels) {
			continue
		}

		// Create instance
		instance, err := r.createInstance(container)
		if err != nil {
			r.logger.Error("Failed to create instance", 
				"container", container.ID[:12],
				"service", serviceName,
				"error", err)
			continue
		}

		// Add to service instances
		if _, exists := newServices[serviceName]; !exists {
			newServices[serviceName] = []*core.ServiceInstance{}
		}
		newServices[serviceName] = append(newServices[serviceName], instance)
	}

	// Update services atomically
	r.mu.Lock()
	r.services = newServices
	r.mu.Unlock()

	r.logger.Info("Service discovery completed", 
		"services", len(newServices))

	return nil
}

// shouldExposeService checks if a service should be exposed
func (r *Registry) shouldExposeService(labels map[string]string) bool {
	// Check for explicit enable label
	enableKey := fmt.Sprintf("%s.enable", r.config.LabelPrefix)
	if enable, exists := labels[enableKey]; exists {
		return enable == "true"
	}

	// Check for port label (implicit enable)
	portKey := fmt.Sprintf("%s.port", r.config.LabelPrefix)
	_, hasPort := labels[portKey]
	return hasPort
}

// createInstance creates a service instance from a container
func (r *Registry) createInstance(container types.Container) (*core.ServiceInstance, error) {
	// Get service port
	port, err := r.getServicePort(container)
	if err != nil {
		return nil, err
	}

	// Get container IP
	ip := r.getContainerIP(container)
	if ip == "" {
		return nil, fmt.Errorf("no IP address found")
	}

	// Extract metadata
	metadata := r.extractMetadata(container.Labels)

	// Add container metadata
	metadata["container.id"] = container.ID[:12]
	metadata["container.name"] = strings.TrimPrefix(container.Names[0], "/")
	metadata["container.image"] = container.Image

	instance := &core.ServiceInstance{
		ID:       container.ID[:12],
		Address:  ip,
		Port:     port,
		Scheme:   "http", // Default scheme, can be overridden by label
		Healthy:  true,   // Assume healthy if container is running
		Metadata: metadata,
	}

	// Check for scheme override
	schemeKey := fmt.Sprintf("%s.scheme", r.config.LabelPrefix)
	if scheme, exists := container.Labels[schemeKey]; exists {
		instance.Scheme = scheme
	}

	return instance, nil
}

// getServicePort extracts the service port from labels or container ports
func (r *Registry) getServicePort(container types.Container) (int, error) {
	// Check for explicit port label
	portKey := fmt.Sprintf("%s.port", r.config.LabelPrefix)
	if portStr, exists := container.Labels[portKey]; exists {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return 0, fmt.Errorf("invalid port %s: %w", portStr, err)
		}
		return port, nil
	}

	// Try to get from exposed ports
	for _, port := range container.Ports {
		if port.PrivatePort > 0 {
			return int(port.PrivatePort), nil
		}
	}

	return 0, fmt.Errorf("no service port found")
}

// getContainerIP gets the container's IP address
func (r *Registry) getContainerIP(container types.Container) string {
	// Prefer network mode specific IPs
	for networkName, network := range container.NetworkSettings.Networks {
		if network.IPAddress != "" {
			r.logger.Debug("Using container IP", 
				"container", container.ID[:12],
				"network", networkName,
				"ip", network.IPAddress)
			return network.IPAddress
		}
	}

	return ""
}

// extractMetadata extracts metadata from labels
func (r *Registry) extractMetadata(labels map[string]string) map[string]any {
	metadata := make(map[string]any)
	prefix := r.config.LabelPrefix + "."

	for key, value := range labels {
		if strings.HasPrefix(key, prefix) {
			metaKey := strings.TrimPrefix(key, prefix)
			// Skip certain keys that are used internally
			if metaKey != "enable" && metaKey != "port" && metaKey != "scheme" {
				metadata[metaKey] = value
			}
		}
	}

	// Add compose-specific metadata
	if project := labels["com.docker.compose.project"]; project != "" {
		metadata["compose.project"] = project
	}
	if service := labels["com.docker.compose.service"]; service != "" {
		metadata["compose.service"] = service
	}

	return metadata
}
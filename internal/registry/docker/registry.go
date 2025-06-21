package docker

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	
	"gateway/internal/core"
	"gateway/pkg/errors"
)

// Config represents Docker registry configuration
type Config struct {
	// Docker connection settings
	Host          string `yaml:"host"`           // Docker daemon host
	Version       string `yaml:"version"`        // Docker API version
	CertPath      string `yaml:"certPath"`       // Path to certificates for TLS
	
	// Service discovery settings
	LabelPrefix   string `yaml:"labelPrefix"`    // Label prefix for gateway config
	Network       string `yaml:"network"`        // Docker network to use
	RefreshInterval int  `yaml:"refreshInterval"` // Service refresh interval in seconds
}

// DefaultConfig returns default Docker registry configuration
func DefaultConfig() *Config {
	return &Config{
		Host:            "", // Use default Docker socket
		Version:         "", // Use default API version
		LabelPrefix:     "gateway",
		RefreshInterval: 10,
	}
}

// Registry implements service discovery using Docker
type Registry struct {
	config    *Config
	client    *client.Client
	services  map[string][]core.ServiceInstance
	mu        sync.RWMutex
	logger    *slog.Logger
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// NewRegistry creates a new Docker registry
func NewRegistry(config *Config, logger *slog.Logger) (*Registry, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Create Docker client
	opts := []client.Opt{
		client.FromEnv,
	}
	
	if config.Host != "" {
		opts = append(opts, client.WithHost(config.Host))
	}
	
	if config.Version != "" {
		opts = append(opts, client.WithVersion(config.Version))
	}
	
	if config.CertPath != "" {
		opts = append(opts, client.WithTLSClientConfig(
			fmt.Sprintf("%s/ca.pem", config.CertPath),
			fmt.Sprintf("%s/cert.pem", config.CertPath),
			fmt.Sprintf("%s/key.pem", config.CertPath),
		))
	}
	
	dockerClient, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, errors.NewError(errors.ErrorTypeInternal, "failed to create Docker client").WithCause(err)
	}
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if _, err := dockerClient.Ping(ctx); err != nil {
		return nil, errors.NewError(errors.ErrorTypeUnavailable, "failed to connect to Docker daemon").WithCause(err)
	}
	
	r := &Registry{
		config:   config,
		client:   dockerClient,
		services: make(map[string][]core.ServiceInstance),
		logger:   logger,
		stopCh:   make(chan struct{}),
	}
	
	// Initial service discovery
	if err := r.refresh(context.Background()); err != nil {
		return nil, fmt.Errorf("initial service discovery: %w", err)
	}
	
	// Start refresh goroutine
	if config.RefreshInterval > 0 {
		r.wg.Add(1)
		go r.refreshLoop()
	}
	
	return r, nil
}

// GetService returns instances for a service
func (r *Registry) GetService(name string) ([]core.ServiceInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	instances, ok := r.services[name]
	if !ok {
		return nil, errors.NewError(errors.ErrorTypeNotFound, "service not found").
			WithDetail("service", name)
	}
	
	// Return copy to avoid race conditions
	result := make([]core.ServiceInstance, len(instances))
	copy(result, instances)
	
	return result, nil
}

// Close stops the registry
func (r *Registry) Close() error {
	close(r.stopCh)
	r.wg.Wait()
	return r.client.Close()
}

// refresh discovers services from Docker
func (r *Registry) refresh(ctx context.Context) error {
	// List containers with gateway labels
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("%s.enable=true", r.config.LabelPrefix))
	
	containers, err := r.client.ContainerList(ctx, container.ListOptions{
		Filters: filterArgs,
		All:     false, // Only running containers
	})
	if err != nil {
		return errors.WrapWithType(err, errors.ErrorTypeUnavailable, "listing containers")
	}
	
	// Group containers by service
	services := make(map[string][]core.ServiceInstance)
	
	for _, cont := range containers {
		// Extract service info from labels
		serviceName := cont.Labels[fmt.Sprintf("%s.service", r.config.LabelPrefix)]
		if serviceName == "" {
			r.logger.Warn("Container missing service label",
				"container", cont.ID[:12],
				"names", cont.Names,
			)
			continue
		}
		
		// Extract port
		portStr := cont.Labels[fmt.Sprintf("%s.port", r.config.LabelPrefix)]
		if portStr == "" {
			r.logger.Warn("Container missing port label",
				"container", cont.ID[:12],
				"service", serviceName,
			)
			continue
		}
		
		port, err := strconv.Atoi(portStr)
		if err != nil {
			r.logger.Warn("Invalid port label",
				"container", cont.ID[:12],
				"service", serviceName,
				"port", portStr,
			)
			continue
		}
		
		// Get container IP
		ip := r.getContainerIP(cont, r.config.Network)
		if ip == "" {
			r.logger.Warn("Container has no IP address",
				"container", cont.ID[:12],
				"service", serviceName,
			)
			continue
		}
		
		// Extract metadata
		metadata := make(map[string]any)
		for k, v := range cont.Labels {
			if strings.HasPrefix(k, r.config.LabelPrefix+".meta.") {
				key := strings.TrimPrefix(k, r.config.LabelPrefix+".meta.")
				metadata[key] = v
			}
		}
		
		// Create instance
		instance := core.ServiceInstance{
			ID:       cont.ID[:12],
			Name:     serviceName,
			Address:  ip,
			Port:     port,
			Scheme:   cont.Labels[fmt.Sprintf("%s.scheme", r.config.LabelPrefix)],
			Healthy:  cont.State == "running",
			Metadata: metadata,
		}
		
		// Add container info to metadata
		instance.Metadata["container_name"] = strings.TrimPrefix(cont.Names[0], "/")
		instance.Metadata["container_image"] = cont.Image
		
		services[serviceName] = append(services[serviceName], instance)
		
		r.logger.Debug("Discovered container",
			"service", serviceName,
			"instance", instance.ID,
			"address", fmt.Sprintf("%s:%d", ip, port),
		)
	}
	
	// Update services
	r.mu.Lock()
	r.services = services
	r.mu.Unlock()
	
	r.logger.Info("Service discovery completed",
		"services", len(services),
		"instances", r.countInstances(services),
	)
	
	return nil
}

// getContainerIP extracts the container IP for the given network
func (r *Registry) getContainerIP(cont types.Container, network string) string {
	// If no specific network is specified, try to find any IP
	if network == "" {
		// Prefer bridge network
		if net, ok := cont.NetworkSettings.Networks["bridge"]; ok && net.IPAddress != "" {
			return net.IPAddress
		}
		
		// Use first available network
		for _, net := range cont.NetworkSettings.Networks {
			if net.IPAddress != "" {
				return net.IPAddress
			}
		}
		
		return ""
	}
	
	// Look for specific network
	if net, ok := cont.NetworkSettings.Networks[network]; ok {
		return net.IPAddress
	}
	
	return ""
}

// refreshLoop periodically refreshes services
func (r *Registry) refreshLoop() {
	defer r.wg.Done()
	
	ticker := time.NewTicker(time.Duration(r.config.RefreshInterval) * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := r.refresh(ctx); err != nil {
				r.logger.Error("Failed to refresh services", "error", err)
			}
			cancel()
			
		case <-r.stopCh:
			return
		}
	}
}

// countInstances counts total instances across all services
func (r *Registry) countInstances(services map[string][]core.ServiceInstance) int {
	count := 0
	for _, instances := range services {
		count += len(instances)
	}
	return count
}
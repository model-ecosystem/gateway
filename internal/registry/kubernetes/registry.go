package kubernetes

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"gateway/internal/core"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Config holds Kubernetes registry configuration
type Config struct {
	// Kubeconfig path (optional, uses in-cluster config if empty)
	Kubeconfig string `yaml:"kubeconfig"`
	// Namespace to watch (empty means all namespaces)
	Namespace string `yaml:"namespace"`
	// Label selector for services
	LabelSelector string `yaml:"labelSelector"`
	// Service annotation for gateway configuration
	GatewayAnnotation string `yaml:"gatewayAnnotation"`
	// Port name or number to use (default: first port)
	PortName string `yaml:"portName"`
	// Enable endpoints watching for pod discovery
	WatchEndpoints bool `yaml:"watchEndpoints"`
	// Refresh interval for full resync
	RefreshInterval time.Duration `yaml:"refreshInterval"`
}

// Registry implements service discovery using Kubernetes
type Registry struct {
	config    Config
	client    kubernetes.Interface
	services  map[string]*core.Service
	mu        sync.RWMutex
	logger    *slog.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	watchers  []watch.Interface
}

// NewRegistry creates a new Kubernetes registry
func NewRegistry(config Config, logger *slog.Logger) (*Registry, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Create Kubernetes client
	var k8sConfig *rest.Config
	var err error

	if config.Kubeconfig != "" {
		// Use kubeconfig file
		k8sConfig, err = clientcmd.BuildConfigFromFlags("", config.Kubeconfig)
	} else {
		// Use in-cluster config
		k8sConfig, err = rest.InClusterConfig()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
	}

	client, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Set defaults
	if config.GatewayAnnotation == "" {
		config.GatewayAnnotation = "gateway/enabled"
	}
	if config.RefreshInterval == 0 {
		config.RefreshInterval = 5 * time.Minute
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Registry{
		config:   config,
		client:   client,
		services: make(map[string]*core.Service),
		logger:   logger.With("component", "kubernetes_registry"),
		ctx:      ctx,
		cancel:   cancel,
		watchers: make([]watch.Interface, 0),
	}, nil
}

// Start starts watching Kubernetes resources
func (r *Registry) Start() error {
	// Initial sync
	if err := r.syncServices(); err != nil {
		return fmt.Errorf("initial sync failed: %w", err)
	}

	// Start watching services
	go r.watchServices()

	// Start watching endpoints if enabled
	if r.config.WatchEndpoints {
		go r.watchEndpoints()
	}

	// Periodic full resync
	go r.resyncLoop()

	r.logger.Info("Kubernetes service discovery started",
		"namespace", r.config.Namespace,
		"labelSelector", r.config.LabelSelector)

	return nil
}

// Stop stops the registry
func (r *Registry) Stop() error {
	r.cancel()

	// Stop all watchers
	for _, watcher := range r.watchers {
		watcher.Stop()
	}

	r.logger.Info("Kubernetes service discovery stopped")
	return nil
}

// GetService returns a service by name
func (r *Registry) GetService(name string) (*core.Service, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	service, exists := r.services[name]
	if !exists {
		return nil, fmt.Errorf("service %s not found", name)
	}

	// Filter healthy instances
	healthyInstances := make([]*core.ServiceInstance, 0)
	for _, instance := range service.Instances {
		if instance.Healthy {
			healthyInstances = append(healthyInstances, instance)
		}
	}

	if len(healthyInstances) == 0 {
		return nil, fmt.Errorf("no healthy instances available for service %s", name)
	}

	// Return copy with only healthy instances
	serviceCopy := *service
	serviceCopy.Instances = healthyInstances

	return &serviceCopy, nil
}

// ListServices returns all discovered services
func (r *Registry) ListServices() ([]*core.Service, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	services := make([]*core.Service, 0, len(r.services))
	for _, service := range r.services {
		services = append(services, service)
	}

	return services, nil
}

// syncServices performs a full sync of services
func (r *Registry) syncServices() error {
	// Parse label selector
	selector := labels.Everything()
	if r.config.LabelSelector != "" {
		var err error
		selector, err = labels.Parse(r.config.LabelSelector)
		if err != nil {
			return fmt.Errorf("invalid label selector: %w", err)
		}
	}

	// List services
	listOptions := metav1.ListOptions{
		LabelSelector: selector.String(),
	}

	var serviceList *corev1.ServiceList
	var err error

	if r.config.Namespace != "" {
		serviceList, err = r.client.CoreV1().Services(r.config.Namespace).List(r.ctx, listOptions)
	} else {
		serviceList, err = r.client.CoreV1().Services("").List(r.ctx, listOptions)
	}

	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	// Update services
	newServices := make(map[string]*core.Service)

	for _, svc := range serviceList.Items {
		// Check if service should be exposed to gateway
		if !r.shouldExposeService(&svc) {
			continue
		}

		service := r.k8sServiceToCoreService(&svc)
		if service != nil {
			newServices[service.Name] = service
		}
	}

	// Update services map
	r.mu.Lock()
	r.services = newServices
	r.mu.Unlock()

	r.logger.Info("Services synced", "count", len(newServices))
	return nil
}

// shouldExposeService checks if a service should be exposed to the gateway
func (r *Registry) shouldExposeService(svc *corev1.Service) bool {
	// Check gateway annotation
	if r.config.GatewayAnnotation != "" {
		if val, ok := svc.Annotations[r.config.GatewayAnnotation]; ok {
			return val == "true" || val == "enabled"
		}
		// If annotation is required but not present, don't expose
		return false
	}

	// Skip headless services
	if svc.Spec.ClusterIP == "None" {
		return false
	}

	// Skip services without ports
	if len(svc.Spec.Ports) == 0 {
		return false
	}

	return true
}

// k8sServiceToCoreService converts a Kubernetes service to core.Service
func (r *Registry) k8sServiceToCoreService(svc *corev1.Service) *core.Service {
	// Find the target port
	var targetPort int32
	if r.config.PortName != "" {
		// Find by name
		for _, port := range svc.Spec.Ports {
			if port.Name == r.config.PortName {
				targetPort = port.Port
				break
			}
		}
	}

	// If not found or not specified, use first port
	if targetPort == 0 && len(svc.Spec.Ports) > 0 {
		targetPort = svc.Spec.Ports[0].Port
	}

	if targetPort == 0 {
		r.logger.Warn("No valid port found for service", "service", svc.Name)
		return nil
	}

	// Create service
	service := &core.Service{
		Name:      svc.Name,
		Instances: make([]*core.ServiceInstance, 0),
		Metadata: map[string]string{
			"namespace": svc.Namespace,
			"uid":       string(svc.UID),
		},
	}

	// Add service annotations as metadata
	for key, value := range svc.Annotations {
		service.Metadata["annotation."+key] = value
	}

	// Create instances based on service type
	switch svc.Spec.Type {
	case corev1.ServiceTypeLoadBalancer:
		// Use load balancer ingress
		for _, ingress := range svc.Status.LoadBalancer.Ingress {
			address := ingress.IP
			if address == "" {
				address = ingress.Hostname
			}
			if address != "" {
				instance := &core.ServiceInstance{
					ID:      fmt.Sprintf("%s-%s", svc.Name, address),
					Address: address,
					Port:    int(targetPort),
					Healthy: true,
					Metadata: make(map[string]any),
				}
				instance.Metadata["type"] = "loadbalancer"
				service.Instances = append(service.Instances, instance)
			}
		}

	case corev1.ServiceTypeNodePort:
		// For NodePort, we'd need to get node IPs
		// This is simplified - in production you'd query nodes
		r.logger.Warn("NodePort services require node discovery", "service", svc.Name)

	default:
		// For ClusterIP services, use the cluster IP if not watching endpoints
		if !r.config.WatchEndpoints && svc.Spec.ClusterIP != "" {
			instance := &core.ServiceInstance{
				ID:      fmt.Sprintf("%s-clusterip", svc.Name),
				Address: svc.Spec.ClusterIP,
				Port:    int(targetPort),
				Healthy: true,
				Metadata: make(map[string]any),
			}
			instance.Metadata["type"] = "clusterip"
			service.Instances = append(service.Instances, instance)
		}
	}

	return service
}

// watchServices watches for service changes
func (r *Registry) watchServices() {
	for {
		select {
		case <-r.ctx.Done():
			return
		default:
		}

		// Create watch options
		selector := labels.Everything()
		if r.config.LabelSelector != "" {
			var err error
			selector, err = labels.Parse(r.config.LabelSelector)
			if err != nil {
				r.logger.Error("Invalid label selector", "error", err)
				time.Sleep(5 * time.Second)
				continue
			}
		}

		watchOptions := metav1.ListOptions{
			LabelSelector: selector.String(),
			Watch:         true,
		}

		// Start watching
		var watcher watch.Interface
		var err error

		if r.config.Namespace != "" {
			watcher, err = r.client.CoreV1().Services(r.config.Namespace).Watch(r.ctx, watchOptions)
		} else {
			watcher, err = r.client.CoreV1().Services("").Watch(r.ctx, watchOptions)
		}

		if err != nil {
			r.logger.Error("Failed to create service watcher", "error", err)
			time.Sleep(5 * time.Second)
			continue
		}

		r.watchers = append(r.watchers, watcher)
		r.handleServiceEvents(watcher)
		watcher.Stop()
	}
}

// handleServiceEvents handles service watch events
func (r *Registry) handleServiceEvents(watcher watch.Interface) {
	for event := range watcher.ResultChan() {
		switch event.Type {
		case watch.Added, watch.Modified:
			svc, ok := event.Object.(*corev1.Service)
			if !ok {
				continue
			}

			if r.shouldExposeService(svc) {
				service := r.k8sServiceToCoreService(svc)
				if service != nil {
					r.mu.Lock()
					r.services[service.Name] = service
					r.mu.Unlock()
					r.logger.Info("Service updated", "name", service.Name, "event", event.Type)
				}
			} else {
				// Remove if it was previously exposed
				r.mu.Lock()
				delete(r.services, svc.Name)
				r.mu.Unlock()
			}

		case watch.Deleted:
			svc, ok := event.Object.(*corev1.Service)
			if !ok {
				continue
			}

			r.mu.Lock()
			delete(r.services, svc.Name)
			r.mu.Unlock()
			r.logger.Info("Service removed", "name", svc.Name)

		case watch.Error:
			r.logger.Error("Watch error", "event", event.Object)
			return
		}
	}
}

// watchEndpoints watches for endpoint changes to discover pods
func (r *Registry) watchEndpoints() {
	// Implementation for endpoint watching
	// This would update service instances based on ready endpoints
	r.logger.Info("Endpoint watching not yet implemented")
}

// resyncLoop performs periodic full resyncs
func (r *Registry) resyncLoop() {
	ticker := time.NewTicker(r.config.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			if err := r.syncServices(); err != nil {
				r.logger.Error("Resync failed", "error", err)
			}
		}
	}
}
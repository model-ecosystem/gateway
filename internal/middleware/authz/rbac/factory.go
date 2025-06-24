package rbac

import (
	"fmt"
	"log/slog"
	"time"

	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "rbac-middleware"

// Component implements factory.Component for RBAC middleware
type Component struct {
	config     *config.RBACConfig
	rbac       *RBAC
	middleware core.Middleware
	logger     *slog.Logger
}

// NewComponent creates a new RBAC middleware component
func NewComponent(logger *slog.Logger) factory.Component {
	return &Component{
		logger: logger,
	}
}

// Name returns the component name
func (c *Component) Name() string {
	return ComponentName
}

// Init initializes the component with configuration
func (c *Component) Init(parser factory.ConfigParser) error {
	// Parse the RBAC configuration
	var rbacConfig config.RBACConfig
	if err := parser(&rbacConfig); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	c.config = &rbacConfig

	// Only create RBAC if enabled
	if c.config.Enabled {
		// Convert policies
		var policies []*Policy
		for _, policyConfig := range c.config.Policies {
			policy := &Policy{
				Name:        policyConfig.Name,
				Description: policyConfig.Description,
				Roles:       make(map[string]*Role),
				Bindings:    make(map[string][]string),
			}
			
			// Convert roles
			for roleName, roleConfig := range policyConfig.Roles {
				policy.Roles[roleName] = &Role{
					Name:        roleConfig.Name,
					Description: roleConfig.Description,
					Permissions: roleConfig.Permissions,
					Inherits:    roleConfig.Inherits,
					Metadata:    roleConfig.Metadata,
				}
			}
			
			// Convert bindings
			for subject, roles := range policyConfig.Bindings {
				policy.Bindings[subject] = roles
			}
			
			policies = append(policies, policy)
		}
		
		// Create RBAC config
		rbacConfig := &Config{
			Policies:  policies,
			CacheSize: c.config.CacheSize,
			CacheTTL:  time.Duration(c.config.CacheTTL) * time.Second,
		}
		
		// Create RBAC instance
		rbacInstance, err := New(rbacConfig, c.logger)
		if err != nil {
			return fmt.Errorf("create RBAC: %w", err)
		}
		
		c.rbac = rbacInstance
		
		// Create middleware config
		middlewareConfig := &MiddlewareConfig{
			Enabled:              c.config.Enabled,
			SubjectKey:           c.config.SubjectKey,
			EnforcementMode:      c.config.EnforcementMode,
			DefaultAllow:         c.config.DefaultAllow,
			SkipPaths:            c.config.SkipPaths,
			PolicyRefreshInterval: c.config.PolicyRefreshInterval,
		}
		
		// Create middleware
		middlewareInstance := NewMiddleware(rbacInstance, middlewareConfig, c.logger)
		c.middleware = middlewareInstance.Middleware()
	}

	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.config == nil {
		return fmt.Errorf("RBAC config not initialized")
	}
	
	// RBAC and middleware can be nil if RBAC is disabled
	if c.config.Enabled {
		if c.rbac == nil {
			return fmt.Errorf("RBAC enabled but RBAC instance not created")
		}
		if c.middleware == nil {
			return fmt.Errorf("RBAC enabled but middleware not created")
		}
		
		// Validate configuration
		if len(c.config.Policies) == 0 {
			return fmt.Errorf("RBAC policies not specified")
		}
	}

	return nil
}

// Build returns the middleware
func (c *Component) Build() core.Middleware {
	// Can return nil if RBAC is disabled
	return c.middleware
}

// IsEnabled returns whether RBAC is enabled
func (c *Component) IsEnabled() bool {
	return c.config != nil && c.config.Enabled
}

// GetRBAC returns the RBAC instance
func (c *Component) GetRBAC() *RBAC {
	return c.rbac
}

// Ensure Component implements factory.Component
var _ factory.Component = (*Component)(nil)
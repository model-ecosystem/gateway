package transform

import (
	"fmt"
	"log/slog"

	"gateway/internal/config"
	"gateway/internal/core"
	"gateway/pkg/factory"
)

// ComponentName is the name used to register this component
const ComponentName = "transform-middleware"

// Component implements factory.Component for transform middleware
type Component struct {
	config      *Config
	middleware  *Middleware
	logger      *slog.Logger
}

// NewComponent creates a new transform middleware component
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
	// Parse the transform configuration from internal/config
	var transformConfig config.TransformConfig
	if err := parser(&transformConfig); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	// Convert to middleware config
	c.config = &Config{
		Enabled: transformConfig.Enabled,
	}
	
	// Convert request transforms
	if transformConfig.RequestTransforms != nil {
		c.config.RequestTransforms = make(map[string]TransformConfig)
		for path, rule := range transformConfig.RequestTransforms {
			c.config.RequestTransforms[path] = c.convertTransformRule(rule)
		}
	}
	
	// Convert response transforms
	if transformConfig.ResponseTransforms != nil {
		c.config.ResponseTransforms = make(map[string]TransformConfig)
		for path, rule := range transformConfig.ResponseTransforms {
			c.config.ResponseTransforms[path] = c.convertTransformRule(rule)
		}
	}
	
	// Convert global transforms
	if transformConfig.GlobalRequest != nil {
		global := c.convertTransformRule(*transformConfig.GlobalRequest)
		c.config.GlobalRequest = &global
	}
	if transformConfig.GlobalResponse != nil {
		global := c.convertTransformRule(*transformConfig.GlobalResponse)
		c.config.GlobalResponse = &global
	}

	// Only create middleware if enabled
	if c.config.Enabled {
		c.middleware = NewMiddleware(c.config, c.logger)
	}

	return nil
}

// Validate validates the component state
func (c *Component) Validate() error {
	if c.config == nil {
		return fmt.Errorf("transform config not initialized")
	}
	
	// Middleware can be nil if transform is disabled
	if c.config.Enabled && c.middleware == nil {
		return fmt.Errorf("transform enabled but middleware not created")
	}

	return nil
}

// Build returns the middleware
func (c *Component) Build() core.Middleware {
	// Can return nil if transform is disabled
	if c.middleware == nil {
		return nil
	}
	return c.middleware.Middleware()
}

// IsEnabled returns whether transform is enabled
func (c *Component) IsEnabled() bool {
	return c.config != nil && c.config.Enabled
}

// convertTransformRule converts from config to internal transform rule
func (c *Component) convertTransformRule(rule config.TransformRule) TransformConfig {
	var tc TransformConfig
	
	if rule.Headers != nil {
		tc.Headers = &HeaderConfig{
			Add:      rule.Headers.Add,
			Remove:   rule.Headers.Remove,
			Rename:   rule.Headers.Rename,
			Modify:   rule.Headers.Modify,
		}
	}
	
	if rule.Body != nil {
		tc.Body = &BodyConfig{
			Operations: c.convertOperations(rule.Body.Operations),
			Format:     rule.Body.Format,
		}
	}
	
	if rule.Conditions != nil {
		for _, cond := range rule.Conditions {
			tc.Conditions = append(tc.Conditions, Condition{
				Header:      cond.Header,
				Value:       cond.Value,
				ContentType: cond.ContentType,
				Method:      cond.Method,
			})
		}
	}
	
	return tc
}

// convertOperations converts body operations
func (c *Component) convertOperations(ops []config.TransformOperation) []Operation {
	var operations []Operation
	for _, op := range ops {
		operations = append(operations, Operation{
			Type:   op.Type,
			Path:   op.Path,
			Value:  op.Value,
			From:   op.From,
			To:     op.To,
		})
	}
	return operations
}

// Ensure Component implements factory.Component
var _ factory.Component = (*Component)(nil)
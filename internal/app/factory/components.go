package factory

import (
	"log/slog"

	pkgFactory "gateway/pkg/factory"
)

// ComponentFactory is the base interface for all component factories
type ComponentFactory interface {
	pkgFactory.Component
}

// BaseComponentFactory provides common functionality for component factories
type BaseComponentFactory struct {
	logger *slog.Logger
}

// NewBaseComponentFactory creates a new base component factory
func NewBaseComponentFactory(logger *slog.Logger) BaseComponentFactory {
	return BaseComponentFactory{logger: logger}
}

// GetLogger returns the logger
func (f *BaseComponentFactory) GetLogger() *slog.Logger {
	return f.logger
}

// ParseConfig is a helper to parse configuration
func (f *BaseComponentFactory) ParseConfig(src interface{}, dst interface{}) error {
	return pkgFactory.ParseConfig(src, dst)
}
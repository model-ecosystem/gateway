package factory

// ParseConfig is a helper to create a ConfigParser from source configuration
func ParseConfig(source any, target any) error {
	return parseConfig(source, target)
}
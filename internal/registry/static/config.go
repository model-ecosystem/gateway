package static

// Config represents static registry configuration
type Config struct {
	Services []Service `yaml:"services"`
}

// Service represents a service definition
type Service struct {
	Name      string     `yaml:"name"`
	Instances []Instance `yaml:"instances"`
}

// Instance represents a service instance
type Instance struct {
	ID      string `yaml:"id"`
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
	Weight  int    `yaml:"weight"`
	Health  string `yaml:"health"`
	Tags    []string `yaml:"tags"`
}
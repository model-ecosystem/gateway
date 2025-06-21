package tls

// Config represents TLS configuration for frontend connections
type Config struct {
	Enabled            bool   `yaml:"enabled"`
	CertFile           string `yaml:"certFile"`
	KeyFile            string `yaml:"keyFile"`
	MinVersion         string `yaml:"minVersion"`
	MaxVersion         string `yaml:"maxVersion"`
	CipherSuites       []int  `yaml:"cipherSuites"`
	PreferServerCipher bool   `yaml:"preferServerCipher"`
}

// BackendConfig represents TLS configuration for backend connections
type BackendConfig struct {
	Enabled            bool   `yaml:"enabled"`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify"`
	ServerName         string `yaml:"serverName"`
	ClientCertFile     string `yaml:"clientCertFile"`
	ClientKeyFile      string `yaml:"clientKeyFile"`
	RootCAFile         string `yaml:"rootCAFile"`
	MinVersion         string `yaml:"minVersion"`
	MaxVersion         string `yaml:"maxVersion"`
	PreferServerCipher bool   `yaml:"preferServerCipher"`
	Renegotiation      bool   `yaml:"renegotiation"`
}
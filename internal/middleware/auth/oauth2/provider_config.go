package oauth2

// ProviderConfig represents the configuration for an OAuth2/OIDC provider
type ProviderConfig struct {
	// Basic configuration
	Name         string `yaml:"name"`
	ClientID     string `yaml:"clientId"`
	ClientSecret string `yaml:"clientSecret"`
	
	// Endpoints
	AuthorizationURL string `yaml:"authorizationUrl"`
	TokenURL         string `yaml:"tokenUrl"`
	UserInfoURL      string `yaml:"userInfoUrl"`
	JWKSEndpoint     string `yaml:"jwksEndpoint"`
	
	// OIDC Discovery
	IssuerURL       string `yaml:"issuerUrl"`
	DiscoveryURL    string `yaml:"discoveryUrl"`
	UseDiscovery    bool   `yaml:"useDiscovery"`
	
	// Token validation
	ValidateIssuer   bool     `yaml:"validateIssuer"`
	ValidateAudience bool     `yaml:"validateAudience"`
	Audience         []string `yaml:"audience"`
	
	// Scopes
	Scopes []string `yaml:"scopes"`
	
	// Claims mapping
	ClaimsMapping map[string]string `yaml:"claimsMapping"`
}
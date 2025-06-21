package connector

import (
	"net/http"
	"time"

	httpImpl "gateway/internal/connector/http"
)

// NewHTTPConnector creates a new HTTP connector
func NewHTTPConnector(client *http.Client, defaultTimeout time.Duration) Connector {
	return httpImpl.NewHTTPConnector(client, defaultTimeout)
}

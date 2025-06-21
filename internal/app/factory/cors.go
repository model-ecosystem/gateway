package factory

import (
	"net/http"

	"gateway/internal/config"
	"gateway/internal/middleware/cors"
)

// CreateCORSHandler creates CORS HTTP handler from config
func CreateCORSHandler(cfg *config.CORS, next http.Handler) http.Handler {
	if cfg == nil || !cfg.Enabled {
		return nil
	}

	// Convert config to CORS config
	corsConfig := cors.Config{
		AllowedOrigins:       cfg.AllowedOrigins,
		AllowedMethods:       cfg.AllowedMethods,
		AllowedHeaders:       cfg.AllowedHeaders,
		ExposedHeaders:       cfg.ExposedHeaders,
		AllowCredentials:     cfg.AllowCredentials,
		MaxAge:               cfg.MaxAge,
		OptionsPassthrough:   cfg.OptionsPassthrough,
		OptionsSuccessStatus: cfg.OptionsSuccessStatus,
	}

	// Set defaults if not specified
	if len(corsConfig.AllowedOrigins) == 0 {
		corsConfig.AllowedOrigins = []string{"*"}
	}
	if len(corsConfig.AllowedMethods) == 0 {
		corsConfig.AllowedMethods = []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodHead,
			http.MethodOptions,
		}
	}
	if len(corsConfig.AllowedHeaders) == 0 {
		corsConfig.AllowedHeaders = []string{"*"}
	}
	if corsConfig.MaxAge == 0 {
		corsConfig.MaxAge = 86400 // 24 hours
	}
	if corsConfig.OptionsSuccessStatus == 0 {
		corsConfig.OptionsSuccessStatus = http.StatusNoContent
	}

	corsHandler := cors.New(corsConfig)
	return corsHandler.Handler(next)
}

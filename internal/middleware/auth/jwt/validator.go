package jwt

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"gateway/internal/core"
	"gateway/internal/middleware/auth"
	"gateway/pkg/errors"
)

// TokenValidator validates JWT tokens periodically for long-lived connections
type TokenValidator struct {
	provider *Provider
	logger   *slog.Logger
	mu       sync.RWMutex
	timers   map[string]*time.Timer
}

// NewTokenValidator creates a new token validator
func NewTokenValidator(provider *Provider, logger *slog.Logger) *TokenValidator {
	return &TokenValidator{
		provider: provider,
		logger:   logger,
		timers:   make(map[string]*time.Timer),
	}
}

// ValidateConnection starts periodic validation of a connection's token
func (v *TokenValidator) ValidateConnection(ctx context.Context, connectionID string, token string, onExpired func()) error {
	// Initial validation using the provider's Authenticate method
	authInfo, err := v.provider.Authenticate(ctx, &auth.BearerCredentials{Token: token})
	if err != nil {
		return err
	}

	// Check if token has expiration
	if authInfo.ExpiresAt == nil {
		// No expiration, token is valid indefinitely
		v.logger.Debug("Token has no expiration", "connectionID", connectionID)
		return nil
	}

	expiration := *authInfo.ExpiresAt
	now := time.Now()

	if expiration.Before(now) {
		return errors.NewError(errors.ErrorTypeUnauthorized, "token has expired")
	}

	// Calculate when to check again (5 seconds before expiration)
	checkDuration := expiration.Sub(now) - 5*time.Second
	if checkDuration <= 0 {
		// Token expires in less than 5 seconds
		checkDuration = 1 * time.Second
	}

	v.logger.Debug("Scheduling token validation",
		"connectionID", connectionID,
		"expiration", expiration,
		"checkIn", checkDuration,
	)

	// Create timer for validation
	timer := time.AfterFunc(checkDuration, func() {
		// Re-validate token
		_, err := v.provider.Authenticate(ctx, &auth.BearerCredentials{Token: token})
		if err != nil {
			v.logger.Info("Token expired for connection",
				"connectionID", connectionID,
				"error", err,
			)
			// Call the expiration handler
			onExpired()
		} else {
			// Token is still valid but might expire soon
			// Check again in 1 second
			v.scheduleRecheck(ctx, connectionID, token, onExpired, 1*time.Second)
		}
	})

	// Store timer
	v.mu.Lock()
	v.timers[connectionID] = timer
	v.mu.Unlock()

	// Clean up when context is cancelled
	go func() {
		<-ctx.Done()
		v.StopValidation(connectionID)
	}()

	return nil
}

// scheduleRecheck schedules another validation check
func (v *TokenValidator) scheduleRecheck(ctx context.Context, connectionID string, token string, onExpired func(), duration time.Duration) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	timer := time.AfterFunc(duration, func() {
		// Re-validate token
		authInfo, err := v.provider.Authenticate(ctx, &auth.BearerCredentials{Token: token})
		if err != nil {
			v.logger.Info("Token expired for connection",
				"connectionID", connectionID,
				"error", err,
			)
			onExpired()
			return
		}

		// Check if token will expire soon
		if authInfo.ExpiresAt != nil {
			remaining := time.Until(*authInfo.ExpiresAt)
			
			if remaining <= 5*time.Second {
				// Token expires very soon, check every second
				v.scheduleRecheck(ctx, connectionID, token, onExpired, 1*time.Second)
			} else if remaining <= 30*time.Second {
				// Token expires soon, check in 5 seconds
				v.scheduleRecheck(ctx, connectionID, token, onExpired, 5*time.Second)
			} else {
				// Schedule next check 5 seconds before expiration
				nextCheck := remaining - 5*time.Second
				v.scheduleRecheck(ctx, connectionID, token, onExpired, nextCheck)
			}
		}
	})

	// Update timer
	v.mu.Lock()
	if oldTimer, exists := v.timers[connectionID]; exists {
		oldTimer.Stop()
	}
	v.timers[connectionID] = timer
	v.mu.Unlock()
}

// StopValidation stops validation for a connection
func (v *TokenValidator) StopValidation(connectionID string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if timer, exists := v.timers[connectionID]; exists {
		timer.Stop()
		delete(v.timers, connectionID)
		v.logger.Debug("Stopped token validation", "connectionID", connectionID)
	}
}

// StopAll stops all validations
func (v *TokenValidator) StopAll() {
	v.mu.Lock()
	defer v.mu.Unlock()

	for id, timer := range v.timers {
		timer.Stop()
		delete(v.timers, id)
	}
	v.logger.Debug("Stopped all token validations")
}

// ExtractTokenFromRequest extracts JWT token from a request
func ExtractTokenFromRequest(req core.Request) (string, error) {
	headers := req.Headers()
	
	// Check Authorization header
	if authHeaders, ok := headers["Authorization"]; ok && len(authHeaders) > 0 {
		authHeader := authHeaders[0]
		// Bearer token
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			return authHeader[7:], nil
		}
	}

	// Check cookie
	if cookieHeaders, ok := headers["Cookie"]; ok && len(cookieHeaders) > 0 {
		cookies := cookieHeaders[0]
		// Simple cookie parser for jwt token
		for _, cookie := range splitCookies(cookies) {
			if name, value := parseCookie(cookie); name == "jwt" || name == "token" {
				return value, nil
			}
		}
	}

	// For WebSocket/SSE, we might need to check query parameters
	// This would require parsing the URL, which we can do from req.URL()
	
	return "", errors.NewError(errors.ErrorTypeUnauthorized, "no token found in request")
}

// Helper functions for cookie parsing
func splitCookies(cookies string) []string {
	var result []string
	start := 0
	for i := 0; i < len(cookies); i++ {
		if cookies[i] == ';' {
			result = append(result, cookies[start:i])
			start = i + 1
			// Skip whitespace
			for start < len(cookies) && cookies[start] == ' ' {
				start++
			}
			i = start - 1
		}
	}
	if start < len(cookies) {
		result = append(result, cookies[start:])
	}
	return result
}

func parseCookie(cookie string) (name, value string) {
	for i := 0; i < len(cookie); i++ {
		if cookie[i] == '=' {
			return cookie[:i], cookie[i+1:]
		}
	}
	return cookie, ""
}
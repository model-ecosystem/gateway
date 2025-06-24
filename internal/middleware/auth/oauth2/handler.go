package oauth2

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"
	
	"gateway/pkg/errors"
)

// CallbackHandler handles OAuth2 callback requests
type CallbackHandler struct {
	providers    map[string]*Provider
	redirectBase string
	logger       *slog.Logger
}

// NewCallbackHandler creates a new OAuth2 callback handler
func NewCallbackHandler(providers map[string]*Provider, redirectBase string, logger *slog.Logger) *CallbackHandler {
	if logger == nil {
		logger = slog.Default()
	}
	
	return &CallbackHandler{
		providers:    providers,
		redirectBase: redirectBase,
		logger:       logger.With("handler", "oauth2_callback"),
	}
}

// Handle processes OAuth2 callback requests
func (h *CallbackHandler) Handle(w http.ResponseWriter, r *http.Request) error {
	// Get provider from query
	providerName := r.URL.Query().Get("provider")
	if providerName == "" {
		return errors.NewError(errors.ErrorTypeBadRequest, "missing provider parameter")
	}
	
	provider, ok := h.providers[providerName]
	if !ok {
		return errors.NewError(errors.ErrorTypeNotFound, fmt.Sprintf("provider %s not found", providerName))
	}
	
	// Check for error response
	if errCode := r.URL.Query().Get("error"); errCode != "" {
		errDesc := r.URL.Query().Get("error_description")
		return errors.NewError(errors.ErrorTypeUnauthorized, fmt.Sprintf("OAuth2 error: %s - %s", errCode, errDesc))
	}
	
	// Get authorization code
	code := r.URL.Query().Get("code")
	if code == "" {
		return errors.NewError(errors.ErrorTypeBadRequest, "missing authorization code")
	}
	
	// Get state (optional, for CSRF protection)
	// state := r.URL.Query().Get("state")
	// TODO: Validate state against stored value
	
	// Build redirect URI
	redirectURI := h.buildRedirectURI(providerName)
	
	// Exchange code for tokens
	tokenResp, err := provider.ExchangeCode(r.Context(), code, redirectURI)
	if err != nil {
		return errors.NewError(errors.ErrorTypeUnauthorized, "failed to exchange code").WithCause(err)
	}
	
	// Validate ID token if present
	var claims *Claims
	if tokenResp.IDToken != "" {
		claims, err = provider.ValidateToken(tokenResp.IDToken)
		if err != nil {
			return errors.NewError(errors.ErrorTypeUnauthorized, "invalid ID token").WithCause(err)
		}
	} else if tokenResp.AccessToken != "" {
		// Try to get user info with access token
		userInfo, err := provider.GetUserInfo(r.Context(), tokenResp.AccessToken)
		if err != nil {
			h.logger.Warn("Failed to get user info", "error", err)
		} else {
			// Build claims from user info
			claims = h.userInfoToClaims(userInfo)
		}
	}
	
	// Create session or JWT token
	// This is a simplified example - in production you'd want proper session management
	sessionToken := h.createSessionToken(claims, tokenResp)
	
	// Set cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   tokenResp.ExpiresIn,
	})
	
	// Return success response
	response := map[string]interface{}{
		"success": true,
		"provider": providerName,
	}
	
	if claims != nil {
		response["user"] = map[string]interface{}{
			"subject": claims.Subject,
			"email":   claims.Email,
			"name":    claims.Name,
		}
	}
	
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(response)
}

// AuthorizeHandler initiates the OAuth2 authorization flow
type AuthorizeHandler struct {
	providers    map[string]*Provider
	redirectBase string
	logger       *slog.Logger
}

// NewAuthorizeHandler creates a new OAuth2 authorize handler
func NewAuthorizeHandler(providers map[string]*Provider, redirectBase string, logger *slog.Logger) *AuthorizeHandler {
	if logger == nil {
		logger = slog.Default()
	}
	
	return &AuthorizeHandler{
		providers:    providers,
		redirectBase: redirectBase,
		logger:       logger.With("handler", "oauth2_authorize"),
	}
}

// Handle initiates OAuth2 authorization
func (h *AuthorizeHandler) Handle(w http.ResponseWriter, r *http.Request) error {
	// Get provider from query
	providerName := r.URL.Query().Get("provider")
	if providerName == "" {
		return errors.NewError(errors.ErrorTypeBadRequest, "missing provider parameter")
	}
	
	provider, ok := h.providers[providerName]
	if !ok {
		return errors.NewError(errors.ErrorTypeNotFound, fmt.Sprintf("provider %s not found", providerName))
	}
	
	// Generate state for CSRF protection
	state := h.generateState()
	// TODO: Store state in session or cache
	
	// Build redirect URI
	redirectURI := h.buildRedirectURI(providerName)
	
	// Get authorization URL
	authURL := provider.GetAuthorizationURL(state, redirectURI)
	
	// Redirect to provider
	http.Redirect(w, r, authURL, http.StatusFound)
	return nil
}

// Helper methods

func (h *CallbackHandler) buildRedirectURI(provider string) string {
	return fmt.Sprintf("%s/auth/callback?provider=%s", h.redirectBase, url.QueryEscape(provider))
}

func (h *AuthorizeHandler) buildRedirectURI(provider string) string {
	return fmt.Sprintf("%s/auth/callback?provider=%s", h.redirectBase, url.QueryEscape(provider))
}

func (h *AuthorizeHandler) generateState() string {
	// In production, use a cryptographically secure random generator
	return fmt.Sprintf("state-%d", time.Now().UnixNano())
}

func (h *CallbackHandler) createSessionToken(claims *Claims, tokenResp *TokenResponse) string {
	// In production, create a proper JWT or session token
	// This is just a placeholder
	if claims != nil {
		return fmt.Sprintf("session-%s-%d", claims.Subject, time.Now().Unix())
	}
	return fmt.Sprintf("session-anonymous-%d", time.Now().Unix())
}

func (h *CallbackHandler) userInfoToClaims(userInfo map[string]interface{}) *Claims {
	claims := &Claims{
		Raw:    userInfo,
		Custom: make(map[string]interface{}),
	}
	
	// Extract standard fields
	if sub, ok := userInfo["sub"].(string); ok {
		claims.Subject = sub
	}
	if email, ok := userInfo["email"].(string); ok {
		claims.Email = email
	}
	if name, ok := userInfo["name"].(string); ok {
		claims.Name = name
	}
	if username, ok := userInfo["preferred_username"].(string); ok {
		claims.PreferredUsername = username
	}
	
	// Copy all fields to custom
	for k, v := range userInfo {
		claims.Custom[k] = v
	}
	
	return claims
}

// RegisterHandlers registers OAuth2 routes
// Note: This is a placeholder - actual registration depends on the router implementation
func RegisterHandlers(providers map[string]*Provider, redirectBase string, logger *slog.Logger) (authorizeHandler *AuthorizeHandler, callbackHandler *CallbackHandler) {
	authorizeHandler = NewAuthorizeHandler(providers, redirectBase, logger)
	callbackHandler = NewCallbackHandler(providers, redirectBase, logger)
	
	// Routes to register:
	// - /auth/authorize -> authorizeHandler.Handle
	// - /auth/callback -> callbackHandler.Handle
	
	return authorizeHandler, callbackHandler
}

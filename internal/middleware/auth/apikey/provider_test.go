package apikey

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"testing"
	"time"

	"gateway/internal/middleware/auth"
	"gateway/pkg/errors"
)

func TestAPIKeyProvider_NewProvider(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &Config{
				Keys: map[string]*KeyConfig{
					"key1": {
						Key:     "secret-key-1",
						Subject: "user1",
						Type:    "user",
						Scopes:  []string{"read", "write"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "config with defaults",
			config: &Config{
				Keys: map[string]*KeyConfig{
					"key1": {
						Key: "secret-key-1",
						// Subject and Type should default
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty keys map",
			config: &Config{
				Keys: nil,
			},
			wantErr: false,
		},
		{
			name: "empty key value",
			config: &Config{
				Keys: map[string]*KeyConfig{
					"key1": {
						Key: "",
					},
				},
			},
			wantErr: true,
			errMsg:  "API key key1 has empty key value",
		},
		{
			name: "hashed keys config",
			config: &Config{
				HashKeys: true,
				Keys: map[string]*KeyConfig{
					"key1": {
						Key:     "hashed-value",
						Subject: "user1",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewProvider(tt.config, logger)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("Expected error message %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if provider == nil {
				t.Error("Expected provider but got nil")
				return
			}

			if provider.Name() != "apikey" {
				t.Errorf("Expected provider name 'apikey', got %s", provider.Name())
			}

			// Check defaults were applied
			if tt.name == "config with defaults" {
				key := provider.keys["key1"]
				if key.Subject != "key1" {
					t.Errorf("Expected default subject 'key1', got %s", key.Subject)
				}
				if key.Type != "service" {
					t.Errorf("Expected default type 'service', got %s", key.Type)
				}
			}
		})
	}
}

func TestAPIKeyProvider_Authenticate(t *testing.T) {
	expiresInFuture := time.Now().Add(time.Hour)
	expiresInPast := time.Now().Add(-time.Hour)

	config := &Config{
		DefaultScopes: []string{"default"},
		Keys: map[string]*KeyConfig{
			"key1": {
				Key:     "secret-key-1",
				Subject: "user1",
				Type:    "user",
				Scopes:  []string{"read", "write"},
			},
			"key2": {
				Key:       "secret-key-2",
				Subject:   "service1",
				Type:      "service",
				Scopes:    []string{"admin"},
				ExpiresAt: &expiresInFuture,
			},
			"key3": {
				Key:       "secret-key-3",
				Subject:   "user2",
				Type:      "user",
				ExpiresAt: &expiresInPast,
			},
			"key4": {
				Key:      "secret-key-4",
				Subject:  "user3",
				Disabled: true,
			},
			"key5": {
				Key:     "secret-key-5",
				Subject: "device1",
				Type:    "device",
				Metadata: map[string]interface{}{
					"location": "lab",
					"model":    "sensor-v2",
				},
			},
		},
	}

	provider, err := NewProvider(config, slog.Default())
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	tests := []struct {
		name        string
		credentials auth.Credentials
		wantErr     bool
		errorType   errors.ErrorType
		wantSubject string
		wantType    auth.SubjectType
		wantScopes  []string
	}{
		{
			name: "valid user key",
			credentials: &auth.APIKeyCredentials{
				Key: "secret-key-1",
			},
			wantErr:     false,
			wantSubject: "user1",
			wantType:    auth.SubjectTypeUser,
			wantScopes:  []string{"default", "read", "write"},
		},
		{
			name: "valid service key",
			credentials: &auth.APIKeyCredentials{
				Key: "secret-key-2",
			},
			wantErr:     false,
			wantSubject: "service1",
			wantType:    auth.SubjectTypeService,
			wantScopes:  []string{"default", "admin"},
		},
		{
			name: "expired key",
			credentials: &auth.APIKeyCredentials{
				Key: "secret-key-3",
			},
			wantErr:   true,
			errorType: errors.ErrorTypeBadRequest,
		},
		{
			name: "disabled key",
			credentials: &auth.APIKeyCredentials{
				Key: "secret-key-4",
			},
			wantErr:   true,
			errorType: errors.ErrorTypeBadRequest,
		},
		{
			name: "invalid key",
			credentials: &auth.APIKeyCredentials{
				Key: "invalid-key",
			},
			wantErr:   true,
			errorType: errors.ErrorTypeBadRequest,
		},
		{
			name: "device key with metadata",
			credentials: &auth.APIKeyCredentials{
				Key: "secret-key-5",
			},
			wantErr:     false,
			wantSubject: "device1",
			wantType:    auth.SubjectTypeDevice,
			wantScopes:  []string{"default"},
		},
		{
			name: "wrong credential type",
			credentials: &auth.BearerCredentials{
				Token: "secret-key-1",
			},
			wantErr:   true,
			errorType: errors.ErrorTypeBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authInfo, err := provider.Authenticate(context.Background(), tt.credentials)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}

				if gwErr, ok := err.(*errors.Error); ok {
					if gwErr.Type != tt.errorType {
						t.Errorf("Expected error type %s, got %s", tt.errorType, gwErr.Type)
					}
				} else {
					t.Errorf("Expected gateway error, got %v", err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if authInfo == nil {
				t.Error("Expected auth info but got nil")
				return
			}

			if authInfo.Subject != tt.wantSubject {
				t.Errorf("Expected subject %s, got %s", tt.wantSubject, authInfo.Subject)
			}

			if authInfo.Type != tt.wantType {
				t.Errorf("Expected type %s, got %s", tt.wantType, authInfo.Type)
			}

			if len(authInfo.Scopes) != len(tt.wantScopes) {
				t.Errorf("Expected %d scopes, got %d", len(tt.wantScopes), len(authInfo.Scopes))
			} else {
				for i, scope := range tt.wantScopes {
					if authInfo.Scopes[i] != scope {
						t.Errorf("Expected scope %s at index %d, got %s", scope, i, authInfo.Scopes[i])
					}
				}
			}

			// Check metadata for device key
			if tt.name == "device key with metadata" {
				if authInfo.Metadata["location"] != "lab" {
					t.Errorf("Expected metadata location 'lab', got %v", authInfo.Metadata["location"])
				}
				if authInfo.Metadata["model"] != "sensor-v2" {
					t.Errorf("Expected metadata model 'sensor-v2', got %v", authInfo.Metadata["model"])
				}
				if authInfo.Metadata["keyId"] != "key5" {
					t.Errorf("Expected metadata keyId 'key5', got %v", authInfo.Metadata["keyId"])
				}
			}
		})
	}
}

func TestAPIKeyProvider_HashKeys(t *testing.T) {
	// Pre-compute hash for testing
	hash := sha256.Sum256([]byte("secret-key-1"))
	hashedKey := hex.EncodeToString(hash[:])

	config := &Config{
		HashKeys: true,
		Keys: map[string]*KeyConfig{
			"key1": {
				Key:     hashedKey,
				Subject: "user1",
				Scopes:  []string{"read"},
			},
		},
	}

	provider, err := NewProvider(config, slog.Default())
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Test with plain key (should be hashed before comparison)
	credentials := &auth.APIKeyCredentials{
		Key: "secret-key-1",
	}

	authInfo, err := provider.Authenticate(context.Background(), credentials)
	if err != nil {
		t.Errorf("Authentication failed: %v", err)
		return
	}

	if authInfo.Subject != "user1" {
		t.Errorf("Expected subject user1, got %s", authInfo.Subject)
	}

	// Test with wrong key
	credentials = &auth.APIKeyCredentials{
		Key: "wrong-key",
	}

	_, err = provider.Authenticate(context.Background(), credentials)
	if err == nil {
		t.Error("Expected error for wrong key but got none")
	}
}

func TestAPIKeyProvider_Refresh(t *testing.T) {
	config := &Config{
		Keys: map[string]*KeyConfig{
			"key1": {
				Key:     "secret-key-1",
				Subject: "user1",
			},
		},
	}

	provider, err := NewProvider(config, slog.Default())
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Refresh should not be supported
	_, err = provider.Refresh(context.Background(), "key1")
	if err == nil {
		t.Error("Expected error for refresh but got none")
		return
	}

	gwErr, ok := err.(*errors.Error)
	if !ok {
		t.Errorf("Expected gateway error, got %v", err)
		return
	}

	if gwErr.Type != errors.ErrorTypeBadRequest {
		t.Errorf("Expected error type %s, got %s", errors.ErrorTypeBadRequest, gwErr.Type)
	}
}

func TestAPIKeyProvider_DynamicKeyManagement(t *testing.T) {
	config := &Config{
		Keys: map[string]*KeyConfig{
			"key1": {
				Key:     "secret-key-1",
				Subject: "user1",
			},
		},
	}

	provider, err := NewProvider(config, slog.Default())
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Test adding a key
	newKey := &KeyConfig{
		Key:     "secret-key-2",
		Subject: "user2",
		Type:    "user",
		Scopes:  []string{"read"},
	}

	err = provider.AddKey("key2", newKey)
	if err != nil {
		t.Errorf("Failed to add key: %v", err)
	}

	// Test authenticating with new key
	credentials := &auth.APIKeyCredentials{
		Key: "secret-key-2",
	}

	authInfo, err := provider.Authenticate(context.Background(), credentials)
	if err != nil {
		t.Errorf("Failed to authenticate with new key: %v", err)
		return
	}

	if authInfo.Subject != "user2" {
		t.Errorf("Expected subject user2, got %s", authInfo.Subject)
	}

	// Test adding duplicate key
	err = provider.AddKey("key2", newKey)
	if err == nil {
		t.Error("Expected error for duplicate key but got none")
	}

	// Test disabling key
	err = provider.DisableKey("key2")
	if err != nil {
		t.Errorf("Failed to disable key: %v", err)
	}

	// Test authenticating with disabled key
	_, err = provider.Authenticate(context.Background(), credentials)
	if err == nil {
		t.Error("Expected error for disabled key but got none")
	}

	// Test enabling key
	err = provider.EnableKey("key2")
	if err != nil {
		t.Errorf("Failed to enable key: %v", err)
	}

	// Should authenticate again
	authInfo, err = provider.Authenticate(context.Background(), credentials)
	if err != nil {
		t.Errorf("Failed to authenticate with re-enabled key: %v", err)
		return
	}

	// Test removing key
	err = provider.RemoveKey("key2")
	if err != nil {
		t.Errorf("Failed to remove key: %v", err)
	}

	// Test authenticating with removed key
	_, err = provider.Authenticate(context.Background(), credentials)
	if err == nil {
		t.Error("Expected error for removed key but got none")
	}

	// Test operations on non-existent key
	err = provider.DisableKey("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent key but got none")
	}

	err = provider.EnableKey("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent key but got none")
	}

	err = provider.RemoveKey("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent key but got none")
	}
}

func TestAPIKeyProvider_AddKeyWithHashing(t *testing.T) {
	config := &Config{
		HashKeys: true,
		Keys:     make(map[string]*KeyConfig),
	}

	provider, err := NewProvider(config, slog.Default())
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Add a key (should be hashed)
	newKey := &KeyConfig{
		Key:     "plain-text-key",
		Subject: "user1",
		Scopes:  []string{"read"},
	}

	err = provider.AddKey("key1", newKey)
	if err != nil {
		t.Errorf("Failed to add key: %v", err)
	}

	// Verify the key was hashed
	storedKey := provider.keys["key1"]
	if storedKey.Key == "plain-text-key" {
		t.Error("Key was not hashed")
	}

	// Test authentication with plain text key
	credentials := &auth.APIKeyCredentials{
		Key: "plain-text-key",
	}

	authInfo, err := provider.Authenticate(context.Background(), credentials)
	if err != nil {
		t.Errorf("Failed to authenticate: %v", err)
		return
	}

	if authInfo.Subject != "user1" {
		t.Errorf("Expected subject user1, got %s", authInfo.Subject)
	}
}

func TestAPIKeyProvider_EmptyKeyInAdd(t *testing.T) {
	config := &Config{
		Keys: make(map[string]*KeyConfig),
	}

	provider, err := NewProvider(config, slog.Default())
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Try to add key with empty value
	newKey := &KeyConfig{
		Key:     "",
		Subject: "user1",
	}

	err = provider.AddKey("key1", newKey)
	if err == nil {
		t.Error("Expected error for empty key value but got none")
		return
	}

	if err.Error() != "key value cannot be empty" {
		t.Errorf("Expected error 'key value cannot be empty', got %v", err)
	}
}

func TestAPIKeyProvider_ConcurrentAccess(t *testing.T) {
	config := &Config{
		Keys: map[string]*KeyConfig{
			"key1": {
				Key:     "secret-key-1",
				Subject: "user1",
			},
		},
	}

	provider, err := NewProvider(config, slog.Default())
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Run concurrent operations
	done := make(chan bool)
	errors := make(chan error, 100)

	// Concurrent authentications
	for i := 0; i < 10; i++ {
		go func() {
			credentials := &auth.APIKeyCredentials{
				Key: "secret-key-1",
			}
			_, err := provider.Authenticate(context.Background(), credentials)
			if err != nil {
				errors <- err
			}
			done <- true
		}()
	}

	// Concurrent key management
	for i := 0; i < 10; i++ {
		keyID := "key" + string(rune('2'+i))
		go func(id string) {
			newKey := &KeyConfig{
				Key:     "secret-" + id,
				Subject: "user-" + id,
			}
			err := provider.AddKey(id, newKey)
			if err != nil {
				errors <- err
			}
			done <- true
		}(keyID)
	}

	// Wait for all operations
	for i := 0; i < 20; i++ {
		<-done
	}

	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
	}
}

// Helper function to create a string pointer
func stringPtr(s string) *string {
	return &s
}

package rbac

import (
	"context"
	"testing"
	"time"
)

func TestRBAC(t *testing.T) {
	// Create test policy
	policy := &Policy{
		Name:        "test-policy",
		Description: "Test policy",
		Roles: map[string]*Role{
			"admin": {
				Name:        "admin",
				Description: "Administrator role",
				Permissions: []string{"*:*"}, // All permissions
			},
			"viewer": {
				Name:        "viewer",
				Description: "Viewer role",
				Permissions: []string{"*:read"}, // Read-only
			},
			"editor": {
				Name:        "editor",
				Description: "Editor role",
				Permissions: []string{
					"documents:read",
					"documents:create",
					"documents:update",
				},
				Inherits: []string{"viewer"}, // Inherits viewer permissions
			},
		},
		Bindings: map[string][]string{
			"user1": {"admin"},
			"user2": {"editor"},
			"user3": {"viewer"},
		},
	}
	
	config := &Config{
		Policies:  []*Policy{policy},
		CacheSize: 100,
		CacheTTL:  1 * time.Minute,
	}
	
	rbac, err := New(config, nil)
	if err != nil {
		t.Fatalf("Failed to create RBAC: %v", err)
	}
	
	ctx := context.Background()
	
	// Test admin permissions
	if !rbac.HasPermission(ctx, "user1", "documents", "read") {
		t.Error("Admin should have read permission on documents")
	}
	if !rbac.HasPermission(ctx, "user1", "users", "delete") {
		t.Error("Admin should have delete permission on users")
	}
	
	// Test editor permissions
	if !rbac.HasPermission(ctx, "user2", "documents", "create") {
		t.Error("Editor should have create permission on documents")
	}
	if !rbac.HasPermission(ctx, "user2", "documents", "read") {
		t.Error("Editor should have inherited read permission")
	}
	if rbac.HasPermission(ctx, "user2", "documents", "delete") {
		t.Error("Editor should not have delete permission")
	}
	
	// Test viewer permissions
	if !rbac.HasPermission(ctx, "user3", "documents", "read") {
		t.Error("Viewer should have read permission")
	}
	if rbac.HasPermission(ctx, "user3", "documents", "create") {
		t.Error("Viewer should not have create permission")
	}
	
	// Test non-existent user
	if rbac.HasPermission(ctx, "user4", "documents", "read") {
		t.Error("Non-existent user should not have any permission")
	}
}

func TestRoleBinding(t *testing.T) {
	policy := &Policy{
		Name: "test-policy",
		Roles: map[string]*Role{
			"reader": {
				Name:        "reader",
				Permissions: []string{"docs:read"},
			},
			"writer": {
				Name:        "writer",
				Permissions: []string{"docs:write"},
			},
		},
		Bindings: make(map[string][]string),
	}
	
	config := &Config{
		Policies: []*Policy{policy},
	}
	
	rbac, err := New(config, nil)
	if err != nil {
		t.Fatalf("Failed to create RBAC: %v", err)
	}
	
	// Bind role
	err = rbac.BindRole("test-policy", "user1", "reader")
	if err != nil {
		t.Fatalf("Failed to bind role: %v", err)
	}
	
	// Check permission
	if !rbac.HasPermission(context.Background(), "user1", "docs", "read") {
		t.Error("User should have read permission after binding")
	}
	
	// Get roles
	roles := rbac.GetRoles("user1")
	if len(roles) != 1 || roles[0] != "reader" {
		t.Errorf("Expected roles [reader], got %v", roles)
	}
	
	// Unbind role
	err = rbac.UnbindRole("test-policy", "user1", "reader")
	if err != nil {
		t.Fatalf("Failed to unbind role: %v", err)
	}
	
	// Check permission removed
	if rbac.HasPermission(context.Background(), "user1", "docs", "read") {
		t.Error("User should not have read permission after unbinding")
	}
}

func TestWildcardPermissions(t *testing.T) {
	policy := &Policy{
		Name: "test-policy",
		Roles: map[string]*Role{
			"api-user": {
				Name: "api-user",
				Permissions: []string{
					"api:*",      // All API actions
					"users:read", // Specific permission
				},
			},
		},
		Bindings: map[string][]string{
			"user1": {"api-user"},
		},
	}
	
	config := &Config{
		Policies: []*Policy{policy},
	}
	
	rbac, err := New(config, nil)
	if err != nil {
		t.Fatalf("Failed to create RBAC: %v", err)
	}
	
	ctx := context.Background()
	
	// Test wildcard match
	if !rbac.HasPermission(ctx, "user1", "api", "read") {
		t.Error("Should have api:read permission")
	}
	if !rbac.HasPermission(ctx, "user1", "api", "write") {
		t.Error("Should have api:write permission")
	}
	if !rbac.HasPermission(ctx, "user1", "api", "delete") {
		t.Error("Should have api:delete permission")
	}
	
	// Test specific permission
	if !rbac.HasPermission(ctx, "user1", "users", "read") {
		t.Error("Should have users:read permission")
	}
	if rbac.HasPermission(ctx, "user1", "users", "write") {
		t.Error("Should not have users:write permission")
	}
}

func TestPermissionCache(t *testing.T) {
	policy := &Policy{
		Name: "test-policy",
		Roles: map[string]*Role{
			"user": {
				Name:        "user",
				Permissions: []string{"resource:action"},
			},
		},
		Bindings: map[string][]string{
			"user1": {"user"},
		},
	}
	
	config := &Config{
		Policies:  []*Policy{policy},
		CacheSize: 10,
		CacheTTL:  100 * time.Millisecond,
	}
	
	rbac, err := New(config, nil)
	if err != nil {
		t.Fatalf("Failed to create RBAC: %v", err)
	}
	
	ctx := context.Background()
	
	// First check - should hit RBAC
	if !rbac.HasPermission(ctx, "user1", "resource", "action") {
		t.Error("Should have permission")
	}
	
	// Second check - should hit cache
	if !rbac.HasPermission(ctx, "user1", "resource", "action") {
		t.Error("Should have permission (from cache)")
	}
	
	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)
	
	// Third check - should hit RBAC again
	if !rbac.HasPermission(ctx, "user1", "resource", "action") {
		t.Error("Should have permission (cache expired)")
	}
}

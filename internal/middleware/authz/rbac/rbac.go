package rbac

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Role represents a role with permissions
type Role struct {
	Name        string              `yaml:"name"`
	Description string              `yaml:"description"`
	Permissions []string            `yaml:"permissions"`
	Inherits    []string            `yaml:"inherits"` // Role inheritance
	Metadata    map[string]string   `yaml:"metadata"`
}

// Policy represents an RBAC policy
type Policy struct {
	Name        string              `yaml:"name"`
	Description string              `yaml:"description"`
	Roles       map[string]*Role    `yaml:"roles"`
	Bindings    map[string][]string `yaml:"bindings"` // subject -> roles mapping
	CreatedAt   time.Time           `yaml:"createdAt"`
	UpdatedAt   time.Time           `yaml:"updatedAt"`
}

// RBAC implements role-based access control
type RBAC struct {
	policies  map[string]*Policy
	mu        sync.RWMutex
	logger    *slog.Logger
	cache     *permissionCache
	cacheSize int
	cacheTTL  time.Duration
}

// Config represents RBAC configuration
type Config struct {
	Policies      []*Policy     `yaml:"policies"`
	CacheSize     int           `yaml:"cacheSize"`
	CacheTTL      time.Duration `yaml:"cacheTTL"`
	DefaultPolicy string        `yaml:"defaultPolicy"`
}

// New creates a new RBAC instance
func New(config *Config, logger *slog.Logger) (*RBAC, error) {
	if logger == nil {
		logger = slog.Default()
	}
	
	rbac := &RBAC{
		policies:  make(map[string]*Policy),
		logger:    logger.With("component", "rbac"),
		cacheSize: config.CacheSize,
		cacheTTL:  config.CacheTTL,
	}
	
	// Set defaults
	if rbac.cacheSize == 0 {
		rbac.cacheSize = 1000
	}
	if rbac.cacheTTL == 0 {
		rbac.cacheTTL = 5 * time.Minute
	}
	
	// Initialize cache
	rbac.cache = newPermissionCache(rbac.cacheSize, rbac.cacheTTL)
	
	// Load policies
	for _, policy := range config.Policies {
		if err := rbac.AddPolicy(policy); err != nil {
			return nil, fmt.Errorf("failed to add policy %s: %w", policy.Name, err)
		}
	}
	
	return rbac, nil
}

// AddPolicy adds a new policy
func (r *RBAC) AddPolicy(policy *Policy) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if policy.Name == "" {
		return fmt.Errorf("policy name is required")
	}
	
	// Validate roles
	for _, role := range policy.Roles {
		if err := r.validateRole(role, policy); err != nil {
			return fmt.Errorf("invalid role %s: %w", role.Name, err)
		}
	}
	
	// Set timestamps
	now := time.Now()
	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = now
	}
	policy.UpdatedAt = now
	
	r.policies[policy.Name] = policy
	
	// Clear cache as permissions may have changed
	r.cache.Clear()
	
	r.logger.Info("Policy added", "policy", policy.Name, "roles", len(policy.Roles))
	return nil
}

// RemovePolicy removes a policy
func (r *RBAC) RemovePolicy(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if _, exists := r.policies[name]; !exists {
		return fmt.Errorf("policy %s not found", name)
	}
	
	delete(r.policies, name)
	r.cache.Clear()
	
	r.logger.Info("Policy removed", "policy", name)
	return nil
}

// HasPermission checks if a subject has a specific permission
func (r *RBAC) HasPermission(ctx context.Context, subject, resource, action string) bool {
	// Check cache first
	permission := formatPermission(resource, action)
	cacheKey := subject + ":" + permission
	
	if allowed, found := r.cache.Get(cacheKey); found {
		return allowed
	}
	
	// Check permissions
	allowed := r.checkPermission(subject, permission)
	
	// Cache result
	r.cache.Set(cacheKey, allowed)
	
	return allowed
}

// GetRoles returns all roles for a subject
func (r *RBAC) GetRoles(subject string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var allRoles []string
	seen := make(map[string]bool)
	
	for _, policy := range r.policies {
		if roles, exists := policy.Bindings[subject]; exists {
			for _, role := range roles {
				if !seen[role] {
					allRoles = append(allRoles, role)
					seen[role] = true
				}
			}
		}
	}
	
	return allRoles
}

// GetPermissions returns all permissions for a subject
func (r *RBAC) GetPermissions(subject string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	permissions := make(map[string]bool)
	
	for _, policy := range r.policies {
		if roles, exists := policy.Bindings[subject]; exists {
			for _, roleName := range roles {
				if role, exists := policy.Roles[roleName]; exists {
					r.collectRolePermissions(role, policy, permissions)
				}
			}
		}
	}
	
	// Convert to slice
	var result []string
	for perm := range permissions {
		result = append(result, perm)
	}
	
	return result
}

// BindRole binds a role to a subject in a policy
func (r *RBAC) BindRole(policyName, subject, role string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	policy, exists := r.policies[policyName]
	if !exists {
		return fmt.Errorf("policy %s not found", policyName)
	}
	
	if _, exists := policy.Roles[role]; !exists {
		return fmt.Errorf("role %s not found in policy %s", role, policyName)
	}
	
	if policy.Bindings == nil {
		policy.Bindings = make(map[string][]string)
	}
	
	// Check if already bound
	for _, existingRole := range policy.Bindings[subject] {
		if existingRole == role {
			return nil // Already bound
		}
	}
	
	policy.Bindings[subject] = append(policy.Bindings[subject], role)
	policy.UpdatedAt = time.Now()
	
	// Clear cache for this subject
	r.cache.ClearSubject(subject)
	
	r.logger.Info("Role bound", "policy", policyName, "subject", subject, "role", role)
	return nil
}

// UnbindRole unbinds a role from a subject
func (r *RBAC) UnbindRole(policyName, subject, role string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	policy, exists := r.policies[policyName]
	if !exists {
		return fmt.Errorf("policy %s not found", policyName)
	}
	
	if roles, exists := policy.Bindings[subject]; exists {
		var newRoles []string
		for _, r := range roles {
			if r != role {
				newRoles = append(newRoles, r)
			}
		}
		
		if len(newRoles) == 0 {
			delete(policy.Bindings, subject)
		} else {
			policy.Bindings[subject] = newRoles
		}
		
		policy.UpdatedAt = time.Now()
		r.cache.ClearSubject(subject)
		
		r.logger.Info("Role unbound", "policy", policyName, "subject", subject, "role", role)
	}
	
	return nil
}

// Internal methods

func (r *RBAC) checkPermission(subject, permission string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	for _, policy := range r.policies {
		if roles, exists := policy.Bindings[subject]; exists {
			for _, roleName := range roles {
				if role, exists := policy.Roles[roleName]; exists {
					if r.roleHasPermission(role, permission, policy) {
						return true
					}
				}
			}
		}
	}
	
	return false
}

func (r *RBAC) roleHasPermission(role *Role, permission string, policy *Policy) bool {
	// Check direct permissions
	for _, perm := range role.Permissions {
		if matchPermission(perm, permission) {
			return true
		}
	}
	
	// Check inherited roles
	for _, inheritedRoleName := range role.Inherits {
		if inheritedRole, exists := policy.Roles[inheritedRoleName]; exists {
			if r.roleHasPermission(inheritedRole, permission, policy) {
				return true
			}
		}
	}
	
	return false
}

func (r *RBAC) collectRolePermissions(role *Role, policy *Policy, permissions map[string]bool) {
	// Add direct permissions
	for _, perm := range role.Permissions {
		permissions[perm] = true
	}
	
	// Add inherited permissions
	for _, inheritedRoleName := range role.Inherits {
		if inheritedRole, exists := policy.Roles[inheritedRoleName]; exists {
			r.collectRolePermissions(inheritedRole, policy, permissions)
		}
	}
}

func (r *RBAC) validateRole(role *Role, policy *Policy) error {
	if role.Name == "" {
		return fmt.Errorf("role name is required")
	}
	
	// Check for circular inheritance
	visited := make(map[string]bool)
	return r.checkCircularInheritance(role.Name, role, policy, visited)
}

func (r *RBAC) checkCircularInheritance(originalRole string, role *Role, policy *Policy, visited map[string]bool) error {
	if visited[role.Name] {
		return fmt.Errorf("circular inheritance detected")
	}
	
	visited[role.Name] = true
	
	for _, inheritedRoleName := range role.Inherits {
		if inheritedRoleName == originalRole {
			return fmt.Errorf("circular inheritance detected: %s inherits from itself", originalRole)
		}
		
		if inheritedRole, exists := policy.Roles[inheritedRoleName]; exists {
			if err := r.checkCircularInheritance(originalRole, inheritedRole, policy, visited); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// Helper functions

func formatPermission(resource, action string) string {
	return resource + ":" + action
}

func matchPermission(pattern, permission string) bool {
	// Exact match
	if pattern == permission {
		return true
	}
	
	// Full wildcard
	if pattern == "*:*" {
		return true
	}
	
	// Resource wildcard (e.g., "documents:*")
	if strings.HasSuffix(pattern, ":*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(permission, prefix)
	}
	
	// Action wildcard (e.g., "*:read")
	if strings.HasPrefix(pattern, "*:") {
		suffix := strings.TrimPrefix(pattern, "*:")
		return strings.HasSuffix(permission, ":"+suffix)
	}
	
	return false
}

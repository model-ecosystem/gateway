package openapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gateway/internal/core"
)

func TestLoadSpec(t *testing.T) {
	// Create test spec
	specYAML := `
openapi: "3.0.0"
info:
  title: "Test API"
  version: "1.0.0"
  description: "Test API Description"
servers:
  - url: "https://api.example.com/v1"
paths:
  /users:
    get:
      operationId: "getUsers"
      summary: "Get all users"
      tags:
        - users
      x-gateway:
        serviceName: "user-service"
        loadBalance: "round_robin"
        timeout: 30
        authRequired: true
        requiredScopes:
          - "users:read"
    post:
      operationId: "createUser"
      summary: "Create a user"
      tags:
        - users
      x-gateway:
        serviceName: "user-service"
        rateLimit: 100
  /users/{id}:
    get:
      operationId: "getUser"
      summary: "Get a user by ID"
      parameters:
        - name: id
          in: path
          required: true
          description: "User ID"
      tags:
        - users
tags:
  - name: users
    description: "User operations"
    x-service: "user-service"
`

	// Write test file
	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specFile, []byte(specYAML), 0644); err != nil {
		t.Fatalf("Failed to write test spec: %v", err)
	}

	// Test loading from file
	loader := NewLoader(nil)
	spec, err := loader.Load(specFile)
	if err != nil {
		t.Fatalf("Failed to load spec: %v", err)
	}

	// Verify spec
	if spec.Info.Title != "Test API" {
		t.Errorf("Expected title 'Test API', got %s", spec.Info.Title)
	}
	if spec.Info.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", spec.Info.Version)
	}
	if len(spec.Paths) != 2 {
		t.Errorf("Expected 2 paths, got %d", len(spec.Paths))
	}

	// Check operations
	usersPath := spec.Paths["/users"]
	if usersPath.Get == nil {
		t.Error("Expected GET /users operation")
	} else {
		if usersPath.Get.OperationID != "getUsers" {
			t.Errorf("Expected operationId 'getUsers', got %s", usersPath.Get.OperationID)
		}
		if usersPath.Get.XGateway == nil {
			t.Error("Expected x-gateway extension")
		} else {
			if usersPath.Get.XGateway.ServiceName != "user-service" {
				t.Errorf("Expected serviceName 'user-service', got %s", usersPath.Get.XGateway.ServiceName)
			}
			if !usersPath.Get.XGateway.AuthRequired {
				t.Error("Expected authRequired to be true")
			}
		}
	}
}

func TestLoadFromURL(t *testing.T) {
	// Create test server
	specJSON := `{
		"openapi": "3.0.0",
		"info": {
			"title": "URL Test API",
			"version": "2.0.0"
		},
		"paths": {
			"/health": {
				"get": {
					"operationId": "healthCheck",
					"summary": "Health check"
				}
			}
		}
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(specJSON))
	}))
	defer server.Close()

	// Test loading from URL
	loader := NewLoader(nil)
	spec, err := loader.Load(server.URL + "/openapi.json")
	if err != nil {
		t.Fatalf("Failed to load spec from URL: %v", err)
	}

	if spec.Info.Title != "URL Test API" {
		t.Errorf("Expected title 'URL Test API', got %s", spec.Info.Title)
	}
}

func TestToRouteRules(t *testing.T) {
	spec := &Spec{
		OpenAPI: "3.0.0",
		Info: Info{
			Title:   "Test API",
			Version: "1.0.0",
		},
		Tags: []Tag{
			{
				Name:     "products",
				XService: "product-service",
			},
		},
		Paths: map[string]PathItem{
			"/products": {
				Get: &Operation{
					OperationID: "listProducts",
					Summary:     "List products",
					Tags:        []string{"products"},
				},
				Post: &Operation{
					OperationID: "createProduct",
					Summary:     "Create product",
					Tags:        []string{"products"},
					XGateway: &GatewayExtension{
						LoadBalance:  "consistent_hash",
						Timeout:      60,
						RateLimit:    50,
						AuthRequired: true,
						RequiredScopes: []string{"products:write"},
					},
				},
			},
			"/products/{id}": {
				Get: &Operation{
					OperationID: "getProduct",
					Summary:     "Get product",
					Tags:        []string{"products"},
				},
			},
		},
	}

	loader := NewLoader(nil)
	rules := loader.ToRouteRules(spec, "default-service")

	// Check number of rules
	if len(rules) != 3 {
		t.Fatalf("Expected 3 rules, got %d", len(rules))
	}

	// Check specific rules
	var createProductRule *core.RouteRule
	for i := range rules {
		if rules[i].ID == "createProduct" {
			createProductRule = &rules[i]
			break
		}
	}

	if createProductRule == nil {
		t.Fatal("createProduct rule not found")
	}

	// Verify rule properties
	if createProductRule.Path != "/products" {
		t.Errorf("Expected path '/products', got %s", createProductRule.Path)
	}
	if len(createProductRule.Methods) != 1 || createProductRule.Methods[0] != "POST" {
		t.Errorf("Expected methods [POST], got %v", createProductRule.Methods)
	}
	if createProductRule.ServiceName != "product-service" {
		t.Errorf("Expected service 'product-service', got %s", createProductRule.ServiceName)
	}
	if createProductRule.LoadBalance != core.LoadBalanceConsistentHash {
		t.Errorf("Expected load balance 'consistent_hash', got %s", createProductRule.LoadBalance)
	}
	if createProductRule.Timeout != 60*time.Second {
		t.Errorf("Expected timeout 60s, got %v", createProductRule.Timeout)
	}

	// Check metadata
	if authRequired, ok := createProductRule.Metadata["authRequired"].(bool); !ok || !authRequired {
		t.Error("Expected authRequired to be true")
	}
	if rateLimit, ok := createProductRule.Metadata["rateLimit"].(int); !ok || rateLimit != 50 {
		t.Errorf("Expected rateLimit 50, got %v", rateLimit)
	}
}

func TestConvertPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/users", "/users"},
		{"/users/{id}", "/users/*"},
		{"/users/{userId}/posts/{postId}", "/users/*/posts/*"},
		{"/api/v1/{version}/users", "/api/v1/*/users"},
	}

	for _, test := range tests {
		result := convertPath(test.input)
		if result != test.expected {
			t.Errorf("convertPath(%s) = %s, expected %s", test.input, result, test.expected)
		}
	}
}

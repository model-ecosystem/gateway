package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Sample private key for testing (matches the public key in config-auth.yaml)
const privateKeyPEM = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA0Z3VS5JJcds6xfn/tZAoDdDqA9kBE2uPDnlfP+ViXeFWv4zc
YzUVlPkCs1/8kA/hYLmZQC5JqupG0efveBMATcVURAsqh7FTgVuDOLHnknUWJGvL
3U6p6lzjMdv9t+M/mr7g+wO+7tg5QNGHqMZZCHB7UgIBSeKr0k8K8wVzXfTZmHMh
2L+B70jGgQC3DpKYwqrVdBTB5gvCVGipnGFs1Ztyc/kFf6SfEBM7GMCB1be4bPme
fU/BdFqEPdAAxmVRMhh/fCNi5HG1BZU5VP6hwGb7qrOlCtrYvkykRFLRJLMh4QUk
Q7iC6x6NUibyg7LBzuTKlCK6jTEhBBiBVvKFOQIDAQABAoIBAGKPDhSjzy6MQ7ho
HO7DQF1m3NRZ0fzJb7JLbGYHiLbPGHivLGLf7d2jlFz7MvAQjBV2dMdCYQ5Pqktc
ffm9LEftvdBNUKnQPtFJKHnSGF+nRzAchO7vT0QAGP7lf3IkQT6oYO5TbqGNf9qv
A3SrQ7VFAGmFbDJxG7RTWdgFkRz7AIZepIfK7TaCPc5p7xeGIE0/A3rD5tYV8lGF
3HDQkB5MSLVbLaDr5I9Rjo2tGo0bvPr7jiCGkTsaW/x2PObqPgVYC1iYIOGKkXLV
rDCW9aCilxVSR5l7M3y/Gvk/kUOmE/V6f9p4RrgxCCHZWQ0LmhtA1MawbFlCkchZ
XKdCEPkCgYEA7WJkDMRmOY5tUrDKGQH7VZdUJBc8VEHGJl+l8MtoknCFj8SgvJ3D
JChg2s2FAeWB7oJr7OuCo5H7p/Tk6I7MQt1Yw7uVQNB+5xCCtNQV9nEfqGjxqBvo
/iFZQ9K0p5JNLvQdqVvLg+1sgtXlNlQx3YzTBYNirXUKLLM7W+YKjQMCgYEA4lLw
oZQxw3Y7kg/73xHQSGb7xTVQNxs6wVp7rs8Ufm6dlaD7zk5Y0j9d7cHhq7yK7oeF
ov6FzvEYJh/42RL8zMvR+eBBNy5dxJUQdWP6nLrdBqiHlSijH5x0eG4RiWTM7GFE
v6OxbAkSLhQ4xBsYVrZhvVPWIG0H5NN0CvC5QAMCgYEAzT/NOmKqRGLBFuNFBqNT
7hKx/xB3Fqw3N6z1R7dIygJVxCfJWKGQ7PAA0sI8xCWQE1RYYPCz2FMFGnzrDkMS
PIFvLm5U6BkSPqxK6Vk9iLNNKlA8SH5E7yGJ7a3abrO9dTEQF+7tZR5UD6gYlBMU
hKTWvBiBYqLLMZ6Dw8fxG48CgYBx1erCCBsFBLIBgXD1G8KDG2jXgC6t8xE1z5ex
1uaKNYyC4wlXiHGUx3hONxGCLiiVYL0kFfCvC+MmDCQWWcGTOH72EfVkg0Y9vGsr
J4bZLgqPDLDqGQEIK9lobjI8DvpOCcT6POFLj8VvXqaOvIyR6YrjsuYYsCGKe0LB
IgJj0wKBgBJQOZqhN3Y5CmAuOfP0daRPHFQC7v7k5KQYx+VpZiGFvH8U4TfCEfIH
DFUDfxCFYBBFH0w/AYN8LjMfX0dMpBZGPZLAKEPqCcDZ3gvLaZWPl3aYqRs9IMNK
xWvNKZ5xQIdfCCGxQS4fJqsQnd1e2jdCj/bPBnaCanXQD9kiJYZ1
-----END RSA PRIVATE KEY-----`

func main() {
	// Parse private key
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		log.Fatal("Failed to parse PEM block")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		log.Fatal("Failed to parse private key:", err)
	}

	// Create claims
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":   "user123",
		"iss":   "https://auth.example.com",
		"aud":   []string{"https://api.example.com"},
		"scope": "api:read api:write",
		"iat":   now.Unix(),
		"exp":   now.Add(24 * time.Hour).Unix(),
		"typ":   "user",
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	// Sign token
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		log.Fatal("Failed to sign token:", err)
	}

	fmt.Println("JWT Token:")
	fmt.Println(tokenString)
	fmt.Println()
	fmt.Println("Use this token with:")
	fmt.Printf("curl -H \"Authorization: Bearer %s\" http://localhost:8080/api/example/test\n", tokenString)
}

func generateKeyPair() (*rsa.PrivateKey, error) {
	// This function generates a new RSA key pair
	// For testing purposes, we're using a pre-generated key above
	return nil, nil
}
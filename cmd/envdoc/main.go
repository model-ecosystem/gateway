package main

import (
	"fmt"
	"os"
	
	"gateway/internal/config"
)

func main() {
	fmt.Println("# Gateway Environment Variables")
	fmt.Println()
	fmt.Println("The gateway supports configuration via environment variables.")
	fmt.Println("Environment variables override values from the configuration file.")
	fmt.Println()
	fmt.Println("## Available Environment Variables")
	fmt.Println()
	
	cfg := &config.Config{}
	examples := config.EnvExample(cfg)
	
	for _, example := range examples {
		fmt.Printf("- `%s`\n", example)
	}
	
	fmt.Println()
	fmt.Println("## Examples")
	fmt.Println()
	fmt.Println("```bash")
	fmt.Println("# Override HTTP port")
	fmt.Println("export GATEWAY_GATEWAY_FRONTEND_HTTP_PORT=9090")
	fmt.Println()
	fmt.Println("# Enable metrics")
	fmt.Println("export GATEWAY_GATEWAY_METRICS_ENABLED=true")
	fmt.Println("export GATEWAY_GATEWAY_METRICS_PATH=/metrics")
	fmt.Println()
	fmt.Println("# Configure CORS")
	fmt.Println("export GATEWAY_GATEWAY_CORS_ENABLED=true")
	fmt.Println("export GATEWAY_GATEWAY_CORS_ALLOWEDORIGINS=https://example.com,https://app.example.com")
	fmt.Println("export GATEWAY_GATEWAY_CORS_ALLOWCREDENTIALS=true")
	fmt.Println()
	fmt.Println("# Run gateway with env vars")
	fmt.Println("./gateway -config gateway.yaml")
	fmt.Println("```")
	
	os.Exit(0)
}
package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// LoadEnv loads configuration from environment variables
func LoadEnv(cfg *Config) error {
	return loadEnvStruct(reflect.ValueOf(cfg).Elem(), "GATEWAY")
}

// loadEnvStruct recursively loads environment variables into a struct
func loadEnvStruct(v reflect.Value, prefix string) error {
	t := v.Type()
	
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		
		// Skip unexported fields
		if !field.CanSet() {
			continue
		}
		
		// Get the YAML tag to use as the env var name
		yamlTag := fieldType.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			continue
		}
		
		// Remove omitempty and other options
		envName := strings.Split(yamlTag, ",")[0]
		envKey := fmt.Sprintf("%s_%s", prefix, strings.ToUpper(envName))
		
		// Handle different field types
		switch field.Kind() {
		case reflect.String:
			if val := os.Getenv(envKey); val != "" {
				field.SetString(val)
			}
			
		case reflect.Int, reflect.Int64:
			if val := os.Getenv(envKey); val != "" {
				intVal, err := strconv.ParseInt(val, 10, 64)
				if err != nil {
					return fmt.Errorf("invalid int value for %s: %v", envKey, err)
				}
				field.SetInt(intVal)
			}
			
		case reflect.Float64:
			if val := os.Getenv(envKey); val != "" {
				floatVal, err := strconv.ParseFloat(val, 64)
				if err != nil {
					return fmt.Errorf("invalid float value for %s: %v", envKey, err)
				}
				field.SetFloat(floatVal)
			}
			
		case reflect.Bool:
			if val := os.Getenv(envKey); val != "" {
				boolVal, err := strconv.ParseBool(val)
				if err != nil {
					return fmt.Errorf("invalid bool value for %s: %v", envKey, err)
				}
				field.SetBool(boolVal)
			}
			
		case reflect.Slice:
			if val := os.Getenv(envKey); val != "" {
				// Handle string slices (comma-separated)
				if field.Type().Elem().Kind() == reflect.String {
					parts := strings.Split(val, ",")
					slice := reflect.MakeSlice(field.Type(), len(parts), len(parts))
					for i, part := range parts {
						slice.Index(i).SetString(strings.TrimSpace(part))
					}
					field.Set(slice)
				}
			}
			
		case reflect.Struct:
			// Recursively handle nested structs
			if err := loadEnvStruct(field, envKey); err != nil {
				return err
			}
			
		case reflect.Ptr:
			// Handle pointer fields
			if field.IsNil() {
				// Check if any env vars exist for this prefix
				if hasEnvVarsWithPrefix(envKey) {
					// Create a new instance
					field.Set(reflect.New(field.Type().Elem()))
				} else {
					continue
				}
			}
			
			if !field.IsNil() {
				if err := loadEnvStruct(field.Elem(), envKey); err != nil {
					return err
				}
			}
			
		case reflect.Map:
			// Skip maps for now (complex to handle via env vars)
			continue
		}
	}
	
	return nil
}

// hasEnvVarsWithPrefix checks if any environment variables exist with the given prefix
func hasEnvVarsWithPrefix(prefix string) bool {
	prefix = prefix + "_"
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, prefix) {
			return true
		}
	}
	return false
}

// EnvExample generates example environment variables for the configuration
func EnvExample(cfg *Config) []string {
	var examples []string
	generateEnvExamples(reflect.TypeOf(cfg).Elem(), "GATEWAY", &examples)
	return examples
}

// generateEnvExamples recursively generates example environment variables
func generateEnvExamples(t reflect.Type, prefix string, examples *[]string) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		
		// Get the YAML tag
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			continue
		}
		
		// Remove omitempty and other options
		envName := strings.Split(yamlTag, ",")[0]
		envKey := fmt.Sprintf("%s_%s", prefix, strings.ToUpper(envName))
		
		// Generate examples based on field type
		switch field.Type.Kind() {
		case reflect.String:
			*examples = append(*examples, fmt.Sprintf("%s=value", envKey))
			
		case reflect.Int, reflect.Int64:
			*examples = append(*examples, fmt.Sprintf("%s=123", envKey))
			
		case reflect.Float64:
			*examples = append(*examples, fmt.Sprintf("%s=1.5", envKey))
			
		case reflect.Bool:
			*examples = append(*examples, fmt.Sprintf("%s=true", envKey))
			
		case reflect.Slice:
			if field.Type.Elem().Kind() == reflect.String {
				*examples = append(*examples, fmt.Sprintf("%s=value1,value2,value3", envKey))
			}
			
		case reflect.Struct:
			generateEnvExamples(field.Type, envKey, examples)
			
		case reflect.Ptr:
			if field.Type.Elem().Kind() == reflect.Struct {
				generateEnvExamples(field.Type.Elem(), envKey, examples)
			}
		}
	}
}
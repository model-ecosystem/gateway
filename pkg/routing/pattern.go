package routing

import "strings"

// ConvertToServeMuxPattern converts a path pattern to ServeMux format
func ConvertToServeMuxPattern(pattern string) string {
	// Replace :param with {param} for ServeMux compatibility
	if strings.Contains(pattern, ":") {
		parts := strings.Split(pattern, "/")
		for i, part := range parts {
			if strings.HasPrefix(part, ":") {
				parts[i] = "{" + part[1:] + "}"
			}
		}
		pattern = strings.Join(parts, "/")
	}

	// Handle wildcard patterns
	// ServeMux uses {$} for matching rest of path, not *
	if strings.HasSuffix(pattern, "/*") {
		pattern = strings.TrimSuffix(pattern, "/*") + "/{path...}"
	} else if strings.HasSuffix(pattern, "*") {
		pattern = strings.TrimSuffix(pattern, "*") + "{path...}"
	}

	return pattern
}

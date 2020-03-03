package templates

import (
	"strings"
	"text/template"
	"time"
)

// DefaultFuncMap creates a map of functions to be possibly referenced in a Go template
func DefaultFuncMap() template.FuncMap {
	return template.FuncMap{
		"now":   getCurrentTime,
		"decr":  decrement,
		"lower": lowercase,
		"mkMap": mkMap,
	}
}

func getCurrentTime() string {
	return time.Now().Format(time.RFC822)
}

func decrement(arg int) int {
	return arg - 1
}

func lowercase(s string) string {
	return strings.ToLower(s)
}

func mkMap(args ...string) map[string]string {
	newMap := make(map[string]string)
	for _, kv := range args {
		parts := strings.Split(kv, ":")
		if len(parts) != 2 {
			continue
		}
		newMap[parts[0]] = parts[1]
	}
	return newMap
}

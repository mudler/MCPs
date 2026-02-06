package main

import (
	"os"
	"strings"
)

// isAdminMode checks if TODO_ADMIN_MODE environment variable is set to "true"
func isAdminMode() bool {
	return strings.ToLower(strings.TrimSpace(os.Getenv("TODO_ADMIN_MODE"))) == "true"
}

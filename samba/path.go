package main

import (
	"fmt"
	"path"
	"strings"
)

// cleanPath turns caller-supplied input into a share-relative slash path, or
// refuses it. This is the single choke point every tool goes through: as long
// as nothing reaches the SMB session without passing here, no request can
// address anything outside the configured share.
func cleanPath(input string) (string, error) {
	if strings.ContainsRune(input, 0) {
		return "", fmt.Errorf("path contains a NUL byte")
	}

	normalised := strings.ReplaceAll(input, `\`, "/")

	trimmed := strings.TrimSpace(normalised)
	if trimmed == "" || trimmed == "." || trimmed == "/" {
		return "", nil
	}

	if strings.HasPrefix(trimmed, "/") {
		return "", fmt.Errorf("path %q is absolute: paths are relative to the root of the share, for example models/model.gguf", input)
	}

	cleaned := path.Clean(trimmed)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("path %q escapes the share root", input)
	}
	if cleaned == "." {
		return "", nil
	}

	return cleaned, nil
}

// globMeta are the characters that make a search pattern a glob rather than a
// plain substring.
const globMeta = `*?[`

// matchName reports whether name matches pattern. Patterns containing glob
// metacharacters are matched as globs, anything else as a substring, because a
// caller searching for "qwen" means "name contains qwen" far more often than
// it means "name is exactly qwen". Matching is case-insensitive: SMB shares
// are, so a case-sensitive search would surprise.
func matchName(pattern, name string) (bool, error) {
	if pattern == "" {
		return true, nil
	}

	lowerPattern := strings.ToLower(pattern)
	lowerName := strings.ToLower(name)

	if !strings.ContainsAny(pattern, globMeta) {
		return strings.Contains(lowerName, lowerPattern), nil
	}

	matched, err := path.Match(lowerPattern, lowerName)
	if err != nil {
		return false, fmt.Errorf("invalid search pattern %q: %w", pattern, err)
	}
	return matched, nil
}

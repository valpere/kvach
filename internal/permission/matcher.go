package permission

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// MatchRule reports whether a permission rule matches the given tool call.
//
// Pattern syntax:
//
//   - Empty pattern matches any call to the named tool.
//   - "command-prefix:*" matches Bash tool calls where the command starts with
//     the prefix (e.g. "git:*" matches "git status", "git push", etc.).
//   - "domain:hostname" matches WebFetch tool calls to a specific hostname.
//   - "//path/glob" matches file tool calls (Read, Write, Edit, Glob, Grep)
//     where the file path matches the glob.
//   - "*" matches everything.
//
// toolName is compared case-insensitively. input is the decoded JSON tool
// arguments.
func MatchRule(rule Rule, toolName string, input map[string]any) bool {
	// Tool name must match.
	if !strings.EqualFold(rule.Tool, toolName) {
		return false
	}

	// Empty pattern = match any call to this tool.
	pattern := strings.TrimSpace(rule.Pattern)
	if pattern == "" || pattern == "*" {
		return true
	}

	switch strings.ToLower(toolName) {
	case "bash":
		return matchBashPattern(pattern, input)
	case "webfetch":
		return matchDomainPattern(pattern, input)
	case "read", "write", "edit", "glob", "grep", "multipatch":
		return matchPathPattern(pattern, input)
	default:
		// For unknown tools, treat pattern as a simple prefix match on
		// the string representation of the first input field.
		return matchGenericPattern(pattern, input)
	}
}

// matchBashPattern matches "prefix:*" against the "command" field.
// Example: pattern "git:*" matches command "git push origin main".
func matchBashPattern(pattern string, input map[string]any) bool {
	prefix, ok := strings.CutSuffix(pattern, ":*")
	if !ok {
		prefix = pattern
	}
	cmd, _ := inputString(input, "command")
	if cmd == "" {
		return false
	}
	// Match if command starts with prefix (possibly followed by a space).
	return cmd == prefix || strings.HasPrefix(cmd, prefix+" ") || strings.HasPrefix(cmd, prefix+"\t")
}

// matchDomainPattern matches "domain:hostname" against the "url" field.
func matchDomainPattern(pattern string, input map[string]any) bool {
	domain, ok := strings.CutPrefix(pattern, "domain:")
	if !ok {
		return false
	}
	url, _ := inputString(input, "url")
	if url == "" {
		return false
	}
	// Extract hostname from URL. Simple approach: find "://" then take until
	// next "/" or end.
	host := extractHost(url)
	return strings.EqualFold(host, domain) ||
		strings.HasSuffix(strings.ToLower(host), "."+strings.ToLower(domain))
}

// matchPathPattern matches "//glob" against file path fields.
func matchPathPattern(pattern string, input map[string]any) bool {
	glob, ok := strings.CutPrefix(pattern, "//")
	if !ok {
		glob = pattern
	}
	// Try common file path field names.
	for _, key := range []string{"filePath", "path", "pattern", "directory"} {
		val, _ := inputString(input, key)
		if val == "" {
			continue
		}
		matched, _ := filepath.Match(glob, val)
		if matched {
			return true
		}
		// Also try prefix match for directory-style globs ending in **.
		if strings.HasSuffix(glob, "**") {
			dir := strings.TrimSuffix(glob, "**")
			if strings.HasPrefix(val, dir) {
				return true
			}
		}
	}
	return false
}

// matchGenericPattern does a simple prefix check against the first string
// value found in input.
func matchGenericPattern(pattern string, input map[string]any) bool {
	for _, v := range input {
		if s, ok := v.(string); ok {
			if strings.HasPrefix(s, pattern) {
				return true
			}
		}
	}
	return false
}

// inputString extracts a string field from the decoded input map.
func inputString(input map[string]any, key string) (string, bool) {
	val, ok := input[key]
	if !ok {
		return "", false
	}
	switch v := val.(type) {
	case string:
		return v, true
	case json.Number:
		return v.String(), true
	default:
		return "", false
	}
}

// extractHost returns the hostname from a URL string.
func extractHost(url string) string {
	// Strip scheme.
	if i := strings.Index(url, "://"); i >= 0 {
		url = url[i+3:]
	}
	// Strip path (before stripping userinfo/port to avoid confusion with
	// slashes in paths).
	if i := strings.IndexByte(url, '/'); i >= 0 {
		url = url[:i]
	}
	// Strip userinfo (user:pass@).
	if i := strings.IndexByte(url, '@'); i >= 0 {
		url = url[i+1:]
	}
	// Strip port.
	if i := strings.LastIndexByte(url, ':'); i >= 0 {
		url = url[:i]
	}
	return url
}

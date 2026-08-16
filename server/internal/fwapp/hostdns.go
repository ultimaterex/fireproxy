package fwapp

import (
	"fmt"
	"strings"
)

// NormalizeHostDNS lowercases and validates a Firewalla local DNS hostname.
// Empty input clears the override. Allowed: a-z, 0-9, '-', '.'; max 63 chars.
func NormalizeHostDNS(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "", nil
	}
	if len(s) > 63 {
		return "", fmt.Errorf("hostname too long")
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '.' {
			continue
		}
		return "", fmt.Errorf("invalid hostname")
	}
	if strings.HasPrefix(s, ".") || strings.HasSuffix(s, ".") || strings.Contains(s, "..") {
		return "", fmt.Errorf("invalid hostname")
	}
	if strings.HasPrefix(s, "-") || strings.HasSuffix(s, "-") {
		return "", fmt.Errorf("invalid hostname")
	}
	return s, nil
}

package fwapp

import (
	"fmt"
	"strings"
	"unicode"
)

// NormalizeMAC returns AA:BB:CC:DD:EE:FF from common MAC spellings.
// If the input is not 12 hex digits, returns the trimmed uppercased string unchanged.
func NormalizeMAC(s string) string {
	var hex []byte
	for _, r := range strings.ToUpper(s) {
		if unicode.Is(unicode.ASCII_Hex_Digit, r) {
			hex = append(hex, byte(r))
		}
	}
	if len(hex) != 12 {
		return strings.ToUpper(strings.TrimSpace(s))
	}
	var b strings.Builder
	for i := 0; i < 12; i += 2 {
		if i > 0 {
			b.WriteByte(':')
		}
		b.Write(hex[i : i+2])
	}
	return b.String()
}

// ParseMAC normalizes and requires a 6-octet address.
// Only hex digits and common separators (whitespace, :, -, .) are allowed.
func ParseMAC(s string) (string, error) {
	for _, r := range s {
		if unicode.Is(unicode.ASCII_Hex_Digit, r) || unicode.IsSpace(r) || r == ':' || r == '-' || r == '.' {
			continue
		}
		return "", fmt.Errorf("invalid mac")
	}
	n := NormalizeMAC(s)
	if len(strings.ReplaceAll(n, ":", "")) != 12 || strings.Count(n, ":") != 5 {
		return "", fmt.Errorf("invalid mac")
	}
	return n, nil
}

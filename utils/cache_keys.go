package utils

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
)

// ShortHash10 returns a short 10-hex digest for identifying long strings in keys.
func ShortHash10(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:])[:10]
}

// sanitizeKeyPart normalizes a key segment: trims, lowers, replaces spaces, and bounds length.
func sanitizeKeyPart(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	// If looks like URL, reduce to domain part to stabilize keys
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.Contains(s, "/") || strings.Contains(s, ":") {
		s = SanitizeDomain(s)
	}
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "_")
	// Bound excessive length to keep keys short in Redis
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

// BuildCacheKey joins parts with ':' after sanitizing each part consistently.
func BuildCacheKey(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	sanitized := make([]string, 0, len(parts))
	for _, p := range parts {
		sanitized = append(sanitized, sanitizeKeyPart(p))
	}
	return strings.Join(sanitized, ":")
}

package helpers

import "strings"

// ExtractHostname extracts hostname
func ExtractHostname(host string) string {
	index := strings.LastIndex(host, ":")
	if index == -1 {
		return host
	}
	return host[0:index]
}

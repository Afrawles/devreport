package main

import (
	"fmt"
	"strings"
)

// parseCommaList splits a comma-separated string and trims whitespace
func parseCommaList(input string) []string {
	if input == "" {
		return []string{}
	}
	
	parts := strings.Split(input, ",")
	result := make([]string, len(parts))
	for i, part := range parts {
		trimmed := strings.TrimSpace(part)
		if strings.Contains(trimmed, "|") {
			bullets := strings.Split(trimmed, "|")
			var bulletList []string
			for _, bullet := range bullets {
				trimmedBullet := strings.TrimSpace(bullet)
				if trimmedBullet != "" {
					bulletList = append(bulletList, fmt.Sprintf("• %s", trimmedBullet))
				}
			}
			result[i] = strings.Join(bulletList, "\n")
		} else {
			result[i] = trimmed
		}
	}
	return result
}


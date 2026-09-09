// Package config provides SMP server configuration with YAML parsing.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all server configuration.
type Config struct {
	Listen           string
	Retention        int
	CFMPath          string
	CFMMaxMB         int
	CFMIntv          int
	Verbose          bool
	BasePermission   []string
	SpecialPermission map[string][]string
	Blacklist        []string
	Whitelist        []string
	WhitelistEnabled bool
	BlacklistEnabled bool
}

// Default returns a Config with sensible defaults.
func Default() Config {
	return Config{
		Listen:           "0.0.0.0:9932",
		Retention:        10,
		CFMPath:          "./cfm-storage",
		CFMMaxMB:         100,
		CFMIntv:          10,
		SpecialPermission: make(map[string][]string),
	}
}

// Load parses a key=value or YAML config file into the Config.
func (c *Config) Load(path string) error {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return c.parseYAML(string(data))
}

// parseYAML implements a minimal YAML parser for the server config.
func (c *Config) parseYAML(content string) error {
	lines := strings.Split(content, "\n")
	currentSection := ""
	currentMapKey := ""

	for _, line := range lines {
		// Remove comments
		if idx := strings.Index(line, "#"); idx != -1 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Section headers like [base-permission]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.TrimSpace(line[1 : len(line)-1])
			currentMapKey = ""
			continue
		}

		// List items like - item
		if strings.HasPrefix(line, "- ") {
			item := strings.TrimSpace(line[2:])
			switch currentSection {
			case "base-permission":
				c.BasePermission = append(c.BasePermission, item)
			case "blacklist":
				c.Blacklist = append(c.Blacklist, item)
			case "whitelist":
				c.Whitelist = append(c.Whitelist, item)
			case "special-permission":
				if currentMapKey != "" {
					if c.SpecialPermission[currentMapKey] == nil {
						c.SpecialPermission[currentMapKey] = []string{}
					}
					c.SpecialPermission[currentMapKey] = append(c.SpecialPermission[currentMapKey], item)
				}
			}
			continue
		}

		// Key-value pairs
		if idx := strings.Index(line, ":"); idx != -1 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])

			switch key {
			case "listen":
				c.Listen = value
			case "retention":
				fmt.Sscanf(value, "%d", &c.Retention)
			case "cfm_path", "cfm_dir":
				c.CFMPath = value
			case "cfm_max_mb":
				fmt.Sscanf(value, "%d", &c.CFMMaxMB)
			case "cfm_interval":
				fmt.Sscanf(value, "%d", &c.CFMIntv)
			case "verbose":
				c.Verbose = value == "true"
			case "whitelist-enabled":
				c.WhitelistEnabled = value == "true"
			case "blacklist-enabled":
				c.BlacklistEnabled = value == "true"
			case "base-permission":
				c.BasePermission = strings.Split(value, ",")
				currentSection = "base-permission"
			case "blacklist":
				c.Blacklist = strings.Split(value, ",")
				currentSection = "blacklist"
			case "whitelist":
				c.Whitelist = strings.Split(value, ",")
				currentSection = "whitelist"
			case "special-permission":
				// Sub-key like admin: read,write,delete
				if value != "" {
					c.SpecialPermission[key] = strings.Split(value, ",")
				} else {
					currentSection = "special-permission"
					currentMapKey = key
				}
			default:
				// Check if it's a sub-key under special-permission
				if currentSection == "special-permission" && value != "" {
					if c.SpecialPermission[key] == nil {
						c.SpecialPermission[key] = []string{}
					}
					c.SpecialPermission[key] = strings.Split(value, ",")
				}
			}
		}
	}

	// Clean up empty strings in slices
	c.BasePermission = cleanSlice(c.BasePermission)
	c.Blacklist = cleanSlice(c.Blacklist)
	c.Whitelist = cleanSlice(c.Whitelist)
	for k, v := range c.SpecialPermission {
		c.SpecialPermission[k] = cleanSlice(v)
	}

	return nil
}

func cleanSlice(s []string) []string {
	result := []string{}
	for _, item := range s {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

// CheckPermission checks if a user has a specific permission.
func (c *Config) CheckPermission(username string, permission string) bool {
	// Check special permissions first
	if perms, ok := c.SpecialPermission[username]; ok {
		for _, p := range perms {
			if p == permission {
				return true
			}
		}
	}

	// Check base permissions
	for _, p := range c.BasePermission {
		if p == permission {
			return true
		}
	}

	return false
}

// IsBlacklisted checks if a user is blacklisted.
func (c *Config) IsBlacklisted(username string) bool {
	if !c.BlacklistEnabled {
		return false
	}
	for _, u := range c.Blacklist {
		if u == username {
			return true
		}
	}
	return false
}

// IsWhitelisted checks if a user is whitelisted.
func (c *Config) IsWhitelisted(username string) bool {
	if !c.WhitelistEnabled {
		return false
	}
	for _, u := range c.Whitelist {
		if u == username {
			return true
		}
	}
	return false
}

// HasAccess checks if a user has access (not blacklisted, or whitelisted).
func (c *Config) HasAccess(username string) bool {
	// If whitelist is enabled, only whitelisted users have access
	if c.WhitelistEnabled {
		return c.IsWhitelisted(username)
	}
	// If blacklist is enabled, blacklisted users don't have access
	if c.BlacklistEnabled {
		return !c.IsBlacklisted(username)
	}
	// No restrictions
	return true
}

// GetPermissions returns the permissions for a user.
func (c *Config) GetPermissions(username string) []string {
	perms := []string{}
	// Add base permissions
	for _, p := range c.BasePermission {
		if p != "" {
			perms = append(perms, p)
		}
	}
	// Override with special permissions
	if special, ok := c.SpecialPermission[username]; ok {
		perms = append(perms, special...)
	}
	return perms
}

// ParseAddr is not used in config but kept for compatibility.
var _ = strconv.Itoa

// Package config provides SMP server configuration with YAML parsing.
package config

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
)

// Config holds all server configuration.
type Config struct {
	Listen            string
	Retention         int
	CFMPath           string
	CFMMaxMB          int
	CFMIntv           int
	Verbose           bool
	ConfMode          string
	BasePermission    []string
	SpecialPermission map[string][]string
	Blacklist         []string
	Whitelist         []string
	WhitelistEnabled  bool
	BlacklistEnabled  bool
}

// Default returns a Config with sensible defaults.
func Default() Config {
	return Config{
		Listen:            "0.0.0.0:9932",
		Retention:         10,
		CFMPath:           "./cfm-storage",
		CFMMaxMB:          100,
		CFMIntv:           10,
		ConfMode:          "blacklist",
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
			item := unquote(strings.TrimSpace(line[2:]))
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
			value := unquote(strings.TrimSpace(line[idx+1:]))

			switch key {
			case "conf-mode", "conf_mode", "config-mode":
				c.ConfMode = value
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
				// Inline form: "special-permission: admin: read,write,delete" --
				// the part after the first colon is "username: perm1,perm2".
				if value != "" {
					if parts := strings.SplitN(value, ":", 2); len(parts) == 2 {
						c.SpecialPermission[strings.TrimSpace(parts[0])] =
							strings.Split(parts[1], ",")
					}
				} else {
					// Block form: this line only opens the section; a nested
					// "admin:" line below it names the user.
					currentSection = "special-permission"
					currentMapKey = ""
				}
			default:
				// A sub-key under special-permission: either "admin: read,write"
				// or a bare "admin:" introducing a "- read" block below it. The
				// bare form must set currentMapKey, otherwise the "- " items
				// below it get attached to the section name instead of the user.
				if currentSection == "special-permission" {
					if value == "" {
						currentMapKey = key
					} else {
						c.SpecialPermission[key] = strings.Split(value, ",")
					}
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

// unquote strips one pair of surrounding double or single quotes, so that
// conf-mode: "whitelist" compares equal to whitelist.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
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

// NormalizeConfMode maps the configured conf-mode to "whitelist" or "blacklist".
// Anything that is not explicitly "whitelist" counts as blacklist, so a missing
// or misspelled value denies nothing.
func (c *Config) NormalizeConfMode() string {
	if c.ConfMode == "whitelist" {
		return "whitelist"
	}
	return "blacklist"
}

// CommandAllowed reports whether username may run the command, plus a short
// reason when it may not. The effective patterns are base-permission plus
// special-permission[username]; each is a glob matched against the command
// string, and conf-mode decides whether a match grants or denies access:
//
//	conf-mode: "whitelist"  ->  allowed only if some pattern matches
//	conf-mode: "blacklist"  ->  allowed only if no pattern matches
func (c *Config) CommandAllowed(username, command string) (bool, string) {
	matched := ""
	for _, p := range c.GetPermissions(username) {
		if ok, _ := path.Match(p, command); ok {
			matched = p
			break
		}
	}
	if c.NormalizeConfMode() == "whitelist" {
		if matched != "" {
			return true, ""
		}
		return false, "no permission matches " + command
	}
	if matched != "" {
		return false, "permission " + matched + " denies " + command
	}
	return true, ""
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

// GetPermissions returns the permissions for a user. Special permissions add to
// the base set rather than replacing it, and duplicates are dropped.
func (c *Config) GetPermissions(username string) []string {
	perms := []string{}
	seen := make(map[string]bool)
	for _, p := range c.BasePermission {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		perms = append(perms, p)
	}
	if special, ok := c.SpecialPermission[username]; ok {
		for _, p := range special {
			if p == "" || seen[p] {
				continue
			}
			seen[p] = true
			perms = append(perms, p)
		}
	}
	return perms
}

// ParseAddr is not used in config but kept for compatibility.
var _ = strconv.Itoa

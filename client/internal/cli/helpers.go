package cli

import (
    "math/rand"
    "net"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "time"
)

// GenerateID creates a random message ID.
func GenerateID() uint64 {
    r := rand.New(rand.NewSource(time.Now().UnixNano()))
    return uint64(r.Int63())
}

// localAddr returns the local IP address.
func localAddr() string {
    conn, err := net.Dial("udp", "8.8.8.8:80")
    if err == nil {
        defer conn.Close()
        return conn.LocalAddr().String()
    }
    return "127.0.0.1"
}

// GetTokenTail extracts the token tail from env, token file, or config.
func GetTokenTail() string {
    // 1. Check SMP_TOKEN env var
    token := os.Getenv("SMP_TOKEN")
    if token != "" {
        return ExtractTokenTail(token)
    }

    // 2. Check ~/.smp-token file
    home, err := os.UserHomeDir()
    if err == nil {
        tokenFile := filepath.Join(home, ".smp-token")
        if data, err := os.ReadFile(tokenFile); err == nil {
            token = strings.TrimSpace(string(data))
            if token != "" {
                return ExtractTokenTail(token)
            }
        }
    }

    return ""
}

// ParseDuration converts a duration string to minutes.
func ParseDuration(d string) int {
    if strings.HasSuffix(d, "h") {
        h, _ := strconv.Atoi(strings.TrimSuffix(d, "h"))
        return h * 60
    }
    if strings.HasSuffix(d, "m") {
        m, _ := strconv.Atoi(strings.TrimSuffix(d, "m"))
        return m
    }
    m, _ := strconv.Atoi(d)
    return m
}

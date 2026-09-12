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

// localAddr returns this machine's own IPv4 address, used for the client
// address field in the message SubHead. It walks the interface table rather
// than dialing an external address, so it works on air-gapped hosts and does
// not depend on how traffic to some fixed outside IP happens to be routed.
// Falls back to loopback when no up, non-loopback IPv4 interface exists.
func localAddr() string {
    const fallback = "127.0.0.1"
    ifaces, err := net.Interfaces()
    if err != nil {
        return fallback
    }
    for _, iface := range ifaces {
        if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
            continue
        }
        addrs, err := iface.Addrs()
        if err != nil {
            continue
        }
        for _, a := range addrs {
            ipNet, ok := a.(*net.IPNet)
            if !ok || ipNet.IP.IsLoopback() {
                continue
            }
            if ip := ipNet.IP.To4(); ip != nil {
                return ip.String()
            }
        }
    }
    return fallback
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

package cli

import (
    "flag"
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

// Create registers a user on a server.
func Create(args []string) error {
    fs := flag.NewFlagSet("create", flag.ExitOnError)
    force := fs.Bool("force", false, "Force create (uses http)")
    useHTTP := fs.Bool("http", false, "Use smp-over-http")
    useSSH := fs.Bool("ssh", false, "Use smp-over-ssh")
    useTCP := fs.Bool("tcp", false, "Use smp-over-tcp")
    useSMP := fs.Bool("smp", false, "Use native smp (default)")

    fs.Parse(args)

    var protocolFlags []string
    if *force {
        protocolFlags = append(protocolFlags, "--force")
    }
    if *useHTTP {
        protocolFlags = append(protocolFlags, "--http")
    }
    if *useSSH {
        protocolFlags = append(protocolFlags, "--ssh")
    }
    if *useTCP {
        protocolFlags = append(protocolFlags, "--tcp")
    }
    if *useSMP {
        protocolFlags = append(protocolFlags, "--smp")
    }

    protocol := SelectProtocol(*force, protocolFlags)

    if fs.NArg() < 2 {
        return fmt.Errorf("usage: smp create [-flags] <smp@server_ip> <smp@username>")
    }

    serverAddr := fs.Arg(0)
    userAddr := fs.Arg(1)

    ip, err := ParseAddr(serverAddr)
    if err != nil {
        return err
    }
    user, err := ParseUser(userAddr)
    if err != nil {
        return err
    }

    tokenTail := GetTokenTail()
    if tokenTail == "" {
        return fmt.Errorf("token not set. Use --token or set SMP_TOKEN")
    }

    data := fmt.Sprintf("user=%s", user)
    msg, err := AssembleMessage(tokenTail, GenerateID(), "_create", []byte(data), nil, "")
    if err != nil {
        return fmt.Errorf("assemble: %w", err)
    }

    transport, err := Connect(protocol, ip, 30)
    if err != nil {
        return err
    }
    defer transport.Close()

    response, err := transport.Send(msg)
    if err != nil {
        return err
    }

    respStr := string(response)
    DisplayResponse(response)

    // Parse and save token if created successfully
    if strings.HasPrefix(respStr, "ok:") {
        parts := strings.Split(respStr, ":")
        for _, part := range parts {
            if strings.HasPrefix(part, "token=") {
                token := strings.TrimPrefix(part, "token=")
                home, _ := os.UserHomeDir()
                tokenFile := filepath.Join(home, ".smp-token")
                if err := os.WriteFile(tokenFile, []byte(token), 0600); err == nil {
                    fmt.Printf("Token saved to %s\n", tokenFile)
                }
                break
            }
        }
    }

    return nil
}

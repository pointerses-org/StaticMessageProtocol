package cli

import (
    "flag"
    "fmt"
)

// Pull retrieves messages from a server for a user.
func Pull(args []string) error {
    fs := flag.NewFlagSet("pull", flag.ExitOnError)
    upstream := fs.Bool("u", false, "Set upstream tracking")
    force := fs.Bool("force", false, "Force pull (uses http)")
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

    cfg := LoadConfig()

    var serverAddr, userAddr string
    if fs.NArg() >= 1 {
        serverAddr = fs.Arg(0)
        if *upstream {
            cfg.Server = serverAddr
            cfg.Protocol = protocol
        }
    } else if cfg.Server != "" {
        serverAddr = cfg.Server
    } else {
        return fmt.Errorf("usage: smp pull [-flags] <smp@server_ip> <smp@username>")
    }

    if fs.NArg() >= 2 {
        userAddr = fs.Arg(1)
        if *upstream {
            cfg.User = userAddr
            cfg.SaveConfig()
        }
    } else if cfg.User != "" {
        userAddr = cfg.User
    } else {
        return fmt.Errorf("usage: smp pull [-flags] <smp@server_ip> <smp@username>")
    }

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

    query := fmt.Sprintf("limit=100&offset=0&route=smp@%s", user)
    msg, err := AssembleMessage(tokenTail, GenerateID(), "_pull", []byte(query), nil, "")
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

    DisplayResponse(response)
    return nil
}

package cli

import (
    "flag"
    "fmt"
)

// List lists all users on a server.
func List(args []string) error {
    fs := flag.NewFlagSet("list", flag.ExitOnError)
    force := fs.Bool("force", false, "Force list (uses http)")
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
    var serverAddr string
    if fs.NArg() >= 1 {
        serverAddr = fs.Arg(0)
    } else if cfg.Server != "" {
        serverAddr = cfg.Server
    } else {
        return fmt.Errorf("usage: smp list [-flags] <smp@server_ip>")
    }

    ip, err := ParseAddr(serverAddr)
    if err != nil {
        return err
    }

    tokenTail := GetTokenTail()
    if tokenTail == "" {
        return fmt.Errorf("token not set. Use --token or set SMP_TOKEN")
    }

    msg, err := AssembleMessage(tokenTail, GenerateID(), "_list", []byte{}, nil, "")
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

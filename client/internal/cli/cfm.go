package cli

import (
    "flag"
    "fmt"
    "os"
)

// CFM handles CFM subcommands.
func CFM(args []string) error {
    if len(args) < 1 {
        return fmt.Errorf("usage: smp cfm <install|push> [-flags] [...]")
    }

    switch args[0] {
    case "install":
        return cfmInstall(args[1:])
    case "push":
        return cfmPush(args[1:])
    default:
        return fmt.Errorf("unknown cfm command: %s", args[0])
    }
}

func cfmInstall(args []string) error {
    fs := flag.NewFlagSet("cfm install", flag.ExitOnError)
    force := fs.Bool("force", false, "Bypass cache, re-extract from cfm.dll")

    fs.Parse(args)

    if *force {
        fmt.Println("Force re-installing CFM...")
    } else {
        fmt.Println("CFM already installed. Use --force to reinstall.")
    }
    return nil
}

func cfmPush(args []string) error {
    fs := flag.NewFlagSet("cfm push", flag.ExitOnError)
    force := fs.Bool("force", false, "Force push (uses http)")
    useHTTP := fs.Bool("http", false, "Use smp-over-http")
    useSSH := fs.Bool("ssh", false, "Use smp-over-ssh")
    useTCP := fs.Bool("tcp", false, "Use smp-over-tcp")
    useSMP := fs.Bool("smp", false, "Use native smp (default)")

    fs.Parse(args)

    if fs.NArg() < 2 {
        return fmt.Errorf("usage: smp cfm push [-flags] <smp@server_ip> <filename>")
    }

    serverAddr := fs.Arg(0)
    filePath := fs.Arg(1)

    ip, err := ParseAddr(serverAddr)
    if err != nil {
        return err
    }

    info, err := os.Stat(filePath)
    if err != nil {
        return fmt.Errorf("stat %s: %w", filePath, err)
    }
    if info.IsDir() {
        return fmt.Errorf("cannot upload directory: %s", filePath)
    }

    data, err := os.ReadFile(filePath)
    if err != nil {
        return fmt.Errorf("read %s: %w", filePath, err)
    }

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

    tokenTail := GetTokenTail()
    if tokenTail == "" {
        return fmt.Errorf("token not set. Use --token or set SMP_TOKEN")
    }

    msgID := GenerateID() | (1 << 63)

    msg, err := AssembleMessage(tokenTail, msgID, "_cfm_upload", data, nil, "expire=1440&public=false")
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

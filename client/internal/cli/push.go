package cli

import (
    "flag"
    "fmt"
    "io"
    "os"
    "time"
)

// Push pushes a message to a server.
func Push(args []string) error {
    fs := flag.NewFlagSet("push", flag.ExitOnError)
    upstream := fs.Bool("u", false, "Set upstream tracking")
    force := fs.Bool("force", false, "Force push (uses http)")
    useHTTP := fs.Bool("http", false, "Use smp-over-http")
    useSSH := fs.Bool("ssh", false, "Use smp-over-ssh")
    useTCP := fs.Bool("tcp", false, "Use smp-over-tcp")
    useSMP := fs.Bool("smp", false, "Use native smp (default)")
    contextID := fs.Uint64("context", 0, "Context ID to reference")

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
        if *upstream {
            cfg.Server = serverAddr
            cfg.Protocol = protocol
            cfg.SaveConfig()
        }
    } else if cfg.Server != "" {
        serverAddr = cfg.Server
    } else {
        return fmt.Errorf("usage: smp push [-flags] <smp@server_ip>")
    }

    ip, err := ParseAddr(serverAddr)
    if err != nil {
        return err
    }

    var userData []byte
    userData, _ = io.ReadAll(os.Stdin)
    if userData == nil {
        userData = []byte{}
    }

    tokenTail := GetTokenTail()
    if tokenTail == "" {
        return fmt.Errorf("token not set. Use --token or set SMP_TOKEN")
    }

    var ctxPairs []ContextPair
    if *contextID != 0 {
        ctxPairs = append(ctxPairs, ContextPair{RefID: *contextID, Timestamp: uint32(time.Now().Unix())})
    }

    msgID := GenerateID()

    msg, err := AssembleMessage(tokenTail, msgID, serverAddr, userData, ctxPairs, "")
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

    cfg.ContextID = msgID
    cfg.SaveConfig()

    DisplayResponse(response)
    return nil
}

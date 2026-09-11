package cli

import (
    "flag"
    "fmt"
    "strconv"
    "strings"
)

// WatchContext queries the most recent message ID.
func WatchContext(args []string) error {
    fs := flag.NewFlagSet("watch-context", flag.ExitOnError)
    useHTTP := fs.Bool("http", false, "Use smp-over-http")
    useSSH := fs.Bool("ssh", false, "Use smp-over-ssh")
    useTCP := fs.Bool("tcp", false, "Use smp-over-tcp")
    useSMP := fs.Bool("smp", false, "Use native smp (default)")

    fs.Parse(args)

    var protocolFlags []string
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

    protocol := SelectProtocol(false, protocolFlags)

    cfg := LoadConfig()

    if cfg.Server == "" {
        return fmt.Errorf("no upstream set. Use smp push -u <smp@server> or smp pull -u <smp@server> <smp@user> first")
    }

    ip, err := ParseAddr(cfg.Server)
    if err != nil {
        return err
    }

    if cfg.Protocol != "" {
        protocol = cfg.Protocol
    }

    tokenTail := GetTokenTail()
    if tokenTail == "" {
        return fmt.Errorf("token not set. Use --token or set SMP_TOKEN")
    }

    msg, err := AssembleMessage(tokenTail, GenerateID(), "_watch", []byte{}, nil, "")
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

    // The server answers "last_id=0x<msgID>" for the caller's own inbox. That is
    // the source of truth -- cfg.ContextID is the last ID pushed *from here*,
    // which is a different question.
    resp := strings.TrimSpace(string(response))
    if strings.HasPrefix(resp, "last_id=0x") {
        if id, err := strconv.ParseUint(resp[len("last_id=0x"):], 16, 64); err == nil {
            fmt.Printf("Last message ID: %x\n", id)
            return nil
        }
    }
    DisplayResponse(response)
    return nil
}

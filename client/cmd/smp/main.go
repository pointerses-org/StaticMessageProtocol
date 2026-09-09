// smp: SMP CLI client entry point.
//
// Usage:
//   smp push [-flags] <smp@server_ip>
//   smp pull [-flags] <smp@server_ip> <smp@username>
//   smp create [-flags] <smp@server_ip> <smp@username>
//   smp list [-flags] <smp@server_ip>
//   smp watch-context [-flags]
//   smp cfm install [-flags]
//   smp cfm push [-flags] <smp@server_ip> <filename>
//
// Build: go build -o smp.exe ./cmd/smp/
package main

import (
	"fmt"
	"os"

	"smp-client/internal/cli"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "push":
		err = cli.Push(args)
	case "pull":
		err = cli.Pull(args)
	case "create":
		err = cli.Create(args)
	case "list":
		err = cli.List(args)
	case "watch-context":
		err = cli.WatchContext(args)
	case "cfm":
		err = cli.CFM(args)
	case "help", "-h", "--help":
		printUsage()
		return
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`SMP CLI - Static Message Protocol Client

Usage:
  smp push [-flags] <smp@server_ip>
    Push a message to a server.
    Flags: -u, --force, --http, --ssh, --tcp, --smp, --context <id>

  smp pull [-flags] <smp@server_ip> <smp@username>
    Pull messages from a server for a user.
    Flags: -u, --force, --http, --ssh, --tcp, --smp

  smp create [-flags] <smp@server_ip> <smp@username>
    Register a user on a server.
    Flags: --http, --ssh, --tcp, --smp

  smp list [-flags] <smp@server_ip>
    List all users on a server.
    Flags: --force, --http, --ssh, --tcp, --smp

  smp watch-context [-flags]
    Query the most recent message ID.
    Flags: --http, --ssh, --tcp, --smp

  smp cfm install [-flags]
    Install CFM DLL.
    Flags: --force (bypass cache)

  smp cfm push [-flags] <smp@server_ip> <filename>
    Upload a file to CFM.
    Flags: --force, --http, --ssh, --tcp, --smp

Environment:
  SMP_TOKEN  Authentication token (smpt128-<32hex>)

Examples:
  smp push -u smp@192.168.1.100
  smp pull -u smp@192.168.1.100 smp@alice
  smp create smp@192.168.1.100 smp@bob
  smp list smp@192.168.1.100
  smp watch-context --http
  smp cfm push smp@192.168.1.100 largefile.zip
`)
}

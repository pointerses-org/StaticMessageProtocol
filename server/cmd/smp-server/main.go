// smp-server: SMP server daemon entry point.
//
// Build: go build -o smp-server.exe ./cmd/smp-server/
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"smp-server/internal/config"
	smpserver "smp-server/internal/server"
)

func main() {
	cfg := config.Default()

	configPath := flag.String("config", "", "Path to config file")
	flag.StringVar(&cfg.Listen, "listen", cfg.Listen, "TCP listen address")
	flag.IntVar(&cfg.Retention, "retention", cfg.Retention, "Message retention (minutes)")
	flag.StringVar(&cfg.CFMPath, "cfm-path", cfg.CFMPath, "CFM storage directory")
	flag.IntVar(&cfg.CFMMaxMB, "cfm-max", cfg.CFMMaxMB, "Max CFM file size (MB)")
	flag.IntVar(&cfg.CFMIntv, "cfm-interval", cfg.CFMIntv, "CFM cleanup interval (minutes)")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "Verbose logging")
	flag.Parse()

	if *configPath != "" {
		if err := cfg.Load(*configPath); err != nil {
			log.Fatalf("Config: %v", err)
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	srv := smpserver.New(cfg)
	if err := srv.Start(); err != nil {
		log.Fatalf("Start: %v", err)
	}

	<-sigCh
	srv.Stop()
}

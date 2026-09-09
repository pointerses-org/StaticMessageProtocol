// Package server provides the SMP server lifecycle.
package server

import (
	"log"
	"net"
	"sync"
	"time"

	"smp-server/internal/auth"
	"smp-server/internal/cache"
	"smp-server/internal/config"
	"smp-server/internal/handler"
	"smp-server/internal/store"
)

// Server is the SMP server.
type Server struct {
	cfg        config.Config
	msgStore   *store.MessageStore
	cfmStore   *store.CFMStore
	idemCache  *cache.IdempotencyCache
	tokenStore *auth.TokenStore
	accountStore *store.AccountStore
	listener   net.Listener
	wg         sync.WaitGroup
	done       chan struct{}
}

// New creates a new server with the given config.
func New(cfg config.Config) *Server {
	return &Server{
		cfg:          cfg,
		msgStore:     store.NewMessageStore(cfg.Retention),
		cfmStore:     store.NewCFMStore(cfg.CFMPath, cfg.CFMMaxMB),
		idemCache:    cache.NewIdempotencyCache(10000),
		tokenStore:   auth.NewTokenStore(),
		accountStore: store.NewAccountStore(),
		done:         make(chan struct{}),
	}
}

// Start begins listening and processing connections.
func (s *Server) Start() error {
	var err error
	s.listener, err = net.Listen("tcp", s.cfg.Listen)
	if err != nil {
		return err
	}

	log.Printf("SMP Server on %s (retention=%dm, cfm=%s/%dMB)",
		s.cfg.Listen, s.cfg.Retention, s.cfg.CFMPath, s.cfg.CFMMaxMB)

	s.wg.Add(1)
	go s.cleanupLoop()

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Stop shuts down the server gracefully.
func (s *Server) Stop() {
	close(s.done)
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
	log.Println("SMP Server stopped")
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	h := handler.NewHandler(s.cfg, s.msgStore, s.cfmStore, s.idemCache, s.tokenStore, s.accountStore)
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return
			default:
				log.Printf("Accept: %v", err)
				continue
			}
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			handler.HandleConnection(conn, h, s.cfg.Verbose)
		}()
	}
}

func (s *Server) cleanupLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Duration(s.cfg.CFMIntv) * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			n1 := s.msgStore.Cleanup()
			n2 := s.cfmStore.Cleanup()
			if n1 > 0 || n2 > 0 {
				log.Printf("Cleanup: -%d msgs, -%d cfm", n1, n2)
			}
		}
	}
}

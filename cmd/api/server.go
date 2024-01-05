package api

import (
	"fmt"
	"log"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/seanburman/kaw/pkg/connection"
)

var manager = &serverManager{
	servers: make(map[string]*Server),
}

type (
	serverManager struct {
		mu      sync.Mutex
		servers map[string]*Server
	}
	Server struct {
		app             *echo.Echo
		cfg             serverConfig
		msgs            chan []byte
		onNewConnection func(c *connection.Connection)
		ConnectionPool  *connection.Pool
	}
	serverConfig struct {
		Port string
		Path string
		Key  string
	}
)

func NewServer(cfg serverConfig) (*Server, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("server path cannot root")
	}
	manager.mu.Lock()
	for port, sv := range manager.servers {
		if port == cfg.Port {
			return nil, fmt.Errorf("server with port %s already exists", cfg.Port)
		}
		if sv.cfg.Path == cfg.Path {
			return nil, fmt.Errorf("server with path %s already exists", cfg.Path)
		}
		if sv.cfg.Key == cfg.Key {
			return nil, fmt.Errorf("server with key %s already exists", cfg.Key)
		}
	}

	server := &Server{
		app:            echo.New(),
		cfg:            cfg,
		msgs:           make(chan []byte, 16),
		ConnectionPool: connection.NewPool(),
	}

	manager.servers[server.cfg.Port] = server
	manager.mu.Unlock()

	server.app.Static("", "public")
	ws := server.app.Group(cfg.Path + "/ws")
	ws.GET("/subscribe", server.handleSubscribe)

	return server, nil
}

func NewConfig(port, path, key string) serverConfig {
	return serverConfig{
		port, path, key,
	}
}

func (s *Server) Config() serverConfig {
	return s.cfg
}

func (s *Server) ListenAndServe() {
	go s.app.Start(s.cfg.Port)
}

func (s *Server) Shutdown() {
	manager.mu.Lock()
	delete(manager.servers, s.cfg.Port)
	manager.mu.Unlock()
	err := s.app.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) handleSubscribe(ctx echo.Context) error {
	conn, err := connection.NewConnection(ctx)
	if err != nil {
		return err
	}
	if err := s.ConnectionPool.AddConnection(conn); err != nil {
		return err
	}
	if s.onNewConnection != nil {
		s.onNewConnection(conn)
	}
	conn.Listen()
	return nil
}

func (s *Server) SetOnNewConnection(callback func(c *connection.Connection)) {
	s.onNewConnection = callback
}

func (s *Server) Publish(msg interface{}) {
	for _, conn := range s.ConnectionPool.Connections() {
		select {
		case conn.Messages <- msg:
		default:
			conn.Close()
		}
	}
}

// func (s *Server) handleCommands(ctx echo.Context) error {

// }

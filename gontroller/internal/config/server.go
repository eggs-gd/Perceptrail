package config

import (
	"fmt"
	"net"
	"strconv"
)

// Server: the `server` section — the HTTP service
type Server struct {
	// Listen host. Default: all interfaces
	Host string `yaml:"host"`
	// Default: 1323
	Port int `yaml:"port"`
	// CORS: origins allowed to call the API (the client in dev runs on another
	// port). Default: any
	AllowedOrigins []string `yaml:"allowed_origins"`
}

const defaultPort = 1323

// Addr is the listen address (IPv6 hosts are bracketed)
func (s Server) Addr() string {
	return net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
}

// resolve fills the unset fields and checks the rest
func (s *Server) resolve() error {
	if s.Port == 0 {
		s.Port = defaultPort
	}
	if s.Port < 0 || s.Port > 65535 {
		return fmt.Errorf("server port %d out of range", s.Port)
	}
	if len(s.AllowedOrigins) == 0 {
		s.AllowedOrigins = []string{"*"}
	}
	return nil
}

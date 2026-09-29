package client

import (
	"fmt"
	"net"
	"strconv"
)

const defaultPort = 1323

// ServerConfig is the `server` section of the config.
type ServerConfig struct {
	// Listen host. Default: all interfaces
	Host string `yaml:"host"`
	// Default: 1323
	Port int `yaml:"port"`
	// CORS: origins allowed to call the API (the client in dev runs on another
	// port). Default: any
	AllowedOrigins []string `yaml:"allowed_origins"`
}

// withDefaults fills unset fields and checks the rest.
func (c ServerConfig) withDefaults() (ServerConfig, error) {
	if c.Port == 0 {
		c.Port = defaultPort
	}
	if c.Port < 0 || c.Port > 65535 {
		return c, fmt.Errorf("server port %d out of range", c.Port)
	}
	if len(c.AllowedOrigins) == 0 {
		c.AllowedOrigins = []string{"*"}
	}
	return c, nil
}

// Addr is the listen address (IPv6 hosts are bracketed).
func (c ServerConfig) Addr() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

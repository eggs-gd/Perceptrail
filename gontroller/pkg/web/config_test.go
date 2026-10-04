package web

import "testing"

func TestServerConfig(t *testing.T) {
	c, err := ServerConfig{}.withDefaults()
	if err != nil || c.Addr() != ":1323" || c.AllowedOrigins[0] != "*" {
		t.Errorf("defaults: %+v, %v", c, err)
	}

	c, err = ServerConfig{Host: "::1", Port: 8080, AllowedOrigins: []string{"http://localhost:5173"}}.withDefaults()
	if err != nil || c.Addr() != "[::1]:8080" || c.AllowedOrigins[0] != "http://localhost:5173" {
		t.Errorf("explicit: %+v, %v", c, err)
	}

	if _, err := (ServerConfig{Port: 70000}).withDefaults(); err == nil {
		t.Error("port 70000: want an error")
	}
}

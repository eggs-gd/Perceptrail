package logger

// RegisterService registers a new service with specified color
func (l *Logger) RegisterService(name string, color string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.services == nil {
		l.services = make(map[string]ServiceConfig)
	}

	l.services[name] = ServiceConfig{
		Name:    name,
		Color:   color,
		Enabled: true,
	}
}

// EnableService enables logging for specified service
func (l *Logger) EnableService(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if cfg, exists := l.services[name]; exists {
		cfg.Enabled = true
		l.services[name] = cfg
	}
}

// DisableService disables logging for specified service
func (l *Logger) DisableService(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if cfg, exists := l.services[name]; exists {
		cfg.Enabled = false
		l.services[name] = cfg
	}
}

// isServiceEnabled checks if service logging is enabled
func (l *Logger) isServiceEnabled(name string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if cfg, exists := l.services[name]; exists {
		return cfg.Enabled
	}
	return true // default enabled
}

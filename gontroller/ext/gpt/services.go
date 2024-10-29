package gpt

type ImporterService struct {
	// Параметри для імпортеру
}

func NewImporterService(ctx *AppContext) *ImporterService {
	return &ImporterService{}
}

func (s *ImporterService) Start() {
	// Логіка імпорту
}

func (s *ImporterService) Stop() {
	// Логіка зупинки імпорту
}

type WebServerService struct {
	// Параметри для веб-сервера
}

func NewWebServerService(ctx *AppContext) *WebServerService {
	return &WebServerService{}
}

func (s *WebServerService) Start() {
	// Логіка запуску веб-сервера
}

func (s *WebServerService) Stop() {
	// Логіка зупинки веб-сервера
}

type MaintenanceService struct {
	// Параметри для веб-сервера
}

func NewMaintenanceService(ctx *AppContext) *MaintenanceService {
	return &MaintenanceService{}
}

func (s *MaintenanceService) Start() {
	// Логіка запуску веб-сервера
}

func (s *MaintenanceService) Stop() {
	// Логіка зупинки веб-сервера
}

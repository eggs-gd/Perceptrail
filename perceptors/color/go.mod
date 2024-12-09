module perceptrail/perseptors/color

go 1.23.2

require perceptrail/perceptors v0.0.0

require (
	go.uber.org/multierr v1.10.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	perceptrail/logger v0.0.0 // indirect
)

replace (
	perceptrail/logger => ../../perceplib/logger
	perceptrail/perceptors => ../../perceplib/perceptors
)

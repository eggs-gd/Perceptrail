module perceptrail/api

go 1.23.2

require (
	perceptrail/chain v0.0.0
	perceptrail/logger v0.0.0
)

require (
	go.uber.org/multierr v1.10.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
)

replace (
	perceptrail/api => ../../perceplib/api
	perceptrail/chain => ../../perceplib/chain
	perceptrail/exiftool => ../../perceplib/exiftool
	perceptrail/logger => ../../perceplib/logger
)

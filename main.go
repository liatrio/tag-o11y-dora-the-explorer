package main

import (
	"encoding/json"

	"go.uber.org/zap"
	// Import your OTEL packages here for instrumentation.
	// The default packages are for manual instrumentation, but you can use
	// auto-instrumentation packages to capture communication at the edge.
	// For more information see https://opentelemetry.io/docs/languages/go/getting-started/
	//"go.opentelemetry.io/otel"
	//"go.opentelemetry.io/otel/trace"
	//"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
)

var (
	logger *zap.Logger
)

func init() {
	var err error

	rawJSON := []byte(`{
        "level": "debug",
        "encoding": "json",
        "outputPaths": ["stdout"],
        "errorOutputPaths": ["stderr"],
        "initialFields": {"service": "dora-the-explorer"},
        "encoderConfig": {
            "messageKey": "message",
            "levelKey": "level",
            "levelEncoder": "lowercase"
            }
        }
    `)

	var cfg zap.Config
	if err = json.Unmarshal(rawJSON, &cfg); err != nil {
		panic(err)
	}

	logger = zap.Must(cfg.Build())
	defer func() {
		if err := logger.Sync(); err != nil {
			return
		}
	}()
}

func main() {
	logger.Info("Hello World!")
}

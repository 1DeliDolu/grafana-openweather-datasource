package instrumentation

import (
	"context"

	"github.com/grafana/grafana-plugin-sdk-go/backend/tracing"
)

// Instrumentation holds all instrumentation tools
type Instrumentation struct {
	Logger  *Logger
	Metrics *Metrics
	Tracing *TracingHelper
}

// New creates a new instrumentation instance
func New(pluginID string) *Instrumentation {
	return &Instrumentation{
		Logger:  NewLogger(),
		Metrics: NewMetrics(pluginID),
		Tracing: NewTracingHelper(tracing.DefaultTracer()),
	}
}

// WithContext adds context to logging and tracing
func (i *Instrumentation) WithContext(ctx context.Context) *Instrumentation {
	return &Instrumentation{
		Logger:  i.Logger.FromContext(ctx),
		Metrics: i.Metrics,
		Tracing: i.Tracing,
	}
}

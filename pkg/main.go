package main

import (
	"os"

	"github.com/1DeliDolu/grafana-openweather-datasource/pkg/plugin"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/tracing"
	"go.opentelemetry.io/otel/attribute"
)

func main() {
	if err := datasource.Manage("grafana-openweather-datasource", plugin.NewDatasource, datasource.ManageOpts{
		TracingOpts: tracing.Opts{
			CustomAttributes: []attribute.KeyValue{
				attribute.String("plugin", "grafana-openweather-datasource"),
			},
		},
	}); err != nil {
		backend.Logger.Error(err.Error())
		os.Exit(1)
	}
}

package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/1DeliDolu/grafana-openweather-datasource/pkg/models" /* meine repository */
	"github.com/1DeliDolu/grafana-openweather-datasource/pkg/plugin/instrumentation"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/backend/tracing"
	"go.opentelemetry.io/otel/attribute"
)

// Make sure Datasource implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin in
// runtime. In this example datasource instance implements backend.QueryDataHandler,
// backend.CheckHealthHandler interfaces. Plugin should not implement all these
// interfaces - only those which are required for a particular task.
var (
	_ backend.QueryDataHandler      = (*Datasource)(nil)
	_ backend.CheckHealthHandler    = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

// Datasource struct with baseURL and logger
type Datasource struct {
	baseURL string
	logger  log.Logger
	tracer  *instrumentation.TracingHelper
	metrics *instrumentation.Metrics
}

// NewDatasourceInstance creates a new datasource instance.
func NewDatasource(_ context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	config, err := models.LoadPluginSettings(settings)
	if err != nil {
		return nil, err
	}

	// Use config.BaseURL or fall back to default if not set
	if config.Path == "" {
		config.Path = "https://api.openweathermap.org/data/2.5/forecast"
	}

	logger := log.New().With("datasource", settings.Name)

	logger.Info("Creating new datasource instance", "baseURL", config.Path)

	return &Datasource{
		baseURL: config.Path,
		logger:  logger,
		tracer:  instrumentation.NewTracingHelper(tracing.DefaultTracer()),
		metrics: instrumentation.NewMetrics("openweather"),
	}, nil
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created. As soon as datasource settings change detected by SDK old datasource instance will
// be disposed and a new one will be created using NewSampleDatasource factory function.
func (d *Datasource) Dispose() {
	d.logger.Info("Disposing datasource instance")
	// Clean up datasource instance resources.
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifier).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).
func (d *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	start := time.Now()
	response, err := d.queryData(ctx, req)
	d.metrics.RecordRequest("query_data", start, err)
	return response, err
}

func (d *Datasource) queryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	// Create response struct
	response := backend.NewQueryDataResponse()

	// Log request details
	d.logger.Info("Processing query data request",
		"context", ctx,
		"request", req.PluginContext.DataSourceInstanceSettings.Name,
		"queries", len(req.Queries))

	// Create span for request tracing
	ctx, span := d.tracer.StartSpan(ctx, "queryData",
		attribute.Int("query_count", len(req.Queries)))
	defer span.End()

	// Process each query in the request
	for _, q := range req.Queries {
		d.logger.Debug("Processing individual query",
			"refID", q.RefID,
			"timeRange", q.TimeRange)

		// Create query-specific span
		_, querySpan := d.tracer.StartSpan(ctx, "process_query",
			attribute.String("query_ref_id", q.RefID))

		// Process query here
		res := d.processQuery(ctx, req.PluginContext, q)
		response.Responses[q.RefID] = res

		querySpan.End()
	}

	return response, nil
}

// Helper method to process individual queries
func (d *Datasource) processQuery(ctx context.Context, pCtx backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	_ = pCtx
	_ = ctx
	var response backend.DataResponse

	// Add your query processing logic here
	// This replaces the previous unused query method

	d.logger.Info("Processing query",
		"refID", query.RefID,
		"timeRange", query.TimeRange)

	return response
}

func (d *Datasource) GetHistoricalWeather(city string, apiKey string, qm queryModel) ([]WeatherResponse, error) {
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -5)

	url := fmt.Sprintf("%s?q=%s&appid=%s&units=metric&start=%d&end=%d", d.baseURL, city, apiKey, startDate.Unix(), endDate.Unix())

	d.logger.Info("Fetching historical weather data", "url", url)

	resp, err := http.Get(url)
	if err != nil {
		d.logger.Error("Failed to fetch historical weather data", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		d.logger.Error("Failed to read response body", "error", err)
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		d.logger.Error("API request failed", "statusCode", resp.StatusCode)
		return nil, fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
	}

	var weatherResponse WeatherResponse
	err = json.Unmarshal(body, &weatherResponse)
	if err != nil {
		d.logger.Error("Failed to unmarshal response body", "error", err)
		return nil, err
	}

	weatherData := make([]WeatherResponse, len(weatherResponse.List))
	for i, forecast := range weatherResponse.List {
		weatherData[i] = weatherResponse
		d.logger.Info("Weather data retrieved for timestamp", "dt", forecast.Dt)
	}

	return weatherData, nil
}

func (d *Datasource) CheckHealth(_ context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	res := &backend.CheckHealthResult{}
	config, err := models.LoadPluginSettings(*req.PluginContext.DataSourceInstanceSettings)

	logger := d.logger.FromContext(context.Background())

	if err != nil {
		logger.Error("Failed to load settings", "error", err)
		res.Status = backend.HealthStatusError
		res.Message = "Unable to load settings"
		return res, nil
	}

	if config.Secrets.ApiKey == "" {
		logger.Error("API key is missing")
		res.Status = backend.HealthStatusError
		res.Message = "API key is missing"
		return res, nil
	}

	logger.Info("Health check successful")
	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Data source is working",
	}, nil
}

package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/1DeliDolu/grafana-openweather-datasource/pkg/models" /* meine repository */
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/backend/tracing"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
	// create response struct
	response := backend.NewQueryDataResponse()

	// loop over queries and execute them individually.
	for _, q := range req.Queries {
		res := d.query(ctx, req.PluginContext, q)

		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = res
	}

	return response, nil
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

func (d *Datasource) query(ctx context.Context, pCtx backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	ctx, span := tracing.DefaultTracer().Start(
		ctx,
		"query processing",
		trace.WithAttributes(
			attribute.String("query.ref_id", query.RefID),
			attribute.String("query.type", query.QueryType),
			attribute.Int64("query.max_data_points", query.MaxDataPoints),
			attribute.Int64("query.interval_ms", query.Interval.Milliseconds()),
			attribute.Int64("query.time_range.from", query.TimeRange.From.Unix()),
			attribute.Int64("query.time_range.to", query.TimeRange.To.Unix()),
		),
	)
	defer span.End()

	var response backend.DataResponse
	var qm queryModel

	err := json.Unmarshal(query.JSON, &qm)
	if err != nil {
		d.logger.Error("Failed to unmarshal query JSON", "error", err)
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	// City boş ise işlemi durdur
	if qm.City == "" {
		d.logger.Warn("City is empty in query")
		return response
	}

	config, err := models.LoadPluginSettings(*pCtx.DataSourceInstanceSettings)
	if err != nil {
		d.logger.Error("Failed to load plugin settings", "error", err)
		response.Error = err
		return response
	}

	url := fmt.Sprintf("%s?q=%s&appid=%s&units=%s", d.baseURL, qm.City, config.Secrets.ApiKey, qm.Units)

	d.logger.Info("Fetching weather data", "url", url)

	resp, err := http.Get(url)
	if err != nil {
		d.logger.Error("Failed to fetch weather data", "error", err)
		response.Error = err
		return response
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		d.logger.Error("Failed to read response body", "error", err)
		response.Error = err
		return response
	}

	if resp.StatusCode != http.StatusOK {
		d.logger.Error("API request failed", "statusCode", resp.StatusCode)
		response.Error = fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
		return response
	}

	var weatherResponse WeatherResponse
	err = json.Unmarshal(body, &weatherResponse)
	if err != nil {
		d.logger.Error("Failed to unmarshal response body", "error", err)
		response.Error = err
		return response
	}

	// Add more detailed structured logging
	logger := d.logger.FromContext(ctx)
	logger.Info("Processing weather query",
		"refId", qm.RefID,
		"city", qm.City,
		"mainParameter", qm.MainParameter,
		"subParameter", qm.SubParameter,
		"units", qm.Units)

	// Create the frame with metadata immediately
	frame := data.NewFrame("weather_data").
		SetMeta(&data.FrameMeta{
			Type: data.FrameTypeTimeSeriesMulti,
		})

	// Extract the requested parameter values
	times := make([]time.Time, len(weatherResponse.List))
	values := make([]float64, len(weatherResponse.List))

	for i, item := range weatherResponse.List {
		times[i] = time.Unix(item.Dt, 0)

		// Select the value based on the query parameters
		switch qm.MainParameter {
		case "main":
			switch qm.SubParameter {
			case "temp":
				values[i] = item.Main.Temp
			case "feels_like":
				values[i] = item.Main.FeelsLike
			case "temp_min":
				values[i] = item.Main.TempMin
			case "temp_max":
				values[i] = item.Main.TempMax
			case "pressure":
				values[i] = float64(item.Main.Pressure)
			case "humidity":
				values[i] = float64(item.Main.Humidity)
			}
		case "wind":
			switch qm.SubParameter {
			case "speed":
				values[i] = item.Wind.Speed
			case "deg":
				values[i] = item.Wind.Deg
			case "gust":
				values[i] = item.Wind.Gust
			}
		case "clouds":
			values[i] = float64(item.Clouds.All)
		case "rain":
			values[i] = item.Rain.ThreeHour
		}
	}

	// Add logging with frame details
	logger.Info("Weather data processed",
		"refId", qm.RefID,
		"city", qm.City,
		"mainParameter", qm.MainParameter,
		"subParameter", qm.SubParameter,
		"dataPoints", len(values),
		"firstValue", fmt.Sprintf("%.2f", values[0]),
		"lastValue", fmt.Sprintf("%.2f", values[len(values)-1]),
		"frameLength", len(frame.Fields))

	// Add fields to frame with labels
	frame.Fields = append(frame.Fields,
		data.NewField("time", map[string]string{
			"city":         qm.City,
			"parameter":    qm.MainParameter,
			"subparameter": qm.SubParameter,
		}, times),
		data.NewField("value", map[string]string{
			"city":         qm.City,
			"parameter":    qm.MainParameter,
			"subparameter": qm.SubParameter,
		}, values))

	// Debug log for frame details
	logger.Debug("Frame details",
		"frameName", frame.Name,
		"fieldCount", len(frame.Fields),
		"rowCount", frame.Fields[0].Len())

	response.Frames = append(response.Frames, frame)
	return response
}

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// datasource configuration page which allows users to verify that
// a datasource is working as expected.
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

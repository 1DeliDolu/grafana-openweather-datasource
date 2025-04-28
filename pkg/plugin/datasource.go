package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/1DeliDolu/grafana-openweather-datasource/pkg/models" /* meine repository */
	"github.com/1DeliDolu/grafana-openweather-datasource/pkg/plugin/instrumentation"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/backend/tracing"
	"github.com/grafana/grafana-plugin-sdk-go/data"
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

	logger := log.New().With("datasource", settings.Name)

	// Make sure we have a proper base URL for the API
	baseURL := config.Path
	if baseURL == "" {
		baseURL = "https://api.openweathermap.org/data/2.5/forecast"
	} else if !strings.HasPrefix(baseURL, "http") {
		// If no protocol is specified, add https
		baseURL = "https://" + baseURL
	}

	// Check if API key exists
	if config.Secrets.ApiKey == "" {
		logger.Error("No API key provided in datasource configuration")
	} else {
		// Don't log the actual API key
		logger.Info("API key found in configuration")
	}

	logger.Info("Creating new datasource instance", "baseURL", baseURL)

	return &Datasource{
		baseURL: baseURL,
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
func (d *Datasource) processQuery(_ context.Context, pCtx backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	var response backend.DataResponse
	var qm queryModel

	// Decode the query JSON into our queryModel
	err := json.Unmarshal(query.JSON, &qm)
	if err != nil {
		d.logger.Error("Failed to parse query", "error", err)
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	d.logger.Info("Processing query",
		"refID", query.RefID,
		"timeRange", query.TimeRange,
		"queryModel", qm)

	// Get API key from datasource settings
	config, err := models.LoadPluginSettings(*pCtx.DataSourceInstanceSettings)
	if err != nil {
		d.logger.Error("Failed to load settings", "error", err)
		return backend.ErrDataResponse(backend.StatusBadRequest, "Unable to load datasource settings")
	}

	// Check if city is provided
	if qm.City == "" {
		d.logger.Error("City is not provided in the query")
		return backend.ErrDataResponse(backend.StatusBadRequest, "City is required")
	}

	// Fetch weather data
	weatherData, err := d.GetHistoricalWeather(qm.City, config.Secrets.ApiKey, qm)
	if err != nil {
		d.logger.Error("Failed to fetch weather data", "error", err)
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("Failed to fetch weather data: %v", err.Error()))
	}

	// Convert the weather data to frames
	frame, err := d.createDataFrames(weatherData, qm)
	if err != nil {
		d.logger.Error("Failed to create frames", "error", err)
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("Failed to create frames: %v", err.Error()))
	}

	// Add the frame to the response
	response.Frames = append(response.Frames, frame)
	d.logger.Info("Successfully processed query", "framesCount", len(response.Frames))

	return response
}

// Function to create data frames from the weather response
func (d *Datasource) createDataFrames(weatherResponses []WeatherResponse, qm queryModel) (*data.Frame, error) {
	if len(weatherResponses) == 0 || len(weatherResponses[0].List) == 0 {
		return nil, fmt.Errorf("no weather data available")
	}

	// Create a new frame for the weather data
	frame := data.NewFrame("weather")

	// Add time field
	var times []time.Time
	var values []float64
	var descriptions []string

	// Extract data from the weather response
	for _, item := range weatherResponses[0].List {
		timestamp := time.Unix(item.Dt, 0)
		times = append(times, timestamp)

		// Extract values based on mainParameter and subParameter
		var value float64

		switch qm.Metric {
		case "main":
			switch qm.Format {
			case "temp":
				value = item.Main.Temp
			case "feels_like":
				value = item.Main.FeelsLike
			case "temp_min":
				value = item.Main.TempMin
			case "temp_max":
				value = item.Main.TempMax
			case "pressure":
				value = item.Main.Pressure
			case "sea_level":
				value = item.Main.SeaLevel
			case "grnd_level":
				value = item.Main.GrndLevel
			case "humidity":
				value = item.Main.Humidity
			default:
				value = item.Main.Temp
			}
		case "wind":
			switch qm.Format {
			case "speed":
				value = item.Wind.Speed
			case "deg":
				value = item.Wind.Deg
			case "gust":
				value = item.Wind.Gust
			default:
				value = item.Wind.Speed
			}
		case "clouds":
			value = item.Clouds.All
		case "rain":
			if item.Rain != nil {
				value = item.Rain.ThreeH
			}
		default:
			value = item.Main.Temp
		}

		values = append(values, value)

		if len(item.Weather) > 0 {
			descriptions = append(descriptions, item.Weather[0].Description)
		} else {
			descriptions = append(descriptions, "")
		}
	}

	// Add fields to the frame
	frame.Fields = append(frame.Fields,
		data.NewField("time", nil, times),
		data.NewField(qm.Format, nil, values),
		data.NewField("description", nil, descriptions),
	)

	// Add city name and selected parameter as labels
	frame.Name = weatherResponses[0].City.Name
	frame.Meta = &data.FrameMeta{
		Custom: map[string]interface{}{
			"city":      weatherResponses[0].City.Name,
			"parameter": qm.Metric + "." + qm.Format,
		},
	}

	d.logger.Info("Created data frame",
		"frameSize", len(times),
		"cityName", weatherResponses[0].City.Name,
		"parameter", qm.Metric+"."+qm.Format)

	return frame, nil
}

func (d *Datasource) GetHistoricalWeather(city string, apiKey string, qm queryModel) ([]WeatherResponse, error) {
	// Validate API key
	if apiKey == "" {
		d.logger.Error("API key is missing")
		return nil, fmt.Errorf("missing API key: please add a valid OpenWeather API key in the datasource configuration")
	}

	// Fix base URL if needed
	baseURL := d.baseURL
	if !strings.HasPrefix(baseURL, "http") {
		baseURL = "https://api.openweathermap.org/data/2.5"
	}

	// Use proper format for OpenWeatherMap API URL
	url := fmt.Sprintf("%s?q=%s&appid=%s&units=metric", baseURL, city, apiKey)

	d.logger.Info("Fetching weather data",
		"city", city,
		"metric", qm.Metric,
		"baseURL", baseURL)

	// Create a new HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		d.logger.Error("Error creating request", "error", err)
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Add additional request headers
	req.Header.Add("Accept", "application/json")

	d.logger.Info("Sending request to OpenWeather API", "url_without_key", strings.Replace(url, apiKey, "API_KEY_HIDDEN", 1))
	resp, err := client.Do(req)
	if err != nil {
		d.logger.Error("Error making request", "error", err)
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		d.logger.Error("Error reading response", "error", err)
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	// Enhanced error handling
	if resp.StatusCode != http.StatusOK {
		errorMsg := string(body)
		d.logger.Error("API returned error",
			"status", resp.StatusCode,
			"body", errorMsg)

		// Check specific error codes
		if resp.StatusCode == 401 {
			return nil, fmt.Errorf("authentication failed: invalid API key (401). Please verify your API key is correct and active")
		} else if resp.StatusCode == 404 {
			return nil, fmt.Errorf("city not found: %s (404)", city)
		} else if resp.StatusCode == 429 {
			return nil, fmt.Errorf("API rate limit exceeded (429). Please check your subscription plan")
		}

		return nil, fmt.Errorf("API request failed with status code: %d - %s", resp.StatusCode, errorMsg)
	}

	var weatherResponse WeatherResponse
	err = json.Unmarshal(body, &weatherResponse)
	if err != nil {
		d.logger.Error("Error unmarshalling response", "error", err, "body", string(body))
		return nil, fmt.Errorf("error unmarshalling response: %w", err)
	}

	// Validate response
	if weatherResponse.Cod != "200" {
		d.logger.Error("API returned error", "code", weatherResponse.Cod, "message", weatherResponse.Message)
		return nil, fmt.Errorf("API returned error code: %s", weatherResponse.Cod)
	}

	if len(weatherResponse.List) == 0 {
		d.logger.Error("API returned no data")
		return nil, fmt.Errorf("API returned no weather data")
	}

	// Return the single weather response in an array
	weatherData := []WeatherResponse{weatherResponse}
	d.logger.Info("Weather data retrieved successfully",
		"city", weatherResponse.City.Name,
		"items", len(weatherResponse.List))

	return weatherData, nil
}

func (d *Datasource) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	logger := d.logger.FromContext(ctx)

	// Load configuration
	config, err := models.LoadPluginSettings(*req.PluginContext.DataSourceInstanceSettings)
	if err != nil {
		logger.Error("Failed to load settings", "error", err)
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Unable to load settings: " + err.Error(),
		}, nil
	}

	// Check if API key exists
	if config.Secrets.ApiKey == "" {
		logger.Error("API key is missing")
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "API key is missing. Please configure a valid OpenWeather API key",
		}, nil
	}

	// Test connection with a simple request
	testCity := "London" // Using a well-known city for the test

	// Fix base URL if needed
	baseURL := d.baseURL
	if !strings.HasPrefix(baseURL, "http") {
		baseURL = "https://api.openweathermap.org/data/2.5/forecast"
	}

	url := fmt.Sprintf("%s?q=%s&appid=%s&units=metric", baseURL, testCity, config.Secrets.ApiKey)

	// Create a client with short timeout for health check
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	logger.Info("Testing API connection", "url", strings.Replace(url, config.Secrets.ApiKey, "API_KEY_HIDDEN", 1))

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger.Error("Failed to create request", "error", err)
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Failed to create test request: " + err.Error(),
		}, nil
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		logger.Error("Failed to connect to API", "error", err)
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Failed to connect to OpenWeather API: " + err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		logger.Error("API test failed", "status", resp.StatusCode, "body", string(body))

		if resp.StatusCode == 401 {
			return &backend.CheckHealthResult{
				Status:  backend.HealthStatusError,
				Message: "Authentication failed: Invalid API key. Please check your API key in the datasource configuration.",
			}, nil
		}

		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: fmt.Sprintf("API returned error: %d - %s", resp.StatusCode, string(body)),
		}, nil
	}

	logger.Info("Health check successful")
	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Successfully connected to OpenWeather API",
	}, nil
}

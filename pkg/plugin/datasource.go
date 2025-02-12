package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/1DeliDolu/grafana-openweather-datasource/pkg/models" /* meine repository */
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/data"
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

// Datasource struct with settings
type Datasource struct {
	settings *models.PluginSettings
}

// NewDatasource creates a new datasource instance.
func NewDatasource(_ context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	pluginSettings, err := models.LoadPluginSettings(settings)
	if err != nil {
		return nil, err
	}

	return &Datasource{
		settings: pluginSettings,
	}, nil
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created. As soon as datasource settings change detected by SDK old datasource instance will
// be disposed and a new one will be created using NewSampleDatasource factory function.
func (d *Datasource) Dispose() {
	// Clean up datasource instance resources.
}

type WeatherResponse struct {
	List []struct {
		Dt   int64 `json:"dt"`
		Main struct {
			Temp      float64 `json:"temp"`
			FeelsLike float64 `json:"feels_like"`
			TempMin   float64 `json:"temp_min"`
			TempMax   float64 `json:"temp_max"`
			Pressure  int     `json:"pressure"`
			SeaLevel  int     `json:"sea_level"`
			GrndLevel int     `json:"grnd_level"`
			Humidity  int     `json:"humidity"`
		} `json:"main"`
		Weather []struct {
			ID          int    `json:"id"`
			Main        string `json:"main"`
			Description string `json:"description"`
		} `json:"weather"`
		Wind struct {
			Speed float64 `json:"speed"`
			Deg   float64 `json:"deg"`
			Gust  float64 `json:"gust"`
		} `json:"wind"`
		Clouds struct {
			All int `json:"all"`
		} `json:"clouds"`
		Rain struct {
			ThreeHour float64 `json:"3h"`
		} `json:"rain"`
	} `json:"list"`
}

type WeatherData struct {
	Time        time.Time
	Temperature float64
	Description string
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

type queryModel struct {
	City          string `json:"city"`
	MainParameter string `json:"mainParameter"`
	SubParameter  string `json:"subParameter"`
	Units         string `json:"units"`
	Path          string `json:"url"`
}

func (d *Datasource) getWeatherData(city string, qm queryModel) ([]WeatherData, error) {

	url := fmt.Sprintf("%s?q=%s&appid=%s&units=metric",
		d.settings.Path,
		city,
		d.settings.Secrets.ApiKey,
	)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
	}

	var weatherResponse WeatherResponse
	err = json.Unmarshal(body, &weatherResponse)
	if err != nil {
		return nil, err
	}

	weatherData := make([]WeatherData, len(weatherResponse.List))
	for i, forecast := range weatherResponse.List {
		var value float64

		switch qm.MainParameter {
		case "main":
			switch qm.SubParameter {
			case "temp":
				value = forecast.Main.Temp
			case "feels_like":
				value = forecast.Main.FeelsLike
			case "temp_min":
				value = forecast.Main.TempMin
			case "temp_max":
				value = forecast.Main.TempMax
			case "pressure":
				value = float64(forecast.Main.Pressure)
			case "humidity":
				value = float64(forecast.Main.Humidity)
			}
		case "wind":
			switch qm.SubParameter {
			case "speed":
				value = forecast.Wind.Speed
			case "deg":
				value = forecast.Wind.Deg
			case "gust":
				value = forecast.Wind.Gust
			}
		case "clouds":
			value = float64(forecast.Clouds.All)
		case "rain":
			value = forecast.Rain.ThreeHour
		}

		weatherData[i] = WeatherData{
			Time:        time.Unix(forecast.Dt, 0),
			Temperature: value,
			Description: forecast.Weather[0].Description,
		}
	}

	return weatherData, nil
}

func (d *Datasource) writeWeatherDataToFile(data []WeatherData, city string) error {
	// Create logs directory if it doesn't exist
	logDir := filepath.Join("logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %v", err)
	}

	// Create or open the log file
	filename := filepath.Join(logDir, fmt.Sprintf("weather_%s_%s.txt",
		city,
		time.Now().Format("2006-01-02")))

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create log file: %v", err)
	}
	defer file.Close()

	// Write header
	_, err = file.WriteString(fmt.Sprintf("Weather data for %s\n", city))
	if err != nil {
		return fmt.Errorf("failed to write header: %v", err)
	}

	// Write data entries
	for _, entry := range data {
		_, err = file.WriteString(fmt.Sprintf(
			"Time: %s, Temperature: %.2f, Description: %s\n",
			entry.Time.Format("2006-01-02 15:04:05"),
			entry.Temperature,
			entry.Description))
		if err != nil {
			return fmt.Errorf("failed to write entry: %v", err)
		}
	}

	return nil
}

func (d *Datasource) query(_ context.Context, pCtx backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	var response backend.DataResponse
	var qm queryModel

	err := json.Unmarshal(query.JSON, &qm)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	// City boş ise işlemi durdur
	if qm.City == "" {
		return response
	}

	weatherData, err := d.getWeatherData(qm.City, qm)
	if err != nil {
		response.Error = err
		return response
	}

	// Write weather data to file
	if err := d.writeWeatherDataToFile(weatherData, qm.City); err != nil {
		backend.Logger.Error("Failed to write weather data to file", "error", err)
	}

	frame := data.NewFrame("response")

	// Extract the requested parameter values based on mainParameter and subParameter
	times := make([]time.Time, len(weatherData))
	values := make([]float64, len(weatherData))

	for i, item := range weatherData {
		times[i] = item.Time
		values[i] = item.Temperature
	}

	// Frame oluştururken etiketleri düzgün ayarla
	frame.Fields = append(frame.Fields,
		data.NewField("time", nil, times).SetConfig(&data.FieldConfig{
			DisplayName: "Time",
		}),
		data.NewField("value", nil, values).SetConfig(&data.FieldConfig{
			DisplayName: fmt.Sprintf("%s - %s", qm.MainParameter, qm.SubParameter),
		}),
	)

	response.Frames = append(response.Frames, frame)
	return response
}

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// datasource configuration page which allows users to verify that
// a datasource is working as expected.
func (d *Datasource) CheckHealth(_ context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	if d.settings.Secrets.ApiKey == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "API key is missing",
		}, nil
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Data source is working",
	}, nil
}

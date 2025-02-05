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

// Datasource struct with baseURL
type Datasource struct {
	baseURL string
}

// NewDatasource creates a new datasource instance.
func NewDatasource(_ context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	config, err := models.LoadPluginSettings(settings)
	if err != nil {
		return nil, err
	}

	// Use config.BaseURL or fall back to default if not set
	baseURL := config.BaseURL



	return &Datasource{
		baseURL: baseURL,
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
}

func (d *Datasource) GetHistoricalWeather(city string, apiKey string, qm queryModel) ([]WeatherData, error) {
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -5)

	url := fmt.Sprintf("%s?q=%s&appid=%s&units=metric&start=%d&end=%d", d.baseURL, city, apiKey, startDate.Unix(), endDate.Unix())

	resp, err := http.Get(url)
	if (err != nil) {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if (err != nil) {
		return nil, err
	}

	if (resp.StatusCode != http.StatusOK) {
		return nil, fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
	}

	var weatherResponse WeatherResponse
	err = json.Unmarshal(body, &weatherResponse)
	if (err != nil) {
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

func (d *Datasource) query(_ context.Context, pCtx backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	var response backend.DataResponse
	var qm queryModel

	err := json.Unmarshal(query.JSON, &qm)
	if (err != nil) {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	// City boş ise işlemi durdur
	if (qm.City == "") {
		return response
	}

	config, err := models.LoadPluginSettings(*pCtx.DataSourceInstanceSettings)
	if (err != nil) {
		response.Error = err
		return response
	}

	url := fmt.Sprintf("%s?q=%s&appid=%s&units=%s", d.baseURL, qm.City, config.Secrets.ApiKey, qm.Units)

	resp, err := http.Get(url)
	if (err != nil) {
		response.Error = err
		return response
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if (err != nil) {
		response.Error = err
		return response
	}

	if (resp.StatusCode != http.StatusOK) {
		response.Error = fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
		return response
	}

	var weatherResponse WeatherResponse
	err = json.Unmarshal(body, &weatherResponse)
	if (err != nil) {
		response.Error = err
		return response
	}

	frame := data.NewFrame("response")

	// Extract the requested parameter values based on mainParameter and subParameter
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
	res := &backend.CheckHealthResult{}
	config, err := models.LoadPluginSettings(*req.PluginContext.DataSourceInstanceSettings)

	if (err != nil) {
		res.Status = backend.HealthStatusError
		res.Message = "Unable to load settings"
		return res, nil
	}

	if (config.Secrets.ApiKey == "") {
		res.Status = backend.HealthStatusError
		res.Message = "API key is missing"
		return res, nil
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Data source is working",
	}, nil
}

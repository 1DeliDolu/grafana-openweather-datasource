package plugin

// Define the query model to parse the query JSON
type queryModel struct {
	City   string `json:"city"`
	Format string `json:"format"`
	Metric string `json:"metric"`
	Units  string `json:"units"`
}

// Weather API response structures
type WeatherResponse struct {
	Cod     string         `json:"cod"`
	Message float64        `json:"message"`
	Cnt     int            `json:"cnt"`
	List    []ForecastItem `json:"list"`
	City    CityInfo       `json:"city"`
}

type ForecastItem struct {
	Dt         int64       `json:"dt"`
	Main       MainWeather `json:"main"`
	Weather    []Weather   `json:"weather"`
	Clouds     Clouds      `json:"clouds"`
	Wind       Wind        `json:"wind"`
	Rain       *Rain       `json:"rain,omitempty"`
	Snow       *Snow       `json:"snow,omitempty"`
	Visibility int         `json:"visibility"`
	Pop        float64     `json:"pop"`
	Sys        Sys         `json:"sys"`
	DtTxt      string      `json:"dt_txt"`
}

type MainWeather struct {
	Temp      float64 `json:"temp"`
	FeelsLike float64 `json:"feels_like"`
	TempMin   float64 `json:"temp_min"`
	TempMax   float64 `json:"temp_max"`
	Pressure  float64 `json:"pressure"`
	SeaLevel  float64 `json:"sea_level"`
	GrndLevel float64 `json:"grnd_level"`
	Humidity  float64 `json:"humidity"`
	TempKf    float64 `json:"temp_kf"`
}

type Weather struct {
	ID          int    `json:"id"`
	Main        string `json:"main"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

type Clouds struct {
	All float64 `json:"all"`
}

type Wind struct {
	Speed float64 `json:"speed"`
	Deg   float64 `json:"deg"`
	Gust  float64 `json:"gust"`
}

type Rain struct {
	ThreeH float64 `json:"3h"`
}

type Snow struct {
	ThreeH float64 `json:"3h"`
}

type Sys struct {
	Pod string `json:"pod"`
}

type CityInfo struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Coord      Coord  `json:"coord"`
	Country    string `json:"country"`
	Population int    `json:"population"`
	Timezone   int    `json:"timezone"`
	Sunrise    int64  `json:"sunrise"`
	Sunset     int64  `json:"sunset"`
}

type Coord struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

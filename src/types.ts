import {  DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export interface MyQuery extends DataQuery {
  city: string;
  mainParameter: 'main' | 'wind' | 'clouds' | 'rain';
  subParameter: string;  // Keep as single string since backend expects one value
  units: 'standard' | 'metric' | 'imperial';
  queryText?: string;  // for template variables
}

export const DEFAULT_QUERY: Partial<MyQuery> = {
  city: 'Marburg',
  mainParameter: 'main',
  subParameter: 'temp',
  units: 'metric',
};

export interface DataPoint {
  Time: number;
  Value: number;
}

export interface DataSourceResponse {
  datapoints: DataPoint[];
  list: WeatherData[];
}

export interface WeatherData {
  dt: number;
  main: {
    temp: number;
    feels_like: number;
    pressure: number;
    humidity: number;
    temp_min: number;
    temp_max: number;
    sea_level?: number;
    grnd_level?: number;
  };
  wind: {
    speed: number;
    deg: number;
    gust?: number;
  };
  clouds: {
    all: number;
  };
  visibility: number;
  weather: Array<{
    id: number;
    main: string;
    description: string;
    icon: string;
  }>;
}

/**
 * These are options configured for each DataSource instance
 */
export interface MyDataSourceOptions extends DataSourceJsonData {
  url?: string;
}

/**
 * Value that is used in the backend, but never sent over HTTP to the frontend
 */
export interface MySecureJsonData {
  apiKey?: string;
}

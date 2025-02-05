import React from 'react';
import { InlineField, Stack, Select } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { DataSource } from '../datasource';
import { MyDataSourceOptions, MyQuery } from '../types';

type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

const unitsOptions: Array<SelectableValue<string>> = [
  { label: 'Standard', value: 'standard' },
  { label: 'Metric', value: 'metric' },
  { label: 'Imperial', value: 'imperial' },
];

const mainParameterOptions: Array<SelectableValue<string>> = [
  { label: 'Main Weather Data', value: 'main' },
  { label: 'Wind', value: 'wind' },
  { label: 'Clouds', value: 'clouds' },
  { label: 'Rain', value: 'rain' },
];

const subParameterOptions: { [key: string]: Array<SelectableValue<string>> } = {
  main: [
    { label: 'Temperature', value: 'temp' },
    { label: 'Feels Like', value: 'feels_like' },
    { label: 'Min Temperature', value: 'temp_min' },
    { label: 'Max Temperature', value: 'temp_max' },
    { label: 'Pressure', value: 'pressure' },
    { label: 'Sea Level', value: 'sea_level' },
    { label: 'Ground Level', value: 'grnd_level' },
    { label: 'Humidity', value: 'humidity' },
  ],
  wind: [
    { label: 'Speed', value: 'speed' },
    { label: 'Direction', value: 'deg' },
    { label: 'Gust', value: 'gust' },
  ],
  clouds: [
    { label: 'Cloudiness', value: 'all' },
  ],
  rain: [
    { label: '3h Rain Volume', value: '3h' },
  ],
};

export function QueryEditor({ query, onChange, onRunQuery }: Props) {
  const onCityNameChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const cityName = e.target.value;
    onChange({
      ...query,
      city: cityName,
      queryText: cityName, // Bu önemli, backend'e gönderilen query metni
    });
    // Hemen çalıştır
    onRunQuery();
  };

  const onMainParameterChange = (value: SelectableValue<string>) => {
    if (!value.value) {return};
    
    // Ana parametre değiştiğinde, alt parametreyi varsayılan değere ayarla
    const mainParam = value.value as MyQuery['mainParameter'];
    const defaultSubParam = subParameterOptions[mainParam][0].value as string;
    
    onChange({
      ...query,
      mainParameter: mainParam,
      subParameter: defaultSubParam,
    });
    onRunQuery();
  };

  const onSubParameterChange = (value: SelectableValue<string>) => {
    if (!value.value) {return};
    
    onChange({
      ...query,
      subParameter: value.value,
    });
    onRunQuery();
  };

  const onUnitsChange = (value: SelectableValue<string>) => {
    onChange({ ...query, units: value.value as 'standard' | 'metric' | 'imperial' });
    onRunQuery();
  };

  return (
    <Stack direction="column" gap={2}>
      <div>
        <InlineField label="City Name" labelWidth={20} tooltip="Enter city name (e.g., London,uk)">
          <input
            type="text"
            value={query.city || query.city || ''}
            onChange={onCityNameChange}
            placeholder="Enter city name (e.g., London,uk)"
            className="gf-form-input width-20"
          />
        </InlineField>
      </div>

      <div>
        <InlineField label="Main Parameter" labelWidth={20} tooltip="Select main weather parameter">
          <Select
            options={mainParameterOptions}
            value={mainParameterOptions.find(opt => opt.value === query.mainParameter)}
            onChange={onMainParameterChange}
            width={40}
          />
        </InlineField>
      </div>

      <div>
        <InlineField label="Parameters" labelWidth={20} tooltip="Select weather parameter">
          <Select
            options={subParameterOptions[query.mainParameter || 'main']}
            value={{
              label: subParameterOptions[query.mainParameter || 'main']
                .find(opt => opt.value === query.subParameter)?.label || '',
              value: query.subParameter || '',
            }}
            onChange={onSubParameterChange}
            isMulti={false}
            width={40}
          />
        </InlineField>
      </div>

      <div>
        <InlineField label="Units" labelWidth={20} tooltip="Select measurement units">
          <Select
            options={unitsOptions}
            value={query.units || 'metric'}
            onChange={onUnitsChange}
            width={40}
          />
        </InlineField>
      </div>
    </Stack>
  );
}

import { DataSourcePlugin } from '@grafana/data';
import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { MyQuery, MyDataSourceOptions, MySecureJsonData } from './types';

export const plugin = new DataSourcePlugin<DataSource, MyQuery, MyDataSourceOptions,MySecureJsonData>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);

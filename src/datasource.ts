import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { MyQuery, MyDataSourceOptions, DEFAULT_QUERY } from './types';

export class DataSource extends DataSourceWithBackend<MyQuery, MyDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<MyDataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_: CoreApp): Partial<MyQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: MyQuery, scopedVars: ScopedVars) {
    // Make sure all parameters are sent to the backend
    const city = getTemplateSrv().replace(query.city || '', scopedVars);
    const mainParameter = query.mainParameter || 'main';
    const subParameter = query.subParameter || 'temp';
    const units = query.units || 'metric';

    // Map the frontend parameters to backend expected format
    return {
      ...query,
      city: city,
      mainParameter: mainParameter, 
      subParameter: subParameter,
      units: units,
      queryText: city,
      // Include these fields for backend compatibility
      metric: mainParameter,
      format: subParameter
    };
  }

  filterQuery(query: MyQuery): boolean {
    // Only execute the query if a city has been provided
    return !!query.city;
  }
}

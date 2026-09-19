export type RackStatus = 'available' | 'reserved' | 'unavailable' | 'maintenance';

export interface RackUtilization {
  power_kw: number;
  airflow_cfm: number;
  rack_units: number;
  power_percent: number;
  airflow_percent: number;
  units_percent: number;
}

export interface Rack {
  id: number;
  zone_id: number;
  zone_code: string;
  rack_code: string;
  row_index: number;
  column_index: number;
  power_limit_kw: number;
  airflow_limit_cfm: number;
  rack_units: number;
  rack_status: RackStatus;
  version: number;
  utilization: RackUtilization;
}

export interface RackInput {
  zone_id: number;
  rack_code?: string;
  row_index: number;
  column_index: number;
  power_limit_kw: number;
  airflow_limit_cfm: number;
  rack_units: number;
  rack_status: RackStatus;
  version?: number;
}

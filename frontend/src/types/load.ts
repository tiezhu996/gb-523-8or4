export type LoadStatus = 'draft' | 'ready' | 'held' | 'placed';

export interface EquipmentLoad {
  id: number;
  name: string;
  power_kw: number;
  heat_kw: number;
  airflow_cfm: number;
  rack_units: number;
  redundancy_group: string;
  preferred_zone_id: number | null;
  preferred_zone_code?: string;
  load_status: LoadStatus;
  heat_ratio: number;
  validation_state: string;
}

export type LoadInput = Omit<EquipmentLoad, 'id' | 'preferred_zone_code' | 'heat_ratio' | 'validation_state'>;

export interface BatchValidation {
  total: number;
  valid_count: number;
  results: Array<{load_id: number; load_name: string; valid: boolean; issues: string[]}>;
}

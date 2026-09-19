export type ScenarioStatus = 'draft' | 'evaluating' | 'pending_review' | 'approved' | 'archived';

export interface ConstraintViolation {
  code: string;
  severity: 'critical' | 'warning';
  entity_type: string;
  entity_id: number;
  message: string;
  actual: number;
  limit: number;
  rack_id?: number;
  zone_id?: number;
}

export interface RackPin {
  load_id: number;
  rack_id: number;
}

export interface RackAssignment {
  load_id: number;
  load_name: string;
  rack_id: number;
  rack_code: string;
  zone_id: number;
  zone_code: string;
  power_kw: number;
  heat_kw: number;
  airflow_cfm: number;
  rack_units: number;
  placement_score: number;
  pinned: boolean;
  explanation: string[];
}

export interface ZoneThermalResult {
  zone_id: number;
  zone_code: string;
  assigned_heat_kw: number;
  neighbor_heat_kw: number;
  estimated_return_c: number;
  temperature_margin_c: number;
  cooling_margin_kw: number;
}

export interface LayoutScenario {
  id: number;
  name: string;
  scenario_status: ScenarioStatus;
  load_ids: number[];
  pins: RackPin[];
  assignments: RackAssignment[];
  zone_results: ZoneThermalResult[];
  violations: ConstraintViolation[];
  total_power_kw: number;
  peak_temp_c: number;
  score: number;
  version: number;
  algorithm_version: string;
  created_by: number;
  approved_by: number | null;
  has_critical_violation: boolean;
}

export interface PinConflict {
  code: string;
  severity: 'critical' | 'warning';
  entity_type: string;
  entity_id: number;
  message: string;
  actual: number;
  limit: number;
  rack_id?: number;
  zone_id?: number;
  load_name?: string;
  rack_code?: string;
  zone_code?: string;
}

export interface ScenarioComparison {
  left: LayoutScenario;
  right: LayoutScenario;
  score_delta: number;
  power_delta_kw: number;
  peak_temp_delta_c: number;
  summary: string[];
}

export type ZoneStatus = 'active' | 'constrained' | 'offline';

export interface ThermalZone {
  id: number;
  zone_code: string;
  name: string;
  cooling_capacity_kw: number;
  supply_temp_c: number;
  max_return_temp_c: number;
  adjacency: Record<string, number>;
  zone_status: ZoneStatus;
  rack_count: number;
  allocated_power_kw: number;
  capacity_utilization: number;
  temperature_headroom_c: number;
}

export interface ZoneInput {
  zone_code?: string;
  name: string;
  cooling_capacity_kw: number;
  supply_temp_c: number;
  max_return_temp_c: number;
  adjacency: Record<string, number>;
  zone_status: ZoneStatus;
}

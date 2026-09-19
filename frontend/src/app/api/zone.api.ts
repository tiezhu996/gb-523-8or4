import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { ThermalZone, ZoneInput } from '../../types/zone';
import { ApiClient, ApiPage } from './api-client';

@Injectable({providedIn: 'root'})
export class ZoneApi {
  private readonly api = inject(ApiClient);
  list(search = ''): Observable<ApiPage<ThermalZone>> { return this.api.get('/zones', {search, size: 200}); }
  get(id: number): Observable<ThermalZone> { return this.api.get(`/zones/${id}`); }
  create(input: ZoneInput & {zone_code: string}): Observable<ThermalZone> { return this.api.post('/zones', input); }
  update(id: number, input: ZoneInput): Observable<ThermalZone> { return this.api.put(`/zones/${id}`, input); }
}

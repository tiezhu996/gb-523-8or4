import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { Rack, RackInput } from '../../types/rack';
import { ApiClient, ApiPage } from './api-client';

@Injectable({providedIn: 'root'})
export class RackApi {
  private readonly api = inject(ApiClient);
  list(zoneId?: number): Observable<ApiPage<Rack>> { return this.api.get('/racks', {zone_id: zoneId, size: 200}); }
  create(input: RackInput & {rack_code: string}): Observable<Rack> { return this.api.post('/racks', input); }
  update(id: number, input: RackInput): Observable<Rack> { return this.api.put(`/racks/${id}`, input); }
}

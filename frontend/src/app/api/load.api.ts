import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { ApiClient, ApiPage } from './api-client';
import { BatchValidation, EquipmentLoad, LoadInput } from '../../types/load';

@Injectable({providedIn: 'root'})
export class LoadApi {
  private readonly api = inject(ApiClient);
  list(search = ''): Observable<ApiPage<EquipmentLoad>> { return this.api.get('/loads', {search, size: 200}); }
  create(input: LoadInput): Observable<EquipmentLoad> { return this.api.post('/loads', input); }
  update(id: number, input: LoadInput): Observable<EquipmentLoad> { return this.api.put(`/loads/${id}`, input); }
  validate(): Observable<BatchValidation> { return this.api.post('/loads/validate', {}); }
}

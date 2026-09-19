import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { AuditEvent } from '../../types/audit';
import { ApiClient, ApiPage } from './api-client';

@Injectable({providedIn: 'root'})
export class AuditApi {
  private readonly api = inject(ApiClient);
  list(entityType = ''): Observable<ApiPage<AuditEvent>> { return this.api.get('/audit-events', {entity_type: entityType, size: 200}); }
}

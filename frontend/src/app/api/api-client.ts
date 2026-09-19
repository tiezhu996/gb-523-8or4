import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { map, Observable } from 'rxjs';

export interface ApiEnvelope<T> {
  data: T;
  request_id: string;
}

export interface ApiPage<T> {
  items: T[];
  total: number;
  page: number;
  size: number;
}

@Injectable({providedIn: 'root'})
export class ApiClient {
  private readonly http = inject(HttpClient);
  private readonly base = '/api/v1';

  get<T>(path: string, query: Record<string, string | number | undefined> = {}): Observable<T> {
    let params = new HttpParams();
    Object.entries(query).forEach(([key, value]) => {
      if (value !== undefined && value !== '') {
        params = params.set(key, String(value));
      }
    });
    return this.http.get<ApiEnvelope<T>>(`${this.base}${path}`, {params}).pipe(map((response) => response.data));
  }

  post<T>(path: string, body: unknown): Observable<T> {
    return this.http.post<ApiEnvelope<T>>(`${this.base}${path}`, body).pipe(map((response) => response.data));
  }

  put<T>(path: string, body: unknown): Observable<T> {
    return this.http.put<ApiEnvelope<T>>(`${this.base}${path}`, body).pipe(map((response) => response.data));
  }
}

import { Injectable, computed, signal } from '@angular/core';
import { Observable, tap } from 'rxjs';
import { ApiClient } from '../api/api-client';
import { LoginResponse, User, UserRole } from '../../types/auth';

const TOKEN_KEY = 'thermal-planner.token';
const USER_KEY = 'thermal-planner.user';

@Injectable({providedIn: 'root'})
export class AuthStore {
  readonly token = signal<string | null>(localStorage.getItem(TOKEN_KEY));
  readonly user = signal<User | null>(this.restoreUser());
  readonly authenticated = computed(() => Boolean(this.token() && this.user()));

  constructor(private readonly api: ApiClient) {}

  login(username: string, password: string): Observable<LoginResponse> {
    return this.api.post<LoginResponse>('/auth/login', {username, password}).pipe(tap((response) => {
      localStorage.setItem(TOKEN_KEY, response.token);
      localStorage.setItem(USER_KEY, JSON.stringify(response.user));
      this.token.set(response.token);
      this.user.set(response.user);
    }));
  }

  logout(): void {
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(USER_KEY);
    this.token.set(null);
    this.user.set(null);
  }

  hasRole(...roles: UserRole[]): boolean {
    const role = this.user()?.role;
    return role ? roles.includes(role) : false;
  }

  private restoreUser(): User | null {
    const raw = localStorage.getItem(USER_KEY);
    if (!raw) return null;
    try { return JSON.parse(raw) as User; } catch { localStorage.removeItem(USER_KEY); return null; }
  }
}

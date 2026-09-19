export type UserRole = 'planner' | 'reviewer' | 'admin';

export interface User {
  id: number;
  username: string;
  display_name: string;
  role: UserRole;
}

export interface LoginResponse {
  token: string;
  expires_at: string;
  user: User;
}

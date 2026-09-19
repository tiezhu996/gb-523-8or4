import { Routes } from '@angular/router';
import { auditGuard, authGuard } from './router/auth.guard';

export const routes: Routes = [
  {path: 'login', loadComponent: () => import('./pages/login.page').then((m) => m.LoginPage)},
  {path: 'zones', canActivate: [authGuard], loadComponent: () => import('./pages/zones.page').then((m) => m.ZonesPage)},
  {path: 'racks', canActivate: [authGuard], loadComponent: () => import('./pages/racks.page').then((m) => m.RacksPage)},
  {path: 'loads', canActivate: [authGuard], loadComponent: () => import('./pages/loads.page').then((m) => m.LoadsPage)},
  {path: 'planner', canActivate: [authGuard], loadComponent: () => import('./pages/planner.page').then((m) => m.PlannerPage)},
  {path: 'audit', canActivate: [auditGuard], loadComponent: () => import('./pages/audit.page').then((m) => m.AuditPage)},
  {path: '', pathMatch: 'full', redirectTo: 'planner'},
  {path: '**', redirectTo: 'planner'}
];

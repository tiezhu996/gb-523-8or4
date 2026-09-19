import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { AuthStore } from '../stores/auth.store';

export const authGuard: CanActivateFn = () => {
  const auth = inject(AuthStore);
  return auth.authenticated() ? true : inject(Router).createUrlTree(['/login']);
};

export const auditGuard: CanActivateFn = () => {
  const auth = inject(AuthStore);
  if (!auth.authenticated()) return inject(Router).createUrlTree(['/login']);
  return auth.hasRole('reviewer', 'admin') ? true : inject(Router).createUrlTree(['/planner']);
};

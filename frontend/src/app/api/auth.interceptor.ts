import { HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { AuthStore } from '../stores/auth.store';

export const authInterceptor: HttpInterceptorFn = (request, next) => {
  const token = inject(AuthStore).token();
  if (!token || request.url.endsWith('/auth/login')) {
    return next(request);
  }
  return next(request.clone({setHeaders: {Authorization: `Bearer ${token}`}}));
};

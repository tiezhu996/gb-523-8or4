import { HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { MatSnackBar } from '@angular/material/snack-bar';
import { catchError, throwError } from 'rxjs';

interface ErrorEnvelope {
  error?: {code?: string; message?: string};
  request_id?: string;
}

export const errorInterceptor: HttpInterceptorFn = (request, next) => {
  const snack = inject(MatSnackBar);
  return next(request).pipe(catchError((error: HttpErrorResponse) => {
    const payload = error.error as ErrorEnvelope | undefined;
    const message = payload?.error?.message ?? (error.status === 0 ? 'Service is unreachable' : `Request failed (${error.status})`);
    snack.open(message, 'Dismiss', {duration: 5500, panelClass: ['error-snack']});
    return throwError(() => error);
  }));
};

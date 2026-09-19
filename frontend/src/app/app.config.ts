import { ApplicationConfig, importProvidersFrom } from '@angular/core';
import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { provideRouter } from '@angular/router';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { routes } from './app.routes';
import { authInterceptor } from './api/auth.interceptor';
import { errorInterceptor } from './api/error.interceptor';
import {
  AlertTriangle, Archive, Boxes, CheckCheck, CheckCircle2, Cpu, Edit3, GitCompareArrows,
  LayoutGrid, LockKeyhole, LogIn, LogOut, LucideAngularModule, Play, Plus, RefreshCw,
  Save, ServerCog, ShieldCheck, Thermometer, Wind, X, Zap
} from 'lucide-angular';

export const appConfig: ApplicationConfig = {
  providers: [
    provideRouter(routes),
    provideHttpClient(withInterceptors([authInterceptor, errorInterceptor])),
    provideAnimationsAsync(),
    importProvidersFrom(LucideAngularModule.pick({
      AlertTriangle, Archive, Boxes, CheckCheck, CheckCircle2, Cpu, Edit3, GitCompareArrows,
      LayoutGrid, LockKeyhole, LogIn, LogOut, Play, Plus, RefreshCw, Save, ServerCog,
      ShieldCheck, Thermometer, Wind, X, Zap
    }))
  ]
};

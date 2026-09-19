import { ChangeDetectionStrategy, Component } from '@angular/core';
import { Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatTooltipModule } from '@angular/material/tooltip';
import { LucideAngularModule } from 'lucide-angular';
import { useAuth } from './hooks/use-auth';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [RouterOutlet, RouterLink, RouterLinkActive, MatButtonModule, MatTooltipModule,
    LucideAngularModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (auth.authenticated()) {
      <div class="app-frame">
        <aside class="side-rail" aria-label="Primary navigation">
          <a class="product" routerLink="/planner" aria-label="Thermal Capacity Planner">
            <span class="product-mark"><lucide-icon name="server-cog" [size]="21" /></span>
            <span><strong>Thermal</strong><small>Capacity planner</small></span>
          </a>
          <nav>
            <a routerLink="/zones" routerLinkActive="active"><lucide-icon name="thermometer" [size]="18" /><span>Zones</span></a>
            <a routerLink="/racks" routerLinkActive="active"><lucide-icon name="layout-grid" [size]="18" /><span>Racks</span></a>
            <a routerLink="/loads" routerLinkActive="active"><lucide-icon name="cpu" [size]="18" /><span>Loads</span></a>
            <a routerLink="/planner" routerLinkActive="active"><lucide-icon name="boxes" [size]="18" /><span>Planner</span></a>
            @if (auth.hasRole('reviewer', 'admin')) {
              <a routerLink="/audit" routerLinkActive="active"><lucide-icon name="git-compare-arrows" [size]="18" /><span>Audit</span></a>
            }
          </nav>
          <div class="identity">
            <span class="role-dot"></span>
            <span><strong>{{ auth.user()?.display_name }}</strong><small>{{ auth.user()?.role }}</small></span>
            <button mat-icon-button matTooltip="Sign out" aria-label="Sign out" (click)="logout()"><lucide-icon name="log-out" [size]="18" /></button>
          </div>
        </aside>
        <main class="workspace"><router-outlet /></main>
      </div>
    } @else {
      <router-outlet />
    }
  `
})
export class AppComponent {
  readonly auth = useAuth();
  constructor(private readonly router: Router) {}
  logout(): void { this.auth.logout(); void this.router.navigate(['/login']); }
}

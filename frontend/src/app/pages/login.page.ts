import { ChangeDetectionStrategy, Component, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { LucideAngularModule } from 'lucide-angular';
import { finalize } from 'rxjs';
import { useAuth } from '../hooks/use-auth';

@Component({
  standalone: true,
  imports: [ReactiveFormsModule, MatButtonModule, MatFormFieldModule, MatInputModule,
    LucideAngularModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <main class="login-shell">
      <section class="facility-visual" aria-label="Data center thermal map">
        <div class="visual-heading">
          <span class="mark"><lucide-icon name="server-cog" [size]="28" /></span>
          <div><strong>Thermal Capacity Planner</strong><small>Offline decision workspace</small></div>
        </div>
        <div class="thermal-map" aria-hidden="true">
          @for (rack of racks; track $index) {
            <div class="rack" [class.warm]="$index === 5 || $index === 8" [class.hot]="$index === 9">
              <span>{{ rack }}</span><i></i><i></i><i></i><i></i>
            </div>
          }
        </div>
        <div class="legend"><span><i class="cool"></i>Nominal</span><span><i class="warm"></i>Constrained</span><span><i class="hot"></i>Critical</span></div>
      </section>
      <section class="login-panel">
        <div class="login-box">
          <lucide-icon name="lock-keyhole" [size]="22" />
          <h1>Sign in</h1>
          <p>Capacity planning access</p>
          <form [formGroup]="form" (ngSubmit)="submit()">
            <mat-form-field appearance="outline">
              <mat-label>Username</mat-label>
              <input matInput formControlName="username" autocomplete="username">
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>Password</mat-label>
              <input matInput type="password" formControlName="password" autocomplete="current-password">
            </mat-form-field>
            <button mat-flat-button color="primary" type="submit" [disabled]="form.invalid || busy()">
              <lucide-icon name="log-in" [size]="17" /> {{ busy() ? 'Signing in...' : 'Sign in' }}
            </button>
          </form>
          <div class="environment"><span></span>Planning environment</div>
        </div>
      </section>
    </main>
  `,
  styles: [`
    .login-shell{min-height:100vh;display:grid;grid-template-columns:minmax(440px,1.2fr) minmax(400px,.8fr);background:#15191d}
    .facility-visual{min-height:100vh;display:flex;flex-direction:column;padding:38px 46px;color:#eef1f3;background:#15191d;border-right:1px solid #343a3f}
    .visual-heading{display:flex;align-items:center;gap:13px}.mark{width:46px;height:46px;display:grid;place-items:center;background:#cf3f2e;border-radius:5px}.visual-heading strong,.visual-heading small{display:block;letter-spacing:0}.visual-heading strong{font-size:17px}.visual-heading small{margin-top:3px;color:#9ea7ad;font-size:11px;text-transform:uppercase}
    .thermal-map{width:min(650px,100%);margin:auto;display:grid;grid-template-columns:repeat(4,minmax(70px,1fr));gap:14px;transform:perspective(900px) rotateX(3deg)}
    .rack{height:152px;display:grid;grid-template-rows:24px repeat(4,1fr);gap:5px;padding:9px;background:#272d32;border:1px solid #596168;border-top:4px solid #4b9a77;box-shadow:0 10px 18px rgba(0,0,0,.2)}.rack.warm{border-top-color:#d79318}.rack.hot{border-top-color:#cf3f2e}.rack span{color:#bfc6cb;font-size:10px;font-weight:700}.rack i{display:block;background:#111416;border:1px solid #3c444a}.rack.warm i:nth-last-child(-n+2){background:#4c3b1d}.rack.hot i{background:#4e211d}
    .legend{display:flex;gap:22px;color:#aeb6bc;font-size:10px;text-transform:uppercase}.legend span{display:flex;align-items:center;gap:7px}.legend i{width:18px;height:4px;background:#4b9a77}.legend i.warm{background:#d79318}.legend i.hot{background:#cf3f2e}
    .login-panel{min-height:100vh;display:grid;place-items:center;padding:32px;background:#f3f5f6}.login-box{width:min(390px,100%);padding:34px;background:#fff;border:1px solid #d8dde1;border-top:4px solid #cf3f2e;border-radius:5px}.login-box>lucide-icon{color:#cf3f2e}.login-box h1{margin:16px 0 0;font-size:25px;letter-spacing:0}.login-box>p{margin:6px 0 24px;color:#68717a;font-size:12px;text-transform:uppercase}.login-box form{display:grid;gap:2px}.login-box button{height:43px;display:flex;gap:8px}.environment{display:flex;align-items:center;justify-content:center;gap:7px;margin-top:22px;color:#68717a;font-size:10px;text-transform:uppercase}.environment span{width:7px;height:7px;border-radius:50%;background:#3e8b68}
    @media(max-width:820px){.login-shell{grid-template-columns:1fr}.facility-visual{display:none}.login-panel{min-height:100vh;padding:20px}.login-box{padding:28px 22px}}
  `]
})
export class LoginPage {
  readonly auth = useAuth();
  readonly busy = signal(false);
  readonly racks = ['A-01','A-02','A-03','A-04','B-01','B-02','B-03','B-04','C-01','C-02','C-03','C-04'];
  readonly form = this.fb.nonNullable.group({username: ['planner', Validators.required], password: ['planner123', [Validators.required, Validators.minLength(8)]]});

  constructor(private readonly fb: FormBuilder, private readonly router: Router) {
    if (this.auth.authenticated()) void this.router.navigate(['/planner']);
  }

  submit(): void {
    if (this.form.invalid) return;
    this.busy.set(true);
    const {username, password} = this.form.getRawValue();
    this.auth.login(username, password).pipe(finalize(() => this.busy.set(false))).subscribe(() => void this.router.navigate(['/planner']));
  }
}

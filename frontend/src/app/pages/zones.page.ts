import { ChangeDetectionStrategy, Component, computed, signal } from '@angular/core';
import { DecimalPipe } from '@angular/common';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSnackBar } from '@angular/material/snack-bar';
import { LucideAngularModule } from 'lucide-angular';
import { finalize } from 'rxjs';
import { ThermalZone, ZoneInput, ZoneStatus } from '../../types/zone';
import { ZoneApi } from '../api/zone.api';
import { CapacityMeterComponent } from '../components/common/capacity-meter.component';
import { useAuth } from '../hooks/use-auth';

@Component({
  standalone: true,
  imports: [DecimalPipe, ReactiveFormsModule, MatButtonModule, MatFormFieldModule, MatInputModule, MatSelectModule,
    CapacityMeterComponent, LucideAngularModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="page">
      <header class="page-header">
        <div><h1>Thermal zones</h1><p>Cooling envelopes, return-temperature boundaries, and neighboring heat influence.</p></div>
        <div class="header-actions">
          <button mat-stroked-button (click)="load()"><lucide-icon name="refresh-cw" [size]="16" /> Refresh</button>
          @if (canWrite()) { <button mat-flat-button color="primary" (click)="openCreate()"><lucide-icon name="plus" [size]="16" /> Add zone</button> }
        </div>
      </header>

      <section class="metric-strip" aria-label="Zone capacity summary">
        <div class="metric"><small>Cooling envelope</small><strong>{{ totalCooling() | number:'1.0-0' }} kW</strong><span>Configured across {{ zones().length }} zones</span></div>
        <div class="metric"><small>Rack allocation</small><strong>{{ totalAllocated() | number:'1.0-0' }} kW</strong><span>Aggregate rack power limits</span></div>
        <div class="metric"><small>Constrained zones</small><strong>{{ constrainedCount() }}</strong><span>Require planning attention</span></div>
        <div class="metric"><small>Minimum headroom</small><strong>{{ minHeadroom() | number:'1.0-1' }} C</strong><span>Supply to return threshold</span></div>
      </section>

      <div class="split">
        <section class="panel">
          <div class="panel-header"><h2>Capacity boundaries</h2><span class="secondary">{{ zones().length }} zones</span></div>
          @if (loading()) { <div class="loading-line"></div> }
          <div class="table-wrap">
            <table class="data-table">
              <thead><tr><th>Zone</th><th>Status</th><th>Cooling</th><th>Rack envelope</th><th>Return limit</th><th>Adjacency</th>@if (canWrite()) {<th></th>}</tr></thead>
              <tbody>
                @for (zone of zones(); track zone.id) {
                  <tr>
                    <td><span class="primary-cell">{{ zone.zone_code }}</span><br><span class="secondary">{{ zone.name }} / {{ zone.rack_count }} racks</span></td>
                    <td><span class="status" [class]="'status ' + zone.zone_status">{{ zone.zone_status }}</span></td>
                    <td class="numeric">{{ zone.cooling_capacity_kw | number:'1.0-1' }} kW</td>
                    <td><app-capacity-meter [value]="zone.capacity_utilization" [label]="zone.zone_code + ' rack allocation'" /></td>
                    <td><span class="primary-cell">{{ zone.max_return_temp_c | number:'1.0-1' }} C</span><br><span class="secondary">{{ zone.temperature_headroom_c | number:'1.0-1' }} C span</span></td>
                    <td class="secondary">{{ adjacencyLabel(zone) }}</td>
                    @if (canWrite()) { <td><button mat-icon-button aria-label="Edit zone" (click)="openEdit(zone)"><lucide-icon name="edit-3" [size]="16" /></button></td> }
                  </tr>
                } @empty { <tr><td colspan="7"><div class="empty"><span><strong>No thermal zones</strong>Capacity boundaries will appear here.</span></div></td></tr> }
              </tbody>
            </table>
          </div>
        </section>

        <section class="panel editor" [class.visible]="editorOpen()">
          <div class="panel-header"><h2>{{ editingId() ? 'Edit boundary' : 'New thermal zone' }}</h2><button mat-icon-button aria-label="Close editor" (click)="closeEditor()"><lucide-icon name="x" [size]="17" /></button></div>
          @if (editorOpen()) {
            <form class="panel-body form-grid" [formGroup]="form" (ngSubmit)="save()">
              <mat-form-field appearance="outline"><mat-label>Zone code</mat-label><input matInput formControlName="zone_code" placeholder="TZ-D"></mat-form-field>
              <mat-form-field appearance="outline"><mat-label>Name</mat-label><input matInput formControlName="name"></mat-form-field>
              <mat-form-field appearance="outline"><mat-label>Cooling capacity (kW)</mat-label><input matInput type="number" formControlName="cooling_capacity_kw"></mat-form-field>
              <mat-form-field appearance="outline"><mat-label>Status</mat-label><mat-select formControlName="zone_status">@for (status of statuses; track status) {<mat-option [value]="status">{{ status }}</mat-option>}</mat-select></mat-form-field>
              <mat-form-field appearance="outline"><mat-label>Supply temperature (C)</mat-label><input matInput type="number" formControlName="supply_temp_c"></mat-form-field>
              <mat-form-field appearance="outline"><mat-label>Maximum return (C)</mat-label><input matInput type="number" formControlName="max_return_temp_c"></mat-form-field>
              <mat-form-field appearance="outline" class="full"><mat-label>Adjacency weights (JSON)</mat-label><textarea matInput rows="4" formControlName="adjacency"></textarea></mat-form-field>
              <div class="form-actions"><button mat-button type="button" (click)="closeEditor()">Cancel</button><button mat-flat-button color="primary" type="submit" [disabled]="form.invalid || saving()"><lucide-icon name="save" [size]="16" /> {{ saving() ? 'Saving...' : 'Save boundary' }}</button></div>
            </form>
          } @else { <div class="empty"><span><strong>Boundary editor</strong>Select a zone or create a new one.</span></div> }
        </section>
      </div>
    </div>
  `,
  styles: [`.editor{position:sticky;top:20px}.editor:not(.visible){min-height:280px} @media(max-width:900px){.editor{position:static}}`]
})
export class ZonesPage {
  readonly auth = useAuth();
  readonly zones = signal<ThermalZone[]>([]);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly editorOpen = signal(false);
  readonly editingId = signal<number | null>(null);
  readonly statuses: ZoneStatus[] = ['active', 'constrained', 'offline'];
  readonly totalCooling = computed(() => this.zones().reduce((sum, zone) => sum + zone.cooling_capacity_kw, 0));
  readonly totalAllocated = computed(() => this.zones().reduce((sum, zone) => sum + zone.allocated_power_kw, 0));
  readonly constrainedCount = computed(() => this.zones().filter((zone) => zone.zone_status !== 'active').length);
  readonly minHeadroom = computed(() => this.zones().length ? Math.min(...this.zones().map((zone) => zone.temperature_headroom_c)) : 0);
  readonly form = this.fb.nonNullable.group({
    zone_code: ['', [Validators.required, Validators.minLength(2)]], name: ['', Validators.required],
    cooling_capacity_kw: [80, [Validators.required, Validators.min(1)]], supply_temp_c: [18, Validators.required],
    max_return_temp_c: [31, Validators.required], adjacency: ['{}', Validators.required], zone_status: ['active' as ZoneStatus, Validators.required]
  });

  constructor(private readonly fb: FormBuilder, private readonly api: ZoneApi, private readonly snack: MatSnackBar) { this.load(); }
  canWrite(): boolean { return this.auth.hasRole('planner', 'admin'); }
  load(): void { this.loading.set(true); this.api.list().pipe(finalize(() => this.loading.set(false))).subscribe((page) => this.zones.set(page.items)); }
  adjacencyLabel(zone: ThermalZone): string { const entries = Object.entries(zone.adjacency); return entries.length ? entries.map(([code, weight]) => `${code} ${weight}`).join(', ') : 'Isolated'; }
  openCreate(): void { this.editingId.set(null); this.form.reset({zone_code: '', name: '', cooling_capacity_kw: 80, supply_temp_c: 18, max_return_temp_c: 31, adjacency: '{}', zone_status: 'active'}); this.form.controls.zone_code.enable(); this.editorOpen.set(true); }
  openEdit(zone: ThermalZone): void { this.editingId.set(zone.id); this.form.reset({...zone, adjacency: JSON.stringify(zone.adjacency)}); this.form.controls.zone_code.disable(); this.editorOpen.set(true); }
  closeEditor(): void { this.editorOpen.set(false); this.editingId.set(null); }
  save(): void {
    if (this.form.invalid) return;
    let adjacency: Record<string, number>;
    try { adjacency = JSON.parse(this.form.controls.adjacency.value) as Record<string, number>; } catch { this.snack.open('Adjacency must be a JSON object', 'Dismiss', {duration: 4000}); return; }
    const raw = this.form.getRawValue();
    const input: ZoneInput = {name: raw.name, cooling_capacity_kw: raw.cooling_capacity_kw, supply_temp_c: raw.supply_temp_c, max_return_temp_c: raw.max_return_temp_c, adjacency, zone_status: raw.zone_status};
    const request = this.editingId() ? this.api.update(this.editingId()!, input) : this.api.create({...input, zone_code: raw.zone_code});
    this.saving.set(true);
    request.pipe(finalize(() => this.saving.set(false))).subscribe(() => { this.snack.open('Thermal boundary saved', undefined, {duration: 2200}); this.closeEditor(); this.load(); });
  }
}

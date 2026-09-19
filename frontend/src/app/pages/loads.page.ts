import { ChangeDetectionStrategy, Component, computed, signal } from '@angular/core';
import { DecimalPipe } from '@angular/common';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSnackBar } from '@angular/material/snack-bar';
import { LucideAngularModule } from 'lucide-angular';
import { finalize, forkJoin } from 'rxjs';
import { BatchValidation, EquipmentLoad, LoadInput, LoadStatus } from '../../types/load';
import { ThermalZone } from '../../types/zone';
import { LoadApi } from '../api/load.api';
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
        <div><h1>Equipment loads</h1><p>Planning inputs for power, heat, airflow, footprint, affinity, and redundancy isolation.</p></div>
        <div class="header-actions"><button mat-stroked-button (click)="load()"><lucide-icon name="refresh-cw" [size]="16" /> Refresh</button>@if (canWrite()) {<button mat-stroked-button (click)="validate()"><lucide-icon name="check-check" [size]="16" /> Validate ready</button><button mat-flat-button color="primary" (click)="openCreate()"><lucide-icon name="plus" [size]="16" /> Add load</button>}</div>
      </header>

      <section class="metric-strip">
        <div class="metric"><small>Ready power</small><strong>{{ readyPower() | number:'1.0-1' }} kW</strong><span>{{ readyCount() }} loads in planning set</span></div>
        <div class="metric"><small>Thermal output</small><strong>{{ readyHeat() | number:'1.0-1' }} kW</strong><span>{{ heatRatio() | number:'1.0-0' }}% heat-to-power</span></div>
        <div class="metric"><small>Requested airflow</small><strong>{{ readyAirflow() | number:'1.0-0' }}</strong><span>CFM across ready loads</span></div>
        <div class="metric"><small>Validation</small><strong>{{ validation()?.valid_count ?? '-' }} / {{ validation()?.total ?? '-' }}</strong><span>Ready inputs passing checks</span></div>
      </section>

      <div class="split">
        <section class="panel">
          <div class="panel-header"><h2>Planning input register</h2><span class="secondary">{{ loads().length }} loads</span></div>
          @if (loading()) { <div class="loading-line"></div> }
          <div class="table-wrap">
            <table class="data-table">
              <thead><tr><th>Equipment load</th><th>Status</th><th>Power / heat</th><th>Heat ratio</th><th>Airflow</th><th>Footprint</th><th>Preference</th>@if (canWrite()) {<th></th>}</tr></thead>
              <tbody>
                @for (item of loads(); track item.id) {
                  <tr>
                    <td><span class="primary-cell">{{ item.name }}</span><br><span class="secondary">Group {{ item.redundancy_group }}</span></td>
                    <td><span [class]="'status ' + item.load_status">{{ item.load_status }}</span></td>
                    <td><span class="primary-cell">{{ item.power_kw | number:'1.0-1' }} kW</span><br><span class="secondary">{{ item.heat_kw | number:'1.0-1' }} kW heat</span></td>
                    <td><app-capacity-meter [value]="item.heat_ratio * 100" label="Heat-to-power ratio" /></td>
                    <td class="numeric">{{ item.airflow_cfm | number:'1.0-0' }} CFM</td>
                    <td class="numeric">{{ item.rack_units }}U</td>
                    <td>{{ item.preferred_zone_code || 'Any active zone' }}</td>
                    @if (canWrite()) {<td><button mat-icon-button aria-label="Edit equipment load" (click)="openEdit(item)"><lucide-icon name="edit-3" [size]="16" /></button></td>}
                  </tr>
                } @empty {<tr><td colspan="8"><div class="empty"><span><strong>No equipment loads</strong>Planning inputs will appear here.</span></div></td></tr>}
              </tbody>
            </table>
          </div>
        </section>

        <section class="panel editor">
          <div class="panel-header"><h2>{{ editorOpen() ? (editingId() ? 'Edit load input' : 'New load input') : 'Batch validation' }}</h2>@if (editorOpen()) {<button mat-icon-button aria-label="Close editor" (click)="closeEditor()"><lucide-icon name="x" [size]="17" /></button>}</div>
          @if (editorOpen()) {
            <form class="panel-body form-grid" [formGroup]="form" (ngSubmit)="save()">
              <mat-form-field appearance="outline" class="full"><mat-label>Name</mat-label><input matInput formControlName="name"></mat-form-field>
              <mat-form-field appearance="outline"><mat-label>Power (kW)</mat-label><input matInput type="number" formControlName="power_kw"></mat-form-field>
              <mat-form-field appearance="outline"><mat-label>Heat (kW)</mat-label><input matInput type="number" formControlName="heat_kw"></mat-form-field>
              <mat-form-field appearance="outline"><mat-label>Airflow (CFM)</mat-label><input matInput type="number" formControlName="airflow_cfm"></mat-form-field>
              <mat-form-field appearance="outline"><mat-label>Rack units</mat-label><input matInput type="number" formControlName="rack_units"></mat-form-field>
              <mat-form-field appearance="outline"><mat-label>Redundancy group</mat-label><input matInput formControlName="redundancy_group"></mat-form-field>
              <mat-form-field appearance="outline"><mat-label>Preferred zone</mat-label><mat-select formControlName="preferred_zone_id"><mat-option [value]="null">Any active zone</mat-option>@for (zone of zones(); track zone.id) {<mat-option [value]="zone.id">{{ zone.zone_code }}</mat-option>}</mat-select></mat-form-field>
              <mat-form-field appearance="outline" class="full"><mat-label>Status</mat-label><mat-select formControlName="load_status">@for (status of statuses; track status) {<mat-option [value]="status">{{ status }}</mat-option>}</mat-select></mat-form-field>
              <div class="form-actions"><button mat-button type="button" (click)="closeEditor()">Cancel</button><button mat-flat-button color="primary" type="submit" [disabled]="form.invalid || saving()"><lucide-icon name="save" [size]="16" /> Save input</button></div>
            </form>
          } @else {
            @if (validation(); as result) {
              <div class="validation-list">
                <div class="validation-summary"><strong>{{ result.valid_count }} / {{ result.total }}</strong><span>ready loads valid</span></div>
                @for (entry of result.results; track entry.load_id) {
                  <div class="validation-row"><span><strong>{{ entry.load_name }}</strong>@for (issue of entry.issues; track issue) {<small>{{ issue }}</small>}</span><i [class.invalid]="!entry.valid">{{ entry.valid ? 'valid' : 'review' }}</i></div>
                }
              </div>
            } @else {<div class="empty"><span><strong>Validation not run</strong>Validate the current ready planning set.</span></div>}
          }
        </section>
      </div>
    </div>
  `,
  styles: [`
    .editor{position:sticky;top:20px}.validation-list{padding:8px 16px 16px}.validation-summary{display:flex;align-items:baseline;gap:8px;padding:14px 0;border-bottom:1px solid #d8dde1}.validation-summary strong{font-size:23px}.validation-summary span{color:#68717a;font-size:11px}.validation-row{display:flex;justify-content:space-between;gap:12px;padding:12px 0;border-bottom:1px solid #e5e8ea;font-size:11px}.validation-row strong,.validation-row small{display:block}.validation-row small{max-width:260px;margin-top:4px;color:#b42318}.validation-row i{align-self:flex-start;color:#237a4b;font-size:9px;font-style:normal;font-weight:750;text-transform:uppercase}.validation-row i.invalid{color:#b42318}@media(max-width:900px){.editor{position:static}}
  `]
})
export class LoadsPage {
  readonly auth = useAuth();
  readonly loads = signal<EquipmentLoad[]>([]);
  readonly zones = signal<ThermalZone[]>([]);
  readonly validation = signal<BatchValidation | null>(null);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly editorOpen = signal(false);
  readonly editingId = signal<number | null>(null);
  readonly statuses: LoadStatus[] = ['draft', 'ready', 'held', 'placed'];
  readonly readyCount = computed(() => this.ready().length);
  readonly readyPower = computed(() => this.ready().reduce((sum, load) => sum + load.power_kw, 0));
  readonly readyHeat = computed(() => this.ready().reduce((sum, load) => sum + load.heat_kw, 0));
  readonly readyAirflow = computed(() => this.ready().reduce((sum, load) => sum + load.airflow_cfm, 0));
  readonly heatRatio = computed(() => this.readyPower() ? this.readyHeat() / this.readyPower() * 100 : 0);
  readonly form = this.fb.group({name: this.fb.nonNullable.control('', Validators.required), power_kw: this.fb.nonNullable.control(10, Validators.min(.1)), heat_kw: this.fb.nonNullable.control(9, Validators.min(.1)), airflow_cfm: this.fb.nonNullable.control(2400, Validators.min(1)), rack_units: this.fb.nonNullable.control(8, [Validators.min(1), Validators.max(60)]), redundancy_group: this.fb.nonNullable.control('', Validators.required), preferred_zone_id: this.fb.control<number | null>(null), load_status: this.fb.nonNullable.control<LoadStatus>('ready', Validators.required)});

  constructor(private readonly fb: FormBuilder, private readonly api: LoadApi, private readonly zoneApi: ZoneApi, private readonly snack: MatSnackBar) { this.load(); }
  canWrite(): boolean { return this.auth.hasRole('planner', 'admin'); }
  ready(): EquipmentLoad[] { return this.loads().filter((load) => load.load_status === 'ready'); }
  load(): void { this.loading.set(true); forkJoin({loads: this.api.list(), zones: this.zoneApi.list()}).pipe(finalize(() => this.loading.set(false))).subscribe(({loads, zones}) => { this.loads.set(loads.items); this.zones.set(zones.items); }); }
  validate(): void { this.api.validate().subscribe((result) => { this.validation.set(result); this.editorOpen.set(false); this.snack.open(`${result.valid_count} of ${result.total} ready loads passed`, undefined, {duration: 2500}); }); }
  openCreate(): void { this.editingId.set(null); this.form.reset({name: '', power_kw: 10, heat_kw: 9, airflow_cfm: 2400, rack_units: 8, redundancy_group: '', preferred_zone_id: null, load_status: 'ready'}); this.editorOpen.set(true); }
  openEdit(load: EquipmentLoad): void { this.editingId.set(load.id); this.form.reset({...load}); this.editorOpen.set(true); }
  closeEditor(): void { this.editorOpen.set(false); }
  save(): void {
    if (this.form.invalid) return;
    const raw = this.form.getRawValue();
    const input: LoadInput = {name: raw.name, power_kw: raw.power_kw, heat_kw: raw.heat_kw, airflow_cfm: raw.airflow_cfm, rack_units: raw.rack_units, redundancy_group: raw.redundancy_group, preferred_zone_id: raw.preferred_zone_id, load_status: raw.load_status};
    const request = this.editingId() ? this.api.update(this.editingId()!, input) : this.api.create(input);
    this.saving.set(true); request.pipe(finalize(() => this.saving.set(false))).subscribe(() => { this.snack.open('Equipment load saved', undefined, {duration: 2200}); this.closeEditor(); this.load(); });
  }
}

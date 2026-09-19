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
import { Rack, RackInput, RackStatus } from '../../types/rack';
import { ThermalZone } from '../../types/zone';
import { EquipmentLoad } from '../../types/load';
import { RackApi } from '../api/rack.api';
import { ZoneApi } from '../api/zone.api';
import { LoadApi } from '../api/load.api';
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
        <div><h1>Rack layout</h1><p>Stable position grid with power, airflow, U-space, and availability boundaries.</p></div>
        <div class="header-actions"><button mat-stroked-button (click)="load()"><lucide-icon name="refresh-cw" [size]="16" /> Refresh</button>@if (canWrite()) {<button mat-flat-button color="primary" (click)="openCreate()"><lucide-icon name="plus" [size]="16" /> Add rack</button>}</div>
      </header>

      <section class="metric-strip">
        <div class="metric"><small>Rack power envelope</small><strong>{{ totalRackPower() | number:'1.0-0' }} kW</strong><span>{{ racks().length }} defined positions</span></div>
        <div class="metric"><small>Ready equipment</small><strong>{{ readyLoads() }}</strong><span>{{ readyPower() | number:'1.0-1' }} kW planning input</span></div>
        <div class="metric"><small>Available positions</small><strong>{{ availableCount() }}</strong><span>Available or reserved</span></div>
        <div class="metric"><small>Unavailable</small><strong>{{ blockedCount() }}</strong><span>Excluded by placement constraints</span></div>
      </section>

      <div class="split">
        <section class="rack-board">
          @if (loading()) { <div class="loading-line"></div> }
          @for (zone of zones(); track zone.id) {
            <div class="zone-row">
              <div class="zone-label"><strong>{{ zone.zone_code }}</strong><span>{{ zone.name }}</span><small>{{ zone.cooling_capacity_kw | number:'1.0-0' }} kW cooling</small></div>
              <div class="rack-grid">
                @for (rack of racksFor(zone.id); track rack.id) {
                  <button type="button" class="rack-tile" [class]="'rack-tile ' + rack.rack_status" (click)="openEdit(rack)" [attr.aria-label]="rack.rack_code + ' ' + rack.rack_status">
                    <span class="rack-top"><strong>{{ rack.rack_code }}</strong><i>{{ rack.rack_status }}</i></span>
                    <span class="rack-vents"><i></i><i></i><i></i></span>
                    <app-capacity-meter [value]="rack.power_limit_kw / 40 * 100" label="Relative rack power density" />
                    <span class="rack-spec"><span><lucide-icon name="zap" [size]="12" />{{ rack.power_limit_kw | number:'1.0-1' }} kW</span><span><lucide-icon name="wind" [size]="12" />{{ rack.airflow_limit_cfm | number:'1.0-0' }}</span></span>
                    <span class="rack-foot">R{{ rack.row_index }} / C{{ rack.column_index }} / {{ rack.rack_units }}U</span>
                  </button>
                } @empty { <div class="zone-empty">No rack positions</div> }
              </div>
            </div>
          }
        </section>

        <section class="panel editor">
          <div class="panel-header"><h2>{{ editingId() ? 'Edit rack boundary' : 'New rack position' }}</h2><button mat-icon-button aria-label="Close editor" (click)="closeEditor()"><lucide-icon name="x" [size]="17" /></button></div>
          @if (editorOpen()) {
            <form class="panel-body form-grid" [formGroup]="form" (ngSubmit)="save()">
              <mat-form-field appearance="outline"><mat-label>Rack code</mat-label><input matInput formControlName="rack_code"></mat-form-field>
              <mat-form-field appearance="outline"><mat-label>Thermal zone</mat-label><mat-select formControlName="zone_id">@for (zone of zones(); track zone.id) {<mat-option [value]="zone.id">{{ zone.zone_code }} / {{ zone.name }}</mat-option>}</mat-select></mat-form-field>
              <mat-form-field appearance="outline"><mat-label>Row</mat-label><input matInput type="number" formControlName="row_index"></mat-form-field>
              <mat-form-field appearance="outline"><mat-label>Column</mat-label><input matInput type="number" formControlName="column_index"></mat-form-field>
              <mat-form-field appearance="outline"><mat-label>Power limit (kW)</mat-label><input matInput type="number" formControlName="power_limit_kw"></mat-form-field>
              <mat-form-field appearance="outline"><mat-label>Airflow limit (CFM)</mat-label><input matInput type="number" formControlName="airflow_limit_cfm"></mat-form-field>
              <mat-form-field appearance="outline"><mat-label>Rack units</mat-label><input matInput type="number" formControlName="rack_units"></mat-form-field>
              <mat-form-field appearance="outline"><mat-label>Status</mat-label><mat-select formControlName="rack_status">@for (status of statuses; track status) {<mat-option [value]="status">{{ status }}</mat-option>}</mat-select></mat-form-field>
              <div class="form-actions"><button mat-button type="button" (click)="closeEditor()">Cancel</button><button mat-flat-button color="primary" type="submit" [disabled]="form.invalid || saving()"><lucide-icon name="save" [size]="16" /> Save rack</button></div>
            </form>
          } @else { <div class="empty"><span><strong>Rack editor</strong>Select a rack tile or create a position.</span></div> }
        </section>
      </div>
    </div>
  `,
  styles: [`
    .rack-board{min-width:0;background:#fff;border:1px solid #d8dde1}.zone-row{display:grid;grid-template-columns:150px minmax(0,1fr);border-bottom:1px solid #d8dde1}.zone-row:last-child{border-bottom:0}.zone-label{padding:17px 15px;background:#f3f5f6;border-right:1px solid #d8dde1}.zone-label strong,.zone-label span,.zone-label small{display:block;letter-spacing:0}.zone-label strong{font-size:15px}.zone-label span{margin-top:4px;color:#4f5860;font-size:11px}.zone-label small{margin-top:12px;color:#68717a;font-size:10px}.rack-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(142px,1fr));gap:10px;min-height:182px;padding:14px;align-content:start}.rack-tile{height:160px;min-width:0;display:grid;grid-template-rows:auto 1fr auto auto auto;gap:6px;padding:10px;color:#23282d;background:#f9fafa;border:1px solid #aeb6bd;border-top:4px solid #3e8b68;border-radius:3px;text-align:left;cursor:pointer}.rack-tile:hover{border-color:#59636b;background:#fff}.rack-tile.reserved{border-top-color:#d79318}.rack-tile.unavailable,.rack-tile.maintenance{border-top-color:#b42318;background:#f0f1f2;color:#68717a}.rack-top{display:flex;justify-content:space-between;gap:6px}.rack-top strong{font-size:13px}.rack-top i{overflow:hidden;text-overflow:ellipsis;color:#68717a;font-size:8px;font-style:normal;text-transform:uppercase}.rack-vents{display:grid;gap:4px}.rack-vents i{display:block;background:#dfe3e5;border:1px solid #c8ced2}.rack-spec{display:flex;justify-content:space-between;gap:7px;color:#485159;font-size:9px}.rack-spec>span{display:flex;align-items:center;gap:3px;white-space:nowrap}.rack-foot{padding-top:5px;border-top:1px solid #d8dde1;color:#68717a;font-size:9px}.zone-empty{display:grid;place-items:center;min-height:130px;color:#68717a;font-size:11px}.editor{position:sticky;top:20px}.form-actions lucide-icon{vertical-align:middle;margin-right:5px}
    @media(max-width:1050px){.zone-row{grid-template-columns:115px minmax(0,1fr)}}@media(max-width:900px){.editor{position:static}}@media(max-width:600px){.zone-row{grid-template-columns:1fr}.zone-label{border-right:0;border-bottom:1px solid #d8dde1}.rack-grid{grid-template-columns:repeat(2,minmax(0,1fr));padding:10px}.rack-tile{height:160px}}
  `]
})
export class RacksPage {
  readonly auth = useAuth();
  readonly racks = signal<Rack[]>([]);
  readonly zones = signal<ThermalZone[]>([]);
  readonly loads = signal<EquipmentLoad[]>([]);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly editorOpen = signal(false);
  readonly editingId = signal<number | null>(null);
  readonly statuses: RackStatus[] = ['available', 'reserved', 'unavailable', 'maintenance'];
  readonly totalRackPower = computed(() => this.racks().reduce((sum, rack) => sum + rack.power_limit_kw, 0));
  readonly readyLoads = computed(() => this.loads().filter((load) => load.load_status === 'ready').length);
  readonly readyPower = computed(() => this.loads().filter((load) => load.load_status === 'ready').reduce((sum, load) => sum + load.power_kw, 0));
  readonly availableCount = computed(() => this.racks().filter((rack) => rack.rack_status === 'available' || rack.rack_status === 'reserved').length);
  readonly blockedCount = computed(() => this.racks().filter((rack) => rack.rack_status === 'unavailable' || rack.rack_status === 'maintenance').length);
  readonly form = this.fb.nonNullable.group({rack_code: ['', Validators.required], zone_id: [0, Validators.min(1)], row_index: [1, Validators.min(0)], column_index: [1, Validators.min(0)], power_limit_kw: [24, Validators.min(1)], airflow_limit_cfm: [6800, Validators.min(1)], rack_units: [42, [Validators.min(12), Validators.max(60)]], rack_status: ['available' as RackStatus, Validators.required], version: [1]});

  constructor(private readonly fb: FormBuilder, private readonly rackApi: RackApi, private readonly zoneApi: ZoneApi, private readonly loadApi: LoadApi, private readonly snack: MatSnackBar) { this.load(); }
  canWrite(): boolean { return this.auth.hasRole('planner', 'admin'); }
  load(): void { this.loading.set(true); forkJoin({racks: this.rackApi.list(), zones: this.zoneApi.list(), loads: this.loadApi.list()}).pipe(finalize(() => this.loading.set(false))).subscribe(({racks, zones, loads}) => { this.racks.set(racks.items); this.zones.set(zones.items); this.loads.set(loads.items); }); }
  racksFor(zoneId: number): Rack[] { return this.racks().filter((rack) => rack.zone_id === zoneId).sort((a, b) => a.row_index - b.row_index || a.column_index - b.column_index); }
  openCreate(): void { this.editingId.set(null); this.form.reset({rack_code: '', zone_id: this.zones()[0]?.id ?? 0, row_index: 1, column_index: 1, power_limit_kw: 24, airflow_limit_cfm: 6800, rack_units: 42, rack_status: 'available', version: 1}); this.form.controls.rack_code.enable(); this.editorOpen.set(true); }
  openEdit(rack: Rack): void { if (!this.canWrite()) return; this.editingId.set(rack.id); this.form.reset({...rack}); this.form.controls.rack_code.disable(); this.editorOpen.set(true); }
  closeEditor(): void { this.editorOpen.set(false); }
  save(): void {
    if (this.form.invalid) return;
    const raw = this.form.getRawValue();
    const input: RackInput = {zone_id: raw.zone_id, row_index: raw.row_index, column_index: raw.column_index, power_limit_kw: raw.power_limit_kw, airflow_limit_cfm: raw.airflow_limit_cfm, rack_units: raw.rack_units, rack_status: raw.rack_status, version: raw.version};
    const request = this.editingId() ? this.rackApi.update(this.editingId()!, input) : this.rackApi.create({...input, rack_code: raw.rack_code});
    this.saving.set(true); request.pipe(finalize(() => this.saving.set(false))).subscribe(() => { this.snack.open('Rack boundary saved', undefined, {duration: 2200}); this.closeEditor(); this.load(); });
  }
}

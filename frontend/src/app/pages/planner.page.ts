import { ChangeDetectionStrategy, Component, computed, signal } from '@angular/core';
import { DecimalPipe } from '@angular/common';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { HttpErrorResponse } from '@angular/common/http';
import { MatButtonModule } from '@angular/material/button';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSnackBar } from '@angular/material/snack-bar';
import { LucideAngularModule } from 'lucide-angular';
import { finalize, forkJoin } from 'rxjs';
import { EquipmentLoad } from '../../types/load';
import { Rack } from '../../types/rack';
import { ConstraintViolation, LayoutScenario, RackAssignment, ScenarioStatus } from '../../types/scenario';
import { LoadApi } from '../api/load.api';
import { RackApi } from '../api/rack.api';
import { ScenarioApi } from '../api/scenario.api';
import { ConstraintBadgeComponent } from '../components/common/constraint-badge.component';
import { useAuth } from '../hooks/use-auth';
import { useScenarioEvaluation } from '../hooks/use-scenario-evaluation';
import { ScenarioStore } from '../stores/scenario.store';

interface PinErrorEnvelope {
  error?: {code?: string; message?: string; details?: ConstraintViolation[]};
}

@Component({
  standalone: true,
  imports: [DecimalPipe, ReactiveFormsModule, MatButtonModule, MatCheckboxModule, MatFormFieldModule, MatInputModule, MatSelectModule,
    ConstraintBadgeComponent, LucideAngularModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="page planner-page">
      <header class="page-header">
        <div><h1>Layout planner</h1><p>Deterministic placement against rack capacity, zone cooling, return temperature, adjacency, and redundancy constraints.</p></div>
        <div class="header-actions"><button mat-stroked-button (click)="load()"><lucide-icon name="refresh-cw" [size]="16" /> Refresh</button>@if (canPlan()) {<button mat-flat-button color="primary" (click)="createOpen.set(!createOpen())"><lucide-icon name="plus" [size]="16" /> New scenario</button>}</div>
      </header>

      @if (createOpen()) {
        <section class="scenario-builder">
          <form [formGroup]="form" (ngSubmit)="createScenario()">
            <mat-form-field appearance="outline"><mat-label>Scenario name</mat-label><input matInput formControlName="name"></mat-form-field>
            <div class="load-picker">
              @for (load of readyLoads(); track load.id) {
                <mat-checkbox [checked]="selectedLoadIds().has(load.id)" (change)="toggleLoad(load.id, $event.checked)">
                  <strong>{{ load.name }}</strong><small>{{ load.power_kw | number:'1.0-1' }} kW / {{ load.rack_units }}U / {{ load.redundancy_group }}</small>
                </mat-checkbox>
              }
            </div>
            <div class="builder-actions"><span>{{ selectedLoadIds().size }} loads selected</span><button mat-button type="button" (click)="createOpen.set(false)">Cancel</button><button mat-flat-button color="primary" type="submit" [disabled]="form.invalid || selectedLoadIds().size === 0 || creating()">Create draft</button></div>
          </form>
        </section>
      }

      <section class="control-bar">
        <mat-form-field appearance="outline" subscriptSizing="dynamic"><mat-label>Active scenario</mat-label><mat-select [value]="selected()?.id" (selectionChange)="selectScenario($event.value)">@for (item of scenarios(); track item.id) {<mat-option [value]="item.id">{{ item.name }} / v{{ item.version }} / {{ item.scenario_status }}</mat-option>}</mat-select></mat-form-field>
        @if (selected(); as active) {
          <span [class]="'status ' + active.scenario_status">{{ active.scenario_status }}</span>
          <span class="algorithm">{{ active.algorithm_version }}</span>
          <span class="control-spacer"></span>
          @if (active.scenario_status === 'draft' && canPlan()) {<button mat-flat-button color="primary" (click)="evaluate()" [disabled]="store.evaluating()"><lucide-icon name="play" [size]="16" /> {{ store.evaluating() ? 'Evaluating...' : 'Evaluate layout' }}</button>}
          @if (active.scenario_status === 'pending_review' && canApprove()) {<button mat-flat-button color="primary" (click)="transition('approved')" [disabled]="active.has_critical_violation"><lucide-icon name="check-circle-2" [size]="16" /> Approve</button>}
        }
      </section>

      @if (pinFailures().length > 0) {
        <section class="pin-failures">
          <div class="pin-failures-head"><lucide-icon name="alert-triangle" [size]="16" /><strong>Evaluation rejected: pinned loads conflict with rack, thermal zone or redundancy limits</strong><span class="secondary">Draft and pins are preserved; resolve the conflicts below and evaluate again.</span></div>
          @for (item of pinFailures(); track item.code + item.entity_id) {
            <div class="pin-failure"><app-constraint-badge [severity]="item.severity" [label]="item.code" /><p>{{ item.message }}</p><small>{{ item.entity_type }} #{{ item.entity_id }} / actual {{ item.actual | number:'1.0-1' }} / limit {{ item.limit | number:'1.0-1' }}</small></div>
          }
        </section>
      }

      @if (selected(); as active) {
        @if (active.scenario_status === 'draft' && canPlan()) {
          <section class="pin-panel">
            <div class="panel-header"><h2>Rack pinning</h2><span class="secondary">Fix a draft load to a rack. Pins still obey power, airflow, U, thermal zone and redundancy isolation rules.</span></div>
            <div class="pin-rows">
              @for (load of scenarioLoads(active); track load.id) {
                <div class="pin-row">
                  <div class="pin-load"><strong>{{ load.name }}</strong><small>{{ load.power_kw | number:'1.0-1' }} kW / {{ load.airflow_cfm | number:'1.0-0' }} CFM / {{ load.rack_units }}U / {{ load.redundancy_group }}</small></div>
                  <mat-form-field appearance="outline" subscriptSizing="dynamic">
                    <mat-label>Pinned rack</mat-label>
                    <mat-select [value]="pinDraftMap().get(load.id) ?? 0" (selectionChange)="setPinDraft(load.id, $event.value)">
                      <mat-option [value]="0">Auto placement</mat-option>
                      @for (rack of pinnableRacks(); track rack.id) {<mat-option [value]="rack.id">{{ rack.rack_code }} / {{ rack.zone_code }} / {{ rack.rack_status }}</mat-option>}
                    </mat-select>
                  </mat-form-field>
                </div>
              } @empty {<div class="aside-empty">This draft has no loads.</div>}
            </div>
            <div class="pin-actions"><span>{{ pinnedCount() }} pinned / {{ scenarioLoads(active).length }} loads</span><button mat-button type="button" (click)="resetPinDraft()">Reset</button><button mat-flat-button color="primary" type="button" (click)="savePins()" [disabled]="savingPins() || !pinDirty()"><lucide-icon name="pin" [size]="15" /> {{ savingPins() ? 'Saving...' : 'Save pins' }}</button></div>
          </section>
        } @else if (active.pins.length > 0) {
          <section class="pin-panel readonly">
            <div class="panel-header"><h2>Rack pinning</h2><span class="secondary">{{ active.pins.length }} pinned load(s) carried from the draft</span></div>
            <div class="pin-readonly">@for (pin of active.pins; track pin.load_id) {<span class="pin-chip"><lucide-icon name="pin" [size]="12" />load #{{ pin.load_id }} &rarr; {{ pin.rack_code }} / {{ pin.zone_code }}</span>}</div>
          </section>
        }

        <section class="metric-strip">
          <div class="metric"><small>Placement score</small><strong>{{ active.score | number:'1.0-1' }}</strong><span>Deterministic weighted score</span></div>
          <div class="metric"><small>Assigned power</small><strong>{{ active.total_power_kw | number:'1.0-1' }} kW</strong><span>{{ active.assignments.length }} placed loads</span></div>
          <div class="metric"><small>Peak return</small><strong>{{ active.peak_temp_c | number:'1.0-1' }} C</strong><span>Simplified thermal estimate</span></div>
          <div class="metric"><small>Constraint evidence</small><strong>{{ active.violations.length }}</strong><span>{{ criticalCount() }} critical</span></div>
        </section>

        <div class="planner-grid">
          <section class="layout-surface">
            <div class="surface-header"><span><strong>Candidate rack map</strong><small>Fixed rack coordinates with placement result</small></span><app-constraint-badge [severity]="active.has_critical_violation ? 'critical' : 'clear'" [label]="active.has_critical_violation ? 'Approval blocked' : 'Constraints clear'" /></div>
            <div class="candidate-grid">
              @for (rack of racks(); track rack.id) {
                <article class="candidate-rack" [class.blocked]="rack.rack_status === 'unavailable' || rack.rack_status === 'maintenance'" [class.warm]="rackHeat(rack.id) > rack.power_limit_kw * .65" [class.hot]="rackHeat(rack.id) > rack.power_limit_kw * .85">
                  <header><strong>{{ rack.rack_code }}</strong><span>{{ rack.zone_code }}</span></header>
                  <div class="assigned-loads">
                    @for (assignment of assignmentsFor(rack.id); track assignment.load_id) {
                      <div [class.pinned-assignment]="assignment.pinned"><lucide-icon [name]="assignment.pinned ? 'pin' : 'cpu'" [size]="12" /><span>{{ assignment.load_name }}</span><strong>{{ assignment.power_kw | number:'1.0-1' }} kW</strong></div>
                    } @empty {<span class="vacant">{{ rack.rack_status === 'available' ? 'Available' : rack.rack_status }}</span>}
                  </div>
                  <footer><span><lucide-icon name="zap" [size]="11" />{{ rackPower(rack.id) | number:'1.0-1' }} / {{ rack.power_limit_kw | number:'1.0-0' }}</span><span>R{{ rack.row_index }} C{{ rack.column_index }}</span></footer>
                </article>
              }
            </div>
          </section>

          <aside class="evidence-panel">
            <div class="evidence-section">
              <h2>Thermal propagation</h2>
              @for (zone of active.zone_results; track zone.zone_id) {
                <div class="thermal-row"><span><strong>{{ zone.zone_code }}</strong><small>{{ zone.assigned_heat_kw | number:'1.0-1' }} + {{ zone.neighbor_heat_kw | number:'1.0-1' }} kW adjacent</small></span><span class="temperature"><strong>{{ zone.estimated_return_c | number:'1.0-1' }} C</strong><small>{{ zone.temperature_margin_c | number:'1.0-1' }} C margin</small></span></div>
              } @empty {<div class="aside-empty">Evaluate the draft to calculate thermal propagation.</div>}
            </div>
            <div class="evidence-section violations">
              <h2>Constraint evidence</h2>
              @for (item of active.violations; track item.code + item.entity_id) {
                <div class="violation"><app-constraint-badge [severity]="item.severity" [label]="item.code" /><p>{{ item.message }}</p><small>{{ item.entity_type }} #{{ item.entity_id }} / actual {{ item.actual | number:'1.0-1' }} / limit {{ item.limit | number:'1.0-1' }}</small></div>
              } @empty {<div class="aside-empty">No reported violations.</div>}
            </div>
          </aside>
        </div>

        <section class="panel assignments-panel">
          <div class="panel-header"><h2>Placement rationale</h2><span class="secondary">Ordered deterministic result</span></div>
          <div class="table-wrap"><table class="data-table"><thead><tr><th>Load</th><th>Placement</th><th>Demand</th><th>Candidate score</th><th>Explanation</th></tr></thead><tbody>
            @for (item of active.assignments; track item.load_id) {<tr [class.pinned-row]="item.pinned"><td class="primary-cell">{{ item.load_name }}@if (item.pinned) {<span class="pinned-tag"><lucide-icon name="pin" [size]="11" /> pinned</span>}</td><td><strong>{{ item.rack_code }}</strong><br><span class="secondary">{{ item.zone_code }}</span></td><td>{{ item.power_kw | number:'1.0-1' }} kW / {{ item.airflow_cfm | number:'1.0-0' }} CFM / {{ item.rack_units }}U</td><td class="numeric">{{ item.placement_score | number:'1.0-1' }}</td><td class="secondary">{{ item.explanation.join(' / ') }}</td></tr>}
          </tbody></table></div>
        </section>
      } @else {
        <section class="panel"><div class="empty"><span><strong>No layout scenario selected</strong>Create a draft from ready equipment loads.</span></div></section>
      }
    </div>
  `,
  styles: [`
    .scenario-builder{margin-bottom:16px;padding:16px;background:#fff;border:1px solid #aeb6bd;border-top:3px solid #cf3f2e}.scenario-builder form{display:grid;grid-template-columns:320px minmax(0,1fr);gap:16px}.load-picker{display:grid;grid-template-columns:repeat(auto-fill,minmax(210px,1fr));gap:6px 12px;max-height:180px;overflow:auto;padding:2px}.load-picker strong,.load-picker small{display:block;letter-spacing:0}.load-picker strong{font-size:11px}.load-picker small{color:#68717a;font-size:9px}.builder-actions{grid-column:1/-1;display:flex;align-items:center;justify-content:flex-end;gap:8px;border-top:1px solid #d8dde1;padding-top:10px}.builder-actions>span{margin-right:auto;color:#68717a;font-size:11px}.control-bar{min-height:66px;display:flex;align-items:center;gap:12px;margin-bottom:16px;padding:9px 12px;background:#fff;border:1px solid #d8dde1}.control-bar mat-form-field{width:min(440px,40vw)}.control-spacer{flex:1}.algorithm{color:#68717a;font:600 10px/1 monospace}.pin-panel{margin-bottom:16px;padding:14px 16px;background:#fff;border:1px solid #d8dde1;border-left:3px solid #2f6f8f}.pin-panel.readonly{padding:12px 16px}.panel-header{display:flex;align-items:baseline;justify-content:space-between;gap:12px;margin-bottom:10px}.panel-header h2{margin:0;font-size:13px;text-transform:uppercase}.panel-header .secondary{font-size:10px}.pin-rows{display:grid;grid-template-columns:repeat(auto-fill,minmax(330px,1fr));gap:8px 16px}.pin-row{display:grid;grid-template-columns:minmax(0,1fr) 200px;gap:10px;align-items:center;padding:8px 10px;background:#f6f8f9;border:1px solid #e2e6e9}.pin-load strong,.pin-load small{display:block;letter-spacing:0}.pin-load strong{font-size:11px}.pin-load small{color:#68717a;font-size:9px;margin-top:2px}.pin-row mat-form-field{width:100%}.pin-actions{display:flex;align-items:center;gap:10px;margin-top:10px;border-top:1px solid #e5e8ea;padding-top:10px}.pin-actions>span{margin-right:auto;color:#68717a;font-size:11px}.pin-readonly{display:flex;flex-wrap:wrap;gap:8px}.pin-chip{display:inline-flex;align-items:center;gap:4px;padding:4px 9px;background:#eef3f6;border:1px solid #c9d7de;color:#234;font-size:10px}.pin-failures{margin-bottom:16px;padding:12px 16px;background:#fdf1ef;border:1px solid #e2b3ab;border-left:3px solid #cf3f2e}.pin-failures-head{display:flex;align-items:center;gap:8px;color:#8c2a1e;font-size:12px;margin-bottom:8px;flex-wrap:wrap}.pin-failures-head .secondary{flex-basis:100%;color:#a4554a;font-size:10px}.pin-failure{padding:9px 0;border-top:1px solid #ecd6d1}.pin-failure p{margin:6px 0 3px;font-size:11px}.pin-failure small{color:#8a5a52;font-size:9px}.planner-grid{display:grid;grid-template-columns:minmax(0,1fr) 360px;gap:16px;align-items:start}.layout-surface{min-width:0;background:#23282d;border:1px solid #101214;color:#eef1f3}.surface-header{min-height:58px;display:flex;align-items:center;justify-content:space-between;gap:16px;padding:11px 15px;border-bottom:1px solid #42494f}.surface-header strong,.surface-header small{display:block;letter-spacing:0}.surface-header strong{font-size:13px}.surface-header small{margin-top:3px;color:#aeb6bc;font-size:10px}.candidate-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(170px,1fr));gap:10px;padding:14px}.candidate-rack{height:158px;display:grid;grid-template-rows:27px 1fr 28px;min-width:0;background:#15191d;border:1px solid #596168;border-top:4px solid #4b9a77;border-radius:3px;overflow:hidden}.candidate-rack.warm{border-top-color:#d79318}.candidate-rack.hot{border-top-color:#cf3f2e}.candidate-rack.blocked{opacity:.58;border-top-color:#778087}.candidate-rack header,.candidate-rack footer{display:flex;align-items:center;justify-content:space-between;gap:6px;padding:0 8px;background:#2b3136}.candidate-rack header strong{font-size:11px}.candidate-rack header span{color:#aeb6bc;font-size:9px}.assigned-loads{display:grid;align-content:start;gap:4px;padding:7px;overflow:auto}.assigned-loads>div{display:grid;grid-template-columns:14px minmax(0,1fr) auto;align-items:center;gap:4px;padding:5px;color:#e8ecee;background:#30373c;border-left:2px solid #d79318;font-size:9px}.assigned-loads>div.pinned-assignment{border-left-color:#5db4d6;background:#27414c}.assigned-loads>div span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.assigned-loads>div strong{font-size:8px}.vacant{margin:auto;color:#7f8990;font-size:9px;text-transform:uppercase}.candidate-rack footer{color:#b8c0c5;font-size:8px}.candidate-rack footer span{display:flex;align-items:center;gap:3px}.evidence-panel{background:#fff;border:1px solid #d8dde1}.evidence-section{padding:14px}.evidence-section+ .evidence-section{border-top:1px solid #d8dde1}.evidence-section h2{margin:0 0 9px;font-size:12px;text-transform:uppercase}.thermal-row{display:flex;justify-content:space-between;gap:12px;padding:10px 0;border-bottom:1px solid #e5e8ea}.thermal-row:last-child{border-bottom:0}.thermal-row strong,.thermal-row small{display:block;letter-spacing:0}.thermal-row strong{font-size:11px}.thermal-row small{margin-top:3px;color:#68717a;font-size:9px}.temperature{text-align:right}.temperature strong{font-size:15px}.violation{padding:11px 0;border-bottom:1px solid #e5e8ea}.violation:last-child{border-bottom:0}.violation p{margin:7px 0 4px;font-size:11px;line-height:1.4}.violation>small{color:#68717a;font-size:9px}.aside-empty{padding:18px 4px;color:#68717a;font-size:11px;text-align:center}.assignments-panel{margin-top:16px}.pinned-tag{display:inline-flex;align-items:center;gap:3px;margin-left:8px;padding:1px 7px;background:#e7f1f6;border:1px solid #bcd6e2;color:#2f6f8f;font-size:9px;text-transform:uppercase;font-weight:700}tr.pinned-row td:first-child{border-left:3px solid #5db4d6}
    @media(max-width:1150px){.planner-grid{grid-template-columns:1fr}.evidence-panel{display:grid;grid-template-columns:1fr 1fr}.evidence-section+.evidence-section{border-top:0;border-left:1px solid #d8dde1}}@media(max-width:760px){.scenario-builder form{grid-template-columns:1fr}.control-bar{align-items:stretch;flex-wrap:wrap}.control-bar mat-form-field{width:100%}.control-spacer{display:none}.candidate-grid{grid-template-columns:repeat(2,minmax(0,1fr));padding:9px}.evidence-panel{grid-template-columns:1fr}.evidence-section+.evidence-section{border-left:0;border-top:1px solid #d8dde1}.pin-rows{grid-template-columns:1fr}}@media(max-width:430px){.candidate-grid{grid-template-columns:1fr}}
  `]
})
export class PlannerPage {
  readonly auth = useAuth();
  readonly evaluation = useScenarioEvaluation();
  readonly scenarios = signal<LayoutScenario[]>([]);
  readonly racks = signal<Rack[]>([]);
  readonly loads = signal<EquipmentLoad[]>([]);
  readonly selected = this.store.selected;
  readonly createOpen = signal(false);
  readonly creating = signal(false);
  readonly selectedLoadIds = signal<Set<number>>(new Set());
  readonly pinDraftMap = signal<Map<number, number>>(new Map());
  readonly pinBaseline = signal<Map<number, number>>(new Map());
  readonly savingPins = signal(false);
  readonly pinFailures = signal<ConstraintViolation[]>([]);
  readonly readyLoads = computed(() => this.loads().filter((load) => load.load_status === 'ready'));
  readonly pinnableRacks = computed(() => this.racks().filter((rack) => rack.rack_status === 'available' || rack.rack_status === 'reserved'));
  readonly criticalCount = computed(() => this.selected()?.violations.filter((item) => item.severity === 'critical').length ?? 0);
  readonly pinnedCount = computed(() => [...this.pinDraftMap().values()].filter((rackId) => rackId > 0).length);
  readonly pinDirty = computed(() => !this.mapsEqual(this.pinDraftMap(), this.pinBaseline()));
  readonly form = this.fb.nonNullable.group({name: ['', [Validators.required, Validators.minLength(3)]]});

  constructor(private readonly fb: FormBuilder, private readonly scenarioApi: ScenarioApi, private readonly rackApi: RackApi, private readonly loadApi: LoadApi, readonly store: ScenarioStore, private readonly snack: MatSnackBar) { this.load(); }
  canPlan(): boolean { return this.auth.hasRole('planner', 'admin'); }
  canApprove(): boolean { return this.auth.hasRole('reviewer', 'admin'); }
  load(): void { forkJoin({scenarios: this.scenarioApi.list(), racks: this.rackApi.list(), loads: this.loadApi.list()}).subscribe(({scenarios, racks, loads}) => { this.scenarios.set(scenarios.items); this.racks.set(racks.items); this.loads.set(loads.items); const current = this.selected(); const selected = scenarios.items.find((item) => item.id === current?.id) ?? scenarios.items[0] ?? null; this.store.select(selected); this.syncPinDraft(selected); this.pinFailures.set([]); if (this.selectedLoadIds().size === 0) this.selectedLoadIds.set(new Set(loads.items.filter((item) => item.load_status === 'ready').map((item) => item.id))); }); }
  selectScenario(id: number): void { const next = this.scenarios().find((item) => item.id === id) ?? null; this.store.select(next); this.syncPinDraft(next); this.pinFailures.set([]); }
  toggleLoad(id: number, checked: boolean): void { const next = new Set(this.selectedLoadIds()); checked ? next.add(id) : next.delete(id); this.selectedLoadIds.set(next); }
  createScenario(): void { if (this.form.invalid || this.selectedLoadIds().size === 0) return; this.creating.set(true); this.scenarioApi.create(this.form.controls.name.value, [...this.selectedLoadIds()]).pipe(finalize(() => this.creating.set(false))).subscribe((scenario) => { this.createOpen.set(false); this.form.reset(); this.scenarios.update((items) => [scenario, ...items]); this.store.select(scenario); this.syncPinDraft(scenario); this.snack.open('Draft scenario created', undefined, {duration: 2200}); }); }
  evaluate(): void { const scenario = this.selected(); if (!scenario) return; this.pinFailures.set([]); this.evaluation.evaluate(scenario).subscribe({ next: (result) => { this.store.select(result); this.scenarios.update((items) => items.map((item) => item.id === result.id ? result : item)); this.syncPinDraft(result); this.snack.open(`Evaluation complete: score ${result.score.toFixed(1)}`, undefined, {duration: 2800}); }, error: (error: unknown) => { const conflicts = this.extractPinConflicts(error); if (conflicts.length > 0) { this.pinFailures.set(conflicts); this.scenarioApi.get(scenario.id).subscribe((fresh) => { this.store.select(fresh); this.scenarios.update((items) => items.map((item) => item.id === fresh.id ? fresh : item)); this.syncPinDraft(fresh); }); } } }); }
  transition(target: ScenarioStatus): void { const scenario = this.selected(); if (!scenario) return; this.scenarioApi.transition(scenario.id, scenario.version, target, 'Reviewed in planning workbench').subscribe((result) => { this.store.select(result); this.scenarios.update((items) => items.map((item) => item.id === result.id ? result : item)); this.syncPinDraft(result); this.snack.open(`Scenario ${target.replace('_', ' ')}`, undefined, {duration: 2200}); }); }
  scenarioLoads(scenario: LayoutScenario): EquipmentLoad[] { const ids = new Set(scenario.load_ids); return this.loads().filter((load) => ids.has(load.id)); }
  setPinDraft(loadId: number, rackId: number): void { const next = new Map(this.pinDraftMap()); if (rackId > 0) { next.set(loadId, rackId); } else { next.delete(loadId); } this.pinDraftMap.set(next); }
  resetPinDraft(): void { this.pinDraftMap.set(new Map(this.pinBaseline())); }
  savePins(): void { const scenario = this.selected(); if (!scenario) return; const pins = [...this.pinDraftMap().entries()].filter(([, rackId]) => rackId > 0).map(([loadId, rackId]) => ({load_id: loadId, rack_id: rackId})).sort((a, b) => a.load_id - b.load_id); this.savingPins.set(true); this.scenarioApi.setPins(scenario.id, scenario.version, pins).pipe(finalize(() => this.savingPins.set(false))).subscribe((result) => { this.store.select(result); this.scenarios.update((items) => items.map((item) => item.id === result.id ? result : item)); this.syncPinDraft(result); this.pinFailures.set([]); this.snack.open(`Pins saved: ${pins.length} fixed placement(s)`, undefined, {duration: 2400}); }); }
  assignmentsFor(rackId: number): RackAssignment[] { return this.selected()?.assignments.filter((item) => item.rack_id === rackId) ?? []; }
  rackPower(rackId: number): number { return this.assignmentsFor(rackId).reduce((sum, item) => sum + item.power_kw, 0); }
  rackHeat(rackId: number): number { return this.assignmentsFor(rackId).reduce((sum, item) => sum + item.heat_kw, 0); }

  private syncPinDraft(scenario: LayoutScenario | null): void {
    const next = new Map<number, number>();
    scenario?.pins.forEach((pin) => next.set(pin.load_id, pin.rack_id));
    this.pinBaseline.set(new Map(next));
    this.pinDraftMap.set(next);
  }

  private mapsEqual(a: Map<number, number>, b: Map<number, number>): boolean {
    if (a.size !== b.size) { return false; }
    for (const [key, value] of a) { if (b.get(key) !== value) { return false; } }
    return true;
  }

  private extractPinConflicts(error: unknown): ConstraintViolation[] {
    if (!(error instanceof HttpErrorResponse)) { return []; }
    const payload = error.error as PinErrorEnvelope | undefined;
    if (payload?.error?.code !== 'PIN_CONFLICT' || !Array.isArray(payload.error.details)) { return []; }
    return payload.error.details;
  }
}

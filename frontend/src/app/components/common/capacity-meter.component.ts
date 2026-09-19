import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

@Component({
  selector: 'app-capacity-meter',
  standalone: true,
  imports: [DecimalPipe],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="meter" [attr.aria-label]="label() + ' ' + normalized() + '%'">
      <div class="track"><span [style.width.%]="normalized()" [class.warn]="normalized() >= 75" [class.danger]="normalized() >= 90"></span></div>
      <span class="value">{{ normalized() | number:'1.0-0' }}%</span>
    </div>
  `,
  styles: [`
    .meter{display:grid;grid-template-columns:minmax(64px,1fr) 34px;align-items:center;gap:8px;min-width:112px}
    .track{height:7px;background:#e3e6e8;border:1px solid #d1d6da;overflow:hidden}
    .track span{display:block;height:100%;background:#3e7f65;transition:width .25s ease}
    .track span.warn{background:#d79318}.track span.danger{background:#cf3f2e}
    .value{color:#4f5860;font:600 10px/1 Inter,"Segoe UI",Arial,sans-serif;text-align:right;font-variant-numeric:tabular-nums}
  `]
})
export class CapacityMeterComponent {
  readonly value = input.required<number>();
  readonly label = input('Capacity');
  readonly normalized = computed(() => Math.max(0, Math.min(100, Number.isFinite(this.value()) ? this.value() : 0)));
}

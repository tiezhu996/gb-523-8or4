import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { LucideAngularModule } from 'lucide-angular';

@Component({
  selector: 'app-constraint-badge',
  standalone: true,
  imports: [LucideAngularModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <span class="badge" [class.critical]="severity() === 'critical'" [class.clear]="severity() === 'clear'">
      <lucide-icon [name]="severity() === 'clear' ? 'check-circle-2' : 'alert-triangle'" [size]="13" />
      {{ label() }}
    </span>
  `,
  styles: [`
    .badge{display:inline-flex;align-items:center;gap:5px;min-height:24px;padding:3px 7px;border:1px solid #e0c66d;border-radius:3px;background:#fff7d6;color:#855900;font-size:10px;font-weight:750;text-transform:uppercase;white-space:nowrap}
    .badge.critical{border-color:#e1a29b;background:#fff0ee;color:#a1281e}.badge.clear{border-color:#9bcbb0;background:#eaf7ef;color:#237a4b}
  `]
})
export class ConstraintBadgeComponent {
  readonly severity = input<'critical' | 'warning' | 'clear'>('clear');
  readonly label = input.required<string>();
}

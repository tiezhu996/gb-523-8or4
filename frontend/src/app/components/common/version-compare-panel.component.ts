import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { DecimalPipe } from '@angular/common';
import { ScenarioComparison } from '../../../types/scenario';

@Component({
  selector: 'app-version-compare-panel',
  standalone: true,
  imports: [DecimalPipe],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (comparison(); as value) {
      <div class="compare-grid">
        <div><small>Baseline</small><strong>{{ value.left.name }}</strong><span>v{{ value.left.version }} / {{ value.left.score | number:'1.0-1' }} pts</span></div>
        <div class="delta"><small>Score delta</small><strong [class.negative]="value.score_delta < 0">{{ value.score_delta > 0 ? '+' : '' }}{{ value.score_delta | number:'1.0-1' }}</strong><span>{{ value.peak_temp_delta_c > 0 ? '+' : '' }}{{ value.peak_temp_delta_c | number:'1.0-1' }} C peak</span></div>
        <div><small>Candidate</small><strong>{{ value.right.name }}</strong><span>v{{ value.right.version }} / {{ value.right.score | number:'1.0-1' }} pts</span></div>
      </div>
      <ul>@for (item of value.summary; track item) { <li>{{ item }}</li> }</ul>
    } @else {
      <div class="blank">Select two evaluated scenarios to compare versions.</div>
    }
  `,
  styles: [`
    .compare-grid{display:grid;grid-template-columns:1fr 140px 1fr;border:1px solid #d8dde1;background:#fff}
    .compare-grid>div{min-width:0;padding:14px;border-right:1px solid #d8dde1}.compare-grid>div:last-child{border-right:0}.delta{text-align:center;background:#f3f5f6}
    small,strong,span{display:block;letter-spacing:0}small{color:#68717a;font-size:10px;text-transform:uppercase}strong{margin-top:5px;overflow:hidden;text-overflow:ellipsis;font-size:15px;white-space:nowrap}span{margin-top:4px;color:#68717a;font-size:11px}.delta strong{color:#237a4b;font-size:20px}.delta strong.negative{color:#b42318}
    ul{margin:12px 0 0;padding-left:18px;color:#4f5860;font-size:12px;line-height:1.7}.blank{min-height:100px;display:grid;place-items:center;color:#68717a;border:1px dashed #aeb6bd;font-size:12px;text-align:center}
    @media(max-width:600px){.compare-grid{grid-template-columns:1fr}.compare-grid>div{border-right:0;border-bottom:1px solid #d8dde1}.compare-grid>div:last-child{border-bottom:0}}
  `]
})
export class VersionComparePanelComponent {
  readonly comparison = input<ScenarioComparison | null>(null);
}

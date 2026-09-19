import { Injectable, signal } from '@angular/core';
import { LayoutScenario } from '../../types/scenario';

@Injectable({providedIn: 'root'})
export class ScenarioStore {
  readonly selected = signal<LayoutScenario | null>(null);
  readonly evaluating = signal(false);
  readonly lastEvaluatedAt = signal<Date | null>(null);

  select(scenario: LayoutScenario | null): void { this.selected.set(scenario); }
}

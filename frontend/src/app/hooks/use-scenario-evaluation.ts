import { inject } from '@angular/core';
import { finalize, Observable, switchMap, takeWhile, timer } from 'rxjs';
import { LayoutScenario } from '../../types/scenario';
import { ScenarioApi } from '../api/scenario.api';
import { ScenarioStore } from '../stores/scenario.store';

export interface ScenarioEvaluationHook {
  evaluate(scenario: LayoutScenario): Observable<LayoutScenario>;
}

export function useScenarioEvaluation(): ScenarioEvaluationHook {
  const api = inject(ScenarioApi);
  const store = inject(ScenarioStore);
  return {
    evaluate(scenario: LayoutScenario): Observable<LayoutScenario> {
      store.evaluating.set(true);
      return api.evaluate(scenario.id, scenario.version).pipe(
        switchMap((started) => started.scenario_status === 'evaluating'
          ? timer(0, 500).pipe(switchMap(() => api.get(scenario.id)), takeWhile((item) => item.scenario_status === 'evaluating', true))
          : [started]),
        finalize(() => {
          store.evaluating.set(false);
          store.lastEvaluatedAt.set(new Date());
        })
      );
    }
  };
}

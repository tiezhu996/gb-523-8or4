package constants

import "testing"

func TestScenarioStateMachine(t *testing.T) {
	tests := []struct {
		name string
		from ScenarioStatus
		to   ScenarioStatus
		want bool
	}{
		{name: "start evaluation", from: ScenarioDraft, to: ScenarioEvaluating, want: true},
		{name: "finish evaluation", from: ScenarioEvaluating, to: ScenarioPendingReview, want: true},
		{name: "approve reviewed", from: ScenarioPendingReview, to: ScenarioApproved, want: true},
		{name: "archive approved", from: ScenarioApproved, to: ScenarioArchived, want: true},
		{name: "cannot skip review", from: ScenarioDraft, to: ScenarioApproved, want: false},
		{name: "terminal immutable", from: ScenarioArchived, to: ScenarioDraft, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanTransitionScenario(tt.from, tt.to); got != tt.want {
				t.Fatalf("CanTransitionScenario(%s, %s)=%t, want %t", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

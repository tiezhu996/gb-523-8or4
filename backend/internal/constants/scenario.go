package constants

type ScenarioStatus string

const (
	ScenarioDraft         ScenarioStatus = "draft"
	ScenarioEvaluating    ScenarioStatus = "evaluating"
	ScenarioPendingReview ScenarioStatus = "pending_review"
	ScenarioApproved      ScenarioStatus = "approved"
	ScenarioArchived      ScenarioStatus = "archived"
)

var scenarioTransitions = map[ScenarioStatus]map[ScenarioStatus]bool{
	ScenarioDraft: {
		ScenarioEvaluating: true,
	},
	ScenarioEvaluating: {
		ScenarioDraft:         true,
		ScenarioPendingReview: true,
	},
	ScenarioPendingReview: {
		ScenarioDraft:    true,
		ScenarioApproved: true,
	},
	ScenarioApproved: {
		ScenarioArchived: true,
	},
	ScenarioArchived: {},
}

func ValidScenarioStatus(value ScenarioStatus) bool {
	_, ok := scenarioTransitions[value]
	return ok
}

func CanTransitionScenario(from, to ScenarioStatus) bool {
	allowed, ok := scenarioTransitions[from]
	return ok && allowed[to]
}

func ScenarioStatusValues() []ScenarioStatus {
	return []ScenarioStatus{
		ScenarioDraft,
		ScenarioEvaluating,
		ScenarioPendingReview,
		ScenarioApproved,
		ScenarioArchived,
	}
}

package booking

import "testing"

func TestScenarios(t *testing.T) {
	r := RunScenarios()
	for _, f := range r.Failures {
		t.Error(f)
	}
	if r.Passed != r.Total {
		t.Fatalf("task completion %d/%d", r.Passed, r.Total)
	}
	t.Logf("booking task completion: %d/%d", r.Passed, r.Total)
}

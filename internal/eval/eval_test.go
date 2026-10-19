package eval

import (
	"testing"

	"github.com/chinmay/switchboard/internal/escalate"
)

func TestRulePolicyOnScenarios(t *testing.T) {
	r := Run(escalate.DefaultRulePolicy(), Scenarios())
	t.Logf("\n%s", r.String())
	if r.Total == 0 {
		t.Fatal("no scenarios")
	}
	if r.TP == 0 {
		t.Fatal("policy escalated nothing")
	}
}

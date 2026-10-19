package eval

import (
	"fmt"
	"sort"

	"github.com/chinmay/switchboard/internal/escalate"
)

type Scenario struct {
	Name           string
	Signal         escalate.Signal
	ShouldEscalate bool
}

type Prediction struct {
	Name      string
	Predicted bool
	Actual    bool
}

type Report struct {
	Total                int      `json:"total"`
	TP                   int      `json:"tp"`
	FP                   int      `json:"fp"`
	TN                   int      `json:"tn"`
	FN                   int      `json:"fn"`
	Precision            float64  `json:"precision"`
	Recall               float64  `json:"recall"`
	F1                   float64  `json:"f1"`
	Accuracy             float64  `json:"accuracy"`
	FalseEscalationRate  float64  `json:"false_escalation_rate"`
	MissedEscalationRate float64  `json:"missed_escalation_rate"`
	Containment          float64  `json:"containment"`
	FalsePositives       []string `json:"false_positives"`
	Missed               []string `json:"missed"`
}

func Run(p escalate.Policy, scenarios []Scenario) Report {
	preds := make([]Prediction, len(scenarios))
	for i, s := range scenarios {
		preds[i] = Prediction{
			Name:      s.Name,
			Predicted: p.Decide(s.Signal).Escalate,
			Actual:    s.ShouldEscalate,
		}
	}
	return Score(preds)
}

func Score(preds []Prediction) Report {
	r := Report{Total: len(preds)}
	for _, p := range preds {
		switch {
		case p.Actual && p.Predicted:
			r.TP++
		case !p.Actual && p.Predicted:
			r.FP++
			r.FalsePositives = append(r.FalsePositives, p.Name)
		case p.Actual && !p.Predicted:
			r.FN++
			r.Missed = append(r.Missed, p.Name)
		default:
			r.TN++
		}
	}
	r.Precision = ratio(r.TP, r.TP+r.FP)
	r.Recall = ratio(r.TP, r.TP+r.FN)
	if r.Precision+r.Recall > 0 {
		r.F1 = 2 * r.Precision * r.Recall / (r.Precision + r.Recall)
	}
	r.Accuracy = ratio(r.TP+r.TN, r.Total)
	r.FalseEscalationRate = ratio(r.FP, r.FP+r.TN)
	r.MissedEscalationRate = ratio(r.FN, r.FN+r.TP)
	r.Containment = ratio(r.TN+r.FN, r.Total)
	sort.Strings(r.FalsePositives)
	sort.Strings(r.Missed)
	return r
}

func (r Report) String() string {
	return fmt.Sprintf(
		"scenarios=%d  TP=%d FP=%d TN=%d FN=%d\n"+
			"precision=%.3f  recall=%.3f  f1=%.3f  accuracy=%.3f\n"+
			"false_escalation_rate=%.3f  missed_escalation_rate=%.3f  containment=%.3f\n"+
			"false_positives=%v\nmissed=%v",
		r.Total, r.TP, r.FP, r.TN, r.FN,
		r.Precision, r.Recall, r.F1, r.Accuracy,
		r.FalseEscalationRate, r.MissedEscalationRate, r.Containment,
		r.FalsePositives, r.Missed,
	)
}

func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

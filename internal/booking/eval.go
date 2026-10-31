package booking

import (
	"context"
	"fmt"

	"github.com/chinmay/switchboard/internal/agent"
	"github.com/chinmay/switchboard/internal/jobs"
)

type Scenario struct {
	Name    string
	Turns   []string
	WantEnd string
	Seed    bool
	Phone   string
	Check   func(*jobs.Store) error
}

type ScenarioReport struct {
	Total    int
	Passed   int
	Failures []string
}

func Scenarios() []Scenario {
	return []Scenario{
		{
			Name:    "book",
			Turns:   []string{"my kitchen sink is leaking", "12 Main Street", "tomorrow morning", "5551234567", "yes"},
			WantEnd: "booked",
			Check: func(st *jobs.Store) error {
				if len(st.ByPhone("5551234567")) != 1 {
					return fmt.Errorf("job not created")
				}
				return nil
			},
		},
		{
			Name:    "cancel",
			Seed:    true,
			Phone:   "5559998888",
			Turns:   []string{"I need to cancel my appointment", "5559998888", "yes"},
			WantEnd: "cancelled",
			Check: func(st *jobs.Store) error {
				if len(st.ByPhone("5559998888")) != 0 {
					return fmt.Errorf("job still booked")
				}
				return nil
			},
		},
		{
			Name:    "reschedule",
			Seed:    true,
			Phone:   "5551112222",
			Turns:   []string{"I want to reschedule", "5551112222", "next tuesday afternoon", "yes"},
			WantEnd: "rescheduled",
			Check: func(st *jobs.Store) error {
				found := st.ByPhone("5551112222")
				if len(found) != 1 || found[0].Window != "afternoon" {
					return fmt.Errorf("window not changed: %+v", found)
				}
				return nil
			},
		},
		{
			Name:    "emergency",
			Turns:   []string{"I smell gas in the kitchen"},
			WantEnd: "emergency",
		},
		{
			Name:    "ungrounded_faq",
			Turns:   []string{"what is your warranty"},
			WantEnd: "ungrounded",
		},
	}
}

func RunScenarios() ScenarioReport {
	r := ScenarioReport{}
	for _, sc := range Scenarios() {
		st := jobs.NewStore()
		if sc.Seed {
			if _, err := st.Book("water heater", "2026-10-30", "morning", "9 B Street", sc.Phone); err != nil {
				r.Total++
				r.Failures = append(r.Failures, sc.Name+": seed failed")
				continue
			}
		}
		b := New(st, nil, "rivera", "Rivera Plumbing")

		var out agent.Outcome
		for _, t := range sc.Turns {
			out, _ = b.Handle(context.Background(), agent.Turn{UserText: t, Lang: "en"})
		}

		ok := out.EndReason == sc.WantEnd
		if ok && sc.Check != nil {
			if err := sc.Check(st); err != nil {
				ok = false
			}
		}
		r.Total++
		if ok {
			r.Passed++
		} else {
			r.Failures = append(r.Failures, fmt.Sprintf("%s: got %q want %q", sc.Name, out.EndReason, sc.WantEnd))
		}
	}
	return r
}

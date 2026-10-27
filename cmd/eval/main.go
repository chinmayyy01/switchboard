package main

import (
	"fmt"
	"os"

	"github.com/chinmay/switchboard/internal/booking"
	"github.com/chinmay/switchboard/internal/escalate"
	"github.com/chinmay/switchboard/internal/eval"
)

func main() {
	fail := false

	esc := eval.Run(escalate.DefaultRulePolicy(), eval.Scenarios())
	fmt.Println(esc.String())
	if esc.Precision < 0.8 || esc.Recall < 0.8 {
		fmt.Println("GATE: escalation precision/recall below 0.80")
		fail = true
	}
	fmt.Println()

	book := booking.RunScenarios()
	fmt.Printf("booking eval: task_completion=%d/%d\n", book.Passed, book.Total)
	for _, f := range book.Failures {
		fmt.Println("  FAIL:", f)
	}
	if book.Passed != book.Total {
		fmt.Println("GATE: booking task completion below 100%")
		fail = true
	}

	if fail {
		os.Exit(1)
	}
}

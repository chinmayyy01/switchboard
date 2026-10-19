package main

import (
	"fmt"

	"github.com/chinmay/switchboard/internal/escalate"
	"github.com/chinmay/switchboard/internal/eval"
)

func main() {
	report := eval.Run(escalate.DefaultRulePolicy(), eval.Scenarios())
	fmt.Println(report.String())
}

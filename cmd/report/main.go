package main

import (
	"fmt"
	"os"

	"github.com/chinmay/switchboard/internal/eval"
	"github.com/chinmay/switchboard/internal/store"
)

func main() {
	dir := os.Getenv("SWITCHBOARD_DATA")
	if dir == "" {
		dir = "data"
	}
	st := store.New(dir, 100000)

	labels := st.Labels()
	var preds []eval.Prediction
	for _, rec := range st.List() {
		if l, ok := labels[rec.ID]; ok {
			preds = append(preds, eval.Prediction{Name: rec.ID, Predicted: rec.Escalated, Actual: l})
		}
	}
	if len(preds) == 0 {
		fmt.Println("no labeled calls yet — run some calls and label them in the supervisor console")
		return
	}
	fmt.Println(eval.Score(preds).String())
}

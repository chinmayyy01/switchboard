package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/chinmay/switchboard/internal/calls"
	"github.com/chinmay/switchboard/internal/eval"
	"github.com/chinmay/switchboard/internal/jobs"
)

func main() {
	ctx := context.Background()
	db, err := jobs.OpenPostgres(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	store := calls.New(db)
	labeled, err := store.Labeled(ctx, os.Getenv("TENANT"))
	if err != nil {
		log.Fatal(err)
	}
	if len(labeled) == 0 {
		fmt.Println("no labeled calls yet — run some calls and label them in the console")
		return
	}
	preds := make([]eval.Prediction, 0, len(labeled))
	for _, item := range labeled {
		preds = append(preds, eval.Prediction{Name: item.ID, Predicted: item.Predicted, Actual: item.Actual})
	}
	fmt.Println(eval.Score(preds).String())
}

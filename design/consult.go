package design

import (
	"context"
	"fmt"
)

// Consult puts to e every task it accepts and has not answered yet, and
// stores each verdict as it arrives. It returns how many verdicts it stored,
// which is fewer than asked when the expert stopped early.
//
// A verdict that fails Check is an error, not a skip: it means the expert's
// backend is broken, and storing it would poison every later aggregate.
func Consult(ctx context.Context, e Expert, tasks []Task, st Store) (int, error) {
	byID := map[string]Task{}
	var todo []Task
	for _, t := range tasks {
		byID[t.ID] = t
		if _, done := st.Get(t.ID, e.ID()); done || !e.Accepts(t) {
			continue
		}
		todo = append(todo, t)
	}
	if len(todo) == 0 {
		return 0, nil
	}
	stored := 0
	err := e.Answer(ctx, todo, func(v Verdict) error {
		t, ok := byID[v.Task]
		switch {
		case !ok:
			return fmt.Errorf("expert %s answered task %s, which it was not given", e.ID(), v.Task)
		case v.Expert != e.ID():
			return fmt.Errorf("expert %s signed a verdict as %s", e.ID(), v.Expert)
		}
		if err := v.Check(t); err != nil {
			return fmt.Errorf("expert %s: %w", e.ID(), err)
		}
		if err := st.Put(v); err != nil {
			return err
		}
		stored++
		return nil
	})
	return stored, err
}

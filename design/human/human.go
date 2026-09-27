// Package human lets a person answer design tasks at a terminal, the way the
// ribice quiz asks its questions.
package human

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/lpapez/ribice/design"
	"github.com/lpapez/ribice/internal/prompt"
)

// Expert is a person at a terminal. Each answer is emitted as soon as it is
// given, so quitting keeps everything answered, and the next run starts at
// the first task not yet answered.
type Expert struct {
	Name string    // who is answering; part of the expert id
	In   io.Reader // what they type
	Out  io.Writer // where the tasks are shown

	// Open shows a photo, in the desktop's image viewer say. When nil, the
	// photo's path is printed and nothing else. Only a task's first photo is
	// opened, so a sitting does not pile up windows; the rest are printed.
	Open func(path string) error

	// Photos, when set, finds pictures of a task's subject to show beside
	// it, such as photos of the entity an assign task is about. They are for
	// display only and never part of the task, whose id must stay the same
	// for every expert asked it.
	Photos func(t design.Task) []string
}

func (e *Expert) ID() string { return "human:" + e.Name }

// Accepts takes every task that says enough for a person to answer it.
func (e *Expert) Accepts(t design.Task) bool {
	switch t.Kind {
	case design.Assign:
		return t.Subject.Entity != ""
	case design.Perceive:
		return t.Subject.Text != ""
	case design.Observe:
		return len(t.Subject.Photos) > 0
	}
	return false
}

// reasonKeys are the keys for abstaining, in the order they are shown.
var reasonKeys = []struct {
	key    string
	reason design.Reason
	label  string
}{
	{"n", design.NotObservable, "(n)ot observable"},
	{"a", design.Ambiguous, "(a)mbiguous"},
	{"c", design.UnclearQuestion, "(c) unclear question"},
	{"d", design.Unknown, "(d)on't know"},
}

func (e *Expert) Answer(ctx context.Context, tasks []design.Task, emit func(design.Verdict) error) error {
	in := bufio.NewScanner(e.In)
	fmt.Fprintf(e.Out, "%d to answer, as %s.\n", len(tasks), e.ID())
	e.help()
	for i := 0; i < len(tasks); {
		if err := ctx.Err(); err != nil {
			return err
		}
		t := tasks[i]
		e.show(t, i, len(tasks))
		for {
			fmt.Fprint(e.Out, "> ")
			if !in.Scan() {
				if err := in.Err(); err != nil {
					return err
				}
				fmt.Fprintf(e.Out, "\ninput ended with %d of %d answered; run this in a terminal to answer the rest.\n", i, len(tasks))
				return nil
			}
			line := strings.ToLower(strings.TrimSpace(in.Text()))
			if line == "q" || line == "quit" {
				return nil
			}
			if line == "b" || line == "back" {
				if i == 0 {
					fmt.Fprintln(e.Out, "   nothing before this one.")
					continue
				}
				i--
				break
			}
			v, ok := e.read(t, line)
			if !ok {
				e.help()
				continue
			}
			if err := emit(v); err != nil {
				return err
			}
			i++
			break
		}
	}
	fmt.Fprintln(e.Out, "\nAll answered.")
	return nil
}

// read turns a typed line into a verdict on t, or reports that it is not an
// answer.
func (e *Expert) read(t design.Task, line string) (design.Verdict, bool) {
	for _, r := range reasonKeys {
		if line == r.key {
			return design.Abstention(t.ID, e.ID(), r.reason), true
		}
	}
	conf := design.High
	if unsure := strings.TrimSuffix(line, "?"); unsure != line {
		conf, line = design.Low, unsure
	}
	picks := prompt.ParsePicks(line, len(t.Options))
	most := max(t.MaxChoices, 1)
	if picks == nil || len(picks) > most {
		if len(picks) > most {
			fmt.Fprintf(e.Out, "   at most %d here.\n", most)
		}
		return design.Verdict{}, false
	}
	p, err := design.Picks(t.Options, picks, conf)
	if err != nil {
		return design.Verdict{}, false
	}
	return design.Verdict{Task: t.ID, Expert: e.ID(), P: p}, true
}

func (e *Expert) show(t design.Task, i, n int) {
	fmt.Fprintf(e.Out, "\n[%s %d/%d]", t.Kind, i+1, n)
	switch t.Kind {
	case design.Assign:
		fmt.Fprintf(e.Out, "  %s\n", t.Subject.Entity)
		if t.Subject.Text != "" {
			fmt.Fprintf(e.Out, "%s\n", t.Subject.Text)
		}
		if e.Photos != nil {
			e.showPhotos(e.Photos(t))
		}
	case design.Perceive:
		fmt.Fprintf(e.Out, "\nWhat is really there: %s\n", t.Subject.Text)
		fmt.Fprintln(e.Out, "Imagine you know nothing about it and take a quick look. What would you answer?")
	case design.Observe:
		fmt.Fprintln(e.Out)
		e.showPhotos(t.Subject.Photos)
	}
	fmt.Fprintf(e.Out, "\n%s\n", t.Question)
	for j, o := range t.Options {
		fmt.Fprintf(e.Out, "  %2d) %s\n", j+1, o.Label)
	}
}

// showPhotos lists photos, opening the first.
func (e *Expert) showPhotos(paths []string) {
	for i, p := range paths {
		fmt.Fprintf(e.Out, "photo  %s\n", p)
		if i == 0 && e.Open != nil {
			if err := e.Open(p); err != nil {
				fmt.Fprintf(e.Out, "       (could not open it: %v)\n", err)
			}
		}
	}
}

func (e *Expert) help() {
	var keys []string
	for _, r := range reasonKeys {
		keys = append(keys, r.label)
	}
	fmt.Fprintln(e.Out, "Answer with a number, or two (1,3); add ? if unsure (3?).")
	fmt.Fprintf(e.Out, "Or: %s, (b)ack, (q)uit.\n", strings.Join(keys, "  "))
}

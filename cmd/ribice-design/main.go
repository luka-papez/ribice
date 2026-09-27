// Command ribice-design finds better questions for a knowledge base: see
// specs/question-design.md. Each step is a subcommand that reads and writes
// files, so any one can be rerun alone.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/lpapez/ribice/design"
	"github.com/lpapez/ribice/design/claude"
	"github.com/lpapez/ribice/design/human"
	"github.com/lpapez/ribice/kb"
)

const usage = `usage: ribice-design <command> [flags]

commands:
  tasks     make tasks for a knowledge base's current questions
  consult   put tasks to an expert, storing each verdict as it comes

Run ribice-design <command> -h for its flags.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var err error
	switch cmd, args := os.Args[1], os.Args[2:]; cmd {
	case "tasks":
		err = tasks(args)
	case "consult":
		err = consult(ctx, args)
	case "-h", "-help", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func tasks(args []string) error {
	fs := flag.NewFlagSet("tasks", flag.ExitOnError)
	var (
		path   = fs.String("kb", "data/clouds.json", "knowledge base")
		kind   = fs.String("kind", "assign", "assign or perceive")
		entity = fs.String("entity", "", "only tasks about this entity")
		attr   = fs.String("attr", "", "only tasks about this attribute")
	)
	fs.Parse(args)
	k, err := kb.LoadFile(*path)
	if err != nil {
		return err
	}
	all, err := design.FromKB(k, design.Kind(*kind))
	if err != nil {
		return err
	}
	var out []design.Task
	for _, t := range all {
		if (*entity == "" || t.Subject.Entity == *entity) && (*attr == "" || t.Attribute == *attr) {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return fmt.Errorf("no tasks match")
	}
	return design.WriteTasks(os.Stdout, out)
}

func consult(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("consult", flag.ExitOnError)
	var (
		tasksPath = fs.String("tasks", "", "tasks file, as written by the tasks command (- for stdin)")
		storePath = fs.String("store", "tools/design/verdicts.jsonl", "verdict store")
		expert    = fs.String("expert", "", "claude or human")

		model     = fs.String("model", "claude-opus-5", "claude: model")
		effort    = fs.String("effort", "low", "claude: effort")
		variant   = fs.String("variant", "a", "claude: prompt wording, a, b or c; all runs each in turn")
		domain    = fs.String("domain", "", "claude: what the knowledge base is about, plural, e.g. clouds")
		jobs      = fs.Int("jobs", 4, "claude: calls at once")
		maxWindow = fs.Float64("max-window", 0.8, "claude: stop starting calls once the five-hour window is this full")

		as   = fs.String("as", "", "human: your name, as it goes into the expert id")
		open = fs.Bool("open", false, "human: open photos in the desktop's image viewer")
	)
	fs.Parse(args)
	if *tasksPath == "" {
		return fmt.Errorf("-tasks is required")
	}
	ts, err := readTasks(*tasksPath)
	if err != nil {
		return err
	}
	st, err := design.OpenStore(*storePath)
	if err != nil {
		return err
	}

	var experts []design.Expert
	switch *expert {
	case "claude":
		if *domain == "" {
			return fmt.Errorf("-domain is required for claude, e.g. -domain clouds")
		}
		client := &claude.Client{MaxWindow: *maxWindow}
		variants := []string{*variant}
		if *variant == "all" {
			variants = []string{"a", "b", "c"}
		}
		for _, v := range variants {
			experts = append(experts, &claude.Expert{Client: client, Model: *model, Effort: *effort,
				Variant: v, Domain: *domain, Jobs: *jobs, Log: os.Stderr})
		}
	case "human":
		if *as == "" {
			return fmt.Errorf("-as is required for human: whose answers these are")
		}
		h := &human.Expert{Name: *as, In: os.Stdin, Out: os.Stdout}
		if *open {
			h.Open = func(p string) error { return exec.Command("xdg-open", p).Start() }
		}
		experts = append(experts, h)
	default:
		return fmt.Errorf("-expert must be claude or human")
	}

	for _, e := range experts {
		n, err := design.Consult(ctx, e, ts, st)
		fmt.Fprintf(os.Stderr, "%s: %d verdicts stored in %s\n", e.ID(), n, *storePath)
		if err != nil {
			return err
		}
	}
	return nil
}

func readTasks(path string) ([]design.Task, error) {
	if path == "-" {
		return design.ReadTasks(os.Stdin)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	ts, err := design.ReadTasks(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(ts) == 0 {
		return nil, fmt.Errorf("%s: no tasks", strings.TrimSpace(path))
	}
	return ts, nil
}

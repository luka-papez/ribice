// Command ribice-design finds better questions for a knowledge base: see
// specs/question-design.md. Each step is a subcommand that reads and writes
// files, so any one can be rerun alone.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/lpapez/ribice/design"
	"github.com/lpapez/ribice/design/claude"
	"github.com/lpapez/ribice/design/human"
	"github.com/lpapez/ribice/engine"
	"github.com/lpapez/ribice/kb"
)

const usage = `usage: ribice-design <command> [flags]

commands:
  analyse   score a knowledge base and find the pairs it mixes up
  propose   ask Claude for new questions aimed at those pairs
  tasks     make tasks for current or proposed questions
  consult   put tasks to an expert, storing each verdict as it comes
  aggregate settle each value against the verdicts; write the disputes as tasks

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
	case "analyse":
		err = analyse(args)
	case "propose":
		err = propose(ctx, args)
	case "tasks":
		err = tasks(args)
	case "consult":
		err = consult(ctx, args)
	case "aggregate":
		err = aggregate(args)
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
		props  = fs.String("proposals", "", "make tasks for these proposed questions instead of the current ones")
		kind   = fs.String("kind", "assign", "assign or perceive")
		entity = fs.String("entity", "", "only tasks about this entity")
		attr   = fs.String("attr", "", "only tasks about this attribute")
	)
	fs.Parse(args)
	k, err := kb.LoadFile(*path)
	if err != nil {
		return err
	}
	var all []design.Task
	if *props != "" {
		set, err := readProposals(*props)
		if err != nil {
			return err
		}
		all, err = design.FromProposals(k, set.Attributes, design.Kind(*kind))
	} else {
		all, err = design.FromKB(k, design.Kind(*kind))
	}
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

func aggregate(args []string) error {
	fs := flag.NewFlagSet("aggregate", flag.ExitOnError)
	var (
		path      = fs.String("kb", "data/clouds.json", "knowledge base whose values are settled")
		tasksPath = fs.String("tasks", "", "assign tasks, as written by the tasks command")
		props     = fs.String("proposals", "", "settle against these proposals' values instead of the knowledge base's")
		storePath = fs.String("store", "tools/design/verdicts.jsonl", "verdict store")
		outDir    = fs.String("out", "", "directory for settled.md and disputes.jsonl; default: next to -tasks")
	)
	fs.Parse(args)
	if *tasksPath == "" {
		return fmt.Errorf("-tasks is required")
	}
	k, err := kb.LoadFile(*path)
	if err != nil {
		return err
	}
	ts, err := readTasks(*tasksPath)
	if err != nil {
		return err
	}
	st, err := design.OpenStore(*storePath)
	if err != nil {
		return err
	}
	dir := *outDir
	if dir == "" {
		dir = filepath.Dir(*tasksPath)
	}

	proposed := design.KBValues(k)
	if *props != "" {
		set, err := readProposals(*props)
		if err != nil {
			return err
		}
		proposed = design.ProposedValues(set.Attributes)
	}
	settled := design.Settle(ts, proposed, st.Verdicts)
	var disputes []design.Task
	for _, s := range settled {
		if s.Status == design.Disputed {
			disputes = append(disputes, s.Task)
		}
	}
	f, err := os.Create(filepath.Join(dir, "disputes.jsonl"))
	if err != nil {
		return err
	}
	if err := design.WriteTasks(f, disputes); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	report := settledReport(k, settled)
	if err := os.WriteFile(filepath.Join(dir, "settled.md"), []byte(report), 0o644); err != nil {
		return err
	}
	counts := map[design.Status]int{}
	for _, s := range settled {
		counts[s.Status]++
	}
	fmt.Printf("%d values: %d agreed, %d disputed, %d abstained, %d not asked yet\n",
		len(settled), counts[design.Agreed], counts[design.Disputed], counts[design.Abstained], counts[design.Unasked])
	fmt.Printf("wrote %s and %s\n", filepath.Join(dir, "settled.md"), filepath.Join(dir, "disputes.jsonl"))
	return nil
}

// settledReport is a table per attribute, then every value not agreed.
func settledReport(k *kb.KB, settled []design.Settlement) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s: the answer key against the experts\n\n", k.Name)
	b.WriteString("Agreed: the experts' combined answer backs the knowledge base's value. ")
	b.WriteString("Disputed: it leans elsewhere. Abstained: most experts could not answer.\n\n")
	b.WriteString("| attribute | agreed | disputed | abstained | not asked |\n| --- | --- | --- | --- | --- |\n")
	type row struct{ agreed, disputed, abstained, unasked int }
	rows := map[string]*row{}
	for _, s := range settled {
		r := rows[s.Task.Attribute]
		if r == nil {
			r = &row{}
			rows[s.Task.Attribute] = r
		}
		switch s.Status {
		case design.Agreed:
			r.agreed++
		case design.Disputed:
			r.disputed++
		case design.Abstained:
			r.abstained++
		default:
			r.unasked++
		}
	}
	for _, a := range k.Attributes {
		if r := rows[a.Name]; r != nil {
			fmt.Fprintf(&b, "| %s | %d | %d | %d | %d |\n", a.Name, r.agreed, r.disputed, r.abstained, r.unasked)
		}
	}

	b.WriteString("\n## Not agreed\n\n| entity | attribute | knowledge base | experts | abstain |\n| --- | --- | --- | --- | --- |\n")
	for _, s := range settled {
		if s.Status != design.Disputed && s.Status != design.Abstained {
			continue
		}
		var kbv []string
		for _, v := range s.Proposed {
			kbv = append(kbv, string(v))
		}
		experts := fmt.Sprintf("%s %.0f%%", s.Top(), 100*s.Combined[s.Top()])
		abstain := fmt.Sprintf("%.0f%%", 100*s.Abstain)
		if s.Abstain > 0 {
			abstain += " " + string(s.MainReason())
		}
		who := ""
		if s.ByPerson {
			who = " (person)"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s%s | %s |\n", s.Task.Subject.Entity, s.Task.Attribute,
			strings.Join(kbv, ", "), experts, who, abstain)
	}
	return b.String()
}

func analyse(args []string) error {
	fs := flag.NewFlagSet("analyse", flag.ExitOnError)
	var (
		path  = fs.String("kb", "data/clouds.json", "knowledge base")
		seeds = fs.Int("seeds", 20, "games per entity, each with its own seed")
		out   = fs.String("out", "", "where to write targets.json (required)")
	)
	fs.Parse(args)
	if *out == "" {
		return fmt.Errorf("-out is required")
	}
	k, err := kb.LoadFile(*path)
	if err != nil {
		return err
	}
	a := design.Analyse(k, engine.DefaultConfig(), *seeds)
	a.KB = *path
	if err := writeJSON(*out, a); err != nil {
		return err
	}
	sc := a.Score
	fmt.Printf("%s, answered with its own error model, %d games:\n", *path, sc.Games)
	fmt.Printf("  identified %.0f%%, %.1f questions, gave up %.0f%%: score %.1f\n",
		100*sc.Accuracy, sc.Questions, 100*sc.GaveUp, sc.Value)
	fmt.Printf("  %d pairs mixed up; the most:\n", len(a.Pairs))
	for i, p := range a.Pairs {
		if i == 10 {
			break
		}
		fmt.Printf("  %4d  %s / %s  (never overlap on: %s)\n", p.Mixups, p.A, p.B, strings.Join(p.SeparatedBy, ", "))
	}
	fmt.Printf("wrote %s\n", *out)
	return nil
}

func propose(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("propose", flag.ExitOnError)
	var (
		path     = fs.String("kb", "data/clouds.json", "knowledge base")
		targets  = fs.String("targets", "", "targets.json from analyse (required)")
		evidence = fs.String("evidence", "", "comma-separated files of evidence on the current questions, such as settled.md")
		maxPairs = fs.Int("max-pairs", 40, "most mixed-up pairs to show the proposer")
		domain   = fs.String("domain", "", "what the knowledge base is about, plural, e.g. clouds (required)")
		model    = fs.String("model", "claude-opus-5", "model")
		effort   = fs.String("effort", "high", "effort")
		out      = fs.String("out", "", "where to write proposals.json (required)")
	)
	fs.Parse(args)
	if *targets == "" || *domain == "" || *out == "" {
		return fmt.Errorf("-targets, -domain and -out are required")
	}
	k, err := kb.LoadFile(*path)
	if err != nil {
		return err
	}
	var a design.Analysis
	if err := readJSON(*targets, &a); err != nil {
		return err
	}
	pairs := a.Pairs
	if len(pairs) > *maxPairs {
		pairs = pairs[:*maxPairs]
	}
	var ev strings.Builder
	if *evidence != "" {
		for _, f := range strings.Split(*evidence, ",") {
			b, err := os.ReadFile(strings.TrimSpace(f))
			if err != nil {
				return err
			}
			fmt.Fprintf(&ev, "## From %s\n\n%s\n\n", filepath.Base(f), b)
		}
	}

	client := &claude.Client{Timeout: 30 * time.Minute, Progress: func(pr claude.Progress) {
		fmt.Fprintf(os.Stderr, "\r  %s: thinking %d characters, answer %d characters so far   ",
			pr.Elapsed.Round(time.Second), pr.Thinking, pr.Writing)
	}}
	p := &claude.Proposer{Client: client, Model: *model, Effort: *effort, Domain: *domain, Log: os.Stderr}
	fmt.Fprintf(os.Stderr, "asking %s for questions (one call at effort %s; this takes a while)\n", p.ID(), *effort)
	props, err := p.Propose(ctx, design.Brief{KB: k, Targets: pairs, Evidence: ev.String()})
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return err
	}
	if err := writeJSON(*out, design.ProposalSet{Proposer: p.ID(), KB: *path, Attributes: props}); err != nil {
		return err
	}
	st := p.Client.Status()
	fmt.Printf("%d questions proposed, about $%.2f at API prices; wrote %s\n", len(props), st.Cost, *out)
	for _, pr := range props {
		re := ""
		if pr.Replaces != "" {
			re = " (replaces " + pr.Replaces + ")"
		}
		fmt.Printf("  %s %s%s: %s\n", pr.ID, pr.Name, re, pr.Question)
	}
	return nil
}

func readProposals(path string) (design.ProposalSet, error) {
	var set design.ProposalSet
	err := readJSON(path, &set)
	return set, err
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
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

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
  select    choose the questions that score best, and write the candidate

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
	case "select":
		err = selectQuestions(args)
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

		as     = fs.String("as", "", "human: your name, as it goes into the expert id")
		open   = fs.Bool("open", false, "human: open each task's first photo in the desktop's image viewer")
		corpus = fs.String("corpus", "", "human: show each entity's reference photo from data/<corpus>.json, then photos from its calibration corpus, e.g. clouds")
		photos = fs.Int("photos", 3, "human, with -corpus: most photos to show per entity")
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
		if *corpus != "" {
			byEntity, err := corpusPhotos(*corpus, *photos)
			if err != nil {
				return err
			}
			h.Photos = func(t design.Task) []string { return byEntity[t.Subject.Entity] }
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
		perceive  = fs.String("perceive", "", "with -proposals: perceive tasks; also fit error rates and write pool.json")
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
	var set design.ProposalSet
	if *props != "" {
		if set, err = readProposals(*props); err != nil {
			return err
		}
		proposed = design.ProposedValues(set.Attributes)
	} else if *perceive != "" {
		return fmt.Errorf("-perceive needs -proposals")
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
	fmt.Printf("%d values: %d agreed, %d corrected by a person, %d disputed, %d abstained, %d not asked yet\n",
		len(settled), counts[design.Agreed], counts[design.Corrected], counts[design.Disputed], counts[design.Abstained], counts[design.Unasked])
	fmt.Printf("wrote %s and %s\n", filepath.Join(dir, "settled.md"), filepath.Join(dir, "disputes.jsonl"))

	if *perceive == "" {
		return nil
	}
	pts, err := readTasks(*perceive)
	if err != nil {
		return err
	}
	pool := design.BuildPool(set.Attributes, ts, pts, st.Verdicts)
	if err := writeJSON(filepath.Join(dir, "pool.json"), pool); err != nil {
		return err
	}
	table := design.PoolReport(pool)
	md := "# Proposed questions after consultation\n\n" +
		"Agreed: values the experts back. To settle: disputed or abstained values, for a person. " +
		"Answer rate, noise, confusion and look-alikes are fitted from the perceive answers.\n\n" + table
	if err := os.WriteFile(filepath.Join(dir, "pool.md"), []byte(md), 0o644); err != nil {
		return err
	}
	ready := 0
	for _, e := range pool {
		if e.Ready {
			ready++
		}
	}
	fmt.Printf("%d of %d questions ready for select; wrote %s and pool.md\n", ready, len(pool), filepath.Join(dir, "pool.json"))
	return nil
}

// settledReport is a table per attribute, then every value not agreed.
func settledReport(k *kb.KB, settled []design.Settlement) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s: the answer key against the experts\n\n", k.Name)
	b.WriteString("Agreed: the experts' combined answer backs the knowledge base's value. ")
	b.WriteString("Corrected: a person answered, with another value. ")
	b.WriteString("Disputed: it leans elsewhere. Abstained: most experts could not answer.\n\n")
	b.WriteString("| attribute | agreed | corrected | disputed | abstained | not asked |\n| --- | --- | --- | --- | --- | --- |\n")
	type row struct{ agreed, corrected, disputed, abstained, unasked int }
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
		case design.Corrected:
			r.corrected++
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
			fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d |\n", a.Name, r.agreed, r.corrected, r.disputed, r.abstained, r.unasked)
		}
	}

	b.WriteString("\n## Not as proposed\n\n| entity | attribute | knowledge base | experts | abstain |\n| --- | --- | --- | --- | --- |\n")
	for _, s := range settled {
		if s.Status != design.Disputed && s.Status != design.Abstained && s.Status != design.Corrected {
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
		maxOp = fs.Int("max-options", design.MaxOptions, "most answers the quiz offers at once; more fold into \"something else\"")
	)
	fs.Parse(args)
	if *out == "" {
		return fmt.Errorf("-out is required")
	}
	k, err := kb.LoadFile(*path)
	if err != nil {
		return err
	}
	cfg := engine.DefaultConfig()
	cfg.MaxOptions = *maxOp
	a := design.Analyse(k, cfg, *seeds)
	a.KB = *path
	if err := writeJSON(*out, a); err != nil {
		return err
	}
	sc := a.Score
	fmt.Printf("%s, answered with its own error model, %d games:\n", *path, sc.Games)
	fmt.Printf("  identified %.0f%%, %.1f questions, %.0f words read, gave up %.0f%%: score %.1f\n",
		100*sc.Accuracy, sc.Questions, sc.Words, 100*sc.GaveUp, sc.Value)
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

func selectQuestions(args []string) error {
	fs := flag.NewFlagSet("select", flag.ExitOnError)
	var (
		path     = fs.String("kb", "data/clouds.json", "knowledge base to start from")
		poolPath = fs.String("pool", "", "pool.json from aggregate (required)")
		out      = fs.String("out", "", "directory for candidate.json, select.md and settle.jsonl (required)")
		seeds    = fs.Int("seeds", 10, "games per entity to screen each candidate move")
		confirm  = fs.Int("confirm-seeds", 100, "games per entity a move's gain must hold up over to be taken")
		minGain  = fs.Float64("min-gain", 2, "score points a move must add, when confirmed")
		disputed = fs.Bool("allow-disputed", false, "use questions with values still to settle, with the proposer's values")
		guessed  = fs.Bool("allow-guessed", false, "use questions without perceive answers, with default error rates")
		refresh  = fs.Bool("refresh", true, "first rewrite questions already in the knowledge base from their pool entries: settled values, fitted error rates")
		refit    = fs.String("refit", "", "comma-separated perceive task files: first refit the error rates of current questions from them")
		store    = fs.String("store", "tools/design/verdicts.jsonl", "verdict store, for -refit")
	)
	fs.Parse(args)
	if *poolPath == "" || *out == "" {
		return fmt.Errorf("-pool and -out are required")
	}
	data, err := os.ReadFile(*path)
	if err != nil {
		return err
	}
	base, err := design.ParseKBFile(data)
	if err != nil {
		return fmt.Errorf("%s: %w", *path, err)
	}
	var pool []design.PoolEntry
	if err := readJSON(*poolPath, &pool); err != nil {
		return err
	}

	// Bring the starting point up to date, so the score it is compared with
	// is measured the same way as the candidates'.
	var prep strings.Builder
	current, err := kb.Load(base.Bytes())
	if err != nil {
		return err
	}
	refreshed := map[string]bool{}
	if *refresh {
		for _, e := range pool {
			if current.Attr(e.Proposal.Name) == nil {
				continue
			}
			m, guess := design.ModelFor(e)
			if err := base.ReplaceAttribute(e.Proposal, m); err != nil {
				return err
			}
			refreshed[e.Proposal.Name] = true
			how := "fitted error rates"
			if guess {
				how = "default error rates, no perceive answers yet"
			}
			fmt.Fprintf(&prep, "- refreshed **%s** from the pool: settled values, %s\n", e.Proposal.Name, how)
		}
	}
	if *refit != "" {
		st, err := design.OpenStore(*store)
		if err != nil {
			return err
		}
		var tasks []design.Task
		for _, f := range strings.Split(*refit, ",") {
			ts, err := readTasks(strings.TrimSpace(f))
			if err != nil {
				return err
			}
			tasks = append(tasks, ts...)
		}
		fitted, missing := design.Refit(current, tasks, st.Verdicts)
		for _, a := range current.Attributes {
			if refreshed[a.Name] {
				continue
			}
			if m, ok := fitted[a.Name]; ok {
				if err := base.SetErrors(a.Name, m); err != nil {
					return err
				}
				fmt.Fprintf(&prep, "- refitted **%s**: noise %.2f → %.2f, answer rate %.0f%%\n",
					a.Name, a.Noise, m.Noise, 100*m.AnswerRate)
			} else if vs, ok := missing[a.Name]; ok {
				fmt.Fprintf(&prep, "- kept **%s**'s error rates: no perceive answers for %v\n", a.Name, vs)
			}
		}
	}
	fmt.Print(prep.String())

	start := time.Now()
	// Games are played as the site plays them: at most design.MaxOptions
	// answers offered at once.
	cfg := engine.DefaultConfig()
	cfg.MaxOptions = design.MaxOptions
	r, err := design.Select(base, pool, cfg, design.SelectOptions{Seeds: *seeds,
		ConfirmSeeds: *confirm, MinGain: *minGain, AllowDisputed: *disputed, AllowGuessed: *guessed})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(*out, "candidate.json"), r.File, 0o644); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(*out, "select.json"), r); err != nil {
		return err
	}
	md := "# Selection from " + *poolPath + "\n\n"
	if prep.Len() > 0 {
		md += "## Starting point\n\nBefore the search, the current knowledge base was brought up to date:\n\n" + prep.String() + "\n"
	}
	md += design.SelectReport(r, pool)
	if err := os.WriteFile(filepath.Join(*out, "select.md"), []byte(md), 0o644); err != nil {
		return err
	}

	// The values still to settle on the questions chosen, as tasks for a person.
	k, err := kb.LoadFile(*path)
	if err != nil {
		return err
	}
	var settle []design.Task
	for _, e := range pool {
		if r.Pending[e.Proposal.Name] == 0 {
			continue
		}
		who := map[string]bool{}
		for _, n := range append(append([]string(nil), e.Disputed...), e.Abstained...) {
			who[n] = true
		}
		ts, err := design.FromProposals(k, []design.Proposal{e.Proposal}, design.Assign)
		if err != nil {
			return err
		}
		for _, t := range ts {
			if who[t.Subject.Entity] {
				settle = append(settle, t)
			}
		}
	}
	f, err := os.Create(filepath.Join(*out, "settle.jsonl"))
	if err != nil {
		return err
	}
	if err := design.WriteTasks(f, settle); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	fmt.Printf("before: identified %.0f%% in %.1f questions, %.0f words read, score %.1f\n", 100*r.Before.Accuracy, r.Before.Questions, r.Before.Words, r.Before.Value)
	for _, s := range r.Steps {
		fmt.Printf("  %-50s score %.1f\n", s.Move, s.Score.Value)
	}
	fmt.Printf("after:  identified %.0f%% in %.1f questions, %.0f words read, score %.1f  (%s)\n", 100*r.After.Accuracy, r.After.Questions,
		r.After.Words, r.After.Value, time.Since(start).Round(time.Second))
	fmt.Printf("%d values to settle on the questions chosen; wrote %s\n", len(settle), *out)
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

// corpusPhotos finds, for each entity, the reference photo the quiz shows
// for it (a link, from data/<name>.json), then up to n photos of it in the
// calibration corpus (tools/calib/corpus/<name>): those the label check
// found to match first, then those it was unsure of; never one it said shows
// something else, one a person dropped, or one not downloaded here.
func corpusPhotos(name string, n int) (map[string][]string, error) {
	out := map[string][]string{}
	if k, err := kb.LoadFile(filepath.Join("data", name+".json")); err == nil {
		for _, e := range k.Entities {
			if e.Image != nil && e.Image.URL != "" {
				out[e.Name] = []string{e.Image.URL}
			}
		}
	}
	dir := filepath.Join("tools", "calib", "corpus", name)
	var manifest struct {
		Photos []struct {
			ID    string `json:"id"`
			File  string `json:"file"`
			Label string `json:"label"`
		} `json:"photos"`
	}
	if err := readJSON(filepath.Join(dir, "manifest.json"), &manifest); err != nil {
		return nil, err
	}
	var check map[string]struct {
		Matches string `json:"matches"`
	}
	if err := readJSON(filepath.Join(dir, "label-check.json"), &check); err != nil {
		return nil, err
	}
	review := map[string]struct {
		Decision *string `json:"decision"`
	}{}
	if err := readJSON(filepath.Join(dir, "review.json"), &review); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	taken := map[string]int{}
	for _, rank := range []string{"yes", "unsure"} {
		for _, p := range manifest.Photos {
			if check[p.ID].Matches != rank || taken[p.Label] >= n {
				continue
			}
			if r, ok := review[p.File]; ok && r.Decision != nil && *r.Decision == "drop" {
				continue
			}
			path := filepath.Join("tools", "calib", "cache", "photos", name, p.ID+".jpg")
			if _, err := os.Stat(path); err != nil {
				continue
			}
			out[p.Label] = append(out[p.Label], path)
			taken[p.Label]++
		}
	}
	return out, nil
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

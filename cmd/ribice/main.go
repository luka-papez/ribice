// Command ribice plays a twenty-questions style identification quiz over a
// JSON knowledge base.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/lpapez/ribice/engine"
	"github.com/lpapez/ribice/internal/prompt"
	"github.com/lpapez/ribice/kb"
)

func main() {
	cfg := engine.DefaultConfig()
	var (
		path     = flag.String("kb", "data/adriatic-fish.json", "path to the knowledge base JSON file")
		top      = flag.Int("top", 3, "how many candidates to show")
		noise    = flag.Float64("noise", 0, "override the default per-question error rate (0 keeps the knowledge base's own)")
		explain  = flag.Bool("explain", false, "show information gain for every question")
		doLint   = flag.Bool("lint", false, "check the knowledge base for problems and exit")
		doStats  = flag.Bool("stats", false, "summarise the knowledge base and exit")
		doSim    = flag.Bool("simulate", false, "self-test: play one game per entity and report, then exit")
		simNoise = flag.Float64("sim-noise", 0, "probability the simulated user answers wrongly")
		simModel = flag.Bool("sim-model", false, "with -simulate: err and skip as each question's own noise, confusion and answer_rate say (overrides -sim-noise)")
		seed     = flag.Int64("seed", 1, "random seed for -simulate")
		replayF  = flag.String("replay", "", "play one game per recorded sighting in this JSON Lines file (- for stdin), report, then exit")
		asJSON   = flag.Bool("json", false, "with -simulate or -replay: print one JSON object per game instead of a summary")
	)
	flag.Float64Var(&cfg.Threshold, "threshold", cfg.Threshold, "stop once a candidate reaches this probability")
	flag.Float64Var(&cfg.MinGain, "min-gain", cfg.MinGain, "stop once the best question is worth fewer bits than this")
	flag.IntVar(&cfg.MaxOptions, "max-options", cfg.MaxOptions, "most choices to offer for one question")
	flag.IntVar(&cfg.MaxQuestions, "max-questions", cfg.MaxQuestions, "hard limit on questions asked (0 = unlimited)")
	flag.Float64Var(&cfg.UnknownPrior, "unknown", cfg.UnknownPrior, "prior that the thing is not in the knowledge base at all (0 disables)")
	flag.IntVar(&cfg.Confirm, "confirm", cfg.Confirm, "questions to spend testing the leading candidate before declaring it")
	flag.IntVar(&cfg.SkipCooldown, "skip-cooldown", cfg.SkipCooldown, "questions to wait before re-offering a skipped question")
	flag.Parse()

	k, err := kb.LoadFile(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if *noise > 0 {
		for _, a := range k.Attributes {
			if !a.NoiseSet {
				a.Noise = *noise
			}
		}
	}

	switch {
	case *doLint:
		os.Exit(lint(k))
	case *doStats:
		stats(k)
	case *doSim:
		simulate(k, cfg, engine.SimOptions{Noise: *simNoise, ErrorModel: *simModel, Seed: *seed}, *asJSON)
	case *replayF != "":
		if err := replay(k, cfg, *replayF, *asJSON); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	default:
		quiz(k, cfg, *top, *explain)
	}
}

// --- quiz ------------------------------------------------------------------

func quiz(k *kb.KB, cfg engine.Config, top int, explain bool) {
	s := engine.New(k, cfg)
	in := bufio.NewScanner(os.Stdin)

	name := k.Name
	if name == "" {
		name = "identification quiz"
	}
	fmt.Printf("\n%s -- %d candidates, %d properties.\n", name, len(k.Entities), len(k.Attributes))
	fmt.Println("Answer with a number, or several (1,3). Other keys: (s)kip, (u)ndo, (w)hy, (g)ive up, (q)uit.")

	for {
		done, reason := s.Done()
		if done {
			result(s, top, reason)
			return
		}
		q := s.Next()

		fmt.Printf("\nQ%d. %s", s.Asked()+1, q.Text)
		if q.Reoffered {
			fmt.Print("   (you skipped this earlier; it decides things now)")
		}
		if explain {
			fmt.Printf("   [%.2f bits, %.2f left]", q.Gain, s.Entropy())
		}
		fmt.Println()
		for i, opt := range q.Options {
			fmt.Printf("  %2d) %s\n", i+1, opt.Label)
		}
		if q.Attr.Multi {
			fmt.Println("      pick all that apply, e.g. 1,3")
		} else if len(q.Options) > 2 {
			fmt.Println("      pick several, e.g. 1,3, if you cannot tell them apart")
		}
		fmt.Print("> ")

		if !in.Scan() {
			fmt.Println()
			result(s, top, "input ended")
			return
		}
		switch line := strings.ToLower(strings.TrimSpace(in.Text())); line {
		case "q", "quit", "exit":
			return
		case "g", "give up", "giveup":
			result(s, top, "you gave up")
			return
		case "s", "skip", "?", "":
			s.Skip(q)
		case "u", "undo", "b", "back":
			if !s.Undo() {
				fmt.Println("   nothing to undo.")
			}
		case "w", "why":
			why(s, q)
		default:
			picks := prompt.ParsePicks(line, len(q.Options))
			if picks == nil {
				fmt.Printf("   pick 1-%d, or several like 1,3, or s/u/w/q.\n", len(q.Options))
				continue
			}
			s.AskMany(q, picks)
		}
		if leaders := s.Top(3); len(leaders) > 0 {
			fmt.Printf("   -> %s\n", summarise(leaders))
		}
	}
}

func why(s *engine.Session, current *engine.Question) {
	fmt.Printf("\n   Uncertainty: %.2f bits over %d candidates.\n", s.Entropy(), len(s.KB.Entities))
	fmt.Println("   Leading candidates:")
	for _, c := range s.Top(5) {
		fmt.Printf("     %5.1f%%  %s\n", c.Prob*100, c.Name)
	}
	fmt.Println("   Most informative questions from here:")
	for i, q := range s.Rank() {
		if i == 5 {
			break
		}
		marker := " "
		if q.Attr == current.Attr {
			marker = "*"
		}
		fmt.Printf("    %s %5.2f bits  %s\n", marker, q.Gain, q.Text)
	}
	if h := s.History(); len(h) > 0 {
		fmt.Println("   You said:")
		for _, a := range h {
			fmt.Printf("     %s: %s\n", a.Attr, a.Label)
		}
	}
}

func result(s *engine.Session, top int, reason string) {
	candidates := s.Top(top)
	fmt.Printf("\n--- %s after %d question(s) ---\n\n", reason, s.Asked())
	if len(candidates) == 0 {
		fmt.Println("No candidates.")
		return
	}
	for i, c := range candidates {
		fmt.Printf("%d. %-28s %5.1f%%  %s\n", i+1, c.Name, c.Prob*100, bar(c.Prob))
		if c.Note != "" {
			fmt.Printf("   %s\n", c.Note)
		}
		if i == 0 {
			if c.Image != nil {
				fmt.Printf("   photo  %s\n", c.Image.URL)
				fmt.Printf("          %s\n", credit(c.Image))
			}
			if c.Link != "" {
				fmt.Printf("   more   %s\n", c.Link)
			}
		}
	}
	if candidates[0].Unknown {
		fmt.Println("\nNothing here fits what you described; treat the runners-up with suspicion.")
		fmt.Println("Adding the species is the fix; -lint and -simulate will")
		fmt.Println("tell you whether it stays distinguishable from what is already there.")
		fmt.Println()
		return
	}
	if best := candidates[0]; best.Prob < s.Cfg.Threshold {
		fmt.Println("\nNot certain. Answering a skipped question, or adding properties that")
		fmt.Println("separate the candidates above, would sharpen this.")
	}
	fmt.Println()
}

// credit is the acknowledgement most of these licences require.
func credit(img *kb.Image) string {
	parts := []string{}
	if img.Credit != "" {
		parts = append(parts, img.Credit)
	}
	if img.License != "" {
		parts = append(parts, img.License)
	}
	line := strings.Join(parts, ", ")
	if img.Source != "" {
		line += " -- " + img.Source
	}
	return line
}

func summarise(cs []engine.Candidate) string {
	parts := make([]string, 0, len(cs))
	for _, c := range cs {
		if c.Prob < 0.01 {
			break
		}
		parts = append(parts, fmt.Sprintf("%s %.0f%%", c.Name, c.Prob*100))
	}
	if len(parts) == 0 {
		return "no clear candidate"
	}
	return strings.Join(parts, " · ")
}

func bar(p float64) string {
	n := int(p*20 + 0.5)
	return strings.Repeat("#", n) + strings.Repeat(".", 20-n)
}

// --- other modes -----------------------------------------------------------

func lint(k *kb.KB) int {
	issues := k.Lint()
	if len(issues) == 0 {
		fmt.Printf("%d entities, %d attributes: no problems found.\n", len(k.Entities), len(k.Attributes))
		return 0
	}
	code := 0
	for _, is := range issues {
		fmt.Printf("%-7s %s\n", is.Severity, is.Message)
		if is.Severity == kb.Error {
			code = 1
		}
	}
	return code
}

func stats(k *kb.KB) {
	st := k.Stats()
	fmt.Printf("entities        %d\n", st.Entities)
	fmt.Printf("attributes      %d (%d boolean)\n", st.Attributes, st.Boolean)
	fmt.Printf("values          %d\n", st.Values)
	fmt.Printf("uncertainty     %.2f bits before any question (%.2f if priors were flat)\n", st.PriorEntropy, math.Log2(float64(st.Entities)))
	fmt.Printf("best possible   %.1f questions, at up to %.2f bits each\n\n", st.MinQuestions, math.Log2(float64(st.MaxOptions)))
	fmt.Printf("%-24s %-12s %-6s %s\n", "ATTRIBUTE", "KIND", "NOISE", "VALUES")
	for _, a := range k.Attributes {
		vals := make([]string, 0, len(a.Domain))
		for _, v := range a.Domain {
			vals = append(vals, fmt.Sprintf("%s(%d)", v, a.Holders(v)))
		}
		fmt.Printf("%-24s %-12s %-6.2f %s\n", a.Name, a.Kind, a.Noise, strings.Join(vals, " "))
	}
}

func simulate(k *kb.KB, cfg engine.Config, opts engine.SimOptions, asJSON bool) {
	r := engine.Simulate(k, cfg, opts)
	if asJSON {
		writeGames(r, nil)
		return
	}
	if opts.ErrorModel {
		fmt.Printf("self-test: %d entities, answering as each question's error model says\n\n", len(r.Results))
	} else {
		fmt.Printf("self-test: %d entities, answer error rate %.0f%%\n\n", len(r.Results), opts.Noise*100)
	}
	summary(k, r)
	if opts.ErrorModel {
		skips(r)
	}
	if len(r.Worst) == 0 {
		fmt.Println("\nEvery entity was identified.")
		return
	}
	fmt.Printf("\nnot identified (%d):\n", len(r.Worst))
	for _, res := range r.Worst {
		fmt.Printf("  %-28s ranked #%d at %4.1f%%, guessed %s (%d questions)\n",
			res.Target.Name, res.Rank, res.Prob*100, res.GuessName, res.Questions)
	}
}

// replay plays one game per recorded sighting, answering from the record.
func replay(k *kb.KB, cfg engine.Config, path string, asJSON bool) error {
	in := os.Stdin
	if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		in = f
	}
	sightings, err := engine.LoadSightings(in, k)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if len(sightings) == 0 {
		return fmt.Errorf("%s: no sightings", path)
	}
	r := engine.Replay(k, cfg, sightings)
	if asJSON {
		writeGames(r, sightings)
		return nil
	}

	fmt.Printf("replay: %d sightings from %s\n\n", len(sightings), path)
	summary(k, r)
	skips(r)
	gaps := 0
	for _, res := range r.Results {
		for _, st := range res.Steps {
			if st.Gap {
				gaps++
			}
		}
	}
	if gaps > 0 {
		fmt.Printf("gaps            %d questions had no recorded answer -- ask them before trusting this\n", gaps)
	}

	// Per split, when there is more than one.
	var splits []string
	by := map[string][]engine.SimResult{}
	for i, sg := range sightings {
		if _, ok := by[sg.Split]; !ok {
			splits = append(splits, sg.Split)
		}
		by[sg.Split] = append(by[sg.Split], r.Results[i])
	}
	if len(splits) > 1 {
		sort.Strings(splits)
		fmt.Println()
		for _, name := range splits {
			sr := by[name]
			correct, asked := 0, 0
			for _, res := range sr {
				if res.Correct {
					correct++
				}
				asked += res.Questions
			}
			label := name
			if label == "" {
				label = "(none)"
			}
			fmt.Printf("  %-12s %4d games  identified %3.0f%%  %.1f questions\n", label, len(sr),
				100*float64(correct)/float64(len(sr)), float64(asked)/float64(len(sr)))
		}
	}

	if len(r.Worst) == 0 {
		fmt.Println("\nEvery sighting was identified.")
		return nil
	}
	var misses []int
	for i, res := range r.Results {
		if !res.Correct {
			misses = append(misses, i)
		}
	}
	sort.SliceStable(misses, func(a, b int) bool { return r.Results[misses[a]].Prob < r.Results[misses[b]].Prob })
	fmt.Printf("\nnot identified (%d):\n", len(misses))
	const show = 20
	for n, i := range misses {
		if n == show {
			fmt.Printf("  ... and %d more (-json has them all)\n", len(misses)-show)
			break
		}
		res := r.Results[i]
		fmt.Printf("  %-12s %-28s ranked #%d at %4.1f%%, guessed %s (%d questions)\n",
			sightings[i].ID, res.Target.Name, res.Rank, res.Prob*100, res.GuessName, res.Questions)
	}
	return nil
}

// summary prints the totals -simulate and -replay share.
func summary(k *kb.KB, r engine.SimReport) {
	n := len(r.Results)
	fmt.Printf("identified      %d/%d (%.0f%%)\n", r.Correct, n, 100*float64(r.Correct)/float64(n))
	if wrong := n - r.Correct - r.Unsure; r.Unsure > 0 || wrong > 0 {
		fmt.Printf("named wrongly   %d (%.0f%%)\n", wrong, 100*float64(wrong)/float64(n))
		fmt.Printf("said unsure     %d (%.0f%%)\n", r.Unsure, 100*float64(r.Unsure)/float64(n))
	}
	fmt.Printf("questions       %.1f average, %d worst\n", r.MeanAsked, r.MaxAsked)
	fmt.Printf("uncertainty     %.2f bits to resolve (%.2f if priors were flat)\n",
		k.Stats().PriorEntropy, math.Log2(float64(len(k.Entities))))
}

// skips prints how often games were answered "not sure" and given up, for the
// modes where the answerer can fail to answer.
func skips(r engine.SimReport) {
	skipped, gaveUp := 0, 0
	for _, res := range r.Results {
		skipped += res.Skipped
		if res.GaveUp {
			gaveUp++
		}
	}
	fmt.Printf("not sure        %.1f per game\n", float64(skipped)/float64(len(r.Results)))
	if gaveUp > 0 {
		fmt.Printf("gave up         %d (only unanswerable questions were left)\n", gaveUp)
	}
}

type gameJSON struct {
	Photo     string     `json:"photo,omitempty"`
	Split     string     `json:"split,omitempty"`
	Target    string     `json:"target"`
	Guess     string     `json:"guess"`
	Correct   bool       `json:"correct"`
	Unsure    bool       `json:"unsure"`
	GaveUp    bool       `json:"gave_up"`
	Rank      int        `json:"rank"`
	Prob      float64    `json:"prob"`
	Questions int        `json:"questions"`
	Skipped   int        `json:"skipped"`
	Start     float64    `json:"entropy_start"`
	Steps     []stepJSON `json:"steps"`
}

type stepJSON struct {
	Attr      string       `json:"attr"`
	Offered   [][]kb.Value `json:"offered"`
	Picked    []int        `json:"picked"`
	Skipped   bool         `json:"skipped"`
	Gap       bool         `json:"gap,omitempty"`
	Reoffered bool         `json:"reoffered,omitempty"`
	Entropy   float64      `json:"entropy_after"`
}

// writeGames prints one JSON object per game, in the order played. sightings
// is nil for -simulate, where each game is named by its target alone.
func writeGames(r engine.SimReport, sightings []engine.Sighting) {
	enc := json.NewEncoder(os.Stdout)
	for i, res := range r.Results {
		g := gameJSON{Target: res.Target.Name, Guess: res.GuessName, Correct: res.Correct,
			Unsure: res.Unsure, GaveUp: res.GaveUp, Rank: res.Rank, Prob: res.Prob,
			Questions: res.Questions, Skipped: res.Skipped, Start: res.Start, Steps: []stepJSON{}}
		if sightings != nil {
			g.Photo, g.Split = sightings[i].ID, sightings[i].Split
		}
		for _, st := range res.Steps {
			picked := st.Picked
			if picked == nil {
				picked = []int{}
			}
			g.Steps = append(g.Steps, stepJSON{Attr: st.Attr, Offered: st.Offered, Picked: picked,
				Skipped: st.Skipped, Gap: st.Gap, Reoffered: st.Reoffered, Entropy: st.Entropy})
		}
		if err := enc.Encode(g); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	}
}

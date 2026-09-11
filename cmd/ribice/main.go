// Command ribice plays a twenty-questions style identification quiz over a
// JSON knowledge base.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/lpapez/ribice/engine"
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
		seed     = flag.Int64("seed", 1, "random seed for -simulate")
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
		simulate(k, cfg, engine.SimOptions{Noise: *simNoise, Seed: *seed})
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
			picks := parsePicks(line, len(q.Options))
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

// parsePicks reads "2", "1,3" or "1 3" into option indices, or nil if the line
// is not a valid selection.
func parsePicks(line string, n int) []int {
	fields := strings.FieldsFunc(line, func(r rune) bool {
		return r == ',' || r == ' ' || r == '+' || r == '\t'
	})
	if len(fields) == 0 {
		return nil
	}
	out := make([]int, 0, len(fields))
	for _, f := range fields {
		v, err := strconv.Atoi(f)
		if err != nil || v < 1 || v > n {
			return nil
		}
		out = append(out, v-1)
	}
	return out
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

func simulate(k *kb.KB, cfg engine.Config, opts engine.SimOptions) {
	r := engine.Simulate(k, cfg, opts)
	n := len(r.Results)
	fmt.Printf("self-test: %d entities, answer error rate %.0f%%\n\n", n, opts.Noise*100)
	fmt.Printf("identified      %d/%d (%.0f%%)\n", r.Correct, n, 100*float64(r.Correct)/float64(n))
	if wrong := n - r.Correct - r.Unsure; r.Unsure > 0 || wrong > 0 {
		fmt.Printf("named wrongly   %d (%.0f%%)\n", wrong, 100*float64(wrong)/float64(n))
		fmt.Printf("said unsure     %d (%.0f%%)\n", r.Unsure, 100*float64(r.Unsure)/float64(n))
	}
	fmt.Printf("questions       %.1f average, %d worst\n", r.MeanAsked, r.MaxAsked)
	fmt.Printf("uncertainty     %.2f bits to resolve (%.2f if priors were flat)\n",
		k.Stats().PriorEntropy, math.Log2(float64(len(k.Entities))))
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

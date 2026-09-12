//go:build js && wasm

// Command wasm exposes the identification engine to JavaScript.
//
// It is the browser's equivalent of cmd/ribice: same knowledge base, same
// engine, a different way of talking to the user. The Go side owns the belief
// state and decides what to ask; JavaScript owns the pixels and never needs to
// know how either works.
//
// Build with:
//
//	GOOS=js GOARCH=wasm go build -o web/ribice.wasm ./cmd/wasm
//
// The knowledge base is not embedded. JavaScript fetches the JSON and hands it
// to ribice.load, so the data can be edited and redeployed without recompiling.
package main

import (
	"syscall/js"

	"github.com/lpapez/ribice/engine"
	"github.com/lpapez/ribice/kb"
)

// session pairs a run of the quiz with the question currently on screen, so an
// answer arriving from JavaScript as a list of option indices can be resolved
// against the question those indices were rendered from.
type session struct {
	s *engine.Session
	q *engine.Question
}

var (
	bases    = map[int]*kb.KB{} // every knowledge base loaded so far
	lastBase int                // the one a session defaults to
	sessions = map[int]*session{}
	nextBase = 1
	nextID   = 1
)

func main() {
	js.Global().Set("ribice", js.ValueOf(map[string]any{
		"load":    js.FuncOf(load),
		"start":   js.FuncOf(start),
		"state":   js.FuncOf(state),
		"answer":  js.FuncOf(answer),
		"skip":    js.FuncOf(skip),
		"undo":    js.FuncOf(undo),
		"rank":    js.FuncOf(rank),
		"release": js.FuncOf(release),
	}))
	select {} // keep the exported functions alive
}

// --- exported functions ----------------------------------------------------

// load(json) parses a knowledge base and keeps it. The returned id may be
// passed to start as {kb: id}; a page embedding two quizzes over different
// knowledge bases needs that, and one embedding a single quiz can ignore it,
// since start falls back to whichever was loaded last.
func load(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return fail("load: expected the knowledge base JSON")
	}
	k, err := kb.Load([]byte(args[0].String()))
	if err != nil {
		return fail(err.Error())
	}
	id := nextBase
	nextBase++
	bases[id], lastBase = k, id
	return map[string]any{
		"kb":         id,
		"name":       k.Name,
		"entities":   len(k.Entities),
		"attributes": len(k.Attributes),
	}
}

// start(opts) begins a session and returns its first view. Every Config field
// may be overridden; anything absent keeps engine.DefaultConfig's value.
func start(_ js.Value, args []js.Value) any {
	base := lastBase
	cfg := engine.DefaultConfig()
	if len(args) > 0 && args[0].Type() == js.TypeObject {
		o := args[0]
		base = optInt(o, "kb", base)
		cfg.Threshold = optFloat(o, "threshold", cfg.Threshold)
		cfg.MinGain = optFloat(o, "minGain", cfg.MinGain)
		cfg.MinSupport = optFloat(o, "minSupport", cfg.MinSupport)
		cfg.UnknownPrior = optFloat(o, "unknownPrior", cfg.UnknownPrior)
		cfg.MaxOptions = optInt(o, "maxOptions", cfg.MaxOptions)
		cfg.MaxQuestions = optInt(o, "maxQuestions", cfg.MaxQuestions)
		cfg.SkipCooldown = optInt(o, "skipCooldown", cfg.SkipCooldown)
		cfg.Confirm = optInt(o, "confirm", cfg.Confirm)
	}
	k := bases[base]
	if k == nil {
		return fail("no knowledge base loaded")
	}
	id := nextID
	nextID++
	sessions[id] = &session{s: engine.New(k, cfg)}
	return view(id)
}

// state(id) returns the current view without changing anything.
func state(_ js.Value, args []js.Value) any {
	id, _, err := lookup(args)
	if err != nil {
		return err
	}
	return view(id)
}

// answer(id, picks) records the options chosen for the question on screen.
// picks is an array of indices into that question's options; an empty array is
// a skip. Returns the view that follows.
func answer(_ js.Value, args []js.Value) any {
	id, sn, err := lookup(args)
	if err != nil {
		return err
	}
	if sn.q == nil {
		return fail("no question is open")
	}
	var picks []int
	if len(args) > 1 && args[1].Type() == js.TypeObject {
		for i := 0; i < args[1].Length(); i++ {
			picks = append(picks, args[1].Index(i).Int())
		}
	}
	sn.s.AskMany(sn.q, picks) // an empty selection skips, by AskMany's contract
	return view(id)
}

// skip(id) records that the user cannot answer the question on screen.
func skip(_ js.Value, args []js.Value) any {
	id, sn, err := lookup(args)
	if err != nil {
		return err
	}
	if sn.q == nil {
		return fail("no question is open")
	}
	sn.s.Skip(sn.q)
	return view(id)
}

// undo(id) takes back the most recent answer.
func undo(_ js.Value, args []js.Value) any {
	id, sn, err := lookup(args)
	if err != nil {
		return err
	}
	if !sn.s.Undo() {
		return fail("nothing to undo")
	}
	return view(id)
}

// rank(id) lists the questions still worth asking, best first -- the "why"
// view: what the engine is considering and how much each would be worth.
func rank(_ js.Value, args []js.Value) any {
	_, sn, err := lookup(args)
	if err != nil {
		return err
	}
	ranked := sn.s.Rank()
	out := make([]any, 0, len(ranked))
	for _, q := range ranked {
		out = append(out, map[string]any{
			"attr": q.Attr.Name,
			"text": q.Text,
			"gain": q.Gain,
		})
	}
	return out
}

// release(id) discards a session. Sessions are cheap, but a long-lived page
// that starts many of them would otherwise hold every one forever.
func release(_ js.Value, args []js.Value) any {
	if len(args) > 0 {
		delete(sessions, args[0].Int())
	}
	return nil
}

// --- view ------------------------------------------------------------------

// view is everything needed to render one screen: whether the quiz is over, the
// question to ask if it is not, the standing candidates, and what has been
// answered so far. JavaScript re-renders from this wholesale and keeps no state
// of its own beyond the session id.
func view(id int) any {
	sn := sessions[id]
	done, reason := sn.s.Done()
	sn.q = nil
	out := map[string]any{
		"id":      id,
		"done":    done,
		"reason":  reason,
		"asked":   sn.s.Asked(),
		"entropy": sn.s.Entropy(),
		"top":     candidates(sn.s, 5),
		"history": history(sn.s),
		"canUndo": sn.s.Asked() > 0,
	}
	if done {
		return out
	}
	q := sn.s.Next()
	if q == nil {
		out["done"] = true
		out["reason"] = "no questions left to ask"
		return out
	}
	sn.q = q
	options := make([]any, 0, len(q.Options))
	for _, opt := range q.Options {
		options = append(options, map[string]any{
			"label": opt.Label,
			"other": opt.Other,
			"prob":  opt.Prob,
		})
	}
	out["question"] = map[string]any{
		"attr":       q.Attr.Name,
		"text":       q.Text,
		"multi":      q.Attr.Multi,
		"boolean":    q.Attr.Kind == kb.Boolean,
		"reoffered":  q.Reoffered,
		"confirming": q.Confirming,
		"gain":       q.Gain,
		"options":    options,
	}
	return out
}

func candidates(s *engine.Session, n int) []any {
	top := s.Top(n)
	out := make([]any, 0, len(top))
	for _, c := range top {
		e := map[string]any{
			"name":    c.Name,
			"prob":    c.Prob,
			"note":    c.Note,
			"link":    c.Link,
			"unknown": c.Unknown,
		}
		if c.Image != nil {
			e["image"] = map[string]any{
				"url":     c.Image.URL,
				"source":  c.Image.Source,
				"credit":  c.Image.Credit,
				"license": c.Image.License,
			}
		}
		out = append(out, e)
	}
	return out
}

func history(s *engine.Session) []any {
	h := s.History()
	out := make([]any, 0, len(h))
	for _, a := range h {
		text := a.Attr
		if attr := s.KB.Attr(a.Attr); attr != nil {
			text = attr.Title()
		}
		out = append(out, map[string]any{
			"attr":    a.Attr,
			"text":    text,
			"label":   a.Label,
			"skipped": a.Skipped,
		})
	}
	return out
}

// --- helpers ---------------------------------------------------------------

// lookup resolves the session id in args[0]. The second error return is a ready
// -to-return {error} object, so callers can hand it straight back.
func lookup(args []js.Value) (int, *session, any) {
	if len(args) < 1 {
		return 0, nil, fail("expected a session id")
	}
	id := args[0].Int()
	sn := sessions[id]
	if sn == nil {
		return 0, nil, fail("unknown session")
	}
	return id, sn, nil
}

func fail(msg string) any { return map[string]any{"error": msg} }

func optFloat(o js.Value, key string, def float64) float64 {
	if v := o.Get(key); v.Type() == js.TypeNumber {
		return v.Float()
	}
	return def
}

func optInt(o js.Value, key string, def int) int {
	if v := o.Get(key); v.Type() == js.TypeNumber {
		return v.Int()
	}
	return def
}

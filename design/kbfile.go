package design

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/lpapez/ribice/kb"
)

// KBFile is a knowledge base's JSON, edited in place: keys keep their order
// and untouched values keep their text, so writing it back changes only what
// was edited and the diff a person reviews shows nothing else.
type KBFile struct {
	top      *object
	attrs    *object
	entities []*object
	newline  bool   // whether the file ended in one
	indent   string // as the file was indented
}

// object is a JSON object that remembers its key order.
type object struct {
	keys []string
	vals map[string]json.RawMessage
}

func parseObject(b []byte) (*object, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, fmt.Errorf("not a JSON object")
	}
	o := &object{vals: map[string]json.RawMessage{}}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key := t.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		if _, dup := o.vals[key]; !dup {
			o.keys = append(o.keys, key)
		}
		o.vals[key] = v
	}
	return o, nil
}

// set replaces key's value where it stands, or inserts it before the first
// key for which before is true, or at the end.
func (o *object) set(key string, v json.RawMessage, before func(string) bool) {
	if _, ok := o.vals[key]; !ok {
		at := len(o.keys)
		for i, k := range o.keys {
			if before != nil && before(k) {
				at = i
				break
			}
		}
		o.keys = append(o.keys[:at], append([]string{key}, o.keys[at:]...)...)
	}
	o.vals[key] = v
}

func (o *object) del(key string) {
	if _, ok := o.vals[key]; !ok {
		return
	}
	delete(o.vals, key)
	for i, k := range o.keys {
		if k == key {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			break
		}
	}
}

func (o *object) marshal() json.RawMessage {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(marshal(k))
		b.WriteByte(':')
		b.Write(o.vals[k])
	}
	b.WriteByte('}')
	return b.Bytes()
}

// marshal encodes v without escaping <, > and &, as Python's json.dump does.
func marshal(v any) json.RawMessage {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		panic(err) // only plain values are marshalled here
	}
	return bytes.TrimRight(b.Bytes(), "\n")
}

// ParseKBFile reads a knowledge base written as an object with "attributes"
// and "entities".
func ParseKBFile(data []byte) (*KBFile, error) {
	top, err := parseObject(data)
	if err != nil {
		return nil, err
	}
	f := &KBFile{top: top, newline: bytes.HasSuffix(data, []byte("\n")), indent: " "}
	if lines := bytes.SplitN(data, []byte("\n"), 3); len(lines) > 1 {
		if n := len(lines[1]) - len(bytes.TrimLeft(lines[1], " ")); n > 0 {
			f.indent = strings.Repeat(" ", n)
		}
	}
	if raw, ok := top.vals["attributes"]; ok {
		if f.attrs, err = parseObject(raw); err != nil {
			return nil, fmt.Errorf("attributes: %w", err)
		}
	} else {
		f.attrs = &object{vals: map[string]json.RawMessage{}}
	}
	var ents []json.RawMessage
	if err := json.Unmarshal(top.vals["entities"], &ents); err != nil {
		return nil, fmt.Errorf("entities: %w", err)
	}
	for i, raw := range ents {
		e, err := parseObject(raw)
		if err != nil {
			return nil, fmt.Errorf("entity %d: %w", i+1, err)
		}
		f.entities = append(f.entities, e)
	}
	return f, nil
}

// Clone copies f, so a candidate can be edited without touching the original.
func (f *KBFile) Clone() *KBFile {
	g, err := ParseKBFile(f.Bytes())
	if err != nil {
		panic(err) // f wrote it, so it reads
	}
	return g
}

// Bytes writes the file back in the style it was read in: its indent, and no
// ASCII escaping, as tools/clouds.py's json.dump writes it.
//
// It only reads f, so several goroutines may write out one file at once.
func (f *KBFile) Bytes() []byte {
	top := &object{keys: append([]string(nil), f.top.keys...), vals: map[string]json.RawMessage{}}
	for k, v := range f.top.vals {
		top.vals[k] = v
	}
	if _, ok := top.vals["attributes"]; !ok {
		top.set("attributes", nil, func(k string) bool { return k == "entities" })
	}
	top.vals["attributes"] = f.attrs.marshal()
	var ents bytes.Buffer
	ents.WriteByte('[')
	for i, e := range f.entities {
		if i > 0 {
			ents.WriteByte(',')
		}
		ents.Write(e.marshal())
	}
	ents.WriteByte(']')
	top.vals["entities"] = ents.Bytes()

	var compact, out bytes.Buffer
	if err := json.Compact(&compact, top.marshal()); err != nil {
		panic(err)
	}
	if err := json.Indent(&out, compact.Bytes(), "", f.indent); err != nil {
		panic(err)
	}
	if f.newline {
		out.WriteByte('\n')
	}
	return out.Bytes()
}

// Attributes lists the attribute names the file declares metadata for, in
// order.
func (f *KBFile) Attributes() []string { return append([]string(nil), f.attrs.keys...) }

// DropAttribute removes an attribute's metadata and every entity's value.
func (f *KBFile) DropAttribute(name string) {
	f.attrs.del(name)
	for _, e := range f.entities {
		e.del(name)
	}
}

// AttrMeta is the metadata an attribute is written with, in the order
// tools/clouds.py writes it.
type AttrMeta struct {
	Question   string            `json:"question"`
	Noise      float64           `json:"noise"`
	Confusion  float64           `json:"confusion,omitempty"`
	Confusable [][]string        `json:"confusable,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Cost       float64           `json:"cost"`
	AnswerRate float64           `json:"answer_rate,omitempty"`
}

// AddAttribute writes a proposal into the file with the given error model:
// its metadata at the end of the attributes, and each entity's settled value
// just before the entity's underscored keys. A yes/no attribute is written
// only where it is true, as the closed world reads a missing one as no; an
// entity with no settled value is given every value, which tells the quiz
// nothing false about it.
func (f *KBFile) AddAttribute(p Proposal, m ErrorModel) error {
	if _, exists := f.attrs.vals[p.Name]; exists {
		return fmt.Errorf("attribute %q is already there", p.Name)
	}
	meta := AttrMeta{Question: p.Question, Noise: round3(m.Noise), Confusion: round3(m.Confusion),
		Cost: round3(m.Cost), AnswerRate: round3(m.AnswerRate), Labels: map[string]string{}}
	if meta.AnswerRate == 1 {
		meta.AnswerRate = 0 // the default; leave it out
	}
	for _, c := range m.Confusable {
		meta.Confusable = append(meta.Confusable, []string{string(c[0]), string(c[1])})
	}
	boolean := p.Kind == "boolean"
	if !boolean {
		for _, o := range p.Values {
			meta.Labels[string(o.Value)] = o.Label
		}
	}
	f.attrs.set(p.Name, marshal(meta), nil)

	underscore := func(k string) bool { return strings.HasPrefix(k, "_") }
	for _, e := range f.entities {
		var name string
		if err := json.Unmarshal(e.vals["name"], &name); err != nil {
			return fmt.Errorf("an entity without a name: %w", err)
		}
		vs := p.Assign[name]
		if len(vs) == 0 {
			for _, o := range p.Values {
				vs = append(vs, o.Value)
			}
		}
		switch {
		case boolean && len(vs) == 1 && vs[0] == kb.No:
			continue
		case boolean && len(vs) == 1:
			e.set(p.Name, marshal(true), underscore)
		case len(vs) == 1:
			e.set(p.Name, marshal(string(vs[0])), underscore)
		default:
			strs := make([]string, len(vs))
			for i, v := range vs {
				strs[i] = string(v)
			}
			e.set(p.Name, marshal(strs), underscore)
		}
	}
	return nil
}

func round3(x float64) float64 { return math.Round(x*1000) / 1000 }

func jsonString(raw json.RawMessage, s *string) error { return json.Unmarshal(raw, s) }

// SetErrors rewrites an attribute's error model in place: noise,
// confusion, look-alikes, cost and answer_rate, leaving its question, labels
// and every entity's value as they are. A field at its default is removed.
func (f *KBFile) SetErrors(name string, m ErrorModel) error {
	raw, ok := f.attrs.vals[name]
	if !ok {
		raw = json.RawMessage("{}")
	}
	meta, err := parseObject(raw)
	if err != nil {
		return fmt.Errorf("attribute %q: %w", name, err)
	}
	meta.set("noise", marshal(round3(m.Noise)), nil)
	if c := round3(m.Confusion); c > 0 && len(m.Confusable) > 0 {
		var pairs [][]string
		for _, p := range m.Confusable {
			pairs = append(pairs, []string{string(p[0]), string(p[1])})
		}
		meta.set("confusion", marshal(c), func(k string) bool { return k != "question" && k != "noise" })
		meta.set("confusable", marshal(pairs), func(k string) bool { return k == "labels" || k == "cost" })
	} else {
		meta.del("confusion")
		meta.del("confusable")
	}
	meta.set("cost", marshal(round3(m.Cost)), nil)
	if r := round3(m.AnswerRate); r < 1 {
		meta.set("answer_rate", marshal(r), nil)
	} else {
		meta.del("answer_rate")
	}
	f.attrs.set(name, meta.marshal(), nil)
	return nil
}

// ReplaceAttribute swaps an attribute for a newer version of it, with its
// settled values and error model, keeping its place among the attributes.
func (f *KBFile) ReplaceAttribute(p Proposal, m ErrorModel) error {
	at := -1
	for i, k := range f.attrs.keys {
		if k == p.Name {
			at = i
		}
	}
	f.DropAttribute(p.Name)
	if err := f.AddAttribute(p, m); err != nil {
		return err
	}
	if at >= 0 {
		keys := slices.DeleteFunc(f.attrs.keys, func(k string) bool { return k == p.Name })
		f.attrs.keys = slices.Insert(keys, min(at, len(keys)), p.Name)
	}
	return nil
}

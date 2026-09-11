package kb

import (
	"math"
	"strings"
	"testing"
)

// The shape the user writes by hand, typos and all.
const sample = `[
  {"name": "catfish", "color": "gray",  "pattern": "none", "moustache": true},
  {"name": "trout",   "colour": "gray", "pattern": "checkered", "stripes": true}
]`

func load(t *testing.T, src string) *KB {
	t.Helper()
	k, err := Load([]byte(src))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return k
}

func TestLoadBareArray(t *testing.T) {
	k := load(t, sample)
	if len(k.Entities) != 2 {
		t.Fatalf("got %d entities, want 2", len(k.Entities))
	}
	// color and colour are separate attributes: the KB is taken at its word.
	for _, name := range []string{"color", "colour", "pattern", "moustache", "stripes"} {
		if k.Attr(name) == nil {
			t.Errorf("attribute %q missing", name)
		}
	}
}

func TestBooleanInferenceAndClosedWorld(t *testing.T) {
	k := load(t, sample)
	a := k.Attr("moustache")
	if a.Kind != Boolean {
		t.Fatalf("moustache kind = %v, want boolean", a.Kind)
	}
	if len(a.Domain) != 2 {
		t.Fatalf("boolean domain = %v, want two values", a.Domain)
	}
	catfish, trout := k.Entities[0], k.Entities[1]
	if !catfish.Has("moustache", Yes) {
		t.Error("catfish should have a moustache")
	}
	// trout never mentions a moustache, so it has none.
	if !trout.Has("moustache", No) {
		t.Error("undeclared boolean should default to no")
	}
	// trout never mentions "color", so that categorical value is absent.
	if !trout.Has("color", Absent) {
		t.Error("undeclared categorical should default to none")
	}
}

func TestExplicitNoneEqualsAbsent(t *testing.T) {
	k := load(t, sample)
	if !k.Entities[0].Has("pattern", Absent) {
		t.Error(`"none" should normalise to the absent value`)
	}
	if n := k.Attr("pattern").Holders(Absent); n != 1 {
		t.Errorf("holders of absent pattern = %d, want 1", n)
	}
}

func TestMultipleValues(t *testing.T) {
	k := load(t, `[
	  {"name": "a", "colour": ["grey", "silver"]},
	  {"name": "b", "colour": "red"}
	]`)
	vals := k.Entities[0].Values("colour")
	if len(vals) != 2 || !vals["grey"] || !vals["silver"] {
		t.Fatalf("colour values = %v, want grey and silver", vals)
	}
}

func TestMetadataAndPriors(t *testing.T) {
	k := load(t, `{
	  "attributes": {"colour": {"question": "What colour?", "noise": 0.3, "cost": 2}},
	  "entities": [
	    {"name": "a", "_prior": 3, "colour": "red"},
	    {"name": "b", "_prior": 1, "colour": "blue"}
	  ]
	}`)
	a := k.Attr("colour")
	if a.Title() != "What colour?" || a.Noise != 0.3 || a.Cost != 2 {
		t.Errorf("metadata not applied: %q %v %v", a.Title(), a.Noise, a.Cost)
	}
	if got := k.Entities[0].Prior; got != 0.75 {
		t.Errorf("prior = %v, want 0.75 after normalising", got)
	}
}

func TestAutoQuestionText(t *testing.T) {
	k := load(t, `[{"name": "a", "big_eyes": true}, {"name": "b"}]`)
	if got := k.Attr("big_eyes").Title(); got != "Big eyes?" {
		t.Errorf("auto question = %q, want %q", got, "Big eyes?")
	}
}

func TestLoadErrors(t *testing.T) {
	for _, tc := range []struct{ name, src string }{
		{"no entities", `[]`},
		{"missing name", `[{"colour": "red"}]`},
		{"nested array", `[{"name": "a", "colour": [["red"]]}]`},
		{"bad json", `{`},
	} {
		if _, err := Load([]byte(tc.src)); err == nil {
			t.Errorf("%s: want an error, got none", tc.name)
		}
	}
}

func TestLintFindsSpellingVariants(t *testing.T) {
	if !hasIssue(load(t, sample).Lint(), `"color" and "colour"`) {
		t.Error("lint should flag color/colour as the same property spelled two ways")
	}
}

func TestLintFindsIndistinguishableEntities(t *testing.T) {
	k := load(t, `[
	  {"name": "a", "colour": "red"},
	  {"name": "b", "colour": "red"},
	  {"name": "c", "colour": "blue"}
	]`)
	if !hasIssue(k.Lint(), "no question can tell these apart: a, b") {
		t.Error("lint should flag identically described entities")
	}
}

func TestLintAcceptsGenuinelyDifferentValues(t *testing.T) {
	k := load(t, `[{"name": "a", "colour": "green"}, {"name": "b", "colour": "grey"}]`)
	if hasIssue(k.Lint(), "spelled two ways") {
		t.Error("green and grey are different colours, not a typo")
	}
}

func hasIssue(issues []Issue, substr string) bool {
	for _, is := range issues {
		if strings.Contains(is.Message, substr) {
			return true
		}
	}
	return false
}

func TestReportIsADistribution(t *testing.T) {
	k := load(t, `{
	  "attributes": {"shape": {"noise": 0.06, "confusion": 0.3,
	     "confusable": [["torpedo", "elongated"], ["flat", "disc"]]}},
	  "entities": [
	    {"name": "a", "shape": "torpedo"}, {"name": "b", "shape": "elongated"},
	    {"name": "c", "shape": "flat"},    {"name": "d", "shape": "disc"},
	    {"name": "e", "shape": "seahorse"}
	  ]
	}`)
	a := k.Attr("shape")
	for _, actual := range a.Domain {
		total := 0.0
		for _, reported := range a.Domain {
			p := a.Report(reported, actual)
			if p < 0 {
				t.Fatalf("Report(%q|%q) = %v, must not be negative", reported, actual, p)
			}
			total += p
		}
		if total < 0.999999 || total > 1.000001 {
			t.Errorf("given the truth is %q, the answers sum to %v, want 1", actual, total)
		}
	}
}

// A look-alike must absorb far more error than an unrelated value.
func TestReportFavoursLookalikes(t *testing.T) {
	k := load(t, `{
	  "attributes": {"shape": {"noise": 0.06, "confusion": 0.3,
	     "confusable": [["torpedo", "elongated"]]}},
	  "entities": [
	    {"name": "a", "shape": "torpedo"},  {"name": "b", "shape": "elongated"},
	    {"name": "c", "shape": "seahorse"}, {"name": "d", "shape": "eel"}
	  ]
	}`)
	a := k.Attr("shape")
	near := a.Report("elongated", "torpedo")
	far := a.Report("seahorse", "torpedo")
	if near <= far {
		t.Errorf("saying elongated about a torpedo-shaped fish scores %.3f, "+
			"no better than saying seahorse (%.3f)", near, far)
	}
	if want := 0.3; near != want {
		t.Errorf("the only look-alike should take the whole confusion mass: got %.3f, want %.3f", near, want)
	}
}

// Without look-alike groups the old uniform behaviour must be untouched.
func TestReportWithoutGroupsIsUniform(t *testing.T) {
	k := load(t, `[{"name": "a", "shape": "torpedo"}, {"name": "b", "shape": "eel"},
	               {"name": "c", "shape": "flat"}]`)
	a := k.Attr("shape")
	if got, want := a.Report("torpedo", "torpedo"), 1-DefaultNoise; math.Abs(got-want) > 1e-9 {
		t.Errorf("P(correct) = %v, want %v", got, want)
	}
	if x, y := a.Report("eel", "torpedo"), a.Report("flat", "torpedo"); math.Abs(x-y) > 1e-9 {
		t.Errorf("with no groups declared every mistake should be equally likely: %v vs %v", x, y)
	}
}

func TestConfusionRejectsImpossibleRates(t *testing.T) {
	_, err := Load([]byte(`{
	  "attributes": {"shape": {"noise": 0.6, "confusion": 0.5, "confusable": [["a", "b"]]}},
	  "entities": [{"name": "x", "shape": "a"}, {"name": "y", "shape": "b"}]
	}`))
	if err == nil {
		t.Fatal("noise plus confusion above 1 should be rejected")
	}
}

func TestLintFlagsUnknownLookalike(t *testing.T) {
	k := load(t, `{
	  "attributes": {"shape": {"confusable": [["torpedo", "tropedo"]]}},
	  "entities": [{"name": "a", "shape": "torpedo"}, {"name": "b", "shape": "eel"}]
	}`)
	if !hasIssue(k.Lint(), `lists "tropedo" as a look-alike`) {
		t.Error("lint should flag a look-alike value that is not in the domain")
	}
}

func TestLoadImageAndLink(t *testing.T) {
	k := load(t, `[{
	  "name": "a",
	  "_link": "https://example.org/wiki/A",
	  "_image": {
	    "url": "https://example.org/a.jpg",
	    "source": "https://example.org/file/a.jpg",
	    "credit": "Someone",
	    "license": "CC BY-SA 4.0"
	  },
	  "colour": "red"
	}, {"name": "b", "colour": "blue"}]`)

	a, b := k.Entities[0], k.Entities[1]
	if a.Link != "https://example.org/wiki/A" {
		t.Errorf("link = %q", a.Link)
	}
	if a.Image == nil || a.Image.URL != "https://example.org/a.jpg" ||
		a.Image.Credit != "Someone" || a.Image.License != "CC BY-SA 4.0" {
		t.Errorf("image not parsed: %+v", a.Image)
	}
	if b.Image != nil {
		t.Error("an entity without _image should have a nil Image")
	}
	// Picture metadata must not become a question.
	for _, attr := range k.Attributes {
		if attr.Name == "_image" || attr.Name == "_link" || attr.Name == "image" {
			t.Errorf("%q leaked in as an attribute", attr.Name)
		}
	}
	if len(k.Attributes) != 1 {
		t.Errorf("got %d attributes, want just colour", len(k.Attributes))
	}
}

func TestImageWithoutURLIsAnError(t *testing.T) {
	_, err := Load([]byte(`[{"name": "a", "_image": {"credit": "Someone"}}, {"name": "b"}]`))
	if err == nil {
		t.Fatal("an _image with no url should be rejected")
	}
}

func TestLintFlagsUnattributedPicture(t *testing.T) {
	k := load(t, `[
	  {"name": "a", "_image": {"url": "https://example.org/a.jpg"}, "colour": "red"},
	  {"name": "b", "_image": {"url": "https://example.org/b.jpg", "source": "https://example.org/f/b",
	                           "credit": "X", "license": "CC0"}, "colour": "blue"}
	]`)
	issues := k.Lint()
	if !hasIssue(issues, "a: picture has no licence recorded") {
		t.Error("lint should flag a picture with no licence")
	}
	if !hasIssue(issues, "a: picture has no author recorded") {
		t.Error("lint should flag a picture with no author")
	}
	if hasIssue(issues, "b: picture has no") {
		t.Error("a fully attributed picture should not be flagged")
	}
}

// Every picture in the shipped knowledge base must be safe to publish.
func TestShippedPicturesAreAttributed(t *testing.T) {
	k, err := LoadFile("../data/adriatic-fish.json")
	if err != nil {
		t.Fatal(err)
	}
	withImage := 0
	for _, e := range k.Entities {
		if e.Image == nil {
			continue
		}
		withImage++
		for field, v := range map[string]string{
			"url": e.Image.URL, "source": e.Image.Source,
			"credit": e.Image.Credit, "license": e.Image.License,
		} {
			if v == "" {
				t.Errorf("%s: picture is missing %s", e.Name, field)
			}
		}
		if !strings.HasPrefix(e.Image.URL, "https://") {
			t.Errorf("%s: picture URL is not https", e.Name)
		}
	}
	if withImage < len(k.Entities)-1 {
		t.Errorf("only %d of %d species have a picture", withImage, len(k.Entities))
	}
}

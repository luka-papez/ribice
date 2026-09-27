package design

import (
	"bytes"
	"context"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lpapez/ribice/kb"
)

var hooks = []Option{{"none", "no streaks"}, {"straight", "straight or gently curved"}, {"curled", "curled up at one end"}}

func hooksTask() Task {
	return NewTask(Assign, "hooks", Subject{Entity: "Cirrus uncinus", Text: "commas"}, "Did the ends curl?", hooks, 2)
}

func TestTaskIDFollowsWhatTheExpertSees(t *testing.T) {
	a, b := hooksTask(), hooksTask()
	if a.ID == "" || a.ID != b.ID {
		t.Fatalf("the same task got ids %q and %q", a.ID, b.ID)
	}
	reworded := NewTask(Assign, "hooks", a.Subject, "Did the ends hook?", hooks, 2)
	relabelled := NewTask(Assign, "hooks", a.Subject, a.Question,
		[]Option{{"none", "no streaks"}, {"straight", "straight"}, {"curled", "curled up at one end"}}, 2)
	other := NewTask(Assign, "hooks", Subject{Entity: "Cirrus fibratus"}, a.Question, hooks, 2)
	for name, x := range map[string]Task{"reworded": reworded, "relabelled": relabelled, "other entity": other} {
		if x.ID == a.ID {
			t.Errorf("%s task kept the id %s", name, a.ID)
		}
	}
}

func TestPicks(t *testing.T) {
	for _, c := range []struct {
		picks []int
		conf  Confidence
		want  map[kb.Value]float64
	}{
		{[]int{2}, High, map[kb.Value]float64{"none": 0, "straight": 0, "curled": 1}},
		{[]int{2}, Medium, map[kb.Value]float64{"none": 0.1, "straight": 0.1, "curled": 0.8}},
		{[]int{1, 2}, Low, map[kb.Value]float64{"none": 0.5, "straight": 0.25, "curled": 0.25}},
		{[]int{0, 1, 2}, Low, map[kb.Value]float64{"none": 1.0 / 3, "straight": 1.0 / 3, "curled": 1.0 / 3}},
	} {
		got, err := Picks(hooks, c.picks, c.conf)
		if err != nil {
			t.Fatalf("Picks(%v, %s): %v", c.picks, c.conf, err)
		}
		for v, p := range c.want {
			if math.Abs(got[v]-p) > 1e-12 {
				t.Errorf("Picks(%v, %s)[%s] = %v, want %v", c.picks, c.conf, v, got[v], p)
			}
		}
		task := hooksTask()
		if err := (Verdict{Task: task.ID, P: got}).Check(task); err != nil {
			t.Errorf("Picks(%v, %s) is not a valid verdict: %v", c.picks, c.conf, err)
		}
	}
	for _, bad := range [][]int{{}, {3}, {-1}} {
		if _, err := Picks(hooks, bad, High); err == nil {
			t.Errorf("Picks(%v) gave no error", bad)
		}
	}
	if _, err := Picks(hooks, []int{0}, "sure"); err == nil {
		t.Errorf("an unknown confidence gave no error")
	}
}

func TestCheck(t *testing.T) {
	task := hooksTask()
	ok := []Verdict{
		{Task: task.ID, P: map[kb.Value]float64{"curled": 0.9, "straight": 0.1}},
		{Task: task.ID, P: map[kb.Value]float64{"curled": 0.6}, Abstain: 0.4, Reason: Ambiguous},
		Abstention(task.ID, "x", NotObservable),
	}
	for _, v := range ok {
		if err := v.Check(task); err != nil {
			t.Errorf("%+v: %v", v, err)
		}
	}
	bad := map[string]Verdict{
		"another task":   {Task: "other", P: map[kb.Value]float64{"curled": 1}},
		"not an option":  {Task: task.ID, P: map[kb.Value]float64{"hooked": 1}},
		"short of one":   {Task: task.ID, P: map[kb.Value]float64{"curled": 0.9}},
		"over one":       {Task: task.ID, P: map[kb.Value]float64{"curled": 0.9, "none": 0.2}},
		"negative":       {Task: task.ID, P: map[kb.Value]float64{"curled": 1.2, "none": -0.2}},
		"no reason":      {Task: task.ID, P: map[kb.Value]float64{"curled": 0.5}, Abstain: 0.5},
		"unknown reason": {Task: task.ID, Abstain: 1, Reason: "tired"},
		"stray reason":   {Task: task.ID, P: map[kb.Value]float64{"curled": 1}, Reason: Unknown},
	}
	for name, v := range bad {
		if v.Check(task) == nil {
			t.Errorf("%s: passed Check", name)
		}
	}
}

func TestStoreKeepsTheLatestVerdict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "verdicts.jsonl")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	first := Verdict{Task: "t1", Expert: "e", P: map[kb.Value]float64{"a": 1}}
	second := Verdict{Task: "t1", Expert: "e", P: map[kb.Value]float64{"b": 1}}
	other := Verdict{Task: "t1", Expert: "d", P: map[kb.Value]float64{"a": 1}}
	for _, v := range []Verdict{first, second, other} {
		if err := s.Put(v); err != nil {
			t.Fatal(err)
		}
	}

	again, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := again.Get("t1", "e"); !ok || v.P["b"] != 1 {
		t.Errorf("reopened store gives %+v, %v; want the second verdict", v, ok)
	}
	if _, ok := again.Get("t2", "e"); ok {
		t.Errorf("found a verdict never given")
	}
	vs := again.Verdicts("t1")
	if len(vs) != 2 || vs[0].Expert != "d" || vs[1].Expert != "e" {
		t.Errorf("Verdicts = %+v, want one each from d and e, in that order", vs)
	}
}

// scripted answers every task with a fixed verdict maker, stopping after
// limit tasks when limit > 0.
type scripted struct {
	id      string
	accepts func(Task) bool
	verdict func(Task) Verdict
	limit   int
	asked   []string
}

func (s *scripted) ID() string          { return s.id }
func (s *scripted) Accepts(t Task) bool { return s.accepts == nil || s.accepts(t) }
func (s *scripted) Answer(_ context.Context, tasks []Task, emit func(Verdict) error) error {
	for i, t := range tasks {
		if s.limit > 0 && i == s.limit {
			return nil
		}
		s.asked = append(s.asked, t.ID)
		if err := emit(s.verdict(t)); err != nil {
			return err
		}
	}
	return nil
}

func sure(expert string) func(Task) Verdict {
	return func(t Task) Verdict {
		return Verdict{Task: t.ID, Expert: expert, P: map[kb.Value]float64{t.Options[0].Value: 1}}
	}
}

func TestConsultAsksOnlyWhatIsMissing(t *testing.T) {
	k := cloudKB(t)
	tasks, err := FromKB(k, Perceive)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := OpenStore(filepath.Join(t.TempDir(), "v.jsonl"))

	stopper := &scripted{id: "e", verdict: sure("e"), limit: 5}
	if n, err := Consult(context.Background(), stopper, tasks, st); err != nil || n != 5 {
		t.Fatalf("first run stored %d, %v; want 5 before the expert stopped", n, err)
	}
	rest := &scripted{id: "e", verdict: sure("e"),
		accepts: func(t Task) bool { return t.Attribute != "halo" }}
	n, err := Consult(context.Background(), rest, tasks, st)
	if err != nil {
		t.Fatal(err)
	}
	halo := len(k.Attr("halo").Domain)
	if want := len(tasks) - 5 - halo; n != want || len(rest.asked) != want {
		t.Errorf("second run stored %d and asked %d, want %d each", n, len(rest.asked), want)
	}
	first := map[string]bool{}
	for _, id := range stopper.asked {
		first[id] = true
	}
	for _, id := range rest.asked {
		if first[id] {
			t.Errorf("task %s asked again after it was answered", id)
		}
	}
}

func TestConsultRejectsBadVerdicts(t *testing.T) {
	tasks := []Task{hooksTask()}
	for name, e := range map[string]*scripted{
		"malformed": {id: "e", verdict: func(t Task) Verdict {
			return Verdict{Task: t.ID, Expert: "e", P: map[kb.Value]float64{"curled": 0.5}}
		}},
		"unsigned":  {id: "e", verdict: sure("someone else")},
		"not given": {id: "e", verdict: func(Task) Verdict { return Abstention("nope", "e", Unknown) }},
	} {
		st, _ := OpenStore(filepath.Join(t.TempDir(), "v.jsonl"))
		if _, err := Consult(context.Background(), e, tasks, st); err == nil {
			t.Errorf("%s verdict was stored", name)
		}
		if _, ok := st.Get(tasks[0].ID, "e"); ok {
			t.Errorf("%s verdict is in the store", name)
		}
	}
}

func TestFromKBAndBack(t *testing.T) {
	k := cloudKB(t)
	assign, err := FromKB(k, Assign)
	if err != nil {
		t.Fatal(err)
	}
	if want := len(k.Entities) * len(k.Attributes); len(assign) != want {
		t.Errorf("%d assign tasks, want %d", len(assign), want)
	}
	for _, task := range assign {
		if task.Subject.Entity == "" || task.Subject.Value != "" {
			t.Fatalf("assign task about %+v", task.Subject)
		}
	}
	if _, err := FromKB(k, Observe); err == nil {
		t.Errorf("made observe tasks without photos")
	}

	var buf bytes.Buffer
	if err := WriteTasks(&buf, assign); err != nil {
		t.Fatal(err)
	}
	back, err := ReadTasks(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(back) != len(assign) || back[7].ID != assign[7].ID {
		t.Errorf("read back %d tasks, want the %d written", len(back), len(assign))
	}

	edited := strings.Replace(buf.String(), assign[0].Question, "Something else?", 1)
	if _, err := ReadTasks(strings.NewReader(edited)); err == nil {
		t.Errorf("a task edited by hand kept its old id")
	}
}

func cloudKB(t *testing.T) *kb.KB {
	t.Helper()
	k, err := kb.LoadFile("../data/clouds.json")
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestSettle(t *testing.T) {
	task := hooksTask()
	proposed := func(Task) []kb.Value { return []kb.Value{"curled"} }
	v := func(expert string, p map[kb.Value]float64, abstain float64, r Reason) Verdict {
		return Verdict{Task: task.ID, Expert: expert, P: p, Abstain: abstain, Reason: r}
	}
	curled := map[kb.Value]float64{"curled": 1}
	straight := map[kb.Value]float64{"straight": 1}
	for _, c := range []struct {
		name     string
		verdicts []Verdict
		want     Status
	}{
		{"none yet", nil, Unasked},
		{"all agree", []Verdict{v("a", curled, 0, ""), v("b", curled, 0, "")}, Agreed},
		{"two of three", []Verdict{v("a", curled, 0, ""), v("b", curled, 0, ""), v("c", straight, 0, "")}, Agreed},
		{"one of three", []Verdict{v("a", curled, 0, ""), v("b", straight, 0, ""), v("c", straight, 0, "")}, Disputed},
		{"most abstain", []Verdict{v("a", nil, 1, NotObservable), v("b", curled, 0, "")}, Abstained},
		{"a person settles it", []Verdict{v("a", straight, 0, ""), v("b", straight, 0, ""), v("human:luka", curled, 0, "")}, Agreed},
		{"a person disputes it", []Verdict{v("a", curled, 0, ""), v("human:luka", straight, 0, "")}, Disputed},
	} {
		got := Settle([]Task{task}, proposed, func(string) []Verdict { return c.verdicts })
		if len(got) != 1 || got[0].Status != c.want {
			t.Errorf("%s: %+v, want %s", c.name, got, c.want)
		}
	}
}

func TestKBValuesAreTheCurrentAnswerKey(t *testing.T) {
	k := cloudKB(t)
	tasks, _ := FromKB(k, Assign)
	values := KBValues(k)
	for _, task := range tasks {
		if task.Subject.Entity == "Cirrus uncinus" && task.Attribute == "hooks" {
			if got := values(task); len(got) != 1 || got[0] != kb.Yes {
				t.Errorf("Cirrus uncinus hooks = %v, want yes", got)
			}
			return
		}
	}
	t.Fatal("no task for Cirrus uncinus hooks")
}

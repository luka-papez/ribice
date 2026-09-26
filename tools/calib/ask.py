"""The answerer: Claude, shown a photo and never its label, answers the quiz.

Every question is asked about every photo once, and each answer is cached in
cache/answers.sqlite. Games are then replayed from the cache for free, and a
knowledge base that keeps a question's wording and options reuses its answers.

The key of an answer is the photo, the question, its option labels in order,
the prompt, the model and the mode, so rewording one question re-asks only
that question, and changing the prompt re-asks everything. The prompt's
"version" is a hash of its text: nobody has to remember to bump it.

Two modes:
  single  one call per photo and question, as the spec has it: a question
          judged alone cannot be answered from what the others suggest.
  photo   one call per photo with every question in it, 15 times cheaper.
          Whether the answers suffer is what a pilot of both measures.

The model sees options as numbered labels, never the knowledge base's value
names, and in photo mode the questions are keyed q1, q2, ...: "mamma" or
"hooks" would say more than the photo does.

    python tools/calib/ask.py --kb data/clouds.json [--mode single] [--split tune]
                              [--per-cloud 1] [--sample 8] [--dry-run]
"""
import argparse, base64, datetime, hashlib, json, os, sqlite3, sys

import backends

HERE = os.path.dirname(os.path.abspath(__file__))
MODEL = "claude-opus-5"
EFFORT = "low"

SYSTEM = """\
You are helping us test a quiz that identifies clouds. It is meant for people
with no training in weather or meteorology. You will see one photograph of the
sky and {what} from the quiz. Answer {it} as a curious person with no
training would, if they were standing where the camera was.

How to answer:
- Judge only what you can see in this photograph. Do not work out which type
  of cloud this is and then answer from what you know about that type. If a
  cloud name comes to mind, set it aside; the quiz is testing whether its
  questions can be answered without one.
- Read the question and the options the way an ordinary reader would. If the
  question depends on a word or an idea you would have to look up, say so
  instead of using its technical meaning.
- Some things cannot be seen in a photograph: whether it rained later, how the
  cloud moved, what the sun looked like when the sun is out of frame. For
  those, answer that you can't tell and why.
- If two options fit about equally well, you may choose both. Never choose
  more than two.
- Do not try to help the quiz get the right answer. An honest answer that
  turns out wrong is more useful to us than a right one you reasoned your
  way to.
{each}
First say in one sentence what in the photo you looked at. Then answer."""

SYSTEMS = {
    "single": SYSTEM.format(what="one question", it="it", each=""),
    "photo": SYSTEM.format(what="every question", it="each one", each="""\
- Answer each question on its own, from the photo, as if it were the only
  one asked. Do not let the other questions suggest what the cloud is.
"""),
}

ANSWER = {
    "type": "object",
    "properties": {
        "looked_at": {"type": "string",
                      "description": "one sentence: what in the photo you looked at"},
        "choice": {"type": "array", "items": {"type": "integer"},
                   "description": "the option numbers you chose: one, or two if torn; "
                                  "empty if you can't tell"},
        "cant_tell": {"type": "string",
                      "enum": ["no", "not_in_photo", "ambiguous", "unclear_question"],
                      "description": "no if you answered; otherwise why you can't: it can't "
                                     "be seen in a photo, the photo is ambiguous, or the "
                                     "question itself is unclear"},
        "confidence": {"type": "string", "enum": ["low", "medium", "high"]},
    },
    "required": ["looked_at", "choice", "cant_tell", "confidence"],
    "additionalProperties": False,
}


def questions(kb):
    """Each question as (attribute, question text, [(value, label), ...]), in
    the knowledge base's order. Yes/no questions are asked as Yes / No."""
    out = []
    for name, a in kb["attributes"].items():
        labels = a.get("labels")
        if labels:
            opts = list(labels.items())
        else:
            opts = [("yes", "Yes"), ("no", "No")]
        out.append((name, a["question"], opts))
    return out


def prompt_version(mode):
    return hashlib.sha256((SYSTEMS[mode] + json.dumps(ANSWER, sort_keys=True))
                          .encode()).hexdigest()[:8]


def key(photo, q, mode, model):
    name, text, opts = q
    blob = json.dumps([photo, text, [l for _, l in opts], prompt_version(mode), model, mode])
    return hashlib.sha256(blob.encode()).hexdigest()


def render(q, number=None):
    _, text, opts = q
    head = f"Question {number}: {text}" if number else f"Question: {text}"
    return head + "\nOptions:\n" + "\n".join(f"{i}. {l}" for i, (_, l) in enumerate(opts, 1))


def request(image_b64, body, mode, model, schema):
    return {
        "model": model,
        "max_tokens": 4000 if mode == "single" else 16000,
        "thinking": {"type": "adaptive"},
        "output_config": {"effort": EFFORT, "format": {"type": "json_schema", "schema": schema}},
        "system": SYSTEMS[mode],
        "messages": [{"role": "user", "content": [
            {"type": "image",
             "source": {"type": "base64", "media_type": "image/jpeg", "data": image_b64}},
            {"type": "text", "text": body},
        ]}],
    }


def photo_schema(qs):
    return {"type": "object",
            "properties": {f"q{i}": ANSWER for i in range(1, len(qs) + 1)},
            "required": [f"q{i}" for i in range(1, len(qs) + 1)],
            "additionalProperties": False}


def decode(reply, q):
    """One reply to one question as a cache row's fields, or None if it
    cannot be read: a choice out of range, more than two, or neither a choice
    nor a reason for having none."""
    if not isinstance(reply, dict) or not set(ANSWER["required"]) <= set(reply):
        return None
    opts = q[2]
    choice = reply["choice"]
    if not isinstance(choice, list) or len(choice) > 2 or len(set(choice)) != len(choice) \
            or not all(isinstance(c, int) and 1 <= c <= len(opts) for c in choice):
        return None
    cant = reply["cant_tell"]
    if cant == "no":
        if not choice:
            return None
        return {"values": [opts[c - 1][0] for c in choice], "cant_tell": None,
                "confidence": reply["confidence"], "looked_at": reply["looked_at"]}
    # a reason not to answer wins over a choice made anyway: a person who
    # says they can't tell would skip the question
    return {"values": [], "cant_tell": cant, "confidence": reply["confidence"],
            "looked_at": reply["looked_at"]}


def open_cache():
    os.makedirs(os.path.join(HERE, "cache"), exist_ok=True)
    db = sqlite3.connect(os.path.join(HERE, "cache", "answers.sqlite"))
    db.execute("""create table if not exists answers (
        key text primary key, kb text, photo text, attr text, question text,
        options text, mode text, prompt text, model text,
        answer_values text, cant_tell text, confidence text, looked_at text,
        raw text, created text)""")
    return db


def lookup(db, photos, qs, mode, model):
    """{photo id: {attribute: row dict}} for every cached answer."""
    keys = {key(p, q, mode, model): (p, q[0]) for p in photos for q in qs}
    out = {}
    rows = db.execute("select key, answer_values, cant_tell, confidence, looked_at from answers")
    for k, vals, cant, conf, looked in rows:
        if k in keys:
            p, attr = keys[k]
            out.setdefault(p, {})[attr] = {"values": json.loads(vals), "cant_tell": cant,
                                           "confidence": conf, "looked_at": looked}
    return out


def select(photos, split, per_cloud, sample=None):
    chosen, count = [], {}
    for p in photos:
        if split != "all" and p["split"] != split:
            continue
        if per_cloud and count.get(p["label"], 0) >= per_cloud:
            continue
        count[p["label"]] = count.get(p["label"], 0) + 1
        chosen.append(p)
    if sample and sample < len(chosen):
        # evenly spaced, so a pilot spans cirrus to cumulonimbus
        chosen = [chosen[i * len(chosen) // sample] for i in range(sample)]
    return chosen


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--kb", required=True)
    ap.add_argument("--mode", choices=["single", "photo"], default="single")
    ap.add_argument("--split", choices=["tune", "held_out", "all"], default="all")
    ap.add_argument("--per-cloud", type=int, help="ask about at most this many photos per cloud")
    ap.add_argument("--sample", type=int, help="then only this many photos, evenly spread")
    ap.add_argument("--model", default=MODEL)
    ap.add_argument("--via", choices=["cli", "batch", "api"], default="cli")
    ap.add_argument("--jobs", type=int, default=4)
    ap.add_argument("--max-window", type=float, default=0.8)
    ap.add_argument("--dry-run", action="store_true", help="count what would be asked, ask nothing")
    a = ap.parse_args()

    kb = json.load(open(a.kb))
    base = os.path.splitext(os.path.basename(a.kb))[0]
    photos = json.load(open(os.path.join(HERE, "corpus", base, "manifest.json")))["photos"]
    photos = select(photos, a.split, a.per_cloud, a.sample)
    qs = questions(kb)
    db = open_cache()
    have = lookup(db, [p["id"] for p in photos], qs, a.mode, a.model)

    # what each request covers: [(photo, question)], keyed by a batch-safe id
    todo = {}
    for p in photos:
        missing = [q for q in qs if q[0] not in have.get(p["id"], {})]
        if a.mode == "single":
            for q in missing:
                todo[key(p["id"], q, a.mode, a.model)[:40]] = (p, [q])
        elif missing:
            # the photo's questions travel together; ask them all again, since
            # an answer depends on the company it was asked in
            todo[f"{p['id']}-{prompt_version(a.mode)}"] = (p, qs)
    n_q = sum(len(v[1]) for v in todo.values())
    print(f"{len(photos)} photos x {len(qs)} questions: {n_q} answers missing, "
          f"{len(todo)} calls to {a.model} ({a.mode} mode)", file=sys.stderr)
    if a.dry_run or not todo:
        return

    images = {}

    def image(p):
        if p["id"] not in images:
            path = os.path.join(HERE, "cache", "photos", base, p["id"] + ".jpg")
            with open(path, "rb") as f:
                images[p["id"]] = base64.b64encode(f.read()).decode()
        return images[p["id"]]

    reqs = {}
    for rid, (p, group) in todo.items():
        if a.mode == "single":
            reqs[rid] = request(image(p), render(group[0]), a.mode, a.model, ANSWER)
        else:
            body = "\n\n".join(render(q, i) for i, q in enumerate(group, 1))
            reqs[rid] = request(image(p), body, a.mode, a.model, photo_schema(group))

    def valid(v):
        if a.mode == "single":
            return isinstance(v, dict) and set(ANSWER["required"]) <= set(v)
        return isinstance(v, dict) and all(f"q{i}" in v for i in range(1, len(qs) + 1))

    stats = {"stored": 0, "unusable": 0}

    def record(rid, reply):
        p, group = todo[rid]
        if reply is None:
            # not cached, so it is asked again next run rather than frozen as a skip
            stats["unusable"] += len(group)
            return
        now = datetime.datetime.now().isoformat(timespec="seconds")
        for i, q in enumerate(group, 1):
            r = reply if a.mode == "single" else reply.get(f"q{i}")
            row = decode(r, q)
            if row is None:
                stats["unusable"] += 1
                continue
            db.execute("insert or replace into answers values (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
                       (key(p["id"], q, a.mode, a.model), base, p["id"], q[0], q[1],
                        json.dumps([l for _, l in q[2]]), a.mode, prompt_version(a.mode),
                        a.model, json.dumps(row["values"]), row["cant_tell"],
                        row["confidence"], row["looked_at"], json.dumps(r), now))
            stats["stored"] += 1
        db.commit()

    backends.run(reqs, record, valid, via=a.via, jobs=a.jobs, max_window=a.max_window,
                 state_path=os.path.join(HERE, "cache", f"ask-{base}.batch.json"))
    print(f"  {stats['stored']} answers stored, {stats['unusable']} unusable "
          "(asked again next run)", file=sys.stderr)


if __name__ == "__main__":
    main()

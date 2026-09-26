"""Ask Claude whether each corpus photo is a fair example of its label.

Commons labels are sometimes wrong, and a mislabelled photo punishes a correct
knowledge base. This pass shows Claude the photo *and* its label, so it never
feeds a score: it only nominates photos for a person to keep or drop.

Photos it doubts are added to corpus/<kb>/review.json with "decision": null.
Set each to "keep" or "drop"; corpus.py then refills whatever was dropped, and
the next run of this script checks the replacements. The gate for phase 1a is
a review.json with no null decisions left.

Results are saved as they arrive and a photo is never asked twice, so
running it again only checks what is new.

Three ways to reach Claude (--via):
  cli    the Claude Code CLI, on a Claude subscription (see claude_cli.py).
         Stops starting calls when the five-hour window is --max-window full;
         run it again after the window resets to go on.
  batch  the Batch API, at half the API price; needs ANTHROPIC_API_KEY. A
         submitted batch is remembered, so an interrupted run resumes it.
  api    the API directly: faster than a batch, twice the price.

    python tools/calib/label_check.py --kb data/clouds.json [--via cli] [--limit 5]
"""
import argparse, base64, concurrent.futures, datetime, json, os, sys, time

HERE = os.path.dirname(os.path.abspath(__file__))
MODEL = "claude-opus-5"
PRICE = {"claude-opus-5": (5.00, 25.00), "claude-sonnet-5": (2.00, 10.00),
         "claude-haiku-4-5": (1.00, 5.00)}     # $ per million tokens, in / out, June 2026

SYSTEM = """\
You check photographs for a test set. Each was filed on Wikimedia Commons as
one type of cloud, and will be used to test a quiz that identifies clouds for
people standing on the ground. Judge whether the photo is a fair example of
the type it was filed as.

Real skies are rarely textbook. A photo that is mostly the filed type is a
fair example, even with some features of a neighbouring type, a contrail, or
a little of another cloud at the edge. Answer "no" only when you would bet
the main cloud is a different type, and set one_type to false only when two
or more types share the frame about equally. Use "unsure" when the photo
could honestly go either way; a person looks at every "no"."""

SCHEMA = {
    "type": "object",
    "properties": {
        "view": {"type": "string", "enum": ["ground", "aerial", "not_a_photo"],
                 "description": "ground: a photograph of the sky taken from the ground. "
                                "aerial: from a plane, a mountain above the clouds, or space. "
                                "not_a_photo: a diagram, painting, render or heavy composite."},
        "one_type": {"type": "boolean",
                     "description": "one cloud type makes up most of the cloud in the photo"},
        "matches": {"type": "string", "enum": ["yes", "no", "unsure"],
                    "description": "the main cloud is the type it was filed as"},
        "seen_as": {"type": "string",
                    "description": "if matches is not yes, the type you would call it; else empty"},
        "reason": {"type": "string", "description": "one sentence"},
    },
    "required": ["view", "one_type", "matches", "seen_as", "reason"],
    "additionalProperties": False,
}


def request(photo, note, image_b64):
    text = f"Filed as: {photo['label']}\n"
    if note:
        text += f"What the quiz means by it: {note}\n"
    return {
        "model": MODEL,
        "max_tokens": 4000,
        "thinking": {"type": "adaptive"},
        "output_config": {"effort": "low",
                          "format": {"type": "json_schema", "schema": SCHEMA}},
        "system": SYSTEM,
        "messages": [{"role": "user", "content": [
            {"type": "image",
             "source": {"type": "base64", "media_type": "image/jpeg", "data": image_b64}},
            {"type": "text", "text": text},
        ]}],
    }


def parse(message):
    """The verdict in a reply, or None for a refusal or a reply cut short."""
    if message.stop_reason != "end_turn":
        return None
    text = next((b.text for b in message.content if b.type == "text"), "")
    try:
        v = json.loads(text)
    except json.JSONDecodeError:
        return None
    return v if valid(v) else None


def doubted(v):
    """What a person must look at. "unsure" is not on the list: arguable
    species calls are what the quiz is tested on, not a labelling error."""
    return v["view"] != "ground" or not v["one_type"] or v["matches"] == "no"


class Usage:
    def __init__(self, batched):
        self.inp = self.out = 0
        self.batched = batched

    def add(self, message):
        u = message.usage
        self.inp += u.input_tokens + (u.cache_creation_input_tokens or 0) + \
                    (u.cache_read_input_tokens or 0)
        self.out += u.output_tokens

    def __str__(self):
        pin, pout = PRICE.get(MODEL, (0, 0))
        cost = (self.inp * pin + self.out * pout) / 1e6 * (0.5 if self.batched else 1)
        return f"{self.inp:,} tokens in, {self.out:,} out, about ${cost:.2f}"


def valid(v):
    return isinstance(v, dict) and set(SCHEMA["required"]) <= set(v)


def run_cli(reqs, record, jobs, max_window):
    """A few at a time through `claude -p`, each retried once if unusable."""
    import claude_cli
    pool = claude_cli.Pool(max_window)

    def one(item):
        pid, params = item
        for _ in range(2):
            v, info = pool.call(params)
            if valid(v):
                return pid, v, None
            if info["error"] == "paused":
                break
        return pid, None, info["error"]

    done = 0
    with concurrent.futures.ThreadPoolExecutor(jobs) as ex:
        for pid, v, err in ex.map(one, reqs.items()):
            if err == "paused":
                continue
            done += 1
            record(pid, v)
            if err:
                print(f"\n  {pid}: {err}", file=sys.stderr)
            print(f"\r  {done}/{len(reqs)}  window {pool.window or 0:.0%}", end="", file=sys.stderr)
    print(f"\n  about ${pool.cost:.2f} at API prices, from the subscription", file=sys.stderr)
    if pool.paused():
        at = datetime.datetime.fromtimestamp(pool.resets).strftime("%H:%M") if pool.resets else "later"
        print(f"  paused with {len(reqs) - done} left: the five-hour window is {pool.window:.0%} "
              f"used (limit {max_window:.0%}); it resets at {at}", file=sys.stderr)


def run_direct(client, reqs, usage, record):
    """Eight at a time, each retried once if its reply is unusable."""
    def one(item):
        pid, params = item
        for _ in range(2):
            m = client.messages.create(**params)
            usage.add(m)
            v = parse(m)
            if v:
                return pid, v
        return pid, None

    with concurrent.futures.ThreadPoolExecutor(8) as pool:
        for n, (pid, v) in enumerate(pool.map(one, reqs.items()), 1):
            record(pid, v)
            print(f"\r  {n}/{len(reqs)}", end="", file=sys.stderr)
    print(f"\n  {usage}", file=sys.stderr)


def run_batch(client, reqs, usage, record, state_path):
    from anthropic.types.message_create_params import MessageCreateParamsNonStreaming
    from anthropic.types.messages.batch_create_params import Request

    state = json.load(open(state_path)) if os.path.exists(state_path) else None
    if state:
        # whatever it asked for, it is paid for: collect it rather than ask again
        print(f"  resuming batch {state['batch']}", file=sys.stderr)
    else:
        batch = client.messages.batches.create(requests=[
            Request(custom_id=pid, params=MessageCreateParamsNonStreaming(**params))
            for pid, params in reqs.items()])
        state = {"batch": batch.id, "ids": list(reqs)}
        json.dump(state, open(state_path, "w"))
        print(f"  submitted batch {batch.id} ({len(reqs)} photos)", file=sys.stderr)

    while True:
        b = client.messages.batches.retrieve(state["batch"])
        if b.processing_status == "ended":
            break
        c = b.request_counts
        print(f"  {c.processing} processing, {c.succeeded} done", file=sys.stderr)
        time.sleep(60)

    for r in client.messages.batches.results(state["batch"]):
        if r.result.type == "succeeded":
            usage.add(r.result.message)
            record(r.custom_id, parse(r.result.message))
        else:
            print(f"  {r.custom_id}: {r.result.type}", file=sys.stderr)
    os.remove(state_path)
    print(f"  {usage}", file=sys.stderr)


def main():
    global MODEL
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--kb", required=True)
    ap.add_argument("--via", choices=["cli", "batch", "api"], default="cli")
    ap.add_argument("--jobs", type=int, default=4, help="concurrent calls with --via cli")
    ap.add_argument("--max-window", type=float, default=0.8,
                    help="with --via cli, stop starting calls once the five-hour window is this full")
    ap.add_argument("--limit", type=int, help="check only this many photos, to try it out")
    ap.add_argument("--model", default=MODEL)
    a = ap.parse_args()
    MODEL = a.model

    kb = json.load(open(a.kb))
    base = os.path.splitext(os.path.basename(a.kb))[0]
    corpus_dir = os.path.join(HERE, "corpus", base)
    cache = os.path.join(HERE, "cache", "photos", base)
    photos = json.load(open(os.path.join(corpus_dir, "manifest.json")))["photos"]
    notes = {e["name"]: e.get("_note", "") for e in kb["entities"]}
    results_path = os.path.join(corpus_dir, "label-check.json")
    results = json.load(open(results_path)) if os.path.exists(results_path) else {}

    todo = [p for p in photos if p["id"] not in results][:a.limit]
    missing = [p for p in todo if not os.path.exists(os.path.join(cache, p["id"] + ".jpg"))]
    if missing:
        sys.exit(f"{len(missing)} photos are not downloaded; run corpus.py first")
    print(f"{len(todo)} of {len(photos)} photos to check with {MODEL}", file=sys.stderr)

    by_id = {p["id"]: p for p in photos}
    bad = []

    def record(pid, v):
        if not v:
            bad.append(pid)
            return
        # the file rides along so a result stays readable if the manifest changes
        results[pid] = {"file": by_id[pid]["file"], "label": by_id[pid]["label"],
                        "model": MODEL, **v}
        with open(results_path, "w") as f:
            json.dump(dict(sorted(results.items())), f, indent=1, ensure_ascii=False)
            f.write("\n")

    if todo:
        reqs = {}
        for p in todo:
            with open(os.path.join(cache, p["id"] + ".jpg"), "rb") as f:
                reqs[p["id"]] = request(p, notes.get(p["label"]), base64.b64encode(f.read()).decode())
        if a.via == "cli":
            run_cli(reqs, record, a.jobs, a.max_window)
        else:
            import anthropic
            client = anthropic.Anthropic()
            usage = Usage(batched=a.via == "batch")
            if a.via == "api":
                run_direct(client, reqs, usage, record)
            else:
                state = os.path.join(HERE, "cache", f"label-check-{base}.batch.json")
                run_batch(client, reqs, usage, record, state)
        if bad:
            print(f"  {len(bad)} unusable replies, asked again next run: {' '.join(bad)}",
                  file=sys.stderr)

    # Only photos still in the manifest: one already dropped needs no decision.
    review_path = os.path.join(corpus_dir, "review.json")
    review = json.load(open(review_path)) if os.path.exists(review_path) else {}
    for p in photos:
        v = results.get(p["id"])
        if v and doubted(v) and p["file"] not in review:
            review[p["file"]] = {"id": p["id"], "label": p["label"], "view": v["view"],
                                 "one_type": v["one_type"], "matches": v["matches"],
                                 "seen_as": v["seen_as"], "reason": v["reason"],
                                 "decision": None}
    with open(review_path, "w") as f:
        json.dump(review, f, indent=1, ensure_ascii=False)
        f.write("\n")

    open_ = [(f, r) for f, r in review.items() if r["decision"] is None]
    print(f"\n{sum(1 for p in photos if p['id'] in results)}/{len(photos)} checked; "
          f"{len(open_)} doubts awaiting a decision in {os.path.relpath(review_path)}",
          file=sys.stderr)
    for f, r in open_:
        seen = f" (looks like {r['seen_as']})" if r["seen_as"] else ""
        print(f"  {r['id']}  {r['label']:<26} {r['matches']:<6} {r['view']:<6} "
              f"{'one type' if r['one_type'] else 'mixed':<8} {r['reason']}{seen}", file=sys.stderr)


if __name__ == "__main__":
    main()

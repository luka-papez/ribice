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

--via picks how to reach Claude: the Claude Code CLI on a subscription (the
default), the Batch API or the API; see backends.py.

    python tools/calib/label_check.py --kb data/clouds.json [--via cli] [--limit 5]
"""
import argparse, base64, json, os, sys

import backends

HERE = os.path.dirname(os.path.abspath(__file__))
MODEL = "claude-opus-5"

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


def doubted(v):
    """What a person must look at. "unsure" is not on the list: arguable
    species calls are what the quiz is tested on, not a labelling error."""
    return v["view"] != "ground" or not v["one_type"] or v["matches"] == "no"


def valid(v):
    return isinstance(v, dict) and set(SCHEMA["required"]) <= set(v)


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
        backends.run(reqs, record, valid, via=a.via, jobs=a.jobs, max_window=a.max_window,
                     state_path=os.path.join(HERE, "cache", f"label-check-{base}.batch.json"))
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

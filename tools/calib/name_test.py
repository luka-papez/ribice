"""Leakage check: can the answerer name the cloud in each photo?

The answerer is told to set aside any cloud name that comes to mind. If it
doesn't, its answers agree with the knowledge base far more often on photos it
recognises than on photos it doesn't, since it answers from the textbook
there. This asks, in a separate call with no quiz in sight, what each photo
shows; report.py then splits agreement by whether the name was right.

Names go to cache/names-<kb>.json, keyed by photo, and are asked once.

    python tools/calib/name_test.py --kb data/clouds.json [--split tune]
"""
import argparse, base64, json, os, sys

import ask, backends

HERE = os.path.dirname(os.path.abspath(__file__))

SYSTEM = """\
You name clouds in photographs using the WMO classification: a genus and,
where it has one, a species (Cirrus fibratus, Cumulus mediocris,
Nimbostratus). Name the main cloud in the photo, as a trained observer would."""

SCHEMA = {
    "type": "object",
    "properties": {
        "name": {"type": "string", "description": "genus and species, e.g. Altocumulus floccus"},
        "confidence": {"type": "string", "enum": ["low", "medium", "high"]},
    },
    "required": ["name", "confidence"],
    "additionalProperties": False,
}


def path(base):
    return os.path.join(HERE, "cache", f"names-{base}.json")


def load(base):
    p = path(base)
    return json.load(open(p)) if os.path.exists(p) else {}


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--kb", required=True)
    ap.add_argument("--split", choices=["tune", "held_out", "all"], default="all")
    ap.add_argument("--per-cloud", type=int)
    ap.add_argument("--sample", type=int)
    ap.add_argument("--model", default=ask.MODEL)
    ap.add_argument("--via", choices=["cli", "batch", "api"], default="cli")
    ap.add_argument("--jobs", type=int, default=4)
    ap.add_argument("--max-window", type=float, default=0.8)
    a = ap.parse_args()

    base = os.path.splitext(os.path.basename(a.kb))[0]
    photos = json.load(open(os.path.join(HERE, "corpus", base, "manifest.json")))["photos"]
    photos = ask.select(photos, a.split, a.per_cloud, a.sample)
    names = load(base)
    todo = [p for p in photos if p["id"] not in names]
    print(f"{len(todo)} of {len(photos)} photos to name", file=sys.stderr)

    reqs = {}
    for p in todo:
        with open(os.path.join(HERE, "cache", "photos", base, p["id"] + ".jpg"), "rb") as f:
            img = base64.b64encode(f.read()).decode()
        reqs[p["id"]] = {
            "model": a.model, "max_tokens": 2000, "thinking": {"type": "adaptive"},
            "output_config": {"effort": ask.EFFORT,
                              "format": {"type": "json_schema", "schema": SCHEMA}},
            "system": SYSTEM,
            "messages": [{"role": "user", "content": [
                {"type": "image", "source": {"type": "base64", "media_type": "image/jpeg",
                                             "data": img}},
                {"type": "text", "text": "What is the main cloud in this photo?"}]}]}

    def record(pid, v):
        if v:
            names[pid] = {**v, "model": a.model}
            with open(path(base), "w") as f:
                json.dump(names, f, indent=1)

    backends.run(reqs, record, lambda v: isinstance(v, dict) and "name" in v, via=a.via,
                 jobs=a.jobs, max_window=a.max_window,
                 state_path=os.path.join(HERE, "cache", f"names-{base}.batch.json"))
    labels = {p["id"]: p["label"] for p in photos}
    named = [pid for pid in labels if pid in names]
    right = sum(names[pid]["name"].strip().lower() == labels[pid].lower() for pid in named)
    genus = sum(names[pid]["name"].split()[0].lower() == labels[pid].split()[0].lower()
                for pid in named if names[pid]["name"].split())
    print(f"named {len(named)}: {right} exactly right, {genus} right genus", file=sys.stderr)


if __name__ == "__main__":
    main()

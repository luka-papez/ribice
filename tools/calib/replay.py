"""Replay the quiz from cached answers: one game per photo, no model involved.

Reads the answers ask.py cached for a knowledge base's current wording,
writes them as the answers file `ribice -replay` reads, and returns the games
it plays. A question with no cached answer is left out, so the engine treats
it as a gap and the report can say so; a photo with no answers at all is not
played.

    python tools/calib/replay.py --kb data/clouds.json [--mode single] [--out DIR]
"""
import argparse, json, os, subprocess, sys

import ask

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(HERE))


def engine():
    """A fresh build of the engine, so a replay never runs stale code."""
    path = os.path.join(HERE, "cache", "ribice")
    subprocess.run(["go", "build", "-o", path, "./cmd/ribice"], cwd=ROOT, check=True)
    return path


def sightings(kb, photos, mode, model):
    """The answers file's lines, and how many answers were missing."""
    qs = ask.questions(kb)
    db = ask.open_cache()
    have = ask.lookup(db, [p["id"] for p in photos], qs, mode, model)
    lines, gaps = [], 0
    for p in photos:
        got = have.get(p["id"])
        if not got:
            continue
        answers = {}
        for attr, _, _ in qs:
            r = got.get(attr)
            if r is None:
                gaps += 1
            elif r["cant_tell"]:
                answers[attr] = {"cant_tell": r["cant_tell"]}
            else:
                answers[attr] = {"values": r["values"], "confidence": r["confidence"]}
        lines.append({"photo": p["id"], "target": p["label"], "split": p["split"],
                      "answers": answers})
    return lines, gaps


def play(kb_path, lines, workdir, flags=()):
    """Run the engine over the answers; return one game dict per line."""
    os.makedirs(workdir, exist_ok=True)
    path = os.path.join(workdir, "answers.jsonl")
    with open(path, "w") as f:
        for line in lines:
            f.write(json.dumps(line) + "\n")
    out = subprocess.run([engine(), "-kb", os.path.abspath(kb_path), "-replay", path, "-json",
                          *flags], capture_output=True, text=True, check=True).stdout
    games = [json.loads(l) for l in out.splitlines() if l.strip()]
    with open(os.path.join(workdir, "games.jsonl"), "w") as f:
        f.write(out)
    return games


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--kb", required=True)
    ap.add_argument("--mode", choices=["single", "photo"], default="single")
    ap.add_argument("--model", default=ask.MODEL)
    ap.add_argument("--out", default=os.path.join(HERE, "cache", "replay"))
    a = ap.parse_args()

    kb = json.load(open(a.kb))
    base = os.path.splitext(os.path.basename(a.kb))[0]
    photos = json.load(open(os.path.join(HERE, "corpus", base, "manifest.json")))["photos"]
    lines, gaps = sightings(kb, photos, a.mode, a.model)
    if not lines:
        sys.exit(f"no cached answers for {base} in {a.mode} mode; run ask.py first")
    games = play(a.kb, lines, a.out)
    right = sum(g["correct"] for g in games)
    print(f"{len(games)} games, {right} right ({right / len(games):.0%}), "
          f"{sum(g['questions'] for g in games) / len(games):.1f} questions on average; "
          f"{gaps} answers missing", file=sys.stderr)


if __name__ == "__main__":
    main()

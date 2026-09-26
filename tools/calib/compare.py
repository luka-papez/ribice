"""Replay two knowledge bases on the same photos and say which wins.

Both are played from the same cached answers, so the comparison is paired:
every photo is one game under each. A knowledge base that rewords a question
has no answers for it until ask.py runs; its games treat the question as a
gap, and compare.py says so rather than scoring it as if nothing changed.

The acceptance rule is the spec's:
  score = accuracy in percent - 5 x mean questions,
so one question saved is worth 5 points of accuracy. The candidate is
accepted when its score is higher and no more than 2 games flipped from
right to wrong. It must also still pass -lint and -simulate, so no two
entities have become inseparable.

    python tools/calib/compare.py data/clouds.json runs/.../clouds.calibrated.json
"""
import argparse, json, os, re, subprocess, sys

import ask, replay

HERE = os.path.dirname(os.path.abspath(__file__))
MAX_FLIPS = 2


def score(games):
    acc = 100 * sum(g["correct"] for g in games) / len(games)
    q = sum(g["questions"] for g in games) / len(games)
    return acc, q, acc - 5 * q


def identified(eng, kb_path):
    """How many entities -simulate identifies. Its exit status does not say:
    it only fails on errors, so the count is read from its summary."""
    sim = subprocess.run([eng, "-kb", kb_path, "-simulate"], capture_output=True, text=True)
    m = re.search(r"identified\s+(\d+)/(\d+)", sim.stdout)
    if sim.returncode or not m:
        return None
    return int(m.group(1))


def checks(base_path, kb_path):
    """-lint and -simulate on a candidate: (ok, what failed). -simulate must
    identify as many entities as it does on the current knowledge base."""
    eng = replay.engine()
    lint = subprocess.run([eng, "-kb", kb_path, "-lint"], capture_output=True, text=True)
    problems = []
    if lint.returncode:
        problems.append("lint: " + (lint.stdout + lint.stderr).strip().splitlines()[-1])
    before, after = identified(eng, base_path), identified(eng, kb_path)
    if after is None:
        problems.append("simulate: did not run")
    elif before is not None and after < before:
        problems.append(f"simulate: identifies {after} entities, down from {before}")
    return not problems, problems


def compare(base_path, cand_path, split="held_out", mode="photo", model=ask.MODEL):
    """Play both on one split. Returns a dict with both scores, the flips and
    the verdict."""
    base_kb, cand_kb = json.load(open(base_path)), json.load(open(cand_path))
    name = os.path.splitext(os.path.basename(base_path))[0]
    photos = json.load(open(os.path.join(HERE, "corpus", name, "manifest.json")))["photos"]
    photos = [p for p in photos if split == "all" or p["split"] == split]

    runs = {}
    for tag, path, kb in (("base", base_path, base_kb), ("cand", cand_path, cand_kb)):
        lines, gaps = replay.sightings(kb, photos, mode, model)
        runs[tag] = {"gaps": gaps, "photos": {l["photo"] for l in lines},
                     "games": replay.play(path, lines, os.path.join(HERE, "cache", "compare", tag))}
    # only photos both could play, so neither side is scored on games the other skipped
    shared = runs["base"]["photos"] & runs["cand"]["photos"]
    base = {g["photo"]: g for g in runs["base"]["games"] if g["photo"] in shared}
    cand = {g["photo"]: g for g in runs["cand"]["games"] if g["photo"] in shared}
    if not shared:
        sys.exit(f"no {split} photos have cached answers under both knowledge bases")

    b, c = score(list(base.values())), score(list(cand.values()))
    lost = sorted(p for p in shared if base[p]["correct"] and not cand[p]["correct"])
    won = sorted(p for p in shared if cand[p]["correct"] and not base[p]["correct"])
    ok, problems = checks(base_path, cand_path)
    accepted = c[2] > b[2] and len(lost) <= MAX_FLIPS and ok and runs["cand"]["gaps"] == 0
    return {"games": len(shared), "base": b, "cand": c, "lost": lost, "won": won,
            "gaps": runs["cand"]["gaps"], "checks": problems, "accepted": accepted,
            "cand_games": cand, "base_games": base}


def describe(r, base_name, cand_name):
    b, c = r["base"], r["cand"]
    out = [f"| | {base_name} | {cand_name} |", "| --- | --- | --- |",
           f"| games | {r['games']} | {r['games']} |",
           f"| right | {b[0]:.0f}% | {c[0]:.0f}% |",
           f"| questions | {b[1]:.1f} | {c[1]:.1f} |",
           f"| score (right - 5 x questions) | {b[2]:.1f} | {c[2]:.1f} |", "",
           f"Right under the candidate only: {len(r['won'])}. "
           f"Right under the current one only: {len(r['lost'])} (at most {MAX_FLIPS} allowed)."]
    if r["gaps"]:
        out.append(f"The candidate has {r['gaps']} unanswered questions (reworded or new): "
                   "run ask.py on it before judging.")
    if r["checks"]:
        out.append("Fails " + "; ".join(r["checks"]) + ".")
    out.append(f"\n**{'Accepted' if r['accepted'] else 'Not accepted'}.**")
    return "\n".join(out) + "\n"


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("base", help="the current knowledge base")
    ap.add_argument("candidate")
    ap.add_argument("--split", choices=["tune", "held_out", "all"], default="held_out")
    ap.add_argument("--mode", choices=["single", "photo"], default="photo")
    ap.add_argument("--model", default=ask.MODEL)
    a = ap.parse_args()
    r = compare(a.base, a.candidate, a.split, a.mode, a.model)
    print(describe(r, os.path.relpath(a.base), os.path.relpath(a.candidate)))
    sys.exit(0 if r["accepted"] else 1)


if __name__ == "__main__":
    main()

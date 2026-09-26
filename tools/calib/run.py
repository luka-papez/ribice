"""Measure a knowledge base: ask what is missing, replay, report.

Writes runs/<date>-<label>/: a copy of the knowledge base it scored,
config.json, the answers file, games.jsonl and report.md. Runs are committed,
so the history of scores sits in git next to the changes that caused them.

Asking stops when the subscription's window is full (see backends.py); the
report is then written from what is cached so far, with the missing answers
counted as gaps, and running the same command again later picks up where it
stopped.

    python tools/calib/run.py --kb data/clouds.json --label baseline
"""
import argparse, datetime, json, os, shutil, subprocess, sys

import ask, calibrate, compare, replay, report

HERE = os.path.dirname(os.path.abspath(__file__))


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--kb", required=True)
    ap.add_argument("--label", required=True, help="names the run directory")
    ap.add_argument("--mode", choices=["single", "photo"], default="single")
    ap.add_argument("--split", choices=["tune", "held_out", "all"], default="all")
    ap.add_argument("--per-cloud", type=int)
    ap.add_argument("--sample", type=int)
    ap.add_argument("--model", default=ask.MODEL)
    ap.add_argument("--via", choices=["cli", "batch", "api"], default="cli")
    ap.add_argument("--jobs", type=int, default=4)
    ap.add_argument("--max-window", type=float, default=0.8)
    ap.add_argument("--no-ask", action="store_true", help="replay and report from the cache only")
    a = ap.parse_args()

    here = [sys.executable, os.path.join(HERE, "ask.py"), "--kb", a.kb, "--mode", a.mode,
            "--split", a.split, "--model", a.model, "--via", a.via, "--jobs", str(a.jobs),
            "--max-window", str(a.max_window)]
    if a.per_cloud:
        here += ["--per-cloud", str(a.per_cloud)]
    if a.sample:
        here += ["--sample", str(a.sample)]
    if not a.no_ask:
        subprocess.run(here, check=True)

    kb = json.load(open(a.kb))
    base = os.path.splitext(os.path.basename(a.kb))[0]
    corpus = json.load(open(os.path.join(HERE, "corpus", base, "manifest.json")))["photos"]
    photos = ask.select(corpus, a.split, a.per_cloud, a.sample)
    lines, gaps = replay.sightings(kb, photos, a.mode, a.model)
    if not lines:
        sys.exit("nothing cached to replay yet")

    run_dir = os.path.join(HERE, "runs", f"{datetime.date.today()}-{a.label}")
    os.makedirs(run_dir, exist_ok=True)
    shutil.copy(a.kb, os.path.join(run_dir, os.path.basename(a.kb)))
    games = replay.play(a.kb, lines, run_dir)
    with open(os.path.join(run_dir, "config.json"), "w") as f:
        json.dump({"kb": base, "model": a.model, "effort": ask.EFFORT, "mode": a.mode,
                   "prompt": ask.prompt_version(a.mode), "split": a.split,
                   "per_cloud": a.per_cloud, "sample": a.sample, "photos": len(lines),
                   "photos_not_asked": len(photos) - len(lines),
                   "answers_missing": gaps}, f, indent=1)
        f.write("\n")

    import label_check
    checks = json.load(open(os.path.join(HERE, "corpus", base, "label-check.json")))
    passed = {pid: not label_check.doubted(v) for pid, v in checks.items()}
    matrix = ask.lookup(ask.open_cache(), [l["photo"] for l in lines], ask.questions(kb),
                        a.mode, a.model)
    title = f"{base}: {a.label} ({a.mode} mode, {a.model})"
    # the whole corpus, so "clouds with 4+ photos" counts photos, not the sample
    import name_test
    text = report.build(kb, corpus, games, matrix, passed, title, name_test.load(base),
                        report.load_human(os.path.join(HERE, "corpus", base)))
    unasked = len(photos) - len(lines)
    if gaps or unasked:
        text = text.replace("## Headline", f"> Incomplete: {unasked} of {len(photos)} photos not "
                            f"asked yet, and {gaps} answers missing on the rest (they replay as "
                            "gaps). Run again to fill them before trusting these numbers.\n\n"
                            "## Headline", 1)

    # phase 1c: calibrate on the tune photos, judge on the held-out ones
    tune = {l["photo"]: l["target"] for l in lines if l["split"] == "tune"}
    if tune:
        cal, notes = calibrate.calibrate(kb, matrix, tune)
        cal_path = os.path.join(run_dir, f"{base}.calibrated.json")
        with open(cal_path, "w") as f:
            json.dump(cal, f, indent=1, ensure_ascii=False)
        text += "\n" + calibrate.describe(notes, kb)
        if any(l["split"] == "held_out" for l in lines):
            r = compare.compare(a.kb, cal_path, "held_out", a.mode, a.model)
            text += ("\n## Calibrated against current, held-out photos\n\n"
                     + compare.describe(r, "current", "calibrated"))
            print(f"calibrated: {'accepted' if r['accepted'] else 'not accepted'} "
                  f"({r['base'][2]:.1f} -> {r['cand'][2]:.1f})", file=sys.stderr)
    with open(os.path.join(run_dir, "report.md"), "w") as f:
        f.write(text)
    right = sum(g["correct"] for g in games)
    print(f"\n{len(games)} games, {right} right ({right / len(games):.0%}); "
          f"{unasked} photos not asked, {gaps} answers missing\n{os.path.relpath(run_dir)}/report.md", file=sys.stderr)


if __name__ == "__main__":
    main()

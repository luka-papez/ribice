"""Measure the quiz from replayed games and the full answer matrix.

The headline is accuracy on held-out photos at the average number of
questions. Below it, per question: how often the answer agrees with the
knowledge base's value for the true cloud, how often it can't be answered and
why, which values get mixed up, and how many bits it removes when the engine
asks it. Per question figures use every cached answer, not only the questions
the engine happened to ask.

Accuracy is also given for two narrower sets: photos the label check passed,
since the doubted photos were kept unreviewed, and clouds with at least 4
photos, as the spec counts them.

    python tools/calib/report.py --kb data/clouds.json --games DIR/games.jsonl
"""
import argparse, collections, json, os, statistics

import ask

HERE = os.path.dirname(os.path.abspath(__file__))


def truth(kb, entity, attr):
    """The knowledge base's value(s) for one entity, as a set of strings."""
    a = kb["attributes"][attr]
    v = entity.get(attr)
    boolean = "labels" not in a
    if v is None or v is False:
        return {"no" if boolean else "none"}
    if v is True:
        return {"yes"}
    if isinstance(v, list):
        return {str(x) for x in v}
    return {str(v)}


def genus(name):
    return name.split()[0]


def pct(n, d):
    return f"{100 * n / d:.0f}%" if d else "-"


def headline(games, label):
    n = len(games)
    if not n:
        return f"| {label} | 0 | | | | | | | |"
    right = sum(g["correct"] for g in games)
    gen = sum(genus(g["guess"]) == genus(g["target"]) for g in games)
    top3 = sum(1 <= g["rank"] <= 3 for g in games)
    qs = [g["questions"] for g in games]
    return (f"| {label} | {n} | {pct(right, n)} | {pct(gen, n)} | {pct(top3, n)} | "
            f"{statistics.mean(qs):.1f} / {max(qs)} | "
            f"{statistics.mean(g['skipped'] for g in games):.1f} | "
            f"{pct(sum(g['unsure'] for g in games), n)} | {pct(sum(g['gave_up'] for g in games), n)} |")


def build(kb, photos, games, matrix, passed, title, names=None, human=None):
    ents = {e["name"]: e for e in kb["entities"]}
    per_cloud_photos = collections.Counter(p["label"] for p in photos)
    by_id = {p["id"]: p for p in photos}
    out = [f"# {title}", ""]

    out += ["## Headline", "",
            "Right: the true cloud ranked first. Genus: the guess is at least the right genus. "
            "Skips count as questions, as they do on the site.", "",
            "| games | n | right | genus | top 3 | questions (mean / worst) | skips | unsure | gave up |",
            "| --- | --- | --- | --- | --- | --- | --- | --- | --- |"]
    for split in ("held_out", "tune"):
        gs = [g for g in games if g["split"] == split]
        out.append(headline(gs, split.replace("_", "-")))
        out.append(headline([g for g in gs if passed.get(g["photo"])], f"{split.replace('_', '-')}, label check passed"))
        out.append(headline([g for g in gs if per_cloud_photos[g["target"]] >= 4],
                            f"{split.replace('_', '-')}, clouds with 4+ photos"))
    out.append(headline(games, "all"))
    out.append("")

    # --- per question, from the whole matrix
    qs = ask.questions(kb)
    out += ["## Questions", "",
            "Agreement: answers that include the knowledge base's value for the true cloud, "
            "out of those answered. Skip causes: not in photo / ambiguous / unclear question. "
            "Asked: share of games that asked it. Gain: bits removed per answered ask, on average.",
            "",
            "| question | answers | agreement | skipped | not in photo / ambiguous / unclear | low confidence | asked | gain |",
            "| --- | --- | --- | --- | --- | --- | --- | --- |"]
    gains = collections.defaultdict(list)
    asked = collections.Counter()
    for g in games:
        before = g["entropy_start"]
        seen = set()
        for s in g["steps"]:
            if not s["skipped"]:
                gains[s["attr"]].append(before - s["entropy_after"])
            before = s["entropy_after"]
            seen.add(s["attr"])
        asked.update(seen)
    mixups = {}
    for attr, text, opts in qs:
        rows = [(pid, r[attr]) for pid, r in matrix.items() if attr in r]
        answered = [(pid, r) for pid, r in rows if not r["cant_tell"]]
        agree = sum(bool(set(r["values"]) & truth(kb, ents[by_id[pid]["label"]], attr))
                    for pid, r in answered)
        causes = collections.Counter(r["cant_tell"] for _, r in rows if r["cant_tell"])
        low = sum(r["confidence"] == "low" for _, r in answered)
        gain = f"{statistics.mean(gains[attr]):.2f}" if gains.get(attr) else "-"
        out.append(f"| {text} | {len(rows)} | {pct(agree, len(answered))} | "
                   f"{pct(len(rows) - len(answered), len(rows))} | "
                   f"{causes['not_in_photo']} / {causes['ambiguous']} / {causes['unclear_question']} | "
                   f"{pct(low, len(answered))} | {pct(asked[attr], len(games))} | {gain} |")
        table = collections.defaultdict(collections.Counter)
        for pid, r in rows:
            for t in truth(kb, ents[by_id[pid]["label"]], attr):
                if r["cant_tell"]:
                    table[t]["(skip)"] += 1
                for v in r["values"]:
                    table[t][v] += 1 / len(r["values"])
        mixups[attr] = (text, [v for v, _ in opts], table)
    out.append("")

    # --- leakage: what a photo cannot show, answered anyway
    out += ["## Leakage check: things a photo cannot show", "",
            "Whether rain was falling, or a halo was round the sun when the sun is out of frame, "
            "can rarely be judged from a photo. Confident answers that match the knowledge base "
            "here suggest the answerer is recalling the cloud type instead of looking.", "",
            "| question | answered | answered with high confidence | agreement when answered |",
            "| --- | --- | --- | --- |"]
    for attr in ("precipitation", "halo"):
        if attr not in kb["attributes"]:
            continue
        rows = [(pid, r[attr]) for pid, r in matrix.items() if attr in r]
        answered = [(pid, r) for pid, r in rows if not r["cant_tell"]]
        high = sum(r["confidence"] == "high" for _, r in answered)
        agree = sum(bool(set(r["values"]) & truth(kb, ents[by_id[pid]["label"]], attr))
                    for pid, r in answered)
        out.append(f"| {kb['attributes'][attr]['question']} | {pct(len(answered), len(rows))} | "
                   f"{pct(high, len(answered))} | {pct(agree, len(answered))} |")
    out.append("")

    # --- leakage: agreement where the answerer knew the cloud, and where it didn't
    if names:
        known = {pid for pid in matrix if pid in names and
                 names[pid]["name"].strip().lower() == by_id[pid]["label"].lower()}
        unknown = {pid for pid in matrix if pid in names} - known
        out += ["## Leakage check: recognised or not", "",
                f"In a separate call the answerer named {len(known)} of {len(known) + len(unknown)} "
                "photos exactly. If its answers agree with the knowledge base far more often on "
                "those than on the rest, it is answering from what it knows about the type. Some "
                "gap is expected either way: a photo that is easy to name is often easy to answer.",
                "", "| question | agreement, named right | agreement, named wrong |", "| --- | --- | --- |"]
        for attr, text, _ in qs:
            cells = []
            for group in (known, unknown):
                answered = [(pid, matrix[pid][attr]) for pid in group
                            if attr in matrix[pid] and not matrix[pid][attr]["cant_tell"]]
                agree = sum(bool(set(r["values"]) & truth(kb, ents[by_id[pid]["label"]], attr))
                            for pid, r in answered)
                cells.append(f"{pct(agree, len(answered))} of {len(answered)}")
            out.append(f"| {text} | {cells[0]} | {cells[1]} |")
        out.append("")

    # --- the answerer against a person, on the same photos
    if human:
        shared = [pid for pid in human if pid in matrix]
        out += ["## Answerer against a person", "",
                f"On {len(shared)} photos a person answered too. Agree: both answered and their "
                "answers share a value. It shows which questions Claude reads differently from a "
                "person, which is the gate before tuning anything on Claude's answers.", "",
                "| question | both answered | agree | only the person couldn't tell | only Claude couldn't tell | neither could |",
                "| --- | --- | --- | --- | --- | --- |"]
        for attr, text, _ in qs:
            both = agree = only_h = only_c = neither = 0
            for pid in shared:
                h, c = human[pid].get(attr), matrix[pid].get(attr)
                if not h or not c:
                    continue
                h_skip, c_skip = bool(h.get("cant_tell")), bool(c["cant_tell"])
                if h_skip and not c_skip:
                    only_h += 1
                elif c_skip and not h_skip:
                    only_c += 1
                elif h_skip:
                    neither += 1
                else:
                    both += 1
                    agree += bool(set(h["values"]) & set(c["values"]))
            out.append(f"| {text} | {both} | {pct(agree, both)} | {only_h} | {only_c} | {neither} |")
        out.append("")

    # --- per cloud
    out += ["## Clouds", "", "| cloud | photos | games | right | questions | most common wrong guess |",
            "| --- | --- | --- | --- | --- | --- |"]
    for e in kb["entities"]:
        gs = [g for g in games if g["target"] == e["name"]]
        wrong = collections.Counter(g["guess"] for g in gs if not g["correct"])
        common = f"{wrong.most_common(1)[0][0]} ({wrong.most_common(1)[0][1]})" if wrong else ""
        q = f"{statistics.mean(g['questions'] for g in gs):.1f}" if gs else "-"
        out.append(f"| {e['name']} | {per_cloud_photos[e['name']]} | {len(gs)} | "
                   f"{pct(sum(g['correct'] for g in gs), len(gs))} | {q} | {common} |")
    out.append("")

    # --- mix-up tables
    out += ["## Mix-ups", "",
            "Rows: the knowledge base's value for the true cloud. Columns: what was answered. "
            "An answer torn between two options counts half to each.", ""]
    for attr, (text, values, table) in mixups.items():
        cols = values + ["(skip)"]
        out += [f"### {text}", "", "| true \\ answered | " + " | ".join(cols) + " |",
                "| --- |" + " --- |" * len(cols)]
        for t in values:
            if t in table:
                out.append(f"| {t} | " + " | ".join(
                    (f"{table[t][c]:.1f}".rstrip("0").rstrip(".") if table[t][c] else "")
                    for c in cols) + " |")
        out.append("")

    # --- misses
    misses = sorted((g for g in games if not g["correct"]), key=lambda g: g["prob"])
    out += ["## Games that went wrong", ""]
    for g in misses[:20]:
        trail = ", ".join(s["attr"] + ("?" if s["skipped"] else "") for s in g["steps"])
        out.append(f"- `{g['photo']}` {g['target']} → {g['guess']} "
                   f"(true cloud ranked #{g['rank']}, {g['questions']} questions): {trail}")
    return "\n".join(out) + "\n"


def load_human(corpus):
    path = os.path.join(corpus, "human.json")
    return json.load(open(path))["answers"] if os.path.exists(path) else None


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--kb", required=True)
    ap.add_argument("--games", required=True, help="games.jsonl from replay.py")
    ap.add_argument("--mode", choices=["single", "photo"], default="single")
    ap.add_argument("--model", default=ask.MODEL)
    ap.add_argument("--out", help="write the report here instead of printing it")
    ap.add_argument("--title", default="Calibration report")
    a = ap.parse_args()

    kb = json.load(open(a.kb))
    base = os.path.splitext(os.path.basename(a.kb))[0]
    corpus = os.path.join(HERE, "corpus", base)
    photos = json.load(open(os.path.join(corpus, "manifest.json")))["photos"]
    games = [json.loads(l) for l in open(a.games) if l.strip()]
    checks = json.load(open(os.path.join(corpus, "label-check.json")))
    import label_check
    passed = {pid: not label_check.doubted(v) for pid, v in checks.items()}
    matrix = ask.lookup(ask.open_cache(), [p["id"] for p in photos], ask.questions(kb),
                        a.mode, a.model)
    import name_test
    text = build(kb, photos, games, matrix, passed, a.title, name_test.load(base), load_human(corpus))
    if a.out:
        with open(a.out, "w") as f:
            f.write(text)
    else:
        print(text)


if __name__ == "__main__":
    main()

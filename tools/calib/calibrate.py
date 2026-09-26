"""Rewrite a knowledge base's error model from measured answers.

Every noise, confusion, confusable and cost value in a hand-written
knowledge base is a guess. This replaces them with what the answer matrix
shows on the tune split, by the rules in specs/calibration-loop.md:

1. A pair (v, v') becomes confusable when v' is answered for a true v at
   least 3 times and in at least 10% of v's cases. Pairs are symmetric, as in
   the engine. Hand-declared pairs are kept; the notes say which ones the
   answers never bore out.
2. confusion is the share of answers that land on a look-alike of the true
   value, counted over true values that have look-alikes, since only those
   leak to them in the engine (kb.Attribute.Report).
3. noise is the share that land anywhere else.
4. Each is blended with the current value as if that were 10 observations,
   and noise is floored at 0.02, so an attribute seen 20 times can't swing to
   zero.
5. cost is 1 + 2 x skip rate + 1 x share of low-confidence answers, clamped
   to 0.7-3.0: a question people usually can't answer should come late.

An answer torn between two values counts half to each. Skips count only
towards cost. The result is a candidate: compare.py decides whether it is
better on held-out photos.
"""
import collections, copy

PRIOR = 10       # the current value counts as this many observations
FLOOR = 0.02     # noise never drops below this
PAIR_MIN, PAIR_SHARE = 3, 0.10
COST_MIN, COST_MAX = 0.7, 3.0
DEFAULT_NOISE, DEFAULT_CONFUSION = 0.12, 0.25     # kb.DefaultNoise, kb.DefaultConfusion


def truth(kb, entity, attr):
    """The knowledge base's value(s) for one entity, as a set of strings."""
    a = kb["attributes"][attr]
    v = entity.get(attr)
    if v is None or v is False:
        return {"none" if "labels" in a else "no"}
    if v is True:
        return {"yes"}
    if isinstance(v, list):
        return {str(x) for x in v}
    return {str(v)}


def blend(measured, n, current):
    return (n * measured + PRIOR * current) / (n + PRIOR)


def calibrate(kb, matrix, labels):
    """A calibrated copy of kb, and a list of notes per attribute.

    matrix: {photo id: {attr: answer row}} as ask.lookup returns it.
    labels: {photo id: true cloud}, the tune photos only."""
    out = copy.deepcopy(kb)
    ents = {e["name"]: e for e in kb["entities"]}
    notes = []
    for attr, meta in kb["attributes"].items():
        rows = [(truth(kb, ents[labels[pid]], attr), m[attr])
                for pid, m in matrix.items() if pid in labels and attr in m]
        if not rows:
            continue
        answered = [(t, r["values"]) for t, r in rows if not r["cant_tell"]]
        skip = 1 - len(answered) / len(rows)
        low = (sum(r["confidence"] == "low" for _, r in rows if not r["cant_tell"])
               / len(answered)) if answered else 0.0
        new = out["attributes"][attr]
        note = {"attr": attr, "answers": len(rows), "answered": len(answered),
                "skip": skip, "low": low}

        # where each answer went, per true value; a torn answer counts half to each
        counts = collections.defaultdict(collections.Counter)
        for t, vals in answered:
            for tv in t:
                for v in vals:
                    counts[tv][v] += 1 / (len(vals) * len(t))

        declared = {frozenset(p) for p in meta.get("confusable", [])}
        found = set()
        if "labels" in meta:
            for tv, c in counts.items():
                n_t = sum(c.values())
                for v, k in c.items():
                    if v != tv and k >= PAIR_MIN and k >= PAIR_SHARE * n_t:
                        found.add(frozenset((tv, v)))
        pairs = declared | found
        near = collections.defaultdict(set)
        for p in pairs:
            x, y = tuple(p)
            near[x].add(y)
            near[y].add(x)

        right = conf = far = 0.0
        eligible = 0.0
        for tv, c in counts.items():
            n_t = sum(c.values())
            if near[tv]:
                eligible += n_t
            for v, k in c.items():
                if v == tv:
                    right += k
                elif v in near[tv]:
                    conf += k
                else:
                    far += k
        n = right + conf + far
        if n:
            cur_noise = meta.get("noise", DEFAULT_NOISE)
            new["noise"] = round(max(FLOOR, blend(far / n, n, cur_noise)), 3)
            note["noise"] = (cur_noise, new["noise"], far / n)
        if pairs and eligible:
            cur_conf = meta.get("confusion", DEFAULT_CONFUSION if declared else 0.0)
            new["confusion"] = round(blend(conf / eligible, eligible, cur_conf), 3)
            note["confusion"] = (cur_conf, new["confusion"], conf / eligible)
        if new.get("noise", 0) + new.get("confusion", 0) >= 0.95:
            # the engine refuses noise + confusion >= 1; a question this unreliable
            # should be reworded, not believed
            scale = 0.95 / (new["noise"] + new["confusion"])
            new["noise"] = round(new["noise"] * scale, 3)
            new["confusion"] = round(new["confusion"] * scale, 3)
        if found - declared:
            new["confusable"] = [sorted(p) for p in sorted(pairs, key=sorted)]
        note["pairs_added"] = sorted(sorted(p) for p in found - declared)
        note["pairs_unseen"] = sorted(sorted(p) for p in declared
                                      if not any(counts[x][y] + counts[y][x] > 0
                                                 for x, y in [tuple(p)]))
        cur_cost = meta.get("cost", 1.0)
        new["cost"] = round(min(COST_MAX, max(COST_MIN, 1 + 2 * skip + low)), 2)
        note["cost"] = (cur_cost, new["cost"])
        notes.append(note)
    return out, notes


def describe(notes, kb):
    """The calibration as markdown, for the report."""
    out = ["## Calibration", "",
           "Measured on the tune split and blended with the current values as 10 "
           "observations (see calibrate.py). Measured rates are in brackets.", "",
           "| question | answered | noise | confusion | cost | look-alikes added | declared, never seen |",
           "| --- | --- | --- | --- | --- | --- | --- |"]
    for n in notes:
        q = kb["attributes"][n["attr"]]["question"]
        noise = (f"{n['noise'][0]:.2f} → {n['noise'][1]:.2f} ({n['noise'][2]:.2f})"
                 if "noise" in n else "-")
        conf = (f"{n['confusion'][0]:.2f} → {n['confusion'][1]:.2f} ({n['confusion'][2]:.2f})"
                if "confusion" in n else "-")
        cost = f"{n['cost'][0]:.1f} → {n['cost'][1]:.1f}"
        added = ", ".join("/".join(p) for p in n["pairs_added"])
        unseen = ", ".join("/".join(p) for p in n["pairs_unseen"])
        out.append(f"| {q} | {n['answered']} of {n['answers']} | {noise} | {conf} | {cost} | "
                   f"{added} | {unseen} |")
    return "\n".join(out) + "\n"

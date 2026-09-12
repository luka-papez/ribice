"""Re-join the FCI index to Wikidata (CC0) after the index changes."""
import csv, json, re, unicodedata

def norm(s):
    if not s: return ""
    s = unicodedata.normalize("NFKD", s).encode("ascii", "ignore").decode().lower()
    s = re.sub(r"\(dog breed\)|\(dog\)", " ", s.replace("&", " and "))
    return " ".join(re.sub(r"[^a-z0-9]+", " ", s).split())

ents, by_name, by_fci = {}, {}, {}
for r in csv.DictReader(open("wd-all.csv")):
    q = r["breed"].rsplit("/", 1)[-1]
    e = ents.setdefault(q, {"qid": q, "label": r["breedLabel"], "origin": "",
                            "image": "", "article": ""})
    for k, c in (("origin", "originLabel"), ("image", "img"), ("article", "article")):
        if not e[k] and r[c]: e[k] = r[c]
    for n in (norm(r["breedLabel"]), norm(r["alias"])):
        if n: by_name.setdefault(n, q)
for r in csv.DictReader(open("wd-spine.csv")):
    try: by_fci[int(r["fci"])] = r["breed"].rsplit("/", 1)[-1]
    except ValueError: pass

OVERRIDE = {257:"Q39315", 319:"Q39147", 311:"Q39306", 313:"Q39157",
            142:"Q38963", 62:"Q39214", 304:"Q39332"}

spine = json.load(open("fci-index.json"))
hit = collections = 0
for b in spine:
    q = by_fci.get(b["fci"]) or OVERRIDE.get(b["fci"])
    how = "fci" if by_fci.get(b["fci"]) else "manual" if q else None
    if not q:
        cands = [b.get("name_en"), b.get("name_fci")]
        for c in list(cands):
            if c and "-" in c: cands += [p.strip() for p in c.split("-")]
        for c in cands:
            q = by_name.get(norm(c))
            if q: how = "name"; break
    if q and q in ents:
        b["wikidata"] = dict(ents[q], matched_by=how)
        hit += 1
json.dump(spine, open("spine.json", "w"), indent=1, ensure_ascii=False)
miss = [b for b in spine if not b.get("wikidata")]
print(f"spine: {len(spine)} breeds, {hit} linked to Wikidata")
print(f"  unlinked ({len(miss)}): " + ", ".join(f"{b['fci']} {b['name_fci']}" for b in miss))

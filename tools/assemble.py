"""Merge the three sources into one working file to code the KB from.

  spine.json          FCI index (name, number, group, section, origin)
                      joined to Wikidata (CC0: label, image, article)
  fci-sections.json   the standards, split into named sections + parsed height
  wd-height.csv       Wikidata height, as a fallback where the standard gives none

Output: fci-extract.json -- local working material, not the knowledge base.
"""
import csv, collections, json, sys

wdh = collections.defaultdict(list)
for r in csv.DictReader(open("wd-height.csv")):
    if r["unitLabel"] == "centimetre":
        try:
            v = float(r["h"])
        except ValueError:
            continue
        if 10 <= v <= 100:
            wdh[r["breed"].rsplit("/", 1)[-1]].append(v)

# Wikidata labels common-noun breed names in sentence case ("border collie");
# a field guide capitalises them.  Connecting particles stay lower case.
SMALL = {"de", "del", "della", "des", "do", "du", "da", "di", "der", "den",
         "van", "von", "of", "and", "the", "la", "le", "el", "y", "à", "au",
         "aux", "et", "dos", "das"}

def titlecase(name):
    words = name.split()
    out = []
    for i, w in enumerate(words):
        if i and w.lower() in SMALL:
            out.append(w.lower())
        elif w[:1].islower():
            out.append(w[:1].upper() + w[1:])
        else:
            out.append(w)
    return " ".join(out)

# Wikidata's English label collapses distinct FCI breeds -- three Belgian
# griffons share one label, as do two Ariegeois, two Braques Francais, two
# Pyrenean Shepherds and two Segugi.  A duplicate name would give the KB two
# entities with the same identity, so these are named explicitly.
DISAMBIGUATE = {
     20: "Ariegeois",                    177: "Ariege Pointer",
     80: "Griffon Bruxellois",            81: "Griffon Belge",
     82: "Petit Brabancon",
    133: "Braque Francais (Gascogne type)", 134: "Braque Francais (Pyrenean type)",
    138: "Pyrenean Shepherd (smooth-faced)", 141: "Pyrenean Shepherd (long-haired)",
    198: "Segugio Italiano (wire-haired)", 337: "Segugio Italiano (short-haired)",
}

secs = {r["fci"]: r for r in json.load(open("fci-sections.json"))}
out = []
for b in json.load(open("spine.json")):
    s = secs.get(b["fci"], {})
    w = b.get("wikidata", {})
    h = s.get("height_cm")
    wt = s.get("weight_kg")
    src = "standard"
    if not h and w.get("qid") in wdh:
        v = wdh[w["qid"]]
        h, src = [min(v), max(v)], "wikidata"
    out.append({
        "fci": b["fci"],
        "name": DISAMBIGUATE.get(b["fci"]) or titlecase(
            w.get("label") or b.get("name_en") or b["name_fci"].lower()),
        "name_fci": b["name_fci"],
        "group": b["group"],
        "provisional": b.get("provisional", False),
        "section": b.get("section"),
        "origin": b.get("origin"),
        "height_cm": h,
        "height_source": src if h else None,
        "weight_kg": wt,
        "wikidata": w.get("qid"),
        "image": w.get("image"),
        "article": w.get("article"),
        "standard": b.get("standard"),
        "sections": s.get("sections", {}),
    })
dups = {n for n in (r["name"] for r in out)
        if [r["name"] for r in out].count(n) > 1}
if dups:
    raise SystemExit(f"duplicate entity names, cannot identify them apart: {sorted(dups)}")
json.dump(out, open("fci-extract.json", "w"), indent=1, ensure_ascii=False)

have = lambda k: sum(1 for r in out if r["sections"].get(k))
n = len(out)
print(f"{n} breeds -> fci-extract.json", file=sys.stderr)
print(f"  standard parsed      {sum(1 for r in out if r['sections']):>4}/{n}", file=sys.stderr)
for k in ("ears", "tail", "coat_hair", "coat_colour", "appearance", "muzzle",
          "stop", "eyes", "head", "neck", "body", "gait", "skin"):
    print(f"  {k:<20} {have(k):>4}/{n}", file=sys.stderr)
print(f"  height (any source)  {sum(1 for r in out if r['height_cm']):>4}/{n}"
      f"  [standard {sum(1 for r in out if r['height_source']=='standard')},"
      f" wikidata {sum(1 for r in out if r['height_source']=='wikidata')}]", file=sys.stderr)
print(f"  image                {sum(1 for r in out if r['image']):>4}/{n}", file=sys.stderr)

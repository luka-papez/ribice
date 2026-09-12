"""Choose one picture per cloud type from the Commons candidates.

Prefers a file whose name states the type in full, and penalises one that names
several -- "Cirrus spissatus cumulonimbogenitus, Altocumulus stratiformis" is a
photograph of a sky, not an example of one cloud. Short names win ties, being
likelier to be a plain example than a catalogued curiosity.
"""
import json, os, re, sys, unicodedata

HERE = os.path.dirname(__file__)
GENERA = ["cirrus", "cirrocumulus", "cirrostratus", "altocumulus", "altostratus",
          "nimbostratus", "stratocumulus", "stratus", "cumulus", "cumulonimbus"]
SPECIES = ["fibratus", "uncinus", "spissatus", "castellanus", "floccus",
           "stratiformis", "nebulosus", "lenticularis", "volutus", "fractus",
           "humilis", "mediocris", "congestus", "calvus", "capillatus"]
# a picture captioned with three supplementary features is a photograph of an
# unusual sky, which is the opposite of what a guide wants to show
FEATURES = ["asperitas", "mamma", "mammatus", "virga", "incus", "tuba", "arcus",
            "praecipitatio", "cavum", "fluctus", "murus", "cauda", "pileus",
            "velum", "pannus", "undulatus", "radiatus", "lacunosus",
            "duplicatus", "intortus", "vertebratus", "perlucidus", "opacus",
            "translucidus", "halo", "nebensonne", "sundog"]

def norm(s):
    s = unicodedata.normalize("NFKD", s).encode("ascii", "ignore").decode().lower()
    return re.sub(r"[^a-z0-9]+", " ", s)

def score(name, filename):
    f = norm(filename[5:])          # drop "File:"
    want = norm(name).split()
    s = 0
    if all(w in f for w in want):
        s += 20
    else:
        s += 4 * sum(w in f for w in want)
    words = set(f.split())
    extra = (words & set(GENERA) | words & set(SPECIES)) - set(want)
    s -= 5 * len(extra)             # a sky with four clouds in it teaches nothing
    s -= 4 * len((words & set(FEATURES)) - set(want))
    s -= len(f) / 25                # prefer the plainly named
    if "genitus" in f or "mutatus" in f:
        s -= 6                      # a cloud named for what it came from
    if "render" in f or "panoramio" in f:
        s -= 25                     # a render is not a photograph of a sky
    if re.match(r"^\d", f) and not all(w in f for w in want):
        s -= 25                     # a dated upload that does not say what it shows
    return s

# Where no candidate is both correctly named and honest about what it shows.
# "Sun through Altostratus" is a picture of altostratus translucidus: the sun
# showing through is precisely what opacus does not do.
OVERRIDE = {
 # every well-named candidate here is a photograph of a palm tree
 "Stratus fractus":
   "File:2026-09-03 08 35 44 Stratus fractus in front of other clouds viewed from Mercer County Route 611 (Scotch Road) in Ewing Township, Mercer County, New Jersey.jpg",
 "Altostratus opacus":
   "File:Altostratus mit scharfer Kante bei Limburg, gesehen am 11. Januar 2023.jpg",
}

def main():
    cands = json.load(open(os.path.join(HERE, "cloud-images.json")))
    picked = {}
    for name, v in cands.items():
        if not v["candidates"]:
            continue
        chosen = OVERRIDE.get(name)
        if chosen and chosen not in v["candidates"]:
            # a hand-picked file that is not among the candidates is a typo, not
            # a choice: say so rather than emit a picture that does not exist
            print(f"  OVERRIDE names no candidate: {name}", file=sys.stderr)
            chosen = None
        best = chosen or max(v["candidates"], key=lambda f: score(name, f))
        picked[name] = best
        print(f"  {name:<28} {best[5:][:60]}", file=sys.stderr)
    json.dump(picked, open(os.path.join(HERE, "cloud-picked.json"), "w"),
              indent=1, ensure_ascii=False)
    print(f"\npicked {len(picked)}/{len(cands)}", file=sys.stderr)

main()

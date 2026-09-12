"""Choose the breeds the guide covers.

Ranked by how often people look the breed up on English Wikipedia over twelve
months -- a measure of what someone is likely to meet and want named, rather
than of my own sense of which breeds matter.  Croatian breeds are kept whatever
their ranking, because this guide is for Croatia; UnknownPrior covers the rest.
"""
import json, sys

TARGET = int(sys.argv[1]) if len(sys.argv) > 1 else 120
KEEP_ORIGIN = "CROATIA"   # matches "CROATIA" and "BOSNIA AND HERZEGOVINA, CROATIA"

E = {r["fci"]: r for r in json.load(open("fci-extract.json"))}
views = {int(k): v.get("en", 0) for k, v in json.load(open("pageviews.json")).items()}

ranked = sorted(E.values(), key=lambda r: -views.get(r["fci"], 0))
chosen, why = [], {}
for r in ranked:
    if len(chosen) >= TARGET:
        break
    chosen.append(r["fci"]); why[r["fci"]] = "top"
for r in E.values():
    if KEEP_ORIGIN in (r.get("origin") or "").upper() and r["fci"] not in why:
        chosen.append(r["fci"]); why[r["fci"]] = "croatian"

json.dump({"fci": sorted(chosen), "why": {str(k): v for k, v in why.items()},
           "views": {str(k): views.get(k, 0) for k in chosen}},
          open("selection.json", "w"), indent=1)

print(f"selected {len(chosen)} breeds "
      f"({sum(1 for v in why.values() if v=='croatian')} kept for being Croatian)",
      file=sys.stderr)
print("\n top 20 by views:", file=sys.stderr)
for r in ranked[:20]:
    print(f"   {views.get(r['fci'],0):>9,}  {r['name']}", file=sys.stderr)
cut = views.get(chosen[TARGET-1], 0) if len(chosen) >= TARGET else 0
print(f"\n cut-off at #{TARGET}: {cut:,} views/year", file=sys.stderr)
print(" just below the line:", file=sys.stderr)
for r in ranked[TARGET:TARGET+8]:
    print(f"   {views.get(r['fci'],0):>9,}  {r['name']}", file=sys.stderr)

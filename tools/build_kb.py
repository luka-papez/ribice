"""Assemble the ribice knowledge base.

Restricted to the selected breeds (see select.py); every picture carries the
author and licence Commons records for it, because the licences require it and
-lint refuses a picture without them.
"""
import json, os, sys, urllib.parse

attrs = json.load(open("attributes-draft.json"))
coded = json.load(open("coded.json"))
sel = set(json.load(open("selection.json"))["fci"])
credits = json.load(open("credits.json"))
extract = {r["fci"]: r for r in json.load(open("fci-extract.json"))}

SCALAR = {"muzzle", "build"}

def image_for(fci):
    r = extract.get(fci, {})
    url = r.get("image")
    if not url:
        return None
    c = credits.get(urllib.parse.unquote(url.rsplit("/", 1)[-1]))
    if not c or not (c["credit"] and c["license"]):
        return None            # a picture we cannot attribute is not usable
    return {"url": url, "source": c["source_page"],
            "credit": c["credit"], "license": c["license"]}

entities, dropped = [], 0
for e in coded:
    if e["_fci"] not in sel:
        dropped += 1
        continue
    out = {"name": e["name"]}
    for k, v in e.items():
        if k in ("name", "_fci", "_image"):
            continue
        if isinstance(v, list):
            if not v:
                continue
            out[k] = v[0] if (k in SCALAR and len(v) == 1) else v
        elif v not in (None, False, ""):
            out[k] = v
    img = image_for(e["_fci"])
    if img:
        out["_image"] = img
    entities.append(out)

kb = {"name": attrs["name"], "attributes": attrs["attributes"], "entities": entities}
out_path = os.path.join(os.path.dirname(__file__), "..", "data", "dogs.json")
json.dump(kb, open(out_path, "w"), indent=1, ensure_ascii=False)
withimg = sum(1 for e in entities if "_image" in e)
print(f"data/dogs.json: {len(entities)} entities ({dropped} not selected), "
      f"{withimg} with an attributed picture", file=sys.stderr)

"""Find a Wikimedia Commons photograph for each cloud type.

Commons keeps a category per cloud type, which is a far better source than a
text search: a file in Category:Cirrus uncinus has been put there by someone
who thought it was cirrus uncinus. Falls back to search where no category
exists, and records which route each picture came from so the doubtful ones
can be checked.
"""
import json, os, sys, time, urllib.parse, urllib.request

UA = "ribice-kb-build/0.1 (+https://codecrane.hr/ribice; building a free identification quiz)"
API = "https://commons.wikimedia.org/w/api.php"
IMG = (".jpg", ".jpeg", ".png", ".webp")

def api(**params):
    params.setdefault("format", "json")
    req = urllib.request.Request(API + "?" + urllib.parse.urlencode(params),
                                 headers={"User-Agent": UA})
    with urllib.request.urlopen(req, timeout=45) as r:
        return json.load(r)

def from_category(name):
    d = api(action="query", list="categorymembers", cmtitle="Category:" + name,
            cmtype="file", cmlimit=25)
    return [m["title"] for m in d.get("query", {}).get("categorymembers", [])
            if m["title"].lower().endswith(IMG)]

def from_search(name):
    d = api(action="query", list="search", srsearch=f'"{name}"',
            srnamespace=6, srlimit=15)
    return [m["title"] for m in d.get("query", {}).get("search", [])
            if m["title"].lower().endswith(IMG)]

def main():
    kb = json.load(open(os.path.join(os.path.dirname(__file__), "..", "data", "clouds.json")))
    out = {}
    for e in kb["entities"]:
        name = e["name"]
        # Both sources, always: a category member is well classified but often
        # named uselessly ("008kaz.renders.jpg"), while a search hit names the
        # type in the filename. Scoring across the union gets the best of each.
        cat, srch = from_category(name), from_search(name)
        files = list(dict.fromkeys(cat[:12] + srch[:12]))
        if not files:
            files = from_category(name.split()[0])[:12]
        out[name] = {"candidates": files, "category": len(cat), "search": len(srch)}
        print(f"  {name:<28} cat {len(cat):>2}  search {len(srch):>2}", file=sys.stderr)
        time.sleep(0.5)
    json.dump(out, open(os.path.join(os.path.dirname(__file__), "cloud-images.json"), "w"),
              indent=1, ensure_ascii=False)
    print(f"\n{sum(1 for v in out.values() if v['candidates'])}/{len(out)} have candidates",
          file=sys.stderr)

main()

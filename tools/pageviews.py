"""How often people actually look each breed up, as the measure of importance.

Wikipedia pageview counts are published by the Wikimedia Foundation and are
free to use.  Ranking by them beats ranking by my own sense of which breeds
matter, and it can be re-run when it drifts.
"""
import json, os, sys, time, urllib.parse, urllib.request

UA = "ribice-kb-build/0.1 (+https://codecrane.hr/ribice; building a free identification quiz)"
API = ("https://wikimedia.org/api/rest_v1/metrics/pageviews/per-article/"
       "{wiki}/all-access/user/{title}/monthly/2024090100/2025083100")

def views(article, wiki="en.wikipedia"):
    title = urllib.parse.quote(
        urllib.parse.unquote(article.rsplit("/wiki/", 1)[-1]), safe="")
    req = urllib.request.Request(API.format(wiki=wiki, title=title),
                                 headers={"User-Agent": UA})
    with urllib.request.urlopen(req, timeout=45) as r:
        d = json.load(r)
    return sum(i["views"] for i in d.get("items", []))

def main():
    E = json.load(open("fci-extract.json"))
    cache = json.load(open("pageviews.json")) if os.path.exists("pageviews.json") else {}
    for i, r in enumerate(E, 1):
        key = str(r["fci"])
        if (key in cache and not cache[key].get("error")) or not r.get("article"):
            continue
        try:
            cache[key] = {"name": r["name"], "en": views(r["article"])}
        except Exception as e:
            cache[key] = {"name": r["name"], "en": 0, "error": str(e)[:60]}
        if i % 40 == 0:
            print(f"  {i}/{len(E)}", file=sys.stderr)
            json.dump(cache, open("pageviews.json", "w"), indent=1)
        time.sleep(0.25)
    json.dump(cache, open("pageviews.json", "w"), indent=1)
    got = [v for v in cache.values() if v.get("en")]
    print(f"{len(got)}/{len(E)} breeds have a 12-month view count", file=sys.stderr)

main()

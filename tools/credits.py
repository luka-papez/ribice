"""Fetch author and licence for each Wikimedia Commons picture.

The README is explicit that a bare URL is a liability rather than a
convenience: almost every licence worth using requires naming the author and
the licence, and -lint refuses a picture without them.  Commons publishes both
as machine-readable metadata, so there is no excuse for omitting them.
"""
import html, json, re, sys, time, urllib.parse, urllib.request

UA = "ribice-kb-build/0.1 (+https://codecrane.hr/ribice; building a free identification quiz)"
API = "https://commons.wikimedia.org/w/api.php"

def filename(url):
    return urllib.parse.unquote(url.rsplit("/", 1)[-1])

def strip(s):
    """extmetadata values are HTML fragments -- an author is often a link."""
    s = re.sub(r"<[^>]+>", " ", s or "")
    return " ".join(html.unescape(s).split())

def fetch(names):
    q = {"action": "query", "format": "json", "prop": "imageinfo",
         "iiprop": "extmetadata|user", "titles": "|".join("File:" + n for n in names)}
    req = urllib.request.Request(API + "?" + urllib.parse.urlencode(q),
                                 headers={"User-Agent": UA})
    with urllib.request.urlopen(req, timeout=60) as r:
        d = json.load(r)
    out = {}
    pages = d.get("query", {}).get("pages", {})
    norm = {n["from"]: n["to"] for n in d.get("query", {}).get("normalized", [])}
    for p in pages.values():
        title = p.get("title", "")
        info = (p.get("imageinfo") or [{}])[0]
        ii = info.get("extmetadata", {})
        # some older uploads carry no Artist; the uploader is then the author of
        # record, which is what Commons' own attribution generator falls back to
        author = strip(ii.get("Artist", {}).get("value")) or \
                 strip(ii.get("Attribution", {}).get("value")) or \
                 (f"{info['user']} (Wikimedia Commons)" if info.get("user") else "")
        out[title] = {
            "credit": author,
            "license": strip(ii.get("LicenseShortName", {}).get("value")),
            "license_url": strip(ii.get("LicenseUrl", {}).get("value")),
            "source_page": "https://commons.wikimedia.org/wiki/" +
                           urllib.parse.quote(title.replace(" ", "_")),
        }
    return out, norm

def credits_for(urls):
    names = sorted({filename(u) for u in urls})
    got = {}
    for i in range(0, len(names), 40):
        batch = names[i:i + 40]
        try:
            res, norm = fetch(batch)
        except Exception as e:
            print(f"  batch {i} failed: {e}", file=sys.stderr)
            continue
        for n in batch:
            key = norm.get("File:" + n, "File:" + n)
            if key in res:
                got[n] = res[key]
        time.sleep(0.5)
    return got

if __name__ == "__main__":
    E = json.load(open("fci-extract.json"))
    urls = [r["image"] for r in E if r.get("image")]
    got = credits_for(urls)
    json.dump(got, open("credits.json", "w"), indent=1, ensure_ascii=False)
    ok = sum(1 for v in got.values() if v["credit"] and v["license"])
    print(f"{len(got)}/{len(urls)} files resolved, {ok} with both author and licence",
          file=sys.stderr)
    import collections
    print(collections.Counter(v["license"] for v in got.values()).most_common(12),
          file=sys.stderr)

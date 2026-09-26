"""Build the photo corpus the calibration loop measures the quiz on.

Many labelled photos per cloud type from Wikimedia Commons, where
cloud_images.py wants one. A file in Category:Cumulus mediocris clouds was put
there by someone who judged it to be that cloud, which is the label we score
against, so categories come first and a filename search is not used at all.

A cloud those leave short draws last on a full-text search of Commons, which
finds files whose descriptions name the cloud without anyone having filed
them under it. That is the weakest label of the three: the hits are ranked
after every category photo, marked as search results in the manifest, and
left to label_check.py to vet.

The manifest pins every choice. Photos already in it keep their place and
their split on every later run, so an answer recorded against a held-out photo
never turns into tuning data; a run only fills clouds that are short, for
instance after review.json drops a photo. The image bytes are not committed:
they are fetched again from the manifest into a gitignored cache, rotated
upright, stripped of metadata and scaled to 1568 px on the long edge.

    python tools/calib/corpus.py --kb data/clouds.json
"""
import argparse, hashlib, io, json, os, sys, time, urllib.error, urllib.parse, urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.dirname(HERE))
from cloud_images import api as _api, UA          # noqa: E402
from cloud_pick import GENERA, SPECIES, norm, score  # noqa: E402
from credits import strip                          # noqa: E402

LONG_EDGE = 1568    # the largest Claude reads without scaling it down again
MIN_SHORT = 800     # smaller than this and the lumps are a few pixels wide
PER_CATEGORY = 500  # the biggest categories hold thousands; this is plenty to choose from
MIMES = {"image/jpeg", "image/png", "image/webp"}
# a diagram, a painting or the view from orbit is not what someone under the
# sky sees, and the quiz is only ever answered from under the sky
NOT_PHOTO = {"diagram", "satellite", "painting", "paint", "stamp", "render", "rendering",
             "drawing", "illustration", "sketch", "map", "chart", "symbol", "icon",
             "logo", "poster", "postcard", "modis", "nasa", "iss", "radar", "animation",
             "watercolor", "watercolour", "lithograph", "engraving", "atlas", "plate",
             "screenshot", "infographic", "3d", "blender"}


def api(**params):
    """cloud_images.api, retried: a run makes hundreds of calls and Commons
    now and then answers one with a 429 or a 503."""
    params.setdefault("maxlag", 5)
    for attempt in range(4):
        try:
            d = _api(**params)
            if d.get("error", {}).get("code") != "maxlag":
                return d
        except (urllib.error.URLError, TimeoutError) as e:
            if attempt == 3:
                raise
            print(f"    commons: {e}; retrying", file=sys.stderr)
        time.sleep(5 * (attempt + 1))
    raise RuntimeError("Commons kept reporting lag")


def photo_id(title):
    return hashlib.sha256(title.encode()).hexdigest()[:8]


def members(category, limit=PER_CATEGORY):
    """Files directly in a category, with the image info needed to judge them.
    Returns [] for a category that does not exist."""
    return query(dict(generator="categorymembers", gcmtitle="Category:" + category,
                      gcmtype="file", gcmlimit=50), limit)


def search(name, limit=200):
    """Files whose title or description contains the cloud's name as a phrase."""
    return query(dict(generator="search", gsrsearch=f'"{name}"', gsrnamespace=6,
                      gsrlimit=50), limit)


def query(gen, limit):
    """The files a generator yields, with the image info needed to judge them."""
    params = dict(action="query", prop="imageinfo", iiprop="mime|size|user|extmetadata",
                  iiextmetadatafilter="Artist|Attribution|LicenseShortName|LicenseUrl", **gen)
    pages, cont = {}, {}
    while len(pages) < limit:
        d = api(**params, **cont)
        for pid, p in d.get("query", {}).get("pages", {}).items():
            # a continuation can deliver a page before its imageinfo; keep the fuller copy
            if "imageinfo" in p or pid not in pages:
                pages[pid] = p
        if "continue" not in d:
            break
        cont = d["continue"]
        time.sleep(0.3)
    out = []
    for p in pages.values():
        if "imageinfo" not in p:
            continue
        ii = p["imageinfo"][0]
        meta = ii.get("extmetadata", {})
        artist = strip(meta.get("Artist", {}).get("value")) or \
                 strip(meta.get("Attribution", {}).get("value"))
        out.append({
            "file": p["title"],
            "mime": ii.get("mime", ""),
            "width": ii.get("width", 0), "height": ii.get("height", 0),
            "user": ii.get("user", ""),
            # uploaders mostly upload their own photos; the uploader stands in
            # where Artist is missing, as it does in credits.py
            "credit": artist or (f"{ii['user']} (Wikimedia Commons)" if ii.get("user") else ""),
            "license": strip(meta.get("LicenseShortName", {}).get("value")),
            "license_url": strip(meta.get("LicenseUrl", {}).get("value")),
        })
    return out


def photographer(c):
    """Who took it, for spreading the picks: eight shots of one afternoon are
    one sample, not eight."""
    return norm(c.get("credit", "")).strip() or norm(c.get("user", "")).strip()


def rejection(name, c, display):
    """Why a candidate cannot be used, or None."""
    if c["file"] == display:
        return "display photo"      # the quiz shows it; testing on it proves nothing
    if c["mime"] not in MIMES:
        return "not a photo (type)"
    words = set(norm(c["file"][5:]).split())
    if words & NOT_PHOTO:
        return "not a photo (name)"
    if (words & set(GENERA) | words & set(SPECIES)) - set(norm(name).split()):
        return "names another cloud"
    if min(c["width"], c["height"]) < MIN_SHORT:
        return "too small"
    if not c["license"]:
        return "no licence"
    return None


def candidates(name, want):
    """Every usable-looking file for one cloud, and where each came from.

    Commons files some species as "Cumulus fractus clouds" and others as plain
    "Cumulus fractus", so both are read. A cloud with fewer than `want` files
    there also draws on its genus category, keeping files whose names give the
    species -- a weaker label, recorded as such. The species word alone is
    enough, since the genus is the category's: "Altoestratos Opacus.jpg"."""
    found = {}
    for cat in (f"{name} clouds", name):
        for c in members(cat):
            found.setdefault(c["file"], {**c, "source": cat})
    genus, *species = norm(name).split()
    if len(found) < want and species:
        cat = f"{name.split()[0]} clouds"
        for c in members(cat, limit=2000):
            if set(species) <= set(norm(c["file"][5:]).split()):
                found.setdefault(c["file"], {**c, "source": f"{cat}, by file name"})
    return list(found.values())


def pick(name, pool, keep, want):
    """Fill up to `want` photos: the kept ones, then the best-scoring
    candidates, one per photographer until every photographer has had a turn."""
    chosen = list(keep)
    have = {c["file"] for c in chosen}
    ranked = sorted((c for c in pool if c["file"] not in have),
                    key=lambda c: (c["source"].startswith("search"),
                                   -score(name, c["file"]), c["file"]))
    for rnd in range(want):
        for c in ranked:
            if len(chosen) >= want:
                return chosen
            if c["file"] in have:
                continue
            taken = sum(photographer(k) == photographer(c) for k in chosen)
            if taken <= rnd:
                chosen.append(c)
                have.add(c["file"])
    return chosen


def split(photos, held):
    """Assign a split to photos that have none, in order of the hash of their
    file name. Held-out is filled first, so a short cloud still gets some."""
    quota = min(held, len(photos) // 2)
    n_held = sum(p.get("split") == "held_out" for p in photos)
    for p in sorted((p for p in photos if not p.get("split")), key=lambda p: p["id"]):
        if n_held < quota:
            p["split"], n_held = "held_out", n_held + 1
        else:
            p["split"] = "tune"


FIELDS = ["id", "file", "label", "split", "source", "credit", "license", "license_url",
          "source_page", "width", "height"]


def build(kb, corpus_dir, per_class, held):
    path = os.path.join(corpus_dir, "manifest.json")
    old = json.load(open(path))["photos"] if os.path.exists(path) else []
    review = load_review(corpus_dir)
    dropped = {f for f, r in review.items() if r.get("decision") == "drop"}

    pools, rejected = {}, {}
    for e in kb["entities"]:
        name = e["name"]
        display = urllib.parse.unquote(e.get("_image", {}).get("source", "").rsplit("/", 1)[-1])
        display = "File:" + display.replace("_", " ").removeprefix("File:") if display else ""
        pool, why = [], {}

        def consider(cands):
            for c in cands:
                r = "dropped in review" if c["file"] in dropped else rejection(name, c, display)
                if r:
                    why[r] = why.get(r, 0) + 1
                else:
                    pool.append(c)

        cands = candidates(name, per_class)
        consider(cands)
        if len(pool) < per_class:
            have = {c["file"] for c in cands}
            before = len(pool)
            consider(c | {"source": f'search: "{name}"'} for c in search(name)
                     if c["file"] not in have)
            why["from search"] = len(pool) - before
        pools[name], rejected[name] = pool, why
        print(f"  {name:<28} {len(pool):>4} usable  "
              + ", ".join(f"{v} {k}" for k, v in sorted(why.items())), file=sys.stderr)

    # A file filed under two clouds is labelled with neither: whoever filed it
    # saw both, and the score would punish the quiz for picking either.
    seen = {}
    for name, pool in pools.items():
        for c in pool:
            seen.setdefault(c["file"], set()).add(name)
    for f, e in ((p["file"], p) for p in old):
        seen.setdefault(f, set()).add(e["label"])
    for name in pools:
        pools[name] = [c for c in pools[name] if len(seen[c["file"]]) == 1]

    photos = []
    for e in kb["entities"]:
        name = e["name"]
        keep = [p for p in old if p["label"] == name and p["file"] not in dropped]
        keep = pick(name, pools[name], keep, per_class)
        for c in keep:
            c.setdefault("id", photo_id(c["file"]))
            c.setdefault("label", name)
            c.setdefault("source_page", "https://commons.wikimedia.org/wiki/" +
                         urllib.parse.quote(c["file"].replace(" ", "_")))
        split(keep, held)
        photos += [{k: c.get(k) for k in FIELDS} for c in keep]

    ids = [p["id"] for p in photos]
    assert len(ids) == len(set(ids)), "two files hash to one id; lengthen photo_id"
    os.makedirs(corpus_dir, exist_ok=True)
    with open(path, "w") as f:
        json.dump({"kb": kb["name"], "per_class": per_class, "held_out": held,
                   "photos": photos}, f, indent=1, ensure_ascii=False)
        f.write("\n")
    return photos


def load_review(corpus_dir):
    path = os.path.join(corpus_dir, "review.json")
    return json.load(open(path)) if os.path.exists(path) else {}


def fetch(photos, cache):
    """Download each photo missing from the cache, upright, without metadata,
    at LONG_EDGE. Image URLs are looked up fresh rather than stored, since
    Commons moves thumbnails around; the file title is what stays put."""
    from PIL import Image, ImageOps
    os.makedirs(cache, exist_ok=True)
    todo = [p for p in photos if not os.path.exists(os.path.join(cache, p["id"] + ".jpg"))]
    print(f"\nfetching {len(todo)} of {len(photos)} photos", file=sys.stderr)
    failed = []
    for i in range(0, len(todo), 40):
        batch = todo[i:i + 40]
        d = api(action="query", prop="imageinfo", iiprop="url", iiurlwidth=1920,
                titles="|".join(p["file"] for p in batch))
        q = d.get("query", {})
        names = {n["from"]: n["to"] for n in q.get("normalized", [])}
        urls = {pg["title"]: pg["imageinfo"][0].get("thumburl") or pg["imageinfo"][0]["url"]
                for pg in q.get("pages", {}).values() if pg.get("imageinfo")}
        for p in batch:
            url = urls.get(names.get(p["file"], p["file"]))
            try:
                if not url:
                    raise LookupError("no longer on Commons")
                img = Image.open(io.BytesIO(download(url)))
                img = ImageOps.exif_transpose(img)
                img = to_srgb(img)
                img.thumbnail((LONG_EDGE, LONG_EDGE), Image.LANCZOS)
                # a fresh save carries no EXIF, XMP or ICC: nothing but pixels
                # reaches the answerer, not even the camera's caption
                img.save(os.path.join(cache, p["id"] + ".jpg"), "JPEG", quality=90)
            except Exception as e:
                failed.append(p)
                print(f"  {p['id']} {p['file'][5:][:60]}: {e}", file=sys.stderr)
            time.sleep(0.5)
    return failed


def download(url):
    for attempt in range(4):
        try:
            req = urllib.request.Request(url, headers={"User-Agent": UA})
            with urllib.request.urlopen(req, timeout=90) as r:
                return r.read()
        except urllib.error.HTTPError as e:
            if e.code not in (429, 500, 502, 503, 504) or attempt == 3:
                raise
            time.sleep(int(e.headers.get("Retry-After") or 10 * (attempt + 1)))


def to_srgb(img):
    """Pixels in sRGB with no profile attached. Dropping an Adobe RGB profile
    without converting would wash out the very greys the quiz asks about."""
    icc = img.info.get("icc_profile")
    if icc:
        try:
            from PIL import ImageCms
            src = ImageCms.ImageCmsProfile(io.BytesIO(icc))
            img = ImageCms.profileToProfile(img, src, ImageCms.createProfile("sRGB"),
                                            outputMode="RGB")
        except Exception:
            pass    # a broken profile: the pixels as they are beat no photo at all
    img.info.pop("icc_profile", None)
    return img.convert("RGB")


def report(kb, photos, per_class, gate_min=5, gate_clouds=27):
    by = {e["name"]: [p for p in photos if p["label"] == e["name"]] for e in kb["entities"]}
    print(f"\n{'cloud':<28} photos  tune  held  sources", file=sys.stderr)
    for name, ps in by.items():
        t = sum(p["split"] == "tune" for p in ps)
        flag = "  <- under 4: left out of accuracy totals" if len(ps) < 4 else ""
        srcs = ", ".join(sorted({p["source"] for p in ps}))
        print(f"  {name:<28} {len(ps):>3} {t:>5} {len(ps) - t:>5}  {srcs}{flag}", file=sys.stderr)
    enough = sum(len(ps) >= gate_min for ps in by.values())
    print(f"\n{len(photos)} photos; {enough}/{len(by)} clouds have at least {gate_min} "
          f"(gate: {gate_clouds})", file=sys.stderr)


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--kb", required=True, help="knowledge base, e.g. data/clouds.json")
    ap.add_argument("--per-class", type=int, default=8)
    ap.add_argument("--held-out", type=int, default=3, help="of those, how many are held out")
    ap.add_argument("--no-fetch", action="store_true", help="write the manifest, skip downloads")
    a = ap.parse_args()

    kb = json.load(open(a.kb))
    base = os.path.splitext(os.path.basename(a.kb))[0]
    corpus_dir = os.path.join(HERE, "corpus", base)
    photos = build(kb, corpus_dir, a.per_class, a.held_out)
    report(kb, photos, a.per_class)
    if not a.no_fetch:
        failed = fetch(photos, os.path.join(HERE, "cache", "photos", base))
        if failed:
            print(f"\n{len(failed)} failed to download; drop them in review.json "
                  "or run again", file=sys.stderr)
            sys.exit(1)


if __name__ == "__main__":
    main()

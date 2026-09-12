"""Build an index of FCI breeds from the ten public group pages.

Records only facts: breed name, FCI number, group, section, origin country and
the URL of the published standard.  No standard text is touched here.
"""
import html, json, re, sys, time, urllib.request

GROUPS = {
    1: "1-Sheepdogs-and-Cattledogs-except-Swiss-Cattledogs",
    2: "2-Pinscher-and-Schnauzer-Molossoid-and-Swiss-Mountain-and-Cattledogs",
    3: "3-Terriers",
    4: "4-Dachshunds",
    5: "5-Spitz-and-primitive-types",
    6: "6-Scent-hounds-and-related-breeds",
    7: "7-Pointing-Dogs",
    8: "8-Retrievers-Flushing-Dogs-Water-Dogs",
    9: "9-Companion-and-Toy-Dogs",
    10: "10-Sighthounds",
}
UA = "ribice-kb-build/0.1 (+https://codecrane.hr/ribice; building a free identification quiz)"
BASE = "https://www.fci.be"

def get(url):
    req = urllib.request.Request(url, headers={"User-Agent": UA})
    with urllib.request.urlopen(req, timeout=60) as r:
        return r.read().decode("utf-8", "replace")

def text(s):
    return re.sub(r"\s+", " ", html.unescape(re.sub(r"<[^>]+>", "", s))).strip()

TOKEN = re.compile(
    r'SectionLabel_\d+"[^>]*>(?P<section>.*?)</span>'
    r'|PaysLabel_\d+"[^>]*>(?P<country>.*?)</span>'
    r'|class="nom"[^>]*?href="(?P<href>[^"]+)"[^>]*>(?P<name>.*?)</a>'
    r'|href="(?P<pdf>[^"]*Standards/[^"]+\.pdf)"',
    re.S)

def parse(group, page):
    section = country = None
    breeds, pending = [], None
    for m in TOKEN.finditer(page):
        if m.group("section"):
            section = text(m.group("section"))
        elif m.group("country"):
            country = re.sub(r"^\d+\.\s*", "", text(m.group("country")))
        elif m.group("name"):
            raw = text(m.group("name"))
            num = re.search(r"\((\d+)\)", raw)
            names = re.findall(r"\(([^)0-9][^)]*)\)", raw)
            pending = {
                "fci": int(num.group(1)) if num else None,
                "name_fci": re.sub(r"\s*\([^)]*\)", "", raw).strip(),
                "name_en": names[-1].strip() if names else None,
                "group": group,
                "section": section,
                "origin": country,
                "page": BASE + m.group("href"),
            }
            breeds.append(pending)
        elif m.group("pdf") and pending is not None:
            pending["standard"] = BASE + "/Nomenclature/" + m.group("pdf").split("Standards/")[0].split("/")[-1] + "Standards/" + m.group("pdf").split("Standards/")[1]
            pending = None
    return breeds

def main():
    out = []
    for g, slug in GROUPS.items():
        page = get(f"{BASE}/en/nomenclature/{slug}.html")
        got = parse(g, page)
        print(f"group {g:>2}: {len(got):>3} breeds", file=sys.stderr)
        out.extend(got)
        time.sleep(1.5)
    out.sort(key=lambda b: b["fci"] or 0)
    json.dump(out, open("fci-index.json", "w"), indent=1, ensure_ascii=False)
    print(f"total {len(out)} breeds -> fci-index.json", file=sys.stderr)

main()

"""Download the published FCI standards into a local cache.

Polite: serial, 1.5 s apart, identifying User-Agent, skips what it already has.
The PDFs stay in the scratchpad -- they are not ours to redistribute.
"""
import json, os, sys, time, urllib.request

UA = "ribice-kb-build/0.1 (+https://codecrane.hr/ribice; building a free identification quiz)"
breeds = [b for b in json.load(open("fci-index.json")) if b.get("standard")]
ok = fail = skip = 0
for i, b in enumerate(breeds, 1):
    dest = f"pdf/{b['fci']:03d}.pdf"
    if os.path.exists(dest) and os.path.getsize(dest) > 1000:
        skip += 1
        continue
    try:
        req = urllib.request.Request(b["standard"], headers={"User-Agent": UA})
        with urllib.request.urlopen(req, timeout=90) as r:
            data = r.read()
        if not data.startswith(b"%PDF"):
            raise ValueError("not a PDF")
        open(dest, "wb").write(data)
        ok += 1
    except Exception as e:
        fail += 1
        print(f"FAIL {b['fci']:>3} {b['name_fci']}: {e}", file=sys.stderr)
    if i % 25 == 0:
        print(f"  {i}/{len(breeds)} (ok {ok}, skipped {skip}, failed {fail})", file=sys.stderr)
    time.sleep(1.5)
print(f"done: {ok} fetched, {skip} already cached, {fail} failed", file=sys.stderr)

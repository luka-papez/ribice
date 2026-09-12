"""Read height and weight figures out of the SIZE section.

Purely mechanical -- a measurement is a fact, and this is the one attribute that
needs no judgement at all.  Standards vary in what they commit to: most give a
height at the withers, the toy breeds usually give only a weight, and a few
(Dachshund, Mastiff) give neither.
"""
import json, re, sys

NUM = r"\d{1,3}(?:[.,]\d{1,2})?"
SEP = r"(?:-|–|—|to)"

def units(*names):
    return r"(?:" + "|".join(names) + r")\b"

CM = units("cm", "cms", "centimetres?", "centimeters?")
INCH = units("ins", "in", "inch", "inches", "\"")
KG = units("kg", "kgs", "kilos?", "kilogram(?:me)?s?")
LB = units("lb", "lbs", "pounds?")

def rules(unit, factor):
    return (re.compile(rf"({NUM})\s*{SEP}\s*({NUM})\s*{unit}", re.I), factor)

def measure(text, spec, lo, hi):
    """All plausible values in `text`, converted to one unit."""
    vals, spans = [], []
    for unit, factor in spec:
        for m in re.finditer(rf"({NUM})\s*{SEP}\s*({NUM})\s*{unit}", text, re.I):
            vals += [num(m.group(1)) * factor, num(m.group(2)) * factor]
            spans.append(m.span())
    for unit, factor in spec:
        for m in re.finditer(rf"({NUM})\s*{unit}", text, re.I):
            if not any(a <= m.start() < b for a, b in spans):
                vals.append(num(m.group(1)) * factor)
    vals = [round(v, 1) for v in vals if lo <= v <= hi]
    return [min(vals), max(vals)] if vals else None

# Three kinds of figure in a size section are not a height at the withers:
# a MEASUREMENTS table of head and body lengths, a tolerance ("2 cm less,
# 4 cm more", "+-5 cm"), and the Dachshund's chest circumference.
# a MEASUREMENTS heading -- all caps, or title case with a colon after it --
# rather than the phrase "measured at the shoulder"
TABLE = re.compile(r"\bMEASUR[A-Z]{2,}\b|\bMeasurements?\s*:")
GIRTH = re.compile(r"circumference", re.I)
WITHERS = re.compile(r"height[^.]{0,30}withers|at the withers|at the shoulder", re.I)
NOISE = re.compile(
    r"[±+]\s*/?\s*-?\s*\d{1,2}(?:[.,]\d)?\s*cm"   # "+/- 1 cm", "± 5 cm"
    r"|\d{1,2}\s*cm\s+(?:less|more|under|over)\b"
    r"|(?:chest\s+)?circumference[^.]{0,120}", re.I)

def usable(text):
    """The part of a size section that can hold a height at the withers."""
    if GIRTH.search(text) and not WITHERS.search(text):
        return ""      # the Dachshund is specified by chest girth, not height
    m = TABLE.search(text)
    if m:
        text = text[:m.start()]
    return NOISE.sub(" ", text)

def num(s):
    return float(s.replace(",", "."))

def main():
    d = json.load(open("fci-sections.json"))
    for r in d:
        t = usable(r["sections"].get("size", ""))
        r["height_cm"] = measure(t, [(CM, 1.0), (INCH, 2.54)], 10, 100)
        if not r["height_cm"]:
            # a few standards state the height in the general appearance instead
            alt = usable(r["sections"].get("appearance", ""))
            r["height_cm"] = measure(alt, [(CM, 1.0), (INCH, 2.54)], 10, 100)
            r["height_from_appearance"] = bool(r["height_cm"])
        r["weight_kg"] = measure(t, [(KG, 1.0), (LB, 0.4536)], 0.5, 100)
    json.dump(d, open("fci-sections.json", "w"), indent=1, ensure_ascii=False)
    h = sum(1 for r in d if r["height_cm"])
    w = sum(1 for r in d if r["weight_kg"])
    either = sum(1 for r in d if r["height_cm"] or r["weight_kg"])
    n = len(d)
    print(f"height {h}/{n}   weight {w}/{n}   either {either}/{n}", file=sys.stderr)
    for r in d:
        if not (r["height_cm"] or r["weight_kg"]):
            print(f"  MISS {r['fci']:>3} {r['name_fci']}: "
                  f"{(r['sections'].get('size') or '(no size section)')[:80]}", file=sys.stderr)

main()

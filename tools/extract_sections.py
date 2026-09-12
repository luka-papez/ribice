"""Split each cached FCI standard into its named sections.

Purely mechanical: the standards all follow the same headings, so this turns a
PDF into {"EARS": "...", "COAT/Hair": "...", ...}.  The output is a local
working file for reading facts off -- none of this text goes into the KB.
"""
import collections, json, re, subprocess, sys, glob, os

TOP = [
    "ORIGIN", "DATE OF PUBLICATION OF THE OFFIC[IA]+L VALID STANDARD",
    "UTILI[SZ]ATION", "FCI[- ]CLASSIFICATION", "BRIEF HISTORICAL SUMMARY",
    "GENERAL APPEARANCE", "IMPORTANT PROPORTIONS",
    "BEHAVIOUR\\s*(?:/|AND)\\s*TEMPERAMENT", "TEMPERAMENT\\s*/\\s*BEHAVIOUR",
    "HEAD", "CRANIAL REGION", "FACIAL REGION", "EYES", "EARS", "NECK",
    "BODY", "TAIL", "LIMBS", "FOREQUARTERS", "HINDQUARTERS", "FEET",
    "GAIT\\s*/?\\s*MOVEMENT", "SKIN", "COAT",
    "SIZES?\\s*(?:AND|/)\\s*WEIGHT", "SIZES", "WEIGHT\\s*(?:AND|/)\\s*SIZE",
    "HEIGHT\\s*(?:AND|/)\\s*WEIGHT", "HEIGHT AT (?:THE )?WITHERS",
    "SIZE", "HEIGHT", "WEIGHT",
    "FAULTS", "SERIOUS FAULTS", "DISQUALIFYING FAULTS", "N\\.?B",
]
SUB = [
    "Colour and Colour Patterns", "Colour and colour patterns",
    "Colour and markings", "Colour and Markings", "Coat colour", "Coat Colour",
    "Colours", "Colors",
    "Skull", "Stop", "Nose", "Muzzle", "Lips", "Jaws/teeth", "Jaws / teeth",
    "Teeth", "Cheeks", "Eyes", "Ears", "Topline", "Withers", "Back", "Loin",
    "Croup", "Rump", "Chest", "Underline and belly", "Underline", "Belly",
    "Shoulder", "Upper arm", "Elbow", "Forearm", "Forefeet", "Hind feet",
    "Thigh", "Stifle", "Hock joint", "Hair", "Colour", "Color",
    "Height at the withers", "Height at withers",
    "Ideal height at withers", "Ideal height", "Height", "Weight",
]
HEAD_RE = re.compile(
    r"^[ \t]*(?:•[ \t]*)?(?P<top>" + "|".join(TOP) + r")[ \t]*(?::|$)"
    r"|^[ \t]*(?P<sub>" + "|".join(re.escape(s) for s in SUB) + r")[ \t]*:",
    re.M)

DROP = re.compile(
    r"^\s*(?:\d{1,3}|St-?\s*FCI.*|FEDERATION CYNOLOGIQUE.*|SECRETARIAT GENERAL.*"
    r"|_{3,}|©.*|FCI-Standard\s*N°.*|\d{2}\.\d{2}\.\d{4}.*/\s*EN\s*|.*/\s*EN)\s*$")

def clean(raw):
    lines = [l for l in raw.splitlines() if not DROP.match(l)]
    text = "\n".join(lines)
    # a standard's body starts at ORIGIN; everything before is the cover page
    m = re.search(r"(?m)^\s*ORIGIN\s*:", text)
    return text[m.start():] if m else text

FAULTY = re.compile(r"FAULTS|N\\.?B")

def sections(text):
    """Headings, in order.  Everything from FAULTS onward describes departures
    from the breed, not the breed, and repeats subheadings (Eyes:, Height:) that
    would otherwise contaminate the descriptive sections -- so once the faults
    begin, only further top-level headings are recognised."""
    out, last, key, in_faults = {}, 0, None, False
    for m in HEAD_RE.finditer(text):
        top = m.group("top")
        if in_faults and not top:
            continue
        if key:
            out[key] = out.get(key, "") + " " + text[last:m.start()]
        name = top or m.group("sub")
        key = re.sub(r"\s+", " ", name).strip().upper() if top else name.strip()
        if top and FAULTY.search(key):
            in_faults = True
        last = m.end()
    if key:
        out[key] = out.get(key, "") + " " + text[last:]
    return {k: re.sub(r"\s+", " ", v).strip() for k, v in out.items() if v.strip()}

CANON = {
    "ORIGIN": "origin", "UTILIZATION": "utilization", "UTILISATION": "utilization",
    "FCI-CLASSIFICATION": "classification", "FCI CLASSIFICATION": "classification",
    "GENERAL APPEARANCE": "appearance", "IMPORTANT PROPORTIONS": "proportions",
    "BEHAVIOUR / TEMPERAMENT": "temperament", "BEHAVIOUR/TEMPERAMENT": "temperament",
    "BEHAVIOUR AND TEMPERAMENT": "temperament", "TEMPERAMENT / BEHAVIOUR": "temperament",
    "HEAD": "head", "Skull": "skull", "Stop": "stop", "Nose": "nose",
    "Muzzle": "muzzle", "Lips": "lips", "Jaws/teeth": "teeth",
    "Jaws / teeth": "teeth", "Teeth": "teeth", "Cheeks": "cheeks",
    "EYES": "eyes", "Eyes": "eyes", "EARS": "ears", "Ears": "ears",
    "NECK": "neck", "BODY": "body", "Topline": "topline", "Withers": "withers",
    "Back": "back", "Loin": "loin", "Croup": "croup", "Rump": "croup",
    "Chest": "chest", "Underline and belly": "underline", "Underline": "underline",
    "Belly": "underline", "TAIL": "tail",
    "FOREQUARTERS": "forequarters", "HINDQUARTERS": "hindquarters",
    "Shoulder": "shoulder", "Upper arm": "upper_arm", "Elbow": "elbow",
    "Forearm": "forearm", "Forefeet": "forefeet", "Hind feet": "hind_feet",
    "Thigh": "thigh", "Stifle": "stifle", "Hock joint": "hock",
    "GAIT / MOVEMENT": "gait", "GAIT/MOVEMENT": "gait", "GAIT/ MOVEMENT": "gait",
    "SKIN": "skin", "COAT": "coat", "Hair": "coat_hair",
    "Colour": "coat_colour", "Color": "coat_colour",
    "Colours": "coat_colour", "Colors": "coat_colour",
    "Colour and Colour Patterns": "coat_colour",
    "Colour and colour patterns": "coat_colour",
    "Colour and markings": "coat_colour", "Colour and Markings": "coat_colour",
    "Coat colour": "coat_colour", "Coat Colour": "coat_colour",
    "COLOUR": "coat_colour", "COLOURS": "coat_colour",
    "Hair texture": "coat_hair",
    "SIZE AND WEIGHT": "size", "SIZE/WEIGHT": "size", "HEIGHT": "size",
    "WEIGHT AND SIZE": "size", "WEIGHT/SIZE": "size",
    "HEIGHT AND WEIGHT": "size", "HEIGHT/WEIGHT": "size",
    "HEIGHT AT WITHERS": "size", "HEIGHT AT THE WITHERS": "size", "SIZE": "size", "WEIGHT": "size",
    "Height at withers": "size", "Ideal height at withers": "size",
    "Height": "size", "Weight": "size", "Ideal height": "size",
    "FAULTS": "faults", "SERIOUS FAULTS": "faults",
    "DISQUALIFYING FAULTS": "disqualifying",
}

def fold(k):
    k = re.sub(r"\s*/\s*", "/", k.strip())
    return re.sub(r"^(SIZE|FAULT)S$", r"\1", k)

CANON = {fold(k): v for k, v in CANON.items()}

# structural headers and matter the quiz has no use for: recognised, not reported
IGNORE = {"BRIEF HISTORICAL SUMMARY", "N.B", "NB", "CRANIAL REGION",
          "FACIAL REGION", "LIMBS", "FEET", "DATE OF PUBLICATION OF THE OFFICIAL VALID STANDARD",
          "DATE OF PUBLICATION OF THE OFFICAL VALID STANDARD"}

SIZEISH = re.compile(r"^(?:IDEAL\s+)?(?:HEIGHT|WEIGHT|SIZE)\b", re.I)
DROPPED = collections.Counter()

def canon(secs):
    out = {}
    for k, v in secs.items():
        c = CANON.get(fold(k))
        if not c and SIZEISH.match(k):
            c = "size"          # "Height at the withers", "Ideal height", ...
        if not c and fold(k) in IGNORE:
            continue
        if not c:
            DROPPED[k] += 1     # never silently: an unmapped heading loses text
            continue
        out[c] = (out[c] + " " + v).strip() if c in out else v
    return out

def main():
    spine = {b["fci"]: b for b in json.load(open("spine.json"))}
    out = []
    for path in sorted(glob.glob("pdf/*.pdf")):
        fci = int(os.path.basename(path)[:-4])
        raw = subprocess.run(["pdftotext", path, "-"], capture_output=True,
                             text=True).stdout
        secs = canon(sections(clean(raw)))
        b = spine.get(fci, {})
        out.append({"fci": fci, "name_fci": b.get("name_fci"),
                    "name_en": b.get("name_en"), "group": b.get("group"),
                    "sections": secs})
    json.dump(out, open("fci-sections.json", "w"), indent=1, ensure_ascii=False)
    want = ["ears", "tail", "coat_hair", "coat_colour", "size", "appearance",
            "muzzle", "stop", "eyes", "classification", "utilization"]
    print(f"{len(out)} standards parsed", file=sys.stderr)
    for k in want:
        n = sum(1 for r in out if r["sections"].get(k))
        print(f"  {k:<20} {n:>4}/{len(out)}", file=sys.stderr)
    if DROPPED:
        print("  unmapped headings (text discarded):", file=sys.stderr)
        for k, n in DROPPED.most_common(15):
            print(f"    {k!r:<34} {n}", file=sys.stderr)

main()

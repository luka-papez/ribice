"""Code the extracted standards into ribice knowledge-base values.

Every attribute is coded from the section the standard devotes to it, by
patterns that are listed here in full so a reader can disagree with one.  Where
an attribute has no textual signal -- muzzle, build, and the booleans -- the
breeds are named in curated.py instead.

A value is a list, meaning "any of these": standards routinely permit two ear
carriages or two coat textures, and saying so is more honest than picking one.
"""
import json, re, sys, unicodedata
import curated, hand_coded, varieties

E = json.load(open("fci-extract.json"))

def norm(s):
    s = unicodedata.normalize("NFKD", s or "").encode("ascii", "ignore").decode().lower()
    return " ".join(re.sub(r"[^a-z0-9]+", " ", s).split())

BY_NAME = {norm(r["name"]): r["fci"] for r in E}
def fciset(names):
    return {BY_NAME[norm(n)] for n in names if norm(n) in BY_NAME}

# ---------------------------------------------------------------- size

def size_of(r):
    h, w = r.get("height_cm"), r.get("weight_kg")
    if h:
        mid = (h[0] + h[1]) / 2
        return [("toy" if mid < 28 else "small" if mid < 40 else "medium"
                 if mid < 55 else "large" if mid < 70 else "giant")]
    if w:
        mid = (w[0] + w[1]) / 2
        return [("toy" if mid < 6 else "small" if mid < 12 else "medium"
                 if mid < 25 else "large" if mid < 45 else "giant")]
    return []

# ---------------------------------------------------------------- coat

# the first sentence of a coat section characterises the coat as a whole;
# later sentences describe particular parts ("on the head, short")
def lead(text, n=2):
    return " ".join(re.split(r"(?<=[.;])\s+", text)[:n])

LENGTH = [
    ("long",   r"\blong\b|abundant|profuse|flowing|\bmane\b|luxuriant|copious"),
    ("medium", r"medium[- ]length|moderately long|medium[- ]long|moderate length"
               r"|semi[- ]long|half[- ]long"),
    ("short",  r"\bshort\b|close[- ]lying|smooth|flat[- ]lying|\bdense and flat\b"),
]
TEXTURE = [
    ("hairless", r"hairless|naked|absence of hair|devoid of hair"),
    ("corded",   r"\bcord(?:s|ed|ing)?\b|dreadlock|felted"),
    ("curly",    r"\bcurl(?:y|ed|s|ing)?\b|woolly|wooly|astrakhan|frizzy"),
    ("harsh",    r"harsh|wiry|\bwire\b|bristl|coarse|\brough\b|broken[- ]coat|rugged"),
    ("wavy",     r"\bwav(?:y|ed|es)\b"),
    ("silky",    r"silk(?:y|en)|\bfine\b.{0,12}\bsoft\b|glossy|sheen"),
    ("smooth",   r"smooth|flat\b|close[- ]lying|lying close|sleek"),
]

# "Fine, smooth, short and glossy, neither harsh nor woolly" must not make a
# Pug harsh and woolly, and "black mask" must not make a fawn Boxer black.
# Both are the same bug: a clause that mentions a value without asserting it.
NEG = re.compile(r"\b(?:never|not|neither|nor|free from|without|except|excluding|"
                 r"apart from|other than|undesirable|objectionable|forbidden|"
                 r"disqualif\w*|must not|should not|no\b)\b", re.I)
ELSEWHERE = re.compile(r"\b(?:mask|nose|eyes?|eyelid|lips?|nails?|pads?|leather|"
                       r"haw|iris|pigment|gums?|tongue|palate)\b", re.I)

def assertive(text, drop_elsewhere=False):
    """The clauses that actually assert something about the dog.

    A negation governs the rest of its sentence, not just its own clause:
    "Not solid black, or white, or black and tan" excludes all three, and
    dropping only the first clause leaves the other two looking asserted.
    """
    keep = []
    for sentence in re.split(r"(?<=[.;])\s+", text):
        m = NEG.search(sentence)
        if m:
            sentence = sentence[:m.start()]
        for c in re.split(r"[,]| but | although ", sentence):
            if drop_elsewhere and ELSEWHERE.search(c):
                continue
            keep.append(c)
    return " ; ".join(keep)

def first_matching(rules, text, limit=2):
    out = []
    for value, pat in rules:
        if re.search(pat, text, re.I):
            out.append(value)
        if len(out) == limit:
            break
    return out

def coatsec(r, key):
    """Most standards split COAT into Hair: and Colour:; a few do not."""
    return r["sections"].get(key) or r["sections"].get("coat", "")

def coat_length(r):
    t = coatsec(r, "coat_hair")
    if not t: return []
    a = assertive(t)
    v = first_matching(LENGTH, lead(a), limit=2)
    return v or first_matching(LENGTH, a, limit=1) or first_matching(LENGTH, t, limit=1)

def coat_texture(r):
    t = coatsec(r, "coat_hair")
    if not t: return []
    a = assertive(t)
    v = first_matching(TEXTURE, lead(a), limit=2)
    return v or first_matching(TEXTURE, a, limit=1)

# ---------------------------------------------------------------- ears

EARS = [
    ("long_hanging", r"reach\w*[^.;]{0,45}\b(?:nose|muzzle)\b"
                     r"|beyond[^.;]{0,30}\b(?:nose|muzzle)\b"
                     r"|very long[^.;]{0,30}(?:leather|ear)|hound[- ]like ear"),
    ("rose",   r"\brose\b"),
    ("button", r"button|folded forward|fold(?:ed|ing)? (?:over )?forward|tip.{0,20}forward"),
    ("semi_erect", r"semi[- ]?erect|semi[- ]?pricked|tip(?:s)? (?:folded|dropping|bent|breaking)"
                   r"|erect with the tip|tipped\b"),
    ("erect",  r"\berect\b|prick(?:ed)?\b|upright|carried up|standing up|straight up"),
    ("dropped", r"\bdrop(?:ping|ped)?\b|hanging|pendulous|pendant|falling|lying close"
                r"|flat against|close to the (?:head|cheek|sides?)|hang down|drooping"
                r"|\blobular\b|curl(?:ed|ing)? in\b|turn(?:ed|ing)?[^.;]{0,12}inward"
                r"|carried flat|flat to (?:the )?side|against the (?:head|cheek)"
                r"|folded[^.;]{0,12}(?:back|against)"),
]

# "when pulled forward the leather should reach the nose" is how a standard
# tells a judge to measure the ear, not how the dog carries it -- but it reads
# exactly like a hound's long hanging leather
MEASURING = re.compile(r"\b(?:when|if|once)\b[^.;]{0,20}\b(?:pulled|drawn|brought|"
                       r"stretched|extended)\b[^.;]*", re.I)

def ears(r):
    t = r["sections"].get("ears", "")
    if not t: return []
    clean = MEASURING.sub(" ", t)
    # "flat and without twisting hanging down close to the head" negates the
    # twisting, not the hanging; if the filtered text yields nothing, the
    # unfiltered text is better than no value at all
    v = (first_matching(EARS, assertive(clean), limit=2)
         or first_matching(EARS, clean, limit=1))
    # "long_hanging" implies dropped; keep only the more specific one
    if "long_hanging" in v and "dropped" in v:
        v.remove("dropped")
    return v

# ---------------------------------------------------------------- tail

TAIL = [
    ("short",    r"\bstump|bob[- ]?tail|naturally short|absent|rudimentary|no tail"),
    ("over_back", r"over the back|curled over|curl(?:ed|ing)? (?:up )?(?:over|on|above)"
                  r"|on the back|in a ring|ring[- ]shaped|screw|rolled|curved over"
                  r"|touch(?:ing|es) the back"),
    ("high",     r"carried high|\bgaily\b|\bgay\b|sickle|above the (?:line of the )?back"
                 r"|raised above|upward|set on high[^.;]{0,40}carried"),
    ("level",    r"level with the back|carried level|horizontal|in line with the back"
                 r"|straight out|continuation of the (?:top ?line|back)|carried straight"),
    ("low",      r"carried low|hanging (?:down|loosely)|hangs? down|pendant|drooping"
                 r"|between the (?:hind ?legs|thighs)|carried down|\bsabre\b|\bsaber\b"
                 r"|set (?:on )?low|not reach\w*[^.;]{0,20}hock|hanging down"),
]

def tail(r):
    t = r["sections"].get("tail", "")
    if not t: return []
    return first_matching(TAIL, assertive(t), limit=2)

# ---------------------------------------------------------------- colour

COLOUR = [
    ("black",  r"\bblack\b"), ("white", r"\bwhite\b"), ("cream", r"\bcream\b|\bivory\b"),
    ("gold",   r"\bgold(?:en)?\b|\byellow\b|\bwheaten\b|\blemon\b|\bapricot\b"),
    ("red",    r"\bred\b|\bmahogany\b|\bchestnut\b|\borange\b|\bfox[- ]red\b|\bruby\b"),
    ("fawn",   r"\bfawn\b|\bsand(?:y|-coloured)?\b|\bbeige\b|\bisabella\b"),
    ("brown",  r"\bbrown\b|\bchocolate\b|\bliver\b|\bhavana\b"),
    ("grey",   r"\bgrey\b|\bgray\b|\bpepper\b|\bwolf[- ]?(?:grey|colour)\b|\bbadger\b|\bash\b"),
    ("blue",   r"\bblue\b|\bslate\b"),
    ("silver", r"\bsilver\b|\bplatinum\b"),
]
# words that mean a colour is excluded rather than permitted
NEGATED = re.compile(r"(?:never|not|no|except|excluded|undesirable|forbidden|disqualif)"
                     r"[^.;]{0,40}\b(black|white|cream|gold|yellow|red|fawn|brown|liver|"
                     r"grey|gray|blue|silver)\b", re.I)

def colour(r):
    t = coatsec(r, "coat_colour")
    if not t: return []
    if re.search(r"all colou?rs|any colou?r|variety of colou?rs"
                 r"|colou?rs? (?:are )?(?:immaterial|indifferent)", t, re.I):
        return [v for v, _ in COLOUR]      # say so, rather than list what appears
    a = assertive(t, drop_elsewhere=True)  # a black mask is not a black dog
    return [v for v, pat in COLOUR if re.search(pat, a, re.I)]

MARKINGS = [
    ("brindle",  r"\bbrindl"),
    ("merle",    r"\bmerle\b|\bharlequin\b|\bdapple\b|\bblue[- ]merle\b"),
    ("ticked",   r"\bticked?\b|\bspeckl|\broan\b|\bmottl|\bspotted\b|\bfleck|\bbelton\b"),
    ("tricolour", r"\btri-?colou?r"),
    ("tan_points", r"tan mark|black and tan|tan (?:above|over) the eyes|\btan points?\b"
                   r"|(?:black|blue|liver|brown)[- ]and[- ]tan"),
    ("saddle",   r"\bsaddle\b|\bblanket\b|\bmantle\b"),
    ("mask",     r"\bmask\b"),
    ("white_markings", r"white mark|\bblaze\b|white patch|white on the (?:chest|feet|toes)"
                       r"|parti-?colou?r|\bpied\b|piebald|white collar|white spot"),
]

def markings(r):
    t = coatsec(r, "coat_colour")
    if not t: return []
    a = assertive(t)
    out = [v for v, p in MARKINGS if re.search(p, a, re.I)]
    if not out and re.search(r"\bsolid\b|\bself[- ]colou?r|\bwhole[- ]colou?r|\buniform", t, re.I):
        out = ["solid"]
    return out or ["solid"]

# ------------------------------------------------------- curated attributes

FLAT, SHORTM, LONGM = map(fciset, (curated.FLAT_FACED, curated.SHORT_MUZZLE, curated.LONG_MUZZLE))
RACY, MASSIVE, STURDY = map(fciset, (curated.RACY, curated.MASSIVE, curated.STURDY))
BOOLS = {
    "short_legs": fciset(curated.SHORT_LEGS), "wrinkled": fciset(curated.WRINKLED),
    "beard": fciset(curated.BEARD), "blue_tongue": fciset(curated.BLUE_TONGUE),
    "double_dewclaws": fciset(curated.DOUBLE_DEWCLAWS),
}

def muzzle(r):
    f = r["fci"]
    if f in FLAT: return ["flat"]
    if f in SHORTM: return ["short"]
    if f in LONGM: return ["long"]
    return ["medium"]

def build(r):
    f = r["fci"]
    if f in MASSIVE: return ["massive"]
    if f in RACY: return ["racy"]
    if f in STURDY: return ["sturdy"]
    return ["athletic"]

def feathering(r):
    t = " ".join(r["sections"].get(k, "") for k in ("coat_hair", "tail", "ears"))
    # "free from feathering" is not feathering
    return bool(re.search(r"feather|fring|culotte|breeches|petticoat",
                          assertive(t), re.I))

# ---------------------------------------------------------------- assembly

# Breeds the patterns cannot separate but a person can.  The three Belgian
# griffons share a head and a size; what tells them apart is coat and colour.
OVERRIDES = {
  80: {"coat_length": ["medium"], "coat_texture": ["harsh"], "colour": ["red"],
       "markings": ["solid"], "beard": True},
  81: {"coat_length": ["medium"], "coat_texture": ["harsh"], "colour": ["black"],
       "markings": ["solid", "tan_points"], "beard": True},
  82: {"coat_length": ["short"], "coat_texture": ["smooth"],
       "colour": ["red", "black", "fawn"], "markings": ["mask"], "beard": False},
}

def code(r):
    e = {
        "name": r["name"],
        "size": size_of(r),
        "coat_length": coat_length(r),
        "coat_texture": coat_texture(r),
        "ears": ears(r),
        "tail": tail(r),
        "colour": colour(r),
        "markings": markings(r),
        "muzzle": muzzle(r),
        "build": build(r),
        "feathering": feathering(r),
    }
    for k, ids in BOOLS.items():
        if r["fci"] in ids:
            e[k] = True
    e.update(OVERRIDES.get(r["fci"], {}))
    return e

def main():
    out, gaps = [], {}
    for r in E:
        base = code(r)
        base["_fci"] = r["fci"]
        base["_note"] = note(r)
        if r.get("image"):
            base["_image"] = {"url": r["image"], "source": r["article"],
                              "credit": "", "license": ""}
        for suffix, over in varieties.SPLITS.get(r["fci"], [(None, {})]):
            e = dict(base)
            if suffix:
                e["name"] = f"{r['name']} ({suffix})"
                for k, v in over.items():
                    e[k] = v
            for attr, table in hand_coded.TABLES.items():
                if e["name"] in table:
                    e[attr] = table[e["name"]]
            out.append(e)

    for e in out:
        for k, v in e.items():
            if isinstance(v, list) and not v:
                gaps.setdefault(k, []).append(e["name"])
    names = {e["name"] for e in out}
    for attr, table in hand_coded.TABLES.items():
        stray = [k for k in table if k not in names]
        if stray:
            print(f" hand_coded.{attr.upper()} names no entity: {stray}", file=sys.stderr)
    json.dump(out, open("coded.json", "w"), indent=1, ensure_ascii=False)
    print(f"{len(out)} entities coded -> coded.json", file=sys.stderr)
    for k in ("size", "coat_length", "coat_texture", "ears", "tail", "colour", "markings"):
        n = sum(1 for e in out if e.get(k))
        print(f"  {k:<14} {n:>4}/{len(out)}", file=sys.stderr)
    print("\n gaps needing a hand-written value:", file=sys.stderr)
    for k, names in sorted(gaps.items()):
        print(f"  {k:<14} {len(names)}: {', '.join(names[:8])}", file=sys.stderr)

def note(r):
    bits = [r["name_fci"].title()] if r["name_fci"].title() != r["name"] else []
    if r.get("origin"): bits.append(r["origin"].title())
    if r.get("group"): bits.append(f"FCI group {r['group']}")
    return " - ".join(bits)

main()

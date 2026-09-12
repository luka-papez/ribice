"""Count how often each candidate attribute value is actually attested.

Word-boundary matching over the relevant section of all 343 standards, so the
proposed vocabulary reflects what the corpus says rather than what I expect.
"""
import json, re, sys

d = json.load(open("fci-extract.json"))

PROBES = {
 "ears": {
   "erect": r"erect|pricked?\b|upright", "semi-erect": r"semi-?erect|semi-?pricked?|tipped",
   "drop/hanging": r"\bdrop\b|dropping|hanging|pendulous|pendant|falling|lying close",
   "rose": r"rose ear|\brose\b", "button": r"button",
   "folded": r"folded|fold\b", "v-shaped": r"v-shaped|v shaped",
   "triangular": r"triangular", "rounded": r"rounded|round(?:ed)? (?:at )?tip",
   "pointed": r"pointed|tapering to a point", "cropped": r"cropped|cropping",
   "long/low-set": r"set (?:on )?low|low[- ]set", "high-set": r"set (?:on )?high|high[- ]set",
   "feathered": r"feather|fringe|silky hair|well clothed",
 },
 "tail": {
   "over the back": r"over the back|curled over|on the back|curved over",
   "curled/ringed": r"curl(?:ed|ing)?\b|in a ring|ring[- ]shaped|screw",
   "sickle": r"sickle", "sabre": r"sabre|saber",
   "carried high": r"carried high|gaily|erect", "carried low": r"carried low|hanging down|pendant",
   "straight": r"straight", "docked": r"dock(?:ed|ing)",
   "bushy/plumed": r"bushy|plume|brush\b|well[- ]furnished|feathered",
   "otter": r"otter", "short/stumpy": r"stump|short tail|bob[- ]?tail",
   "tapering": r"taper", "hock": r"hock",
 },
 "coat_hair": {
   "short": r"\bshort\b", "medium": r"medium length|moderately long|medium[- ]long",
   "long": r"\blong\b", "smooth": r"smooth|flat[- ]lying|close[- ]lying",
   "harsh/wiry": r"harsh|wiry|rough|bristl|coarse",
   "curly": r"curl(?:y|ed)?\b", "wavy": r"wav(?:y|e)", "silky": r"silky|silken",
   "corded": r"cord(?:ed|s)", "double": r"double[- ]coat|undercoat",
   "no undercoat": r"without undercoat|no undercoat", "hairless": r"hairless|naked",
   "feathering": r"feather|fring|breeches|culottes",
 },
 "coat_colour": {
   "black": r"\bblack\b", "white": r"\bwhite\b", "red": r"\bred\b",
   "fawn": r"fawn", "tan": r"\btan\b", "brown": r"brown|chocolate",
   "grey": r"\bgrey\b|\bgray\b", "blue": r"\bblue\b", "cream": r"cream",
   "golden/yellow": r"golden|\bgold\b|yellow", "liver": r"liver",
   "wheaten": r"wheaten", "sandy": r"sand(?:y|-coloured)?", "silver": r"silver",
   "brindle": r"brindle", "merle": r"merle", "sable": r"sable",
   "tricolour": r"tri-?colou?r", "particolour/pied": r"parti-?colou?r|\bpied\b|piebald|broken",
   "spotted/ticked": r"spotted|speckl|ticked|mottl|roan|flecked",
   "saddle": r"saddle|blanket", "mask": r"mask", "white markings": r"white mark|blaze|white patch",
 },
}

def report(field, probes):
    have = [r for r in d if r["sections"].get(field)]
    print(f"\n=== {field}  ({len(have)}/{len(d)} breeds have this section)")
    rows = []
    for label, pat in probes.items():
        rx = re.compile(pat, re.I)
        n = sum(1 for r in have if rx.search(r["sections"][field]))
        rows.append((n, label))
    for n, label in sorted(rows, reverse=True):
        bar = "#" * round(n / len(have) * 40)
        print(f"  {label:<20} {n:>4} {100*n/len(have):>5.1f}%  {bar}")

for f, p in PROBES.items():
    report(f, p)

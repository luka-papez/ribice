"""Write data/clouds.json -- the ten WMO cloud genera and their species.

Unlike the dog knowledge base this one is authored rather than extracted. The
classification is an international standard that has been in every textbook
since Luke Howard named it in 1802, and the WMO's own Cloud Atlas reserves all
rights over its presentation, so there is nothing to scrape and no need to:
the taxonomy is not anyone's to own. Descriptions were checked against the
Wikipedia list of cloud types (CC BY-SA); no wording is copied.

Two decisions shape it.

Varieties are attributes, not entities. Opacity (translucidus, opacus),
arrangement (undulatus, radiatus, duplicatus) and the rest describe a
particular cloud on a particular day; they are things to ask about, not things
to identify. Entities are genus + species, which is what a name answers.

Height is not an attribute, because nobody can judge how high a cloud is. What
an observer can do is hold a hand up and measure the elements: under a finger
is cirrocumulus, one to three fingers altocumulus, more than a fist
stratocumulus. That is the WMO's own field test and it does the work height
would pretend to.
"""
import json, os, urllib.parse

# shared shape of a genus: values every species of it inherits unless it says
# otherwise, so a species entry only states what makes it that species
GENERA = {
 "Cirrus":        dict(depth="none", top="none", element_size="none", shading=False, opacity="transparent",
                       colour="white", base="undefined", precipitation="none",
                       sky_cover="patches", shape="wispy"),
 "Cirrocumulus":  dict(depth="none", top="none", element_size="tiny", shading=False, opacity="transparent",
                       colour="white", base="undefined", precipitation="none",
                       sky_cover="patches", shape="rolls"),
 "Cirrostratus":  dict(depth="none", top="none", element_size="none", shading=False, opacity="transparent",
                       colour="white", base="undefined", precipitation="none",
                       sky_cover="whole_sky", shape="sheet", halo=True),
 "Altocumulus":   dict(depth="none", top="none", element_size="small", shading=True, opacity="translucent",
                       colour="white", base="undefined", precipitation="none",
                       sky_cover="patches", shape="rolls"),
 "Altostratus":   dict(depth="none", top="none", element_size="none", shading=False, opacity="translucent",
                       colour="grey", base="not_visible", precipitation="none",
                       sky_cover="whole_sky", shape="sheet"),
 "Stratocumulus": dict(depth="none", top="none", element_size="large", shading=True, opacity="opaque",
                       colour="grey", base="flat", precipitation="none",
                       sky_cover="most_of_sky", shape="rolls"),
 "Stratus":       dict(depth="none", top="none", element_size="none", shading=False, opacity="opaque",
                       colour="grey", base="not_visible", precipitation="none",
                       sky_cover="whole_sky", shape="sheet"),
 "Nimbostratus":  dict(depth="none", top="none", element_size="none", shading=False, opacity="opaque",
                       colour="dark_grey", base="not_visible",
                       precipitation="steady", sky_cover="whole_sky", shape="sheet"),
 "Cumulus":       dict(element_size="none", shading=True, opacity="opaque",
                       colour="white", base="flat", precipitation="none",
                       sky_cover="isolated", shape="heaped", depth="square", top="cauliflower"),
 "Cumulonimbus":  dict(element_size="none", shading=True, opacity="opaque",
                       colour="dark_grey", base="flat", precipitation="showers",
                       sky_cover="isolated", shape="towering", depth="tall", top="smooth"),
}

# species, as what distinguishes them from the plain genus
SPECIES = [
 ("Cirrus", "fibratus",        dict(shape="wispy"),
  "Long straight or curved filaments -- the classic mare's tails, with no hooks or tufts."),
 ("Cirrus", "uncinus",         dict(shape="wispy", hooks=True, precipitation="virga"),
  "Filaments drawn out into a hook or comma at one end."),
 ("Cirrus", "spissatus",       dict(shape="flat_patches", opacity="opaque", colour="grey"),
  "Dense enough to look grey against the sun; often what is left of a thunderstorm anvil."),
 ("Cirrus", "castellanus",     dict(shape="tufts", turrets=True),
  "Small turrets rising from a common base, a sign of instability high up."),
 ("Cirrus", "floccus",         dict(shape="tufts", base="ragged", precipitation="virga"),
  "Rounded tufts with ragged trailing undersides."),

 ("Cirrocumulus", "stratiformis", dict(shape="rolls", sky_cover="most_of_sky"),
  "A wide sheet of tiny grains -- a mackerel sky."),
 ("Cirrocumulus", "lenticularis", dict(shape="lens"),
  "Smooth almond-shaped patches of very fine grain."),
 ("Cirrocumulus", "floccus",      dict(shape="tufts", base="ragged", precipitation="virga"),
  "Tiny ragged tufts, each too small to show any shading."),
 ("Cirrocumulus", "castellanus",  dict(shape="tufts", turrets=True),
  "Miniature turrets on a common base."),

 ("Cirrostratus", "fibratus",  dict(shape="wispy"),
  "A fibrous veil across the sky; the sun casts sharp shadows through it."),
 ("Cirrostratus", "nebulosus", dict(shape="sheet"),
  "A featureless milky veil, often noticed only by the halo around the sun."),

 ("Altocumulus", "stratiformis", dict(shape="rolls", sky_cover="most_of_sky", mamma=True),
  "Rows or patches of shaded rolls covering much of the sky."),
 ("Altocumulus", "lenticularis", dict(shape="lens", sky_cover="isolated"),
  "Smooth lens or almond shapes, usually standing still downwind of hills."),
 ("Altocumulus", "castellanus",  dict(shape="tufts", turrets=True),
  "Turrets rising from a common base -- often the morning warning of afternoon storms."),
 ("Altocumulus", "floccus",      dict(shape="tufts", base="ragged", precipitation="virga"),
  "Ragged tufts with trailing undersides."),
 ("Altocumulus", "volutus",      dict(shape="roll_cloud", sky_cover="isolated"),
  "A single long tube rolling about its own horizontal axis, detached from any other cloud."),

 ("Altostratus", "translucidus", dict(opacity="translucent"),
  "A grey sheet through which the sun shows as a bright patch, as if through frosted glass."),
 ("Altostratus", "opacus",       dict(opacity="opaque", colour="dark_grey"),
  "Thick enough to hide the sun completely, with no halo."),

 ("Stratocumulus", "stratiformis", dict(shape="rolls", sky_cover="most_of_sky"),
  "A layer of large shaded rolls or slabs with blue gaps between them."),
 ("Stratocumulus", "lenticularis", dict(shape="lens", sky_cover="isolated"),
  "Lens-shaped low cloud, smooth-edged and slow to change."),
 ("Stratocumulus", "castellanus",  dict(shape="tufts", turrets=True),
  "Turrets rising from a low layer."),
 ("Stratocumulus", "floccus",      dict(shape="tufts", base="ragged", precipitation="virga"),
  "Isolated tufts with domed tops and ragged bases."),
 ("Stratocumulus", "volutus",      dict(shape="roll_cloud", sky_cover="isolated"),
  "A long low tube rolling along ahead of a squall."),

 ("Stratus", "nebulosus", dict(shape="sheet"),
  "A featureless grey layer -- fog that has not reached the ground."),
 ("Stratus", "fractus",   dict(shape="ragged", base="ragged", sky_cover="patches"),
  "Torn grey shreds, usually under the base of raining cloud."),

 ("Cumulus", "humilis",   dict(shape="heaped", sky_cover="isolated", depth="flat"),
  "Fair-weather cumulus: wider than they are tall, flat-based, no growth."),
 ("Cumulus", "mediocris", dict(shape="heaped", depth="square", top="cauliflower"),
  "About as tall as they are wide, with sprouting tops."),
 ("Cumulus", "congestus", dict(shape="towering", colour="grey",
                              precipitation="showers", depth="tall", top="cauliflower"),
  "Distinctly taller than wide, a hard cauliflower top still sharp-edged."),
 ("Cumulus", "fractus",   dict(shape="ragged", base="ragged", colour="grey",
                              depth="none", top="none"),
  "Ragged shreds with no flat base, forming or breaking up."),

 ("Cumulonimbus", "calvus",     dict(shape="towering", top="smooth"),
  "The top has lost its cauliflower edge and gone smooth, but has not spread out yet."),
 ("Cumulonimbus", "capillatus", dict(shape="towering", anvil=True, precipitation="thunder",
                                    depth="tall", mamma=True, top="fibrous"),
  "The top has gone fibrous and spread into an anvil. The full thunderstorm."),

 ("Nimbostratus", None, dict(),
  "A thick dark layer raining or snowing steadily, its base lost in the murk."),
]

BOOLS = ("shading", "halo", "anvil", "mamma", "hooks", "turrets")

# Roughly how often you meet one, looking up from mid-latitudes. Without this
# the engine spends questions separating a Stratocumulus volutus from a
# Cirrocumulus castellanus, neither of which you will see this year, at the
# expense of the half-dozen you see most weeks.
PRIOR = {
 "Cumulus humilis": 10, "Stratocumulus stratiformis": 10, "Cirrus fibratus": 9,
 "Cumulus mediocris": 8, "Altocumulus stratiformis": 8, "Stratus nebulosus": 7,
 "Cirrostratus nebulosus": 6, "Altostratus translucidus": 6,
 "Cumulus congestus": 5, "Cirrus uncinus": 5, "Nimbostratus": 5,
 "Altostratus opacus": 4, "Cumulonimbus capillatus": 4, "Cumulus fractus": 4,
 "Stratus fractus": 4, "Cirrocumulus stratiformis": 3, "Cirrus spissatus": 3,
 "Cirrostratus fibratus": 3, "Altocumulus castellanus": 3,
 "Cumulonimbus calvus": 2, "Altocumulus lenticularis": 2,
 "Stratocumulus castellanus": 2, "Cirrus castellanus": 1, "Cirrus floccus": 1,
 "Cirrocumulus lenticularis": 1, "Cirrocumulus floccus": 1,
 "Cirrocumulus castellanus": 1, "Altocumulus floccus": 1,
 "Altocumulus volutus": 1, "Stratocumulus lenticularis": 1,
 "Stratocumulus floccus": 1, "Stratocumulus volutus": 1,
}

def entity(genus, species, over, note):
    e = dict(GENERA[genus])
    e.update(over)
    name = f"{genus} {species}" if species else genus
    out = {"name": name}
    for k, v in e.items():
        if k in BOOLS:
            if v:
                out[k] = True          # false is the absent default
        else:
            out[k] = v
    out["_prior"] = PRIOR.get(name, 1)
    out["_note"] = note
    return out

def pictures():
    """Commons file per cloud, with the author and licence Commons records."""
    here = os.path.dirname(__file__)
    try:
        picked = json.load(open(os.path.join(here, "cloud-picked.json")))
        credits = json.load(open(os.path.join(here, "cloud-credits.json")))
    except FileNotFoundError:
        print("no pictures yet: run cloud_images.py, cloud_pick.py, then credits")
        return {}
    out = {}
    for name, title in picked.items():
        f = title[5:]
        cr = credits.get(f)
        if not cr or not (cr["credit"] and cr["license"]):
            continue          # a picture we cannot attribute is not usable
        out[name] = {
            "url": "https://commons.wikimedia.org/wiki/Special:FilePath/"
                   + urllib.parse.quote(f.replace(" ", "_")),
            "source": cr["source_page"],
            "credit": cr["credit"],
            "license": cr["license"],
        }
    return out

def main():
    ents = [entity(g, s, o, n) for g, s, o, n in SPECIES]
    pics = pictures()
    for e in ents:
        if e["name"] in pics:
            e["_image"] = pics[e["name"]]
    missing = [e["name"] for e in ents if e["name"] not in PRIOR]
    if missing:
        print("no prior for:", missing)
    attrs = json.load(open(os.path.join(os.path.dirname(__file__),
                                        "cloud-attributes.json")))
    kb = {"name": attrs["name"], "attributes": attrs["attributes"], "entities": ents}
    out = os.path.join(os.path.dirname(__file__), "..", "data", "clouds.json")
    json.dump(kb, open(out, "w"), indent=1, ensure_ascii=False)
    print(f"data/clouds.json: {len(ents)} entities, {len(attrs['attributes'])} attributes, "
          f"{sum(1 for e in ents if '_image' in e)} with an attributed picture")

main()

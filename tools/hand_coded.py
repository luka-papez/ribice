"""Values read off the standards by hand, where the patterns could not.

Keyed by entity name, so a split variety can differ from its parent.  Each
entry is the carriage the standard describes, quoted in the comment so the
judgement can be checked against the source rather than taken on trust.
Two values mean the standard describes both -- typically one at rest and one
in movement -- and the KB reads that as "any of these".
"""

TAIL = {
 # carried level with the back
 "English Pointer":                  ["level"],   # "carried on a level with back, no upward curl"
 "Cavalier King Charles Spaniel":    ["level"],   # "never much above the level of the back"
 "Bouvier des Flandres":             ["level"],   # "must continue the line of the backbone"

 # curled or carried right over the back
 "Basenji":                          ["over_back"],  # "curls tightly over spine"
 "Maltese":                          ["over_back"],  # "single big curve... touching the croup"
 "Chow Chow":                        ["over_back"],  # "carried well over back"
 "Lhasa Apso":                       ["over_back"],  # "carried well over back"
 "Japanese Spitz":                   ["over_back"],  # "carried over back"
 "Alaskan Malamute":                 ["over_back"],  # "carried over the back when not working"

 # carried up, but not over the back
 "Scottish Terrier":                 ["high"],    # "upright carriage or slight bend"
 "West Highland White Terrier":      ["high"],    # "carried jauntily, not... over back"
 "Yorkshire Terrier":                ["high"],    # "a little higher than level of back"
 "Boxer":                            ["high"],    # "set on high rather than low... left natural"
 "Bichon Frisé":                     ["high"],    # "carried raised and gracefully curved"
 "Poodle (Standard)":                ["high"],    # "carried at 'ten past nine' to the topline"
 "Poodle (Medium)":                  ["high"],
 "Poodle (Miniature)":               ["high"],
 "Chinese Crested Dog (Hairless)":   ["high"],    # "set high, carried up or out in motion"
 "Chinese Crested Dog (Powder Puff)":["high"],

 # hanging
 "Staffordshire Bull Terrier":       ["low"],     # "low set... carried rather low"
 "Scottish Deerhound":               ["low"],     # "never lifted above line of back"
 "American Staffordshire Terrier":   ["low"],     # "low set... not carried over back"
 "Australian Cattle Dog":            ["low"],     # "at rest it should hang in a very slight curve"
 "Australian Kelpie":                ["low"],     # "during rest should hang in a very slight curve"

 # the standard describes both a resting and a moving carriage
 "Whippet":                          ["low", "level"],  # hangs; in action "not higher than the back"
 "German Shepherd (Normal coat)":    ["low", "level"],  # "hanging downward... raised but not beyond the horizontal"
 "German Shepherd (Long coat)":      ["low", "level"],
 "Xoloitzcuintle (Standard)":        ["high", "low"],   # "carried up in a curve... when resting it should hang"
 "Xoloitzcuintle (Intermediate)":    ["high", "low"],
 "Xoloitzcuintle (Miniature)":       ["high", "low"],
 "Nova Scotia Duck Tolling Retriever": ["low", "high"], # "below the level of the back except when alert"
 "Jack Russell Terrier":             ["low", "high"],   # "may droop at rest... moving should be erect"
 "Tornjak":                          ["low", "high"],   # "relaxed it is hanging... in movement raised"

 "Border Collie":                    ["low"],     # "set on low... never carried over back"

 # a genuinely short tail
 "Boston Terrier":                   ["short"],   # "short... not exceeding a quarter of the way to the hock"
}


# Standards call a stand-off guard coat "harsh", which is a judge's word for
# how it holds its shape, not how it reads to anyone looking at the dog. A
# Samoyed is not bristly. These are the breeds whose coat reads as thick and
# dense, standing out from the body -- the spitz and northern look, the
# mountain guardians, and the heavily coated herders.
THICK = [
 "Samoyed", "Siberian Husky", "Alaskan Malamute", "Akita", "Shiba Inu",
 "Japanese Spitz", "Korean Jindo Dog", "Norwegian Elkhound", "Chow Chow",
 "German Spitz (Wolfspitz (Keeshond))", "German Spitz (Giant)",
 "German Spitz (Medium)", "German Spitz (Pomeranian)",
 "Czechoslovakian Wolfdog", "Pyrenean Mountain Dog", "Caucasian Shepherd Dog",
 "Central Asian Shepherd Dog", "Tibetan Mastiff", "Abruzzo-Maremma Sheepdog",
 "Leonberger", "Newfoundland Dog", "St. Bernard (Long-haired)", "Tornjak",
 "Bernese Mountain Dog", "Rough Collie", "Shetland Sheepdog",
 "Australian Shepherd", "Miniature American Shepherd",
 "German Shepherd (Long coat)", "Belgian Shepherd (Groenendael)",
 "Belgian Shepherd (Tervueren)",
]

COAT_TEXTURE = {
 "Great Dane": ["smooth"],   # "very short and dense, sleek-looking, glossy"
 "Dogue de Bordeaux":  ["smooth"],   # "fine, short and soft to the touch"
 "Neapolitan Mastiff": ["smooth"],   # "short, rough, hard, dense, of the same length"
}
COAT_TEXTURE.update({n: ["thick"] for n in THICK})

# A thick coat makes a dog read as broader and more solid than its frame is.
# Judging build from the outline of a Samoyed gets you "sturdy", not "athletic".
BUILD = {
 "Samoyed": ["sturdy"], "Siberian Husky": ["sturdy"],
 "Alaskan Malamute": ["sturdy"], "Akita": ["sturdy"], "Shiba Inu": ["sturdy"],
 "Japanese Spitz": ["sturdy"], "Norwegian Elkhound": ["sturdy"],
 "German Spitz (Wolfspitz (Keeshond))": ["sturdy"],
 "German Spitz (Giant)": ["sturdy"], "German Spitz (Medium)": ["sturdy"],
 "German Spitz (Pomeranian)": ["sturdy"],
 "Pyrenean Mountain Dog": ["massive"], "Caucasian Shepherd Dog": ["massive"],
 "Central Asian Shepherd Dog": ["massive"], "Tibetan Mastiff": ["massive"],
 "Abruzzo-Maremma Sheepdog": ["massive"], "Tornjak": ["massive"],
}

# Ideal 57 cm for males and 53 cm for females, either side of the line between
# the two bands: both answers are right depending on the dog.
SIZE = {
 "Samoyed": ["medium", "large"],        # ideal 57 cm males, 53 cm females
 "Siberian Husky": ["medium", "large"], # 50.5-60 cm, either side of the line
}

COAT_LENGTH = {
 "Istrian Coarse-haired Hound": ["medium"],  # "about 5-8 cms long"
 "Istrian Shorthaired Hound":   ["short"],   # "length is around 1-2 cm"
 "Tornjak":                     ["long"],    # "long, thick, abundant"
 "Siberian Husky":              ["medium"],  # "medium length, giving a well furred appearance"
 "Alaskan Malamute":            ["medium"],  # "thick, coarse guard coat, never long and soft"
}

MARKINGS = {
 # white ground with coloured patches: the KB's particolour value, not "solid"
 "Istrian Coarse-haired Hound": ["white_markings"],  # "snow-white with orange markings"
 "Istrian Shorthaired Hound":   ["white_markings"],
 "Posavac Hound":               ["white_markings"],
 "Border Collie":               ["white_markings", "merle", "tricolour"],
 "Yorkshire Terrier":           ["tan_points"],
 "Shar Pei":                    ["solid"],
 "Czechoslovakian Wolfdog":     ["mask"],
 "German Shepherd (Normal coat)": ["saddle", "mask", "tan_points", "solid"],
 "German Shepherd (Long coat)":   ["saddle", "mask", "tan_points", "solid"],
 "Dogue de Bordeaux":           ["mask"],
 "Neapolitan Mastiff":          ["solid", "brindle"],
}

ALL = ["black", "white", "cream", "gold", "red", "fawn", "brown",
       "grey", "blue", "silver"]

def all_but(*drop):
    return [c for c in ALL if c not in drop]

COLOUR = {
 # A breed coded with every colour matches every colour answer and so loses
 # its own identity.  Where a standard genuinely permits anything -- Shih Tzu,
 # Afghan, Saluki, Chihuahua -- that is true of the dog and is left alone.
 # These were swept in by the "all colours" rule but do not belong to it.
 "Border Collie":       ["black", "white", "red", "brown", "blue"],
 "English Cocker Spaniel": ["black", "red", "gold", "brown", "white", "cream"],
     # "Black; red; golden; liver... black and white, orange and white, lemon and white"
 "Shetland Sheepdog":   ["gold", "red", "black", "white", "blue", "grey"],
     # sable "pale gold to deep mahogany", tricolour, "blue merle: clear silvery blue"
 "Brittany":            ["white", "gold", "red", "black", "brown"],
     # "white and orange, white and black, white and liver"
 "Tibetan Mastiff":     ["black", "blue", "gold", "red", "fawn"],
     # "rich black... blue... gold, from rich fawn to deep red, sable"
 "Xoloitzcuintle (Standard)":     ["black", "grey", "red", "brown", "gold"],
 "Xoloitzcuintle (Intermediate)": ["black", "grey", "red", "brown", "gold"],
 "Xoloitzcuintle (Miniature)":    ["black", "grey", "red", "brown", "gold"],
     # "black, blackish grey, slate grey, dark grey, reddish, liver, bronze or blond"

 # genuinely any colour, with the one exclusion the standard names
 "Pekingese":           all_but("brown"),   # "except albino or liver"
 "Tibetan Terrier":     all_but("brown"),   # "any colour except chocolate, liver or merle"
 "Shar Pei":            all_but("white"),   # "all solid colours acceptable except white"
 "Central Asian Shepherd Dog": all_but("blue", "brown"),
     # "any, except genetic blue and genetic brown"

 # the clause filter discarded every sentence, leaving nothing
 "Yorkshire Terrier":   ["blue", "gold"],
     # "dark steel blue... hair on chest rich, bright tan"
 "Czechoslovakian Wolfdog": ["grey", "silver", "gold"],
     # "yellowish-gray to silver-gray with a characteristic light mask"

 # this standard has no section structure the extractor can read at all
 "German Shepherd (Normal coat)": ["black", "red", "gold", "grey", "brown"],
 "German Shepherd (Long coat)":   ["black", "red", "gold", "grey", "brown"],

 # this colour section begins with the eye colour, which is not the coat
 "Dogue de Bordeaux":   ["fawn", "red", "brown"],
     # "self-coloured, in all shades of fawn"
 "Neapolitan Mastiff":  ["grey", "blue", "black", "brown", "fawn"],
     # "preferred colours are grey, lead grey and black, but also brown, fawn"
     # "black with reddish-brown, brown and yellow to light grey markings;
     #  single-coloured black, grey with darker shading, black saddle and mask"
}

EARS = {
 # "tipped ears and drooping ears are faulty" is not a description of the breed
 "German Shepherd (Normal coat)": ["erect"],
 "German Shepherd (Long coat)":   ["erect"],
 "Alaskan Malamute":              ["erect"],  # folded back only when working

 # folded forward over the ear opening -- the terrier button ear.  Standards
 # describe it by where the fold sits rather than by naming it.
 "Airedale Terrier":            ["button"],
     # "V-shaped with a side carriage... top line of folded ear slightly above skull"
 "Soft-coated Wheaten Terrier": ["button"],
     # "carried in front, level with skull"; rose ears objectionable
 "Shar Pei":                    ["button"],
     # "very small... tips pointing towards eyes, set well forward over eyes"

 # folded back on itself against the neck
 "Italian Greyhound":           ["rose"],
     # "folded in itself and carried well back on the nape"

 # carried up
 "Chow Chow":                   ["erect"],
     # "carried stiffly and wide apart but tilting well forward over eyes"
 "Norwegian Elkhound":          ["erect"],
     # "set on high, firm and upstanding... pointed"

 # hanging flat against the side of the head
 "Newfoundland Dog":            ["dropped"],  # "well set back on the side of the head and close lying"
 "Brittany":                    ["dropped"],  # "set high, triangular, relatively large and rather short"
 "German Wirehaired Pointer":   ["dropped"],  # "medium size, set on high and wide"
 "Weimaraner (Short-haired)":   ["dropped"],  # "broad and fairly long, just reaching to corner of mouth"
 "Weimaraner (Long-haired)":    ["dropped"],
 "Golden Retriever":            ["dropped"],  # "moderate size, set on approximate level with eyes"
 "Flat-Coated Retriever":       ["dropped"],  # "small and well set on, close to side of head"
 "Cavalier King Charles Spaniel": ["dropped"],# "long, set high, with plenty of feather"
 "Dalmatian":                   ["dropped"],  # "carried close to the lateral part of the head"
 "Afghan Hound":                ["dropped"],  # "set low and well back, carried close to head"
 "Havanese":                    ["dropped"],  # "they fall along the cheeks forming a discreet fold"
 "English Mastiff":             ["dropped"],  # "lying flat and close to cheeks when in repose"
 "Kooikerhondje":               ["dropped"],  # "carried close to the cheeks without a fold"

 # "in repose thrown back, on alert brought forward and carried semi-erect"
 # describes one carriage in two states, not a button ear
 "Rough Collie":                ["semi_erect"],
 "Smooth Collie":               ["semi_erect"],
 "Shetland Sheepdog":           ["semi_erect"],
}

TABLES = {"tail": TAIL, "coat_length": COAT_LENGTH, "markings": MARKINGS,
          "colour": COLOUR, "ears": EARS, "coat_texture": COAT_TEXTURE,
          "build": BUILD, "size": SIZE}

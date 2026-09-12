"""Attributes the standards' prose will not yield to pattern matching.

The survey showed keyword matching finds 5% of flat-faced breeds where the true
figure is nearer 20% -- standards describe a muzzle as "short" in a dozen ways
and never use a word like brachycephalic.  These are named explicitly instead.
Names are resolved against the extract, so a name that is wrong is reported
rather than silently ignored.
"""

FLAT_FACED = [
 "Pug", "Bulldog", "French Bulldog", "Pekingese", "Shih Tzu", "Japanese Chin",
 "Boston Terrier", "Griffon Bruxellois", "Griffon Belge", "Petit Brabancon",
 "Affenpinscher", "Boxer", "Bullmastiff", "Dogue de Bordeaux",
 "Neapolitan Mastiff", "English Mastiff", "Shar Pei", "King Charles Spaniel",
 "Cane Corso", "Tibetan Spaniel",
]
SHORT_MUZZLE = [
 "Cavalier King Charles Spaniel", "Chow Chow", "Lhasa Apso", "Tibetan Terrier",
 "Bichon Frise", "Havanese", "Bolognese", "Maltese", "Chihuahua",
 "Staffordshire Bull Terrier", "American Staffordshire Terrier", "Rottweiler",
 "Leonberger", "Newfoundland Dog", "St. Bernard",
 "Central Asian Shepherd Dog", "Caucasian Shepherd Dog", "Tibetan Mastiff",
 "Pyrenean Mastiff", "Spanish Mastiff", "Estrela Mountain Dog",
]
LONG_MUZZLE = [
 "Rough Collie", "Smooth Collie", "Borzoi", "Saluki", "Afghan Hound",
 "Greyhound", "Whippet", "Italian Greyhound", "Sloughi", "Azawakh",
 "Irish Wolfhound", "Scottish Deerhound", "Ibizan Hound", "Pharaoh Hound",
 "Bull Terrier", "Miniature Bull Terrier", "Dobermann",
 "Bedlington Terrier", "Manchester Terrier", "German Shepherd",
 "Belgian Shepherd", "Smooth Fox Terrier", "Wire Fox Terrier", "Airedale Terrier", "Bloodhound",
]
SHORT_LEGS = [
 "Dachshund", "Basset Hound", "Fawn Brittany Basset",
 "Basset Bleu de Gascogne", "Basset Artesien Normand",
 "Large Vendeen Griffon Basset", "Petit Basset Griffon Vendeen",
 "Cardigan Welsh Corgi", "Pembroke Welsh Corgi", "Skye Terrier",
 "Sealyham Terrier", "Dandie Dinmont Terrier", "Glen of Imaal Terrier",
 "Swedish Vallhund", "Drever", "Westphalian Dachsbracke",
 "Alpine Dachsbracke", "Schweizerischer Niederlaufhund", "Cesky Terrier",
 "Sussex Spaniel", "Clumber Spaniel", "Pekingese", "Lancashire Heeler",
]
WRINKLED = [
 "Shar Pei", "Bulldog", "Pug", "Bloodhound", "English Mastiff", "Neapolitan Mastiff",
 "Bullmastiff", "Dogue de Bordeaux", "Basset Hound", "Pekingese",
 "Spanish Mastiff", "Pyrenean Mastiff", "Fila Brasileiro", "Cane Corso",
 "Bloodhound", "St. Bernard", "Rafeiro do Alentejo",
]
BEARD = [
 "Standard Schnauzer", "Giant Schnauzer", "Miniature Schnauzer",
 "Affenpinscher", "Airedale Terrier", "Scottish Terrier", "Cesky Terrier",
 "West Highland White Terrier", "Cairn Terrier", "Norfolk Terrier",
 "Norwich Terrier", "Border Terrier", "Lakeland Terrier", "Welsh Terrier",
 "Irish Terrier", "Wire Fox Terrier", "Sealyham Terrier", "Dandie Dinmont Terrier",
 "Bearded Collie", "Otterhound", "Bouvier des Flandres", "Bouvier des Ardennes",
 "Briard", "Polish Lowland Sheepdog", "Schapendoes", "Pumi", "Mudi",
 "Griffon Bruxellois", "Griffon Belge", "Petit Brabancon",
 "German Wirehaired Pointer", "Wirehaired Pointing Griffon", "Spinone Italiano",
 "Slovakian Rough-haired Pointer", "Cesky Fousek",
 "Ibizan Hound", "Irish Wolfhound", "Scottish Deerhound",
 "Blue Gascony Griffon", "Fawn Brittany Basset", "Grand Griffon Vendeen",
 "Briquet Griffon Vendeen", "Large Vendeen Griffon Basset",
 "Petit Basset Griffon Vendeen", "Styrian Coarse-haired Hound",
 "Tibetan Terrier", "Lhasa Apso", "Shih Tzu", "Havanese", "Bichon Frise",
]
BLUE_TONGUE = ["Chow Chow", "Shar Pei"]
DOUBLE_DEWCLAWS = [
 "Beauceron", "Briard", "Pyrenean Mountain Dog", "Pyrenean Shepherd (smooth-faced)", "Pyrenean Shepherd (long-haired)",
 "Icelandic Sheepdog", "Estrela Mountain Dog", "Catalan Sheepdog",
 "Anatolian Shepherd",
]
# build, where the standard's prose is too varied to classify
RACY = [
 "Greyhound", "Whippet", "Italian Greyhound", "Borzoi", "Saluki", "Sloughi",
 "Azawakh", "Afghan Hound", "Irish Wolfhound", "Scottish Deerhound",
 "Magyar Agar", "Galgo Espanol", "Polish Greyhound", "Pharaoh Hound",
 "Ibizan Hound", "Cirneco dell'Etna", "Basenji", "Podenco Canario",
 "Manchester Terrier", "Miniature Pinscher",
 "Bedlington Terrier", "Xoloitzcuintle", "Peruvian Hairless Dog",
]
MASSIVE = [
 "Great Dane", "English Mastiff", "Neapolitan Mastiff", "Bullmastiff", "Dogue de Bordeaux",
 "St. Bernard", "Newfoundland Dog", "Leonberger",
 "Spanish Mastiff", "Pyrenean Mastiff", "Tibetan Mastiff", "Tosa",
 "Fila Brasileiro", "Caucasian Shepherd Dog", "Central Asian Shepherd Dog",
 "Bulldog", "Shar Pei", "Rottweiler", "Cane Corso", "Bernese Mountain Dog",
 "Greater Swiss Mountain Dog", "Rafeiro do Alentejo",
 "Anatolian Shepherd", "Pyrenean Mountain Dog", "Landseer",
]
STURDY = [
 "Bulldog", "Pug", "Boston Terrier", "French Bulldog", "Staffordshire Bull Terrier",
 "Scottish Terrier", "Sealyham Terrier", "Cesky Terrier", "Dandie Dinmont Terrier",
 "Cardigan Welsh Corgi", "Pembroke Welsh Corgi", "Norwegian Buhund",
 "Chow Chow", "Clumber Spaniel", "Sussex Spaniel", "Basset Hound",
 "Glen of Imaal Terrier", "Shih Tzu", "Lhasa Apso", "Tibetan Spaniel",
]

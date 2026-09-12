# tools

How `../data/dogs.json` was built. Nothing here is needed to *run* ribice — the
engine reads the finished JSON — but every value in that knowledge base came
out of these scripts, and this is how to check one or rebuild the lot.

Python 3, standard library only, plus `pdftotext` (poppler-utils) for reading
the PDFs. Run them from this directory; they read and write files beside
themselves.

## What is not here

**The FCI standards themselves.** `fetch_pdfs.py` caches ~360 PDFs into `pdf/`
and the extraction step writes their text into `fci-sections.json` and
`fci-extract.json`. Those are FCI's documents, not ours to redistribute, so all
three are ignored by git. Facts read out of them — that a Border Collie carries
its ears erect or semi-erect — are not copyrightable and are what the knowledge
base contains. Sentences are not. Nothing in `data/dogs.json` reproduces the
standards' prose.

Re-running the pipeline downloads them again, politely: serial, 1.5 s apart,
with an identifying User-Agent. It takes about ten minutes.

## The pipeline

| Step | Script | What it does |
| ---- | ------ | ------------ |
| 1 | `fci_index.py` | Breed, number, group, section and standard URL, from the ten FCI group pages and the provisional list. 364 breeds. |
| 2 | `fetch_pdfs.py` | Caches each English standard into `pdf/`. |
| 3 | `wikidata.py` | Three SPARQL queries for the CC0 facts: names, aliases, origin, pictures, heights. |
| 4 | `extract_sections.py` | Splits each standard on its headings into `ears`, `tail`, `coat_hair`, `coat_colour`, … |
| 5 | `heights.py` | Height at withers and weight, in cm and kg. |
| 6 | `rebuild_spine.py` | Joins the FCI index to Wikidata — by FCI number, then by name. |
| 7 | `assemble.py` | Merges the three sources into `fci-extract.json`. |
| 8 | `pageviews.py` | Twelve months of English Wikipedia views per breed. |
| 9 | `select.py` | Picks the breeds the guide covers. Writes `selection.json`. |
| 10 | `credits.py` | Author and licence for every Commons picture. |
| 11 | `code_kb.py` | Codes the attribute values. |
| 12 | `build_kb.py` | Writes `../data/dogs.json`. |

`survey.py` is not part of the build. It counts how often each candidate
attribute value appears across the corpus, and is how the attribute list was
chosen: values nobody uses were dropped, and `ears` was split into carriage
rather than shape because carriage is what the standards actually distinguish.

After any change, from the repository root:

```
go run ./cmd/ribice -kb data/dogs.json -lint
go run ./cmd/ribice -kb data/dogs.json -simulate
```

## Where the values come from

Three kinds, in descending order of how much you should trust them.

**Measured.** `size` comes from the height or weight the standard states, parsed
mechanically. It is cross-checked against Wikidata's `P2048`, which played no
part in deriving it: of 314 breeds with a figure from both, 313 fall inside the
parsed range and none disagree.

**Matched.** `ears`, `tail`, `coat_length`, `coat_texture`, `colour` and
`markings` are read out of the relevant section by the patterns in
`code_kb.py`, which are listed there in full so a reader can disagree with one.
This is the weakest link: a standard says "neither harsh nor woolly" and
"black mask" on a fawn dog, and a pattern that reads those as assertions gets
them backwards. `assertive()` drops negated clauses — a negation governs the
rest of its sentence — and `ELSEWHERE` drops clauses about eyes, nose and nails.
Both help; neither is sufficient.

**Named.** `muzzle`, `build` and the six booleans have no reliable textual
signal — the standards describe a short muzzle a dozen ways and never use a
word like brachycephalic — so the breeds are listed by name in `curated.py`.
`hand_coded.py` holds individual corrections where the patterns were wrong or
silent, each with the phrase it was read from, so the judgement can be checked
against the source rather than taken on trust.

Both files resolve their names against the extract and report any that match no
breed. That check is worth keeping: it caught 29 wrong names out of 240 on the
first run — "Mastiff" is *English Mastiff* here, "Grand Basset Griffon Vendéen"
is *Large Vendeen Griffon Basset*, and Boerboel is not FCI-recognised at all.

## Two things that are decisions, not data

`varieties.py` splits the FCI breeds that cover several visibly different dogs.
One Poodle entry spans toy to large; Belgian Shepherd is four dogs. Colour
varieties are left alone, because colour is already an attribute. Where two
varieties differ by a few centimetres and nothing a person could answer —
Kleinspitz and Zwergspitz, the Rabbit and Miniature Dachshund — they are merged
back, because splitting them only produced a pair no question can separate.

`select.py` chooses which breeds the guide covers, ranked by how often people
look the breed up rather than by anyone's sense of importance. `selection.json`
pins the result, so rebuilding does not silently change the breed list as the
view counts drift. Croatian breeds are kept whatever their ranking.

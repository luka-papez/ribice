# tools/calib

The calibration loop from [`specs/calibration-loop.md`](../../specs/calibration-loop.md):
measure how people answer a quiz from photos, then tune the knowledge base
against the measurements. Built so far: the photo corpus (phase 1a).

```
python3 -m venv tools/calib/.venv
tools/calib/.venv/bin/pip install -r tools/calib/requirements.txt
```

`corpus.py` needs only Pillow. Scripts that ask Claude go through the Claude
Code CLI by default (`--via cli`), which runs on a Claude subscription: no API
key needed, and they stop starting calls once the subscription's five-hour
window is 80% used, so a long run leaves you room. `--via batch` or
`--via api` use the `anthropic` SDK and `ANTHROPIC_API_KEY` instead.
`claude_cli.py` says how the CLI call is locked down: our system prompt in
place of Claude Code's, no tools, no settings or MCP servers, and an empty
working directory.

## The corpus

```
python tools/calib/corpus.py --kb data/clouds.json          # free; about 10 minutes
python tools/calib/label_check.py --kb data/clouds.json     # ~15 min, ~30% of a 5-hour window
# decide each doubt in corpus/clouds/review.json, then:
python tools/calib/corpus.py --kb data/clouds.json          # refills what was dropped
python tools/calib/label_check.py --kb data/clouds.json     # checks only the new photos
```

`corpus.py` takes up to 8 photos per cloud from its Wikimedia Commons species
category, 5 to tune on and 3 held out. It reads both `<Name> clouds` and
`<Name>`, which Commons uses about equally, and a cloud with fewer files than
it wants also draws on the genus category, by species word in the file name.
A cloud still short after that falls back to a full-text search of Commons.
Those hits rank after every category photo and are marked `search:` in the
manifest; most fail the label check, so treat them as candidates only.
It skips:

- anything that isn't a JPEG, PNG or WebP photo, or whose name says diagram,
  satellite, painting, render and the like;
- files whose names mention a second cloud;
- files filed under two clouds' categories (whoever filed them saw both);
- the photo the quiz itself shows;
- photos under 800 px on the short side.

It spreads the picks across photographers. Downloads go to `cache/photos/`,
rotated upright, converted to sRGB, stripped of all metadata and scaled to
1568 px on the long edge. The answerer never sees a file name or caption.

**The manifest pins everything.** Photos already in `corpus/<kb>/manifest.json`
keep their place and split on every later run; a run only fills clouds that
are short. Delete the manifest to start over, but never after answers have
been recorded against it: a held-out photo that becomes a tuning photo leaks.

`label_check.py` shows Claude each photo *with* its label and asks whether it
is a ground-level photo of mainly one cloud, of that type. Photos it thinks
are another type, mixed skies and photos not taken from the ground go into
`review.json` with `"decision": null`; set each to `"keep"` or `"drop"`. An
"unsure" is recorded but not listed: arguable species calls are what the quiz
is tested on, not labelling errors. Since
this pass sees the label, it only nominates photos for review and never feeds
a score.

| File | Committed | Holds |
| --- | --- | --- |
| `corpus/<kb>/manifest.json` | yes | file, label, split, credit and licence per photo |
| `corpus/<kb>/label-check.json` | yes | Claude's verdict per photo |
| `corpus/<kb>/review.json` | yes | doubted photos and a person's keep/drop decisions |
| `cache/` | no | downloaded photos, an in-flight batch id |

Photos are used locally and never published. The credits are kept in case one
is ever promoted into the quiz.

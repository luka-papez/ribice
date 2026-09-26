# tools/calib

The calibration loop from [`specs/calibration-loop.md`](../../specs/calibration-loop.md):
measure how people answer a quiz from photos, then tune the knowledge base
against the measurements. Built so far: the photo corpus (phase 1a), the
baseline measurement (phase 1b) and calibration (phase 1c).

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
| `corpus/<kb>/human.json` | yes | a person's answers to the 10-photo page |
| `runs/<date>-<label>/` | yes | the scored knowledge base, config, answers file, games and report |
| `cache/` | no | downloaded photos, cached answers, names, the engine binary, the 10-photo page |

Photos are used locally and never published. The credits are kept in case one
is ever promoted into the quiz.

## The baseline

```
python tools/calib/run.py --kb data/clouds.json --label baseline     # ask, replay, report
python tools/calib/name_test.py --kb data/clouds.json                # leakage: can it name them?
python tools/calib/human_page.py --kb data/clouds.json               # your 10 photos (deferred for now)
```

`ask.py` is the answerer: Claude sees one photo, never its label or file
name, and answers the quiz's questions as a person without training would, or
says it can't tell and why. Answers are cached in `cache/answers.sqlite` by
photo, question, option labels, prompt, model and mode, so rewording one
question re-asks only that one. `--mode single` asks one question per call,
as the spec has it; `--mode photo` asks all of a photo's questions in one
call, 15 times cheaper, if a pilot shows its answers hold up.

`replay.py` turns the cache into the answers file and plays one game per photo
through `ribice -replay`. `report.py` measures the games and the whole answer
matrix: accuracy on held-out photos (also on label-check-passed photos only,
and on clouds with 4+ photos), then per question agreement with the knowledge
base, skip causes, mix-up tables and bits removed per ask, and per cloud
accuracy. `run.py` does all three and writes `runs/<date>-<label>/`, which is
committed. A run that stops at the window limit still reports, marking itself
incomplete; the same command later fills the gaps.

Two checks say whether to trust the answerer. `name_test.py` asks, in a
separate call, what each photo shows; the report then compares agreement on
photos it named right and wrong. `human_page.py` writes a local page with 10
photos and the same questions; save its download as `corpus/<kb>/human.json`
and the report compares your answers with Claude's, question by question.
The human comparison is deferred for now (see the spec); the name test is
the check that runs.

## Calibration

`run.py` also calibrates. `calibrate.py` rewrites each question's `noise`,
`confusion`, `confusable` and `cost` from the tune photos' answers, by the
spec's rules: look-alike pairs from repeated mix-ups, rates blended with the
current values as 10 observations, cost from skip rate and low confidence. It
writes `runs/<date>-<label>/<kb>.calibrated.json` and a table of what changed.

`compare.py` then plays the current and calibrated knowledge bases on the
same held-out photos from the same answers, and accepts the candidate when
accuracy minus 5 points per question improves, at most 2 games go from right
to wrong, and it still passes `-lint` and identifies as many entities under
`-simulate`. The verdict goes in the report. It works for any two knowledge
bases, so phase 2's edits are judged the same way:

```
python tools/calib/compare.py data/clouds.json tools/calib/runs/<run>/clouds.calibrated.json
```

A candidate that rewords a question has no answers for it yet; `compare.py`
refuses to accept it until `ask.py` has been run on it.

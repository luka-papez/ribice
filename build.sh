#!/bin/sh
# Builds the browser version of the quiz.
#
#   ./build.sh                 build into web/
#   ./build.sh serve           build, then serve web/ on http://localhost:8080
#   ./build.sh publish [DIR]   build, then copy the embeddable assets to DIR
#                              (default dist/) with a README for whoever
#                              embeds them
#   ./build.sh gif             rebuild docs/quiz.gif, the README demo
#
# Only index.html, ribice.js and ribice.css are written by hand; everything else
# is produced here and is not in version control.
set -e
cd "$(dirname "$0")"

# The files an embedding application needs. index.html is the demo page and is
# deliberately not among them.
ASSETS="ribice.js ribice.css ribice.wasm wasm_exec.js adriatic-fish.json"

size() { # human size, and what it costs over the wire
  printf '%8s  %7s gzipped  %s\n' \
    "$(du -h "$1" | cut -f1)" \
    "$(gzip -c "$1" | wc -c | awk '{printf "%.0fK", $1/1024}')" \
    "$(basename "$1")"
}

build() {
  # -trimpath keeps the developer's home directory out of a published asset.
  # -buildvcs=false drops the git revision and dirty flag Go stamps in by
  # default; without it the binary changes whenever the working tree moves at
  # all -- including edits that never reach the engine -- and every consumer
  # vendoring dist/ takes 3.2MB of churn for nothing. Together they leave the
  # binary byte-identical between rebuilds, so a committed dist/ only changes
  # when the code actually does.
  GOOS=js GOARCH=wasm go build -trimpath -buildvcs=false -ldflags="-s -w" -o web/ribice.wasm ./cmd/wasm
  cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/  # version-locked to your Go
  cp data/adriatic-fish.json web/                   # fetched at page load
}

# gif drives a scripted game in headless Chrome, one screenshot per step, and
# stitches the frames together. Needs Chrome and Python; it builds itself a
# virtualenv for Pillow if the system Python has not got it.
make_gif() {
  chrome=$(command -v google-chrome || command -v chromium || command -v chromium-browser) || {
    echo "gif: need google-chrome or chromium on PATH" >&2; exit 1; }
  py=python3
  if ! python3 -c "import PIL" 2>/dev/null; then
    [ -d .gif-venv ] || python3 -m venv .gif-venv
    .gif-venv/bin/pip install --quiet pillow
    py=.gif-venv/bin/python
  fi

  build
  frames=$(mktemp -d)
  trap 'rm -rf "$frames" web/_frames.html' EXIT

  # The demo page, stripped of the page chrome the README already provides.
  python3 - <<'PYEOF'
import re
s = open("web/index.html").read()
s = re.sub(r"  <header>.*?</header>\n|  <footer>.*?</footer>\n", "", s, flags=re.S)
driver = """<script type="module">
  import { createQuiz } from "./ribice.js";
  const quiz = await createQuiz({ mount: "#fish", settings: { maxQuestions: 20 } });
  const pick = i => () => quiz.root.querySelector(`.rb-opt[data-rb-index="${i}"]`)?.click();
  const next = () => () => quiz.root.querySelector(".rb-next")?.click();
  const script = [pick(0), next(), pick(0), next(), pick(0), next(),
                  pick(0), next(), pick(0), next()];
  const n = Number(new URLSearchParams(location.search).get("n") || 0);
  for (let i = 0; i < n && i < script.length; i++) script[i]();
  document.title = quiz.view.done ? "done" : "q" + (quiz.view.asked + 1);
</script>
</body>"""
open("web/_frames.html", "w").write(s.replace("</body>", driver))
PYEOF

  (cd web && python3 -m http.server 8799 >/dev/null 2>&1) &
  server=$!
  # `|| true`: without it set -e aborts the handler when the server is already
  # dead, and the temporary files survive.
  trap 'kill $server 2>/dev/null || true; rm -rf "$frames" web/_frames.html' EXIT
  sleep 2
  n=0
  while [ "$n" -le 10 ]; do
    "$chrome" --headless --disable-gpu --no-sandbox --hide-scrollbars       --virtual-time-budget=12000 --window-size=700,760       --screenshot="$frames/$(printf '%02d' "$n").png"       "http://127.0.0.1:8799/_frames.html?n=$n" 2>/dev/null
    n=$((n + 1))
  done
  kill $server 2>/dev/null || true

  mkdir -p docs
  FRAMES="$frames" "$py" - <<'PYEOF'
from PIL import Image, ImageChops
import os, glob

src = sorted(glob.glob(os.environ["FRAMES"] + "/*.png"))
# A beat to read the question, a shorter one for the click, a long final hold.
holds = [1000, 700, 900, 700, 900, 700, 900, 700, 900, 700, 3500][:len(src)]
ims = [Image.open(p).convert("RGB") for p in src]
bg = ims[0].getpixel((2, 2))

# Crop to the union of what is drawn, so no frame pads out to a height only the
# longest question needs.
box = None
for im in ims:
    b = ImageChops.difference(im, Image.new("RGB", im.size, bg)).getbbox()
    if b:
        box = b if box is None else (min(box[0], b[0]), min(box[1], b[1]),
                                     max(box[2], b[2]), max(box[3], b[3]))
pad = 14
box = (max(0, box[0] - pad), max(0, box[1] - pad),
       min(ims[0].width, box[2] + pad), min(ims[0].height, box[3] + pad))

W = 600
fs = []
for im in ims:
    c = im.crop(box)
    fs.append(c.resize((W, round(c.height * W / c.width)), Image.LANCZOS))
fs[0].save("docs/quiz.gif", save_all=True, append_images=fs[1:],
           duration=holds, loop=0, optimize=True, disposal=2)
print("docs/quiz.gif: %.0f KB, %d frames, %dx%d"
      % (os.path.getsize("docs/quiz.gif") / 1024, len(fs), *fs[0].size))
PYEOF
}

case "$1" in
serve)
  build
  size web/ribice.wasm
  echo "serving http://localhost:8080 -- ctrl-c to stop"
  cd web && exec python3 -m http.server 8080
  ;;

publish)
  out="${2:-dist}"
  # The knowledge base ships as an asset, so a broken one would ship too.
  echo "checking the knowledge base..."
  go run ./cmd/ribice -lint
  build

  mkdir -p "$out"
  for f in $ASSETS; do cp "web/$f" "$out/$f"; done
  # MIT asks that the notice travels with a copy, and the BSD notice covering
  # wasm_exec.js and the Go runtime inside ribice.wasm asks the same, so both
  # ship alongside the assets. Go's text comes from your toolchain for the same
  # reason wasm_exec.js does: the two must not drift apart.
  cp LICENSE "$out/LICENSE"
  cp "$(go env GOROOT)/LICENSE" "$out/GO-LICENSE"
  sed "s|@KB@|$(basename data/adriatic-fish.json)|g" > "$out/README.md" <<'MD'
# ribice -- embeddable identification quiz

Static assets. Serve all five from one directory and mount the widget:

```html
<div id="fish"></div>
<script type="module">
  import { createQuiz } from "/assets/ribice/ribice.js";
  const quiz = await createQuiz({ mount: "#fish", base: "/assets/ribice/" });
</script>
```

`base` is the directory these files are served from, with a trailing slash.

| File | |
| --- | --- |
| `ribice.js` | the widget, an ES module. `createQuiz(options)` |
| `ribice.css` | default theme. Optional -- delete it and style `.rb-*` yourself |
| `ribice.wasm` | the engine |
| `wasm_exec.js` | Go's runtime shim. Do not edit or substitute: it is version-locked to `ribice.wasm` |
| `@KB@` | the knowledge base. Edit and re-upload freely; no rebuild needed |

## Serving

`ribice.wasm` must be served as `application/wasm`. Without it the widget still
works -- it falls back to a buffered compile -- just slower to start. Check with:

    curl -sI https://yoursite/assets/ribice/ribice.wasm | grep -i content-type

All five files are immutable between builds, so cache them hard; the knowledge
base is the one you are likeliest to change, so give that a shorter max-age.

## Styling

The widget writes no styles. Every element carries an `rb-` class and its state
in an attribute: `[aria-pressed="true"]` on a chosen option, `[data-rb-state]`
on the root (`loading`, `question`, `result`, `error`). With no stylesheet it is
plain but fully usable, and inherits your own typography and buttons.

`ribice.css` is scoped under `.rb-quiz` and selects no bare elements, so it
cannot affect the rest of your page. Retheme it without editing it by setting
its custom properties on the mount element:

```css
#fish { --rb-accent: #7b2d8e; --rb-radius: 3px; --rb-font: Georgia, serif; }
```

Also `--rb-bg`, `--rb-card`, `--rb-ink`, `--rb-muted`, `--rb-line`,
`--rb-accent-soft`, `--rb-accent-ink`, `--rb-warn`, `--rb-radius-sm`,
`--rb-gap`, `--rb-pad`, `--rb-size`. Dark mode follows `prefers-color-scheme`;
`data-rb-theme="light"` or `"dark"` on the mount element pins it.

## Options

```js
createQuiz({
  mount: "#fish",        // element or selector (required)
  base: "/assets/ribice/",
  settings: { maxQuestions: 20 },  // engine Config overrides
  keyboard: true,        // digits select, Enter confirms, s skips, u goes back
  standing: true,        // show running leaders and answers so far
  top: 5,                // candidates listed in the result
  strings: {},           // override any rendered string
  onQuestion(view) {},
  onResult(winner, view) {},
  onError(err) {},
});
```

Returns `{root, view, restart(), destroy()}`. Call `destroy()` when unmounting a
route: it releases the session and unbinds the key handler. Several quizzes may
be mounted on one page; the WebAssembly module and each knowledge base are
fetched once and shared, and sessions stay independent.

## Attribution

The photographs are Wikimedia Commons images under CC BY, CC BY-SA or public
domain. **The widget renders each one's author and licence, and that credit must
stay visible.** If you restyle the result, do not hide `.rb-credit`.

## Licence

The widget is MIT-licensed (`LICENSE`): use it for anything, commercially or
not, keeping the notice with your copy. `wasm_exec.js` and the Go runtime
compiled into `ribice.wasm` are not covered by it -- they are BSD-3-Clause,
(c) The Go Authors, and `GO-LICENSE` is their terms. Neither file is served;
both just need to travel with the five that are.
MD

  echo
  echo "published to $out/"
  for f in $ASSETS; do size "$out/$f"; done
  echo
  echo "Upload all five to one directory your app serves, then:"
  echo
  echo "  import { createQuiz } from \"<that directory>/ribice.js\";"
  echo "  createQuiz({ mount: \"#fish\", base: \"<that directory>/\" });"
  echo
  echo "See $out/README.md."
  ;;

gif)
  make_gif
  ;;

"")
  build
  size web/ribice.wasm
  ;;

*)
  echo "usage: $0 [serve | publish [DIR] | gif]" >&2
  exit 2
  ;;
esac

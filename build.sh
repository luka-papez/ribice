#!/bin/sh
# Builds the browser version of the quiz.
#
#   ./build.sh                 build into web/
#   ./build.sh serve           build, then serve web/ on http://localhost:8080
#   ./build.sh publish [DIR]   build, then copy the embeddable assets to DIR
#                              (default dist/) with a README for whoever
#                              embeds them
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
  # -trimpath keeps the developer's home directory out of a published asset,
  # and leaves the binary byte-identical between rebuilds, so a committed dist/
  # only changes when the code actually does.
  GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o web/ribice.wasm ./cmd/wasm
  cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/  # version-locked to your Go
  cp data/adriatic-fish.json web/                   # fetched at page load
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

"")
  build
  size web/ribice.wasm
  ;;

*)
  echo "usage: $0 [serve | publish [DIR]]" >&2
  exit 2
  ;;
esac

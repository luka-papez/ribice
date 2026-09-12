# ribice -- embeddable identification quiz

An engine and a widget. They know nothing about any particular subject: the
questions and the candidates come from a knowledge base **you** supply.

Serve all four files from one directory and mount the widget, pointing it at
your own data:

```html
<div id="quiz"></div>
<script type="module">
  import { createQuiz } from "/assets/ribice/ribice.js";
  const quiz = await createQuiz({
    mount: "#quiz",
    base: "/assets/ribice/",      // where these four files are served from
    kbUrl: "/data/my-guide.json", // your knowledge base -- there is no default
  });
</script>
```

| File | |
| --- | --- |
| `ribice.js` | the widget, an ES module. `createQuiz(options)` |
| `ribice.css` | default theme. Optional -- delete it and style `.rb-*` yourself |
| `ribice.wasm` | the engine |
| `wasm_exec.js` | Go's runtime shim. Do not edit or substitute: it is version-locked to `ribice.wasm` |

## The knowledge base

One JSON file describing the things to be identified. At its simplest:

```json
[
  { "name": "catfish", "colour": "gray", "moustache": true },
  { "name": "trout",  "colour": "gray", "pattern": "checkered" }
]
```

Wrap it in an object to add phrasing, priors and noise per attribute. The full
format, and the `-lint` and `-simulate` commands that check one, are documented
at <https://github.com/lpapez/ribice>.

Pass it as `kbUrl` to be fetched, or as `kb` if your application already has it:

```js
await createQuiz({ mount: "#quiz", base: "/assets/ribice/", kb: myData });
```

Either way the engine owns it from then on. Several quizzes may be mounted on
one page, over the same knowledge base or different ones; the WebAssembly
module and each fetched knowledge base are loaded once and shared.

Because the data is fetched rather than compiled in, correcting an entry means
re-uploading the JSON -- there is nothing to rebuild.

## Serving

`ribice.wasm` must be served as `application/wasm`. Without it the widget still
works -- it falls back to a buffered compile -- just slower to start. Check with:

    curl -sI https://yoursite/assets/ribice/ribice.wasm | grep -i content-type

All four files are immutable between builds, so cache them hard. Your knowledge
base is the thing you will change, and it is served from wherever you put it.

## Styling

The widget writes no styles. Every element carries an `rb-` class and its state
in an attribute: `[aria-pressed="true"]` on a chosen option, `[data-rb-state]`
on the root (`loading`, `question`, `result`, `error`). With no stylesheet it is
plain but fully usable, and inherits your own typography and buttons.

`ribice.css` is scoped under `.rb-quiz` and selects no bare elements, so it
cannot affect the rest of your page. Retheme it without editing it by setting
its custom properties on the mount element:

```css
#quiz { --rb-accent: #7b2d8e; --rb-radius: 3px; --rb-font: Georgia, serif; }
```

Also `--rb-bg`, `--rb-card`, `--rb-ink`, `--rb-muted`, `--rb-line`,
`--rb-accent-soft`, `--rb-accent-ink`, `--rb-warn`, `--rb-radius-sm`,
`--rb-gap`, `--rb-pad`, `--rb-size`. Dark mode follows `prefers-color-scheme`;
`data-rb-theme="light"` or `"dark"` on the mount element pins it.

## Options

```js
createQuiz({
  mount: "#quiz",        // element or selector (required)
  base: "/assets/ribice/",
  kbUrl: "/data/my-guide.json",    // the knowledge base to fetch (required...
  kb: null,              // ...unless you pass the JSON or an object directly)
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

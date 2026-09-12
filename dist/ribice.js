// Embeddable identification quiz.
//
// The Go side owns all the state: it holds the belief distribution, chooses
// what to ask and decides when to stop. This module renders the view it hands
// back and sends answers in. Each call returns a fresh view, so there is
// nothing to keep in sync.
//
// It writes no styles of its own. Every element carries an rb- class and its
// state as a data- or aria- attribute, so a host application styles it from
// its own sheet; ribice.css is one such sheet, and is optional.
//
//   import { createQuiz } from "./ribice.js";
//   const quiz = await createQuiz({ mount: "#quiz", base: "/ribice/",
//                                   kbUrl: "/data/dogs.json" });
//
// It knows nothing about any particular subject. The knowledge base comes from
// the host application, as a URL to fetch or as data you already have; there is
// no default and no bundled dataset.
//
// Several quizzes may be mounted on one page, over the same knowledge base or
// different ones. The WebAssembly module and each knowledge base are fetched
// once and shared between them.

const DEFAULTS = {
  mount: null,        // element or selector to render into (required)
  base: "",           // where the assets live, e.g. "/ribice/"
  wasmUrl: null,      // defaults to base + "ribice.wasm"
  execUrl: null,      // defaults to base + "wasm_exec.js"
  kbUrl: null,        // URL of the knowledge base to fetch (required unless kb)
  kb: null,           // or the knowledge base itself: JSON text, or an object
  settings: {},       // engine Config overrides: threshold, maxQuestions, ...
  keyboard: true,     // number keys select, Enter confirms, s skips, u goes back
  standing: true,     // show the running leaders and the answers so far
  top: 5,             // how many candidates the result lists
  strings: {},        // any of TEXT below, to retitle or translate
  onQuestion: null,   // (view) => void, each time a question is shown
  onResult: null,     // (winner, view) => void, when the quiz settles
  onError: null,      // (Error) => void
};

// Every string the widget renders. Override any of them via `strings`.
const TEXT = {
  loading: "Loading…",
  question: n => `Question ${n}`,
  hint: "Select any or all that apply. Guessing or approximating is fine — or simply say you're not sure.",
  reoffered: "You skipped this earlier — it decides things now.",
  next: "Next",
  enter: "or press Enter",
  skip: "Not sure",
  back: "Back",
  again: "Start again",
  undo: "Change my last answer",
  leading: "Leading: ",
  nothingYet: "No clear front-runner yet.",
  told: n => `What you've told me (${n})`,
  notSure: "not sure",
  sure: p => `${p} sure`,
  alsoPossible: "Also possible",
  readMore: "Read more",
  photo: "Photo: ",
  source: "source",
  settled: (reason, n) => `${reason} after ${n} question${n === 1 ? "" : "s"}`,
  unknown: "Nothing in this guide fits what you described — treat the runners-up with suspicion.",
  unsure: "Not certain. Going back and answering a skipped question would sharpen this.",
  failed: msg => `Something went wrong: ${msg}`,
};

// --- shared runtime --------------------------------------------------------

// The WebAssembly module is global to the page: one instance serves every
// quiz on it. Both caches are keyed promises, so concurrent mounts wait on the
// same fetch rather than racing to start their own.
let runtimePromise = null;
const kbPromises = new Map();

function runtime(execUrl, wasmUrl) {
  if (!runtimePromise) {
    runtimePromise = (async () => {
      if (typeof globalThis.Go === "undefined") await loadScript(execUrl);
      const go = new globalThis.Go();
      const mod = await instantiate(wasmUrl, go);
      go.run(mod.instance); // returns once Go's main blocks
      if (typeof globalThis.ribice === "undefined") {
        throw new Error("the WebAssembly module did not start");
      }
      return globalThis.ribice;
    })().catch(err => { runtimePromise = null; throw err; });
  }
  return runtimePromise;
}

function loadScript(url) {
  return new Promise((resolve, reject) => {
    const s = document.createElement("script");
    s.src = url;
    s.onload = resolve;
    s.onerror = () => reject(new Error(`could not load ${url}`));
    document.head.appendChild(s);
  });
}

// instantiate prefers streaming, but falls back to an ArrayBuffer for hosts
// that serve .wasm without the application/wasm content type.
async function instantiate(url, go) {
  if (WebAssembly.instantiateStreaming) {
    try {
      return await WebAssembly.instantiateStreaming(fetch(url), go.importObject);
    } catch {
      // fall through to the buffered path
    }
  }
  const res = await fetch(url);
  if (!res.ok) throw new Error(`could not load ${url} (${res.status})`);
  return WebAssembly.instantiate(await res.arrayBuffer(), go.importObject);
}

// knowledgeBase returns the id the Go side gave this URL, loading it once.
function knowledgeBase(api, url) {
  if (!kbPromises.has(url)) {
    kbPromises.set(url, (async () => {
      const res = await fetch(url);
      if (!res.ok) throw new Error(`could not load the knowledge base (${res.status})`);
      return parse(api, await res.text());
    })().catch(err => { kbPromises.delete(url); throw err; }));
  }
  return kbPromises.get(url);
}

// parse hands the JSON to the engine, which owns it from then on.
function parse(api, source) {
  const loaded = api.load(typeof source === "string" ? source : JSON.stringify(source));
  if (loaded.error) throw new Error(loaded.error);
  return loaded;
}

// --- the widget ------------------------------------------------------------

export async function createQuiz(options = {}) {
  const opt = { ...DEFAULTS, ...options };
  const text = { ...TEXT, ...opt.strings };
  const root = typeof opt.mount === "string" ? document.querySelector(opt.mount) : opt.mount;
  if (!root) throw new Error("createQuiz: no element to mount into");
  if (!opt.kb && !opt.kbUrl) {
    throw new Error("createQuiz: no knowledge base. Pass kbUrl with the URL of " +
                    "one to fetch, or kb with the JSON itself.");
  }

  const url = (given, name) => given || `${opt.base}${name}`;
  let api = null, kbId = null, kbName = "", sessionId = null, view = null, destroyed = false;
  let picks = new Set();

  root.classList.add("rb-quiz");
  root.dataset.rbState = "loading";
  root.replaceChildren(el("p", "rb-status", text.loading));

  try {
    api = await runtime(url(opt.execUrl, "wasm_exec.js"), url(opt.wasmUrl, "ribice.wasm"));
    // The widget ships with no dataset of its own: one has to be given.
    const loaded = opt.kb ? parse(api, opt.kb)
                          : await knowledgeBase(api, opt.kbUrl);
    kbId = loaded.kb;
    kbName = loaded.name || "";
  } catch (err) {
    showError(err);
    throw err;
  }
  if (destroyed) return instance();

  if (opt.keyboard) document.addEventListener("keydown", onKey);
  restart();
  return instance();

  // --- session ---

  function restart() {
    if (sessionId !== null) api.release(sessionId);
    sessionId = null;
    apply(api.start({ kb: kbId, ...opt.settings }));
    if (view) sessionId = view.id;
  }

  // apply takes the view returned by any engine call and redraws.
  function apply(next) {
    if (!next || next.error) { showError(new Error(next ? next.error : "no response")); return; }
    view = next;
    picks = new Set();
    render();
    if (view.done) {
      opt.onResult?.(view.top[0] ?? null, view);
    } else {
      opt.onQuestion?.(view);
    }
  }

  // A declaration, not a const arrow: everything below sits after the return
  // above, so a const here would never leave the temporal dead zone.
  function send(fn) {
    if (!destroyed && sessionId !== null) apply(fn());
  }

  function submit() {
    if (picks.size === 0) return;
    const chosen = [...picks].sort((a, b) => a - b);
    send(() => api.answer(sessionId, chosen));
  }

  // choose toggles one option. Nothing is sent until the user confirms, so a
  // misread option can be un-picked rather than undone after the fact.
  function choose(i) {
    if (!view || view.done) return;
    picks.has(i) ? picks.delete(i) : picks.add(i);
    root.querySelectorAll(".rb-opt").forEach((b, n) => {
      b.setAttribute("aria-pressed", picks.has(n) ? "true" : "false");
    });
    const next = root.querySelector(".rb-next");
    if (next) next.disabled = picks.size === 0;
    // The `hidden` property rather than a class, so this still behaves when
    // the host ships no stylesheet at all.
    const hint = root.querySelector(".rb-enter-hint");
    if (hint) hint.hidden = picks.size === 0;
  }

  function onKey(e) {
    if (destroyed || !view || view.done) return;
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    // Never steal a keystroke aimed at something the host owns.
    const t = e.target;
    if (t instanceof HTMLElement &&
        (t.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName))) return;
    if (document.querySelectorAll(".rb-quiz").length > 1 && !root.contains(t)) return;

    const key = e.key.toLowerCase();
    if (e.key >= "1" && e.key <= "9") {
      const i = Number(e.key) - 1;
      if (i < view.question.options.length) { choose(i); e.preventDefault(); }
    } else if (e.key === "Enter") {
      submit();
    } else if (key === "s") {
      send(() => api.skip(sessionId));
    } else if ((key === "u" || e.key === "Backspace") && view.canUndo) {
      send(() => api.undo(sessionId));
      e.preventDefault();
    }
  }

  // --- rendering ---

  function render() {
    root.dataset.rbState = view.done ? "result" : "question";
    const parts = [view.done ? resultCard() : questionCard()];
    if (!view.done && opt.standing && view.asked > 0) parts.push(standingCard());
    root.replaceChildren(...parts);
  }

  function questionCard() {
    const q = view.question;
    const card = el("div", "rb-card rb-question");
    card.appendChild(el("div", "rb-qnum", text.question(view.asked + 1)));
    card.appendChild(el("h2", "rb-qtext", q.text));
    if (q.reoffered) card.appendChild(el("p", "rb-flag", text.reoffered));
    card.appendChild(el("p", "rb-hint", text.hint));

    const list = el("div", "rb-options");
    list.setAttribute("role", "group");
    q.options.forEach((o, i) => {
      const b = el("button", "rb-opt");
      b.type = "button";
      b.setAttribute("aria-pressed", "false");
      b.dataset.rbIndex = String(i);
      if (o.other) b.dataset.rbOther = "true";
      b.appendChild(el("span", "rb-opt-key", i < 9 ? String(i + 1) : ""));
      b.appendChild(el("span", "rb-opt-label", o.label));
      b.addEventListener("click", () => choose(i));
      list.appendChild(b);
    });
    card.appendChild(list);

    const actions = el("div", "rb-actions");
    const next = el("button", "rb-btn rb-next", text.next);
    next.type = "button";
    next.disabled = true;
    next.addEventListener("click", submit);
    actions.appendChild(next);
    if (opt.keyboard) {
      const hint = el("span", "rb-enter-hint", text.enter);
      hint.hidden = true;
      actions.appendChild(hint);
    }
    actions.appendChild(el("span", "rb-spacer"));
    actions.appendChild(button("rb-link rb-skip", text.skip, () => send(() => api.skip(sessionId))));
    if (view.canUndo) {
      actions.appendChild(button("rb-link rb-back", text.back, () => send(() => api.undo(sessionId))));
    }
    card.appendChild(actions);
    return card;
  }

  // standingCard is the running commentary: what leads, and what has been said.
  function standingCard() {
    const card = el("div", "rb-card rb-standing");
    const top = view.top.filter(c => c.prob >= 0.01).slice(0, 3);
    const line = el("p", "rb-leaders");
    if (top.length === 0) {
      line.textContent = text.nothingYet;
    } else {
      line.append(text.leading);
      top.forEach((c, i) => {
        if (i) line.append(" · ");
        line.appendChild(el("b", "rb-leader-name", c.name));
        line.append(` ${pct(c.prob)}`);
      });
    }
    card.appendChild(line);

    if (view.history.length) {
      const d = el("details", "rb-history");
      d.appendChild(el("summary", "rb-history-summary", text.told(view.history.length)));
      const ul = el("ul", "rb-history-list");
      for (const h of view.history) {
        const li = el("li", "rb-history-item");
        if (h.skipped) li.dataset.rbSkipped = "true";
        li.append(`${h.text} — ${h.skipped ? text.notSure : h.label}`);
        ul.appendChild(li);
      }
      d.appendChild(ul);
      card.appendChild(d);
    }
    return card;
  }

  function resultCard() {
    const card = el("div", "rb-card rb-result");
    const best = view.top[0];
    if (!best) return card;
    if (best.unknown) card.dataset.rbUnknown = "true";
    card.dataset.rbConfident = best.prob >= 0.6 ? "true" : "false";

    const win = el("div", "rb-winner");
    if (best.image) {
      const img = el("img", "rb-photo");
      img.src = best.image.url;
      img.alt = best.name;
      img.loading = "lazy";
      img.addEventListener("error", () => img.remove());
      win.appendChild(img);
    }
    const who = el("div", "rb-who");
    who.appendChild(el("p", "rb-reason", text.settled(view.reason, view.asked)));
    who.appendChild(el("h2", "rb-name", best.name));
    who.appendChild(el("p", "rb-confidence", text.sure(pct(best.prob))));
    if (best.note) who.appendChild(el("p", "rb-note", best.note));
    if (best.link) {
      const a = el("a", "rb-link-out", text.readMore);
      a.href = best.link; a.target = "_blank"; a.rel = "noopener";
      who.appendChild(a);
    }
    if (best.image) who.appendChild(credit(best.image));
    win.appendChild(who);
    card.appendChild(win);

    if (best.unknown) card.appendChild(el("p", "rb-flag", text.unknown));
    else if (best.prob < 0.6) card.appendChild(el("p", "rb-flag", text.unsure));

    const rest = view.top.slice(1, opt.top).filter(c => c.prob >= 0.005);
    if (rest.length) {
      const others = el("div", "rb-runners");
      others.appendChild(el("p", "rb-qnum", text.alsoPossible));
      for (const c of rest) {
        const r = el("div", "rb-runner");
        r.appendChild(el("span", "rb-runner-name", c.name));
        const bar = el("span", "rb-runner-bar");
        const fill = el("span", "rb-runner-fill");
        // Inline because it is data, not decoration; the custom property lets
        // a host sheet drive its own presentation from the same number.
        fill.style.setProperty("--rb-prob", c.prob.toFixed(4));
        fill.style.width = `${Math.max(2, c.prob * 100)}%`;
        bar.appendChild(fill);
        r.appendChild(bar);
        r.appendChild(el("span", "rb-runner-pct", pct(c.prob)));
        others.appendChild(r);
      }
      card.appendChild(others);
    }

    const actions = el("div", "rb-actions");
    actions.appendChild(button("rb-btn rb-again", text.again, restart));
    if (view.canUndo) {
      actions.appendChild(button("rb-link rb-back", text.undo, () => send(() => api.undo(sessionId))));
    }
    card.appendChild(actions);
    return card;
  }

  function credit(img) {
    const p = el("p", "rb-credit");
    const parts = [img.credit, img.license].filter(Boolean).join(", ");
    if (img.source) {
      const a = el("a", "rb-credit-link", parts || text.source);
      a.href = img.source; a.target = "_blank"; a.rel = "noopener";
      p.append(text.photo);
      p.appendChild(a);
    } else if (parts) {
      p.append(text.photo, parts);
    }
    return p;
  }

  function showError(err) {
    root.dataset.rbState = "error";
    root.replaceChildren(el("p", "rb-status rb-error", text.failed(err.message)));
    opt.onError?.(err);
  }

  // --- the handle handed back ---

  function instance() {
    return {
      root,
      restart,
      // the knowledge base's own name, for a host that wants to title the page
      get name() { return kbName; },
      get view() { return view; },
      destroy() {
        destroyed = true;
        if (opt.keyboard) document.removeEventListener("keydown", onKey);
        if (api && sessionId !== null) api.release(sessionId);
        sessionId = null;
        root.replaceChildren();
        root.classList.remove("rb-quiz");
        delete root.dataset.rbState;
      },
    };
  }
}

// --- helpers ---------------------------------------------------------------

function el(tag, cls, text) {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
  if (text !== undefined) n.textContent = text;
  return n;
}

function button(cls, label, fn) {
  const b = el("button", cls, label);
  b.type = "button";
  b.addEventListener("click", fn);
  return b;
}

function pct(p) { return `${Math.round(p * 100)}%`; }

export default createQuiz;

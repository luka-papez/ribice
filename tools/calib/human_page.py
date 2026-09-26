"""Write a page for a person to answer the quiz on the answerer's photos.

A person's answers, question by question, against Claude's on the same photos
are the ground truth for how human the answerer is, and the gate to tuning
anything. The page shows one photo at a time with exactly the questions and
options the answerer sees, plus "can't tell" and its reasons. Progress is
kept in the browser; "Download answers" saves them as JSON, which goes to
corpus/<kb>/human.json, where report.py compares them with Claude's.

The page opens from disk (file://), no server needed. It shows photos from the
gitignored cache, so it is built locally and not committed.

    python tools/calib/human_page.py --kb data/clouds.json [--count 10]
"""
import argparse, html, json, os, sys

import ask

HERE = os.path.dirname(os.path.abspath(__file__))

PAGE = """<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Cloud quiz answers</title>
<style>
:root { --bg:#f7f7f5; --fg:#1d1d1b; --muted:#6b6b66; --card:#fff; --line:#dcdcd6; --accent:#2f6fb3; }
@media (prefers-color-scheme: dark) {
  :root { --bg:#161615; --fg:#ecece8; --muted:#9a9a93; --card:#212120; --line:#3a3a37; --accent:#7fb0e6; } }
* { box-sizing:border-box } body { margin:0; background:var(--bg); color:var(--fg);
  font:16px/1.45 system-ui, sans-serif }
main { max-width:1100px; margin:0 auto; padding:16px; display:grid; gap:16px;
  grid-template-columns: minmax(0,1.3fr) minmax(0,1fr) }
@media (max-width:800px) { main { grid-template-columns: 1fr } }
header { max-width:1100px; margin:0 auto; padding:16px 16px 0; display:flex; gap:12px;
  align-items:center; flex-wrap:wrap }
header h1 { font-size:18px; margin:0 auto 0 0 }
.photo { position:sticky; top:16px; align-self:start }
.photo img { width:100%; border-radius:8px; display:block }
.q { background:var(--card); border:1px solid var(--line); border-radius:8px; padding:12px; margin-bottom:10px }
.q.done { border-color:var(--accent) }
.q h2 { font-size:15px; margin:0 0 8px }
label { display:flex; gap:8px; padding:3px 0; cursor:pointer }
.cant { color:var(--muted); border-top:1px dashed var(--line); margin-top:6px; padding-top:6px; font-size:14px }
button { font:inherit; padding:6px 12px; border-radius:6px; border:1px solid var(--line);
  background:var(--card); color:var(--fg); cursor:pointer }
button.primary { background:var(--accent); color:#fff; border-color:var(--accent) }
.muted { color:var(--muted); font-size:14px }
</style></head><body>
<header><h1>Answer as you would under this sky</h1>
<span class="muted" id="progress"></span>
<button id="prev">&larr; Previous</button><button id="next">Next &rarr;</button>
<button class="primary" id="save">Download answers</button></header>
<p class="muted" style="max-width:1100px;margin:8px auto 0;padding:0 16px">
Judge only what you can see in the photo. Tick one option, or two if you're torn. If you
can't tell, say why instead. Answers are kept in this browser until you download them.</p>
<main><div class="photo"><img id="img" alt="the sky to judge"></div><div id="qs"></div></main>
<script>
const PHOTOS = __PHOTOS__, QUESTIONS = __QUESTIONS__, KEY = "calib-human-__KB__";
const CANT = [["not_in_photo","can't be seen in a photo"],["ambiguous","the photo is unclear"],
              ["unclear_question","I don't understand the question"]];
let state = {}; try { state = JSON.parse(localStorage.getItem(KEY)) || {} } catch (e) {}
let at = Math.min(state._at || 0, PHOTOS.length - 1);
function store() { state._at = at; try { localStorage.setItem(KEY, JSON.stringify(state)) } catch (e) {} }
function answersFor(id) { return state[id] || (state[id] = {}) }
function done(id) { const a = state[id] || {}; return QUESTIONS.filter(q => a[q.attr]).length }
function render() {
  const p = PHOTOS[at], a = answersFor(p.id);
  document.getElementById("img").src = p.src;
  const complete = PHOTOS.filter(x => done(x.id) === QUESTIONS.length).length;
  document.getElementById("progress").textContent =
    `photo ${at + 1} of ${PHOTOS.length} \\u00b7 ${complete} complete`;
  const qs = document.getElementById("qs"); qs.innerHTML = "";
  QUESTIONS.forEach(q => {
    const cur = a[q.attr] || {values: [], cant_tell: null};
    const box = document.createElement("div"); box.className = "q" + (a[q.attr] ? " done" : "");
    box.innerHTML = `<h2></h2>`; box.querySelector("h2").textContent = q.text;
    q.options.forEach(([value, label]) => {
      const l = document.createElement("label");
      const i = document.createElement("input"); i.type = "checkbox";
      i.checked = cur.values.includes(value);
      i.onchange = () => {
        let v = cur.values.filter(x => x !== value);
        if (i.checked) v = [...v, value].slice(-2);
        a[q.attr] = v.length ? {values: v, cant_tell: null} : undefined;
        if (!v.length) delete a[q.attr];
        store(); render();
      };
      l.append(i, document.createTextNode(label)); box.append(l);
    });
    const c = document.createElement("div"); c.className = "cant"; c.textContent = "Can't tell: ";
    CANT.forEach(([why, label]) => {
      const l = document.createElement("label"); l.style.display = "inline-flex"; l.style.marginRight = "12px";
      const i = document.createElement("input"); i.type = "radio"; i.name = q.attr;
      i.checked = cur.cant_tell === why;
      i.onchange = () => { a[q.attr] = {values: [], cant_tell: why}; store(); render(); };
      l.append(i, document.createTextNode(label)); c.append(l);
    });
    box.append(c); qs.append(box);
  });
}
document.getElementById("prev").onclick = () => { at = Math.max(0, at - 1); store(); render(); scrollTo(0, 0) };
document.getElementById("next").onclick = () => { at = Math.min(PHOTOS.length - 1, at + 1); store(); render(); scrollTo(0, 0) };
document.getElementById("save").onclick = () => {
  const out = {}; PHOTOS.forEach(p => { if (state[p.id]) out[p.id] = state[p.id] });
  const blob = new Blob([JSON.stringify({kb: "__KB__", answers: out}, null, 1)], {type: "application/json"});
  const link = document.createElement("a"); link.href = URL.createObjectURL(blob);
  link.download = "human.json"; link.click();
};
render();
</script></body></html>
"""


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--kb", required=True)
    ap.add_argument("--count", type=int, default=10)
    a = ap.parse_args()

    kb = json.load(open(a.kb))
    base = os.path.splitext(os.path.basename(a.kb))[0]
    photos = json.load(open(os.path.join(HERE, "corpus", base, "manifest.json")))["photos"]
    chosen = ask.select(photos, "tune", 1, a.count)
    out_dir = os.path.join(HERE, "cache", "human")
    os.makedirs(out_dir, exist_ok=True)
    page = (PAGE
            .replace("__PHOTOS__", json.dumps([
                {"id": p["id"], "src": f"../photos/{base}/{p['id']}.jpg"} for p in chosen]))
            .replace("__QUESTIONS__", json.dumps([
                {"attr": attr, "text": text, "options": opts}
                for attr, text, opts in ask.questions(kb)]))
            .replace("__KB__", html.escape(base)))
    path = os.path.join(out_dir, f"{base}.html")
    with open(path, "w") as f:
        f.write(page)
    print(f"{len(chosen)} photos: file://{path}", file=sys.stderr)
    print(f"save the download as {os.path.relpath(os.path.join(HERE, 'corpus', base, 'human.json'))}",
          file=sys.stderr)


if __name__ == "__main__":
    main()

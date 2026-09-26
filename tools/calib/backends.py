"""The three ways the harness reaches Claude, behind one call.

    run(reqs, record, valid, via=..., ...)

`reqs` maps an id to Messages API request params; `record(id, answer)` is
called once per id, from the calling thread, with the parsed JSON reply or
None for one that stayed unusable after a retry. `valid(answer)` says whether
a parsed reply is usable. An id missing from the calls to `record` was not
asked at all (a CLI run that paused), and is simply asked on the next run.

  cli    the Claude Code CLI on a Claude subscription (claude_cli.py). Stops
         starting calls once the five-hour window is `max_window` full.
  batch  the Batch API, at half the API price. A submitted batch is
         remembered in `state_path`, so an interrupted run resumes it.
  api    the Messages API directly: faster than a batch, twice the price.
"""
import concurrent.futures, datetime, json, os, sys, time

PRICE = {"claude-opus-5": (5.00, 25.00), "claude-sonnet-5": (2.00, 10.00),
         "claude-haiku-4-5": (1.00, 5.00)}     # $ per million tokens, in / out, June 2026


def run(reqs, record, valid, via="cli", jobs=4, max_window=0.8, state_path=None):
    if not reqs:
        return
    if via == "cli":
        return _cli(reqs, record, valid, jobs, max_window)
    import anthropic
    client = anthropic.Anthropic()
    model = next(iter(reqs.values()))["model"]
    usage = _Usage(model, batched=via == "batch")
    if via == "api":
        _direct(client, reqs, record, valid, usage, jobs)
    else:
        _batch(client, reqs, record, valid, usage, state_path)


def _parse(message, valid):
    """The JSON in an API reply, or None for a refusal or a reply cut short."""
    if message.stop_reason != "end_turn":
        return None
    text = next((b.text for b in message.content if b.type == "text"), "")
    try:
        v = json.loads(text)
    except json.JSONDecodeError:
        return None
    return v if valid(v) else None


class _Usage:
    def __init__(self, model, batched):
        self.model, self.batched = model, batched
        self.inp = self.out = 0

    def add(self, message):
        u = message.usage
        self.inp += u.input_tokens + (u.cache_creation_input_tokens or 0) + \
                    (u.cache_read_input_tokens or 0)
        self.out += u.output_tokens

    def __str__(self):
        pin, pout = PRICE.get(self.model, (0, 0))
        cost = (self.inp * pin + self.out * pout) / 1e6 * (0.5 if self.batched else 1)
        return f"{self.inp:,} tokens in, {self.out:,} out, about ${cost:.2f}"


def _cli(reqs, record, valid, jobs, max_window):
    """A few at a time through `claude -p`, each retried once if unusable."""
    import claude_cli
    pool = claude_cli.Pool(max_window)

    def one(item):
        rid, params = item
        err = None
        for _ in range(2):
            v, info = pool.call(params)
            if valid(v):
                return rid, v, None
            err = info["error"] or "invalid reply"
            if err == "paused":
                break
        return rid, None, err

    done = 0
    with concurrent.futures.ThreadPoolExecutor(jobs) as ex:
        for rid, v, err in ex.map(one, reqs.items()):
            if err == "paused":
                continue
            done += 1
            record(rid, v)
            if err:
                print(f"\n  {rid}: {err}", file=sys.stderr)
            print(f"\r  {done}/{len(reqs)}  window {pool.window or 0:.0%}", end="", file=sys.stderr)
    print(f"\n  about ${pool.cost:.2f} at API prices, from the subscription", file=sys.stderr)
    if pool.paused():
        at = datetime.datetime.fromtimestamp(pool.resets).strftime("%H:%M") if pool.resets else "later"
        print(f"  paused with {len(reqs) - done} left: the five-hour window is {pool.window:.0%} "
              f"used (limit {max_window:.0%}); it resets at {at}", file=sys.stderr)


def _direct(client, reqs, record, valid, usage, jobs):
    """A few at a time, each retried once if its reply is unusable."""
    def one(item):
        rid, params = item
        for _ in range(2):
            m = client.messages.create(**params)
            usage.add(m)
            v = _parse(m, valid)
            if v:
                return rid, v
        return rid, None

    with concurrent.futures.ThreadPoolExecutor(jobs) as pool:
        for n, (rid, v) in enumerate(pool.map(one, reqs.items()), 1):
            record(rid, v)
            print(f"\r  {n}/{len(reqs)}", end="", file=sys.stderr)
    print(f"\n  {usage}", file=sys.stderr)


def _batch(client, reqs, record, valid, usage, state_path):
    from anthropic.types.message_create_params import MessageCreateParamsNonStreaming
    from anthropic.types.messages.batch_create_params import Request

    state = json.load(open(state_path)) if os.path.exists(state_path) else None
    if state:
        # whatever it asked for, it is paid for: collect it rather than ask again
        print(f"  resuming batch {state['batch']}", file=sys.stderr)
    else:
        batch = client.messages.batches.create(requests=[
            Request(custom_id=rid, params=MessageCreateParamsNonStreaming(**params))
            for rid, params in reqs.items()])
        state = {"batch": batch.id, "ids": list(reqs)}
        json.dump(state, open(state_path, "w"))
        print(f"  submitted batch {batch.id} ({len(reqs)} requests)", file=sys.stderr)

    while True:
        b = client.messages.batches.retrieve(state["batch"])
        if b.processing_status == "ended":
            break
        c = b.request_counts
        print(f"  {c.processing} processing, {c.succeeded} done", file=sys.stderr)
        time.sleep(60)

    for r in client.messages.batches.results(state["batch"]):
        if r.result.type == "succeeded":
            usage.add(r.result.message)
            record(r.custom_id, _parse(r.result.message, valid))
        else:
            print(f"  {r.custom_id}: {r.result.type}", file=sys.stderr)
    os.remove(state_path)
    print(f"  {usage}", file=sys.stderr)

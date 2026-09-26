"""Send a Messages API request through the Claude Code CLI instead of the API.

`claude -p` runs on a Claude subscription, so the loop can run without an API
key. It takes the same request dict the API path builds, and turns it into a
locked-down headless call:

- --system-prompt replaces Claude Code's own prompt. A short preamble remains
  (the date, the working directory, the account e-mail), none of which says
  anything about a photo. Calls run in an empty directory, so no CLAUDE.md or
  project memory is picked up.
- --tools "" leaves only the tool that returns the --json-schema answer. The
  image goes in as a content block over stream-json, never as a file path.
- --setting-sources "" and --strict-mcp-config skip hooks, settings and MCP
  servers; --no-session-persistence keeps a few thousand calls out of the
  session list.

Every call reports how much of the subscription's five-hour window is used.
`Pool` stops starting calls past a threshold, so a long run leaves room for
the person whose subscription it is.
"""
import json, subprocess, tempfile, threading


def call(params, timeout=600):
    """One request. Returns (parsed answer or None, info dict)."""
    fmt = params.get("output_config", {})
    schema = fmt.get("format", {}).get("schema")
    cmd = ["claude", "-p", "--input-format", "stream-json", "--output-format", "stream-json",
           "--verbose", "--system-prompt", params["system"], "--tools", "",
           "--setting-sources", "", "--strict-mcp-config", "--no-session-persistence",
           "--model", params["model"]]
    if fmt.get("effort"):
        cmd += ["--effort", fmt["effort"]]
    if schema:
        cmd += ["--json-schema", json.dumps(schema)]
    msg = json.dumps({"type": "user", "message": params["messages"][0]})
    info = {"cost": 0.0, "window": None, "resets": None, "error": None}
    with tempfile.TemporaryDirectory(prefix="calib-") as empty:
        try:
            p = subprocess.run(cmd, input=msg + "\n", capture_output=True, text=True,
                               cwd=empty, timeout=timeout)
        except subprocess.TimeoutExpired:
            info["error"] = "timed out"
            return None, info
    answer = None
    for line in p.stdout.splitlines():
        try:
            ev = json.loads(line)
        except json.JSONDecodeError:
            continue
        if ev.get("type") == "rate_limit_event":
            w = ev.get("rate_limit_info", {}).get("unifiedWindows", {}).get("five_hour", {})
            info["window"], info["resets"] = w.get("utilization"), w.get("resetsAt")
        elif ev.get("type") == "result":
            info["cost"] = ev.get("total_cost_usd") or 0.0
            if ev.get("is_error") or ev.get("subtype") != "success":
                info["error"] = ev.get("subtype") or "error"
            elif schema:
                answer = ev.get("structured_output")
            else:
                answer = ev.get("result")
    if answer is None and not info["error"]:
        info["error"] = (p.stderr.strip().splitlines() or [f"exit {p.returncode}"])[-1][:200]
    return answer, info


class Pool:
    """Tracks spend and the subscription window across concurrent calls."""

    def __init__(self, max_window):
        self.max_window = max_window
        self.cost = 0.0
        self.window = self.resets = None
        self.lock = threading.Lock()

    def paused(self):
        return self.window is not None and self.window >= self.max_window

    def call(self, params):
        if self.paused():
            return None, {"error": "paused"}
        answer, info = call(params)
        with self.lock:
            self.cost += info["cost"]
            if info["window"] is not None:
                self.window, self.resets = info["window"], info["resets"]
        return answer, info

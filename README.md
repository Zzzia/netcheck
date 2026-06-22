# netcheck

[简体中文](README.zh-CN.md)

`netcheck` is a network quality dashboard for vibe coding developers who need to know whether Codex feels slow because of the model, the client, or the network path underneath it.

Network instability can dominate Codex latency. Switching Codex to a faster mode does not help much if requests keep stalling, reconnecting, or retrying; in a bad network, the same vibe coding task can take twice as long as it does on a healthy link. `netcheck` monitors local, domestic, and international network quality while also reading Codex request outcomes, so the web dashboard can give a concrete network-quality assessment for AI coding sessions instead of relying on vague “it feels slow” impressions.

<table>
  <tr>
    <td width="50%"><img src="img/en-main.webp" width="100%" alt="English netcheck dashboard overview"></td>
    <td width="50%"><img src="img/en-detail.webp" width="100%" alt="English netcheck detailed metrics and Codex timeline"></td>
  </tr>
  <tr>
    <td align="center">Dashboard overview</td>
    <td align="center">Detailed metrics and Codex timeline</td>
  </tr>
</table>

## Highlights

- **Network × Codex on one timeline.** Network probes and real Codex request outcomes (stream retries, affected turns, token usage, turn duration) are aligned on a shared timeline, so you can tell “the model is slow” apart from “the network made the same workflow slower.”
- **Single self-contained binary, zero runtime dependencies.** The whole web UI (HTML, vanilla JavaScript, hand-drawn SVG charts) is compiled into the binary via `go:embed` — no CDN, no frontend framework, no chart library. Storage uses a pure-Go SQLite (`modernc.org/sqlite`, no CGO), so it cross-compiles to a static binary. Drop one file on a machine and run it.
- **Smart Codex log parsing.** Reads from either the Codex text log (`~/.codex/log/codex-tui.log`) or the Codex SQLite logs, automatically picking the most recently updated source. Large text logs are read backwards from the tail with a doubling window, so it never scans the whole file.
- **Three-layer health with a debounced state machine.** Local / domestic / international layers are scored over a sliding window (P95, jitter, failure rate). A degradation state only opens after several consecutive bad samples and only closes after several good ones, and open events survive process restarts.
- **Robust out of the box.** Auto-discovers the default gateway, automatically falls back to the next free port if `8765` is taken, and ships a bilingual (English / 简体中文) UI and CLI.

## Quick Start

Build from source (requires Go 1.25+):

```bash
make build
./netcheck
```

Or install with the Go toolchain:

```bash
go install github.com/Zzzia/netcheck/cmd/netcheck@latest
```

Open the printed local URL in your browser. By default the web dashboard listens on `0.0.0.0:8765`; if the port is busy, `netcheck` automatically switches to the next available port and prints the actual address.

## What It Shows

- Gateway RTT, jitter, and packet loss.
- Domestic and international latency, download speed, and failure rate.
- Incident duration and recent network events for the selected time range.
- Codex stream retries, affected turns, network error candidates, and retry depth.
- A shared timeline that helps distinguish “the model is slow” from “the network made the same Codex workflow slower.”

## Commands

Running `netcheck` with no arguments starts monitoring and the web dashboard together. Sub-commands are available for finer control:

| Command | Purpose |
| --- | --- |
| `netcheck` | Start monitoring and the web dashboard together (default). |
| `netcheck monitor` | Run only the background probes and write samples to storage. |
| `netcheck ui` | Serve only the web dashboard from existing data. |
| `netcheck report` | Render a one-off report for a time range. |
| `netcheck clear` | Remove the local SQLite database and its WAL/SHM files. |
| `netcheck init-config` | Generate a default configuration file. |

Common flags: `--lang en|zh-CN` (global), `--config <path>`, `--addr <host:port>` (for `ui`), `--since` / `--start` / `--end` / `--output` (for `report`). The UI/CLI language can also be set via the `NETCHECK_LANG` environment variable.

## Configuration

`netcheck` works with sensible defaults and no configuration. To customize sampling intervals, degradation thresholds, probe targets, and alerting, generate and edit a JSON config:

```bash
./netcheck init-config
```

By default the config file lives at `<user-config-dir>/netcheck/config.json`. Pass `--config <path>` to any command to use a different file.

## Data & Privacy

- Probe samples and degradation events are stored locally in SQLite at `<user-config-dir>/netcheck/netcheck.sqlite` (use `netcheck clear` to wipe it).
- Codex logs under `~/.codex/` are read **read-only** and are never written to or uploaded anywhere. All analysis happens locally.

## Platform Support

Supported and tested on **Linux** and **macOS**. The default-gateway discovery and `ping` invocation are Unix-style; **Windows is not supported yet**.

## License

Released under the [MIT License](LICENSE).

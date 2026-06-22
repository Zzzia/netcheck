# netcheck

[简体中文](README.zh-CN.md)

`netcheck` is a Linux-first network quality dashboard for vibe coding developers who need to know whether Codex feels slow because of the model, the client, or the network path underneath it.

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

## Quick Start

Build and run on Linux:

```bash
make build
./netcheck
```

Open the printed local URL in your browser. By default, the web dashboard listens on `0.0.0.0:8765`; if the port is busy, `netcheck` automatically switches to the next available port and prints the actual address.

## What It Shows

- Gateway RTT, jitter, and packet loss.
- Domestic and international latency, download speed, and failure rate.
- Incident duration and recent network events for the selected time range.
- Codex stream retries, affected turns, network error candidates, and retry depth.
- A shared timeline that helps distinguish “the model is slow” from “the network made the same Codex workflow slower.”

## License

Not specified yet.

# powerorbit

> **CubeSat power system simulator — answer "will your satellite survive the eclipse?" before it flies.**

`powerorbit` is a small, single-binary CLI that simulates a CubeSat's
electrical power system (EPS) in low Earth orbit: solar panels generate power
in sunlight, the battery stores it and carries the spacecraft through
eclipse, and subsystem loads drain it. It simulates battery state of charge
(SoC) minute by minute over N orbits and reports the minimum SoC, the maximum
depth of discharge (DoD) per orbit, and a **PASS/FAIL verdict** — plus the
smallest fix when the design fails (more panel area, a bigger battery, or a
lower payload duty cycle).

## Why this exists

Power faults are among the most common causes of early CubeSat failure. A
CubeSat in LEO orbits every ~90–95 minutes and spends up to ~36 minutes of
each orbit in darkness with zero generation. Today, student teams answer
"is the power enough?" with a static spreadsheet: one average number per
orbit, no time-series SoC, no real eclipse from the actual orbit, no verdict,
no sizing recommendation. `powerorbit` replaces that spreadsheet with a
runnable, CI-checkable tool.

## Status

**Day 2 — mission config.** YAML mission files (`internal/config`) with a
strict loader (unknown fields are rejected) and full validation: LEO
altitude 200–2000 km, panel/battery efficiencies in (0, 1], DoD ≤ 30% per
orbit, SoC_min ≥ 20%, and a shipped 1U reference config
(`configs/1u-reference.yaml`, numbers from the published literature). The
simulation engine lands over Days 3–7 per the roadmap below.

| Day | Work |
|---|---|
| 1 | Scaffold: README, LICENSE, go.mod, CLI skeleton, `version` |
| 2 | YAML config types + loader + validation + 1U reference config |
| 3 | Orbit engine v1: circular orbit + cylindrical shadow, unit-tested vs literature |
| 4 | PV model (solar constant, efficiency, cosine, degradation, path efficiency) |
| 5 | Battery model (SoC, charge/discharge efficiency, DoD) + load profiles per mode |
| 6 | Core simulator: time-step loop over N orbits, SoC curve, energy balance |
| 7 | Terminal report: table + verdict + ASCII SoC chart |
| 8 | JSON + CSV export |
| 9 | `whatif`: compare scenarios in one table |
| 10 | `suggest`: minimum sizing recommendation |
| 11 | SGP4 adapter + TLE input + worst-orbit search |
| 12 | `check` (CI mode) + GitHub Actions example |
| 13 | HTML report + docs (`PHYSICS.md`, `EXAMPLES.md`) |
| 14 | GoReleaser + cross-platform builds → **v0.1.0** |

## Quickstart

```bash
go build -o powerorbit ./cmd/powerorbit
./powerorbit version
./powerorbit validate --config configs/1u-reference.yaml
./powerorbit --help
```

## Mission config (power budget as code)

A mission is one YAML file — orbit, solar panel, battery, loads, simulation
settings — checked into version control with the repo. See
`configs/1u-reference.yaml` for the canonical 1U example (500 km, β = 0°,
0.01 m² of 30% GaAs cells, one 18650 cell at 9.62 Wh). Validate any file with:

```bash
./powerorbit validate --config mission.yaml
```

The loader is strict: unknown fields (typos) are rejected, and validation
reports every problem at once — bad altitude, efficiency outside (0, 1],
DoD + min-SoC contradicting each other, duplicate load modes, and more.

## Model honesty

v0.1 is a **design-stage estimator**: circular orbit, cylindrical Earth
shadow, no detailed thermal/attitude model. It is not a replacement for
STK/GMAT flight analysis. Every model number is unit-tested against published
data (500 km orbit ≈ 94.6 min period, ~35 min eclipse at β = 0°).

## License

MIT — see `LICENSE`.

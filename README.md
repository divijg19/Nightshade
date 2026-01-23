# `Nightshade`

> *A deterministic, terminal-native multiplayer world where humans and learning agents coexist.*

`Nightshade` is a **live, tick-based multiplayer simulation** implemented as a **single authoritative Go monolith**, with **offline Python tooling** for training synthetic agents in a mirrored world.

Human players connect via terminal, act in a shared world, and interact with agents whose behavior can be scripted or learned. The system is designed to be **deterministic, replayable, inspectable**, and **fast to iterate on**.

This project intentionally avoids over-engineering: no microservices, no cloud infra, no online ML inference.

---

## Core Ideas

* **One runtime, one clock, one world**
* **Humans and agents are treated identically**
* **Deterministic simulation with replay**
* **Offline learning via synthetic simulation**
* **Terminal-first UI**

---

## Architecture Overview

```
Human Terminal
      │
      ▼
[ Networking Layer ]
      │
      ▼
[ Runtime Tick Loop ]  ← authoritative
      │
      ▼
[ Game Rules ] → [ World State ]
      │                │
      │                ├─→ Events → Replay / ML
      │                │
      └─→ Snapshot ────┘
              │
              ▼
          ASCII Renderer
```

* The **Go runtime** is a single monolith and the only source of truth.
* **Python tooling** is used *offline* to simulate worlds, train agents, and export static policies.
* Policies are loaded by the Go runtime and executed as agents.

---

## Repository Structure

```
nightshade/
├── cmd/
│   └── nightshade/
│       └── main.go
├── internal/
│   ├── runtime/    # tick loop, clock, snapshots
│   ├── world/      # pure world state
│   ├── game/       # rules & mechanics
│   ├── agent/      # humans + bots + learned agents
│   ├── net/        # networking (thin)
│   ├── render/     # ASCII rendering
│   ├── event/      # event bus & logging
│   ├── replay/     # determinism & replay
│   └── util/       # minimal helpers
├── specs/           # shared contracts (Go ↔ Python)
├── tools/           # offline ML tooling
│   ├── simulator/
│   ├── training/
│   └── export/
├── assets/
├── go.mod
└── README.md
```

---

## Design Principles

### Monolithic by Design

`Nightshade` is intentionally implemented as a **single Go binary**:

* deterministic simulation
* low latency
* simple debugging
* fast iteration

### Agents Are First-Class

Humans, scripted bots, and learned agents all implement the same interface:

```
Decide(snapshot) → Action
```

No special-case logic exists elsewhere in the system.

### World vs Game

* **World**: state only (grid, entities, visibility)
* **Game**: meaning and rules (combat, resources, validation)

This separation keeps logic testable and simulation clean.

### Events Everywhere

All meaningful changes emit events:

* debugging
* replay
* ML datasets
* observability

---

## Tick Model

* Fixed tick rate (i.e. 10 TPS)
* Single goroutine owns world mutation
* Deterministic RNG with seed
* Inputs → rules → state → events → snapshot

If it can’t be replayed, it’s a bug.

---

## Machine Learning (Offline)

`Nightshade` does **not** run ML models during gameplay.

Instead:

1. A **headless Python simulator** mirrors the Go world rules.
2. Synthetic rollouts are generated.
3. Agents are trained (e.g. tabular Q-learning).
4. Policies are exported as static artifacts.
5. The Go runtime loads and executes them.

This keeps gameplay deterministic and the system simple.

---

## Python Tooling (Using `uv`)

All Python tooling uses **`uv`** for dependency management and execution.

### Setup

```bash
cd tools
uv venv
uv sync
```

### Run Training

```bash
uv run python training/train.py
```

### Export Policy

```bash
uv run python export/policy_export.py
```

Generated policies can then be loaded by the Go runtime.

---

## Building & Running

### Requirements

* Go 1.22+
* Python 3.11+
* `uv`

### Build

```bash
go build ./cmd/nightshade
```

### Run

```bash
./nightshade
```

Players connect via terminal (TCP / WebSocket, depending on config).

---

## What This Project Demonstrates

* Systems design in Go
* Deterministic simulation
* Networking and concurrency
* Clean monolithic architecture
* Synthetic environments for ML
* Offline reinforcement learning
* Human–agent interaction

---

## Explicit Non-Goals

`Nightshade` intentionally does **not** include:

* LLMs
* cloud infrastructure
* microservices
* online ML inference
* heavy ML models

The focus is **clarity, correctness, and completeness**.

---

## Future Extensions (Out of Scope)

* Replay visualizer
* Agent behavior analysis
* Fog-of-war learning
* Human/agent Turing tests

---

## Closing Notes

`Nightshade` is designed to feel like a **small engine**, not a demo.
Every decision favors determinism, inspectability, and long-term understanding over novelty.

If you can replay it, you can reason about it.
If you can reason about it, you can evolve it.
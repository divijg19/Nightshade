# `Nightshade` — Tools

This directory contains **offline tooling** for `Nightshade`.

These tools are **not part of the live runtime**.
They exist to **simulate**, **train**, **evaluate**, and **export** agent behavior using a synthetic version of the world.

Nothing in this directory runs during gameplay.

---

## Purpose

The tools serve four goals:

1. Reproduce the `Nightshade` world logic in a headless environment
2. Generate synthetic rollouts at scale
3. Train learning agents from those rollouts
4. Export static policies consumable by the Go runtime

This separation preserves determinism, debuggability, and simplicity.

---

## Directory Overview

```
tools/
├── simulator/   # headless world + step function
├── training/    # learning algorithms
├── export/      # policy serialization
└── README.md
```

---

## Simulator

### `simulator/`

The simulator mirrors the core mechanics of `Nightshade` without rendering or networking.

**Responsibilities**

* World state representation
* Entity stepping
* Action application
* Reward calculation
* Episode termination

**Non-responsibilities**

* UI
* real-time behavior
* networking
* persistence

The simulator exists solely to produce **state → action → reward** transitions.

---

## Training

### `training/`

Contains learning algorithms that operate on the simulator.

Current scope is intentionally conservative:

* tabular methods (e.g. Q-learning)
* small, interpretable models
* fast iteration

Training is designed to complete in minutes, not hours.

The goal is **behavioral competence**, not optimal play.

---

## Export

### `export/`

Handles conversion of trained artifacts into a format usable by the Go runtime.

Typical outputs:

* JSON policy tables
* lightweight model weights
* metadata (state encoding version, action space)

Exported policies are:

* immutable
* versioned
* loaded at runtime start

The runtime never trains or mutates policies.

---

## Shared Contracts

The tools rely on schemas defined in `/specs`:

* `actions.json`
* `state.json`
* `events.json`

These contracts ensure:

* consistent state encoding
* identical action semantics
* reproducible behavior across languages

Any change to the game rules must be reflected here.

---

## Environment Setup (Using `uv`)

All Python tooling uses **`uv`**.

From the `tools/` directory:

```bash
uv venv
uv sync
```

This creates and synchronizes a local virtual environment.

---

## Running Training

Example:

```bash
uv run python training/train.py
```

This will:

1. Initialize a headless simulator
2. Run multiple episodes
3. Train an agent
4. Produce an intermediate policy artifact

---

## Exporting a Policy

```bash
uv run python export/policy_export.py
```

This converts training output into a static policy file suitable for the Go runtime.

---

## Design Constraints

The tools intentionally avoid:

* online inference
* tight coupling to the runtime
* complex ML frameworks
* hidden state or magic behavior

If a behavior cannot be explained, it is considered a failure.

---

## Debugging Philosophy

If an agent behaves unexpectedly:

1. replay the rollout
2. inspect the transitions
3. verify the reward signal
4. compare against scripted behavior

Learning is treated as **data processing**, not intuition.

---

## Summary

This directory exists to answer a single question:

> *What happens when an entity is allowed to learn inside the rules of `Nightshade`?*
# Contributing

Forks and contributions are welcome — that is what the GPL is for. There is a
lot of room to build here: more recipes, plugin management, backup/restore of
demo data, pre-installed image variants, UI improvements — and AI coding tools
make all of that very approachable (see `AGENTS.md`, written to get an AI
assistant productive in this codebase quickly).

Setting up a dev environment and the build loop are in [DEV.md](DEV.md); the
architecture, invariants, testing workflow and extension points in
[AGENTS.md](AGENTS.md).

## Ground rules

- **Build & check**: `make build test vet fmt-check` must pass; `make image`
  + `make run` (rootful podman) is the smoke test. Full end-to-end
  verification steps are in `AGENTS.md`.
- **Keep it dependency-free**: the Go module is standard library only. Talk to
  [mudev](https://github.com/mutms/mudev) as a binary, do not import it.
- **AI-assisted contributions are fine** — that is how most of this project
  was written — but you must review, understand, and stand behind what you
  submit, same as the maintainers do (see the AI disclosure in README.md).
- **Keep the legal notices**: the web UI footer is a GPL-3.0 §5(d)
  Appropriate Legal Notice. When you fork, add your copyright next to the
  existing one rather than replacing it.
- **Bug reports**: paste the whole block from the web UI's `/debug` page —
  it contains the service states and log tails we need.

## License

By contributing you agree your work is licensed under
[GPL-3.0 or later](LICENSE), like the rest of the project.

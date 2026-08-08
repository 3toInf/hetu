# Hetu

Unified workspace for AI Coding sessions. v0.1 manages local Claude Code sessions: discover, browse, resume, and send prompts.

## Build
```bash
make build      # -> bin/hetu, bin/hetud
make test
make dist       # cross-compile linux/darwin amd64/arm64
```

## Run
```bash
./bin/hetud serve &        # or: it auto-starts when a CLI command runs
./bin/hetu discover        # scan ~/.claude/projects
./bin/hetu sessions        # list sessions (project-first)
./bin/hetu resume <id>     # daemon takes over the session
./bin/hetu send <id> "…"   # send a prompt
```

## Layout
See `docs/superpowers/specs/2026-08-08-hetu-mvp-design.md`.

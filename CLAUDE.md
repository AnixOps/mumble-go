# mumble-go

Phase 1 Go implementation of a headless Mumble client/library.

## Scope
- TLS connection and handshake
- Version / Authenticate / ServerSync
- Channel and user state sync
- Text messages and basic moderation commands
- Audio receive/send pipeline

## Not in phase 1
- Desktop UI
- Overlay
- Plugin ABI compatibility
- Native audio device integration

## Babysitter

This project uses Babysitter for orchestrating complex workflows.

### Commands

- `/babysitter:call <task>` - Start a babysitter-orchestrated run for a complex task
- `/babysitter:plan <task>` - Generate a detailed execution plan without running
- `/babysitter:yolo <task>` - Run in fully autonomous mode (no breakpoints)
- `/babysitter:forever <task>` - Run a recurring task on a schedule

### Installed Skills

- `babysit` - Core orchestration skill for managing complex workflows

### Processes

- `cradle/project-install` - Project setup and onboarding (already completed)
- `gsd/iterative-convergence` - Recommended for iterative improvements
- `methodologies/TDD` - Recommended for Streaming Sender v1.0 completion

### Methodology

This project operates in **autonomous mode** with low breakpoint tolerance.
Babysitter will make decisions without asking for confirmation on routine tasks.

### When to Use Babysitter

- Complex multi-step tasks (e.g., implementing new protocol features)
- Tasks requiring multiple iterations and verification
- Documentation improvements with multiple phases
- TDD-based development for core components

### Development Workflow

Standard Go development:
```bash
go mod download
go vet ./...
go test -v -race ./...
go build ./...
```

# fusion-runner

## Project purpose
Go binary (`cmd/runner`) that acts as the entrypoint for all runner container images in fusion-weave.
Dispatches to a type-specific handler based on `WEAVE_RUNNER_TYPE` env var injected by fusion-flux.

## Build
`go build ./...` — no external dependencies, go.sum is intentionally empty
Docker images live in `dockerfiles/<type>/Dockerfile` (multi-stage: `golang:1.25-alpine` builder + runtime base)
Build into minikube directly: `eval $(minikube docker-env) && docker build -f dockerfiles/python3.12/Dockerfile -t fusion-runner-python312:local .` — no `minikube image load` needed; building inside the daemon makes the image immediately available.

## Minikube image delivery
No registry addon — images use tag `:local` with `imagePullPolicy: IfNotPresent`. Always build inside minikube's Docker daemon (`eval $(minikube docker-env) && docker build ...`) — `minikube image load` is unreliable when the tag already exists.
Matches the pattern used by fusion-forge (`fusion-venv-builder:local`, `fusion-forge:local`).

## Runner types
Each type has its own file in `internal/runner/`. Add a new type: create `<type>.go`, implement `Runner` interface, add a case to `New()` in `runner.go`.
`StreamlitRunner` embeds `PythonRunner` — call `r.PythonRunner.Setup()` first, then add framework-specific env vars.

## Python venv-pack quirks
`venv-pack` archives contain `bin/python3.X` as an absolute symlink to the build-host path (e.g. `/usr/bin/python3.12`).
At container start the runner rewrites these to the container's interpreter via `exec.LookPath`. See `fixPythonSymlinks()` in `python.go`.

Archives produced by fusion-forge have a top-level `venv/` prefix (entries are `venv/bin/...` not bare `bin/...`).
`extractVenv()` in `archive.go` peeks at the first non-empty tar entry's path component to detect this — do NOT use `hdr.Typeflag == TypeDir`, venv-pack does not set it consistently.

## Archive debugging
Inspect archive structure: `tar -tzf <archive> | head -20` — check top-level layout when debugging extraction issues (`venv/bin/...` vs bare `bin/...`).

## WEAVE_* env vars
All injected by fusion-flux from `metadata.yaml` — see `tmp_create_docker.md` for the full reference.
`WEAVE_PORT` comes from `runner.port`; `ENTRYPOINT` comes from `runner.args.ENTRYPOINT`.
Module path: `fusion-platform.io/fusion-runner` (matches fusion-flux convention, not github.com/... like fusion-bff).

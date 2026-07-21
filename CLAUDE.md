# fusion-runner

## Project purpose
Go binary (`cmd/runner`) that acts as the entrypoint for all runner container images in fusion-weave.
Dispatches to a type-specific handler based on `WEAVE_RUNNER_TYPE` env var injected by fusion-flux.

## Cross-repo context
fusion-runner is the consumer end of an artifact pipeline: fusion-forge (builds venv-packs) → fusion-index (stores/serves them via REST) → fusion-flux (injects `WEAVE_*` env vars, optionally pre-downloads via its code-loader init container). Before building any feature that integrates with one of these, read its sibling CLAUDE.md (`../fusion-index`, `../fusion-flux`, `../fusion-forge`) — each is authoritative for its own REST API/CRD shapes.

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
`IndexPythonRunner` (`index.go`) embeds `PythonRunner` too — resolves `WEAVE_ARTIFACT`/`WEAVE_TAG` via fusion-index REST (base URL from `INDEX_URL` env var), downloads every file for the matched version into `MountPath`, then delegates to `PythonRunner.Setup()`.

## fusion-index REST response shapes
Inconsistent across list endpoints — `GET /api/v1/artifacts?name=` is paginated (`{"items":[...]}`), but `GET .../versions` and `GET .../versions/{semver}/files` return bare JSON arrays. Verify against `fusion-index/internal/api/handlers/*.go` before assuming a shape.
`VersionResponse.Version` is already a formatted `major.minor.patch` string and `FileResponse.DownloadURL` is already the ready-to-use relative download path — don't reconstruct either manually.

## Python venv-pack quirks
`venv-pack` archives contain `bin/python3.X` as an absolute symlink to the build-host path (e.g. `/usr/bin/python3.12`).
At container start the runner rewrites these to the container's interpreter via `exec.LookPath`. See `fixPythonSymlinks()` in `python.go`.

Archives produced by fusion-forge have a top-level `venv/` prefix (entries are `venv/bin/...` not bare `bin/...`).
`extractVenv()` in `archive.go` peeks at the first non-empty tar entry's path component to detect this — do NOT use `hdr.Typeflag == TypeDir`, venv-pack does not set it consistently.

## Archive debugging
Inspect archive structure: `tar -tzf <archive> | head -20` — check top-level layout when debugging extraction issues (`venv/bin/...` vs bare `bin/...`).

## Dockerfile preparation
Run `./prepare_Dockerfile.sh` after any Dockerfile change or when `enhance_docker_ssl` changes — it injects the certify SSL builder stage and the cert COPY into all `dockerfiles/*/Dockerfile` in-place.
Creates `Dockerfile_bak` backup before modifying. Idempotent: safe to re-run.

Placeholder comments in Dockerfiles (replaced by the script):
  `# ##CERTIFY_BUILDER##` — replaced with the certify Ubuntu stage
  `# ##CERTIFY_COPY##` — replaced with `COPY --from=certify /etc/ssl/certs /etc/ssl/certs`

`enhance_docker_ssl` (NOT in git, place at project root): body of the certify stage — RUN instructions that curl corporate CA certs. When absent the script injects a minimal `update-ca-certificates` fallback.

## Go module proxy & BuildKit cache
All Dockerfiles use `# syntax=docker/dockerfile:1` (required for `--mount=type=cache`).
`ARG GOPROXY=https://proxy.repo.internal/go,direct` is a mock placeholder — override at build time: `docker build --build-arg GOPROXY=https://real-proxy/go,direct ...`
`go mod download` and `go build` both mount `/root/go/pkg/mod` as a BuildKit cache — module downloads persist across builds on the dev machine without entering image layers.

## helpers/ (runtime helper libraries)
Sibling subprojects, one per language, for code that runs *inside* step pods (as opposed to `cmd`/`internal`, which build the runner entrypoint binary itself). First: `helpers/python/` (pip-installable, package `fusion_runner_helpers`). Future: `helpers/java/`, `helpers/go/` — not yet implemented, but the directory layout anticipates them.
`helpers/python/fusion_runner_helpers.auth.KeycloakAuth` — OAuth2 client-credentials token fetcher. Reads `CLIENT_ID`/`CLIENT_SECRET`/`TOKEN_URL` env vars (key names configurable via constructor args) from a Secret injected by fusion-flux's `WeaveChainSpec.authSecretRef` (see fusion-flux CLAUDE.md). No fusion-runner process-model change needed — `PythonRunner.Exec()` still `syscall.Exec`-replaces itself; the library does the OAuth exchange itself and caches/refreshes the token in-process on each `get_token()` call, so it works fine as a plain import even though the runner process becomes the Python process.
Dev loop: `cd helpers/python && python3.12 -m venv .venv && .venv/bin/pip install -e . && .venv/bin/python -m unittest discover -s tests -v`. Distributed via `requirements.txt` (not baked into the base runner image), so step authors opt in per-artifact.

## WEAVE_* env vars
All injected by fusion-flux from `metadata.yaml` — see `tmp_create_docker.md` for the full reference.
`WEAVE_PORT` comes from `runner.port`; `ENTRYPOINT` comes from `runner.args.ENTRYPOINT`.
Module path: `fusion-platform.io/fusion-runner` (matches fusion-flux convention, not github.com/... like fusion-bff).

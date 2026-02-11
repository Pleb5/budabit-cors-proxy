# AGENTS

## Scope
- This repo hosts a minimal Go CORS proxy plus nginx/certbot Docker setup.
- Instructions here are for agentic coding tools working in this workspace.

## Repository layout
- `corsproxy/` contains the Go proxy service.
- `nginx/` contains TLS and reverse-proxy configuration.
- `docker-compose.yml` wires nginx, corsproxy, and certbot.
- `flake.nix` provides a Nix dev shell with Go tools.

## Local dev setup
- `nix develop` to enter the dev shell (optional but recommended).
- If `go.mod` is missing, initialize it: `go mod init example.com/budabit-cors-proxy` then `go mod tidy`.
- `export ALLOWED_ORIGINS="localhost"` to allow local testing.
- `go run ./corsproxy` to start the proxy server.
- `curl "http://localhost:8080/github.com/owner/repo"` for a quick check.

## Build
- `go build ./corsproxy` to build the package.
- `go build -o corsproxy ./corsproxy` to produce a local binary.
- `docker compose build corsproxy` to build the container.
- `docker compose up -d nginx corsproxy` to run the stack.

## Lint and format
- `gofmt -w corsproxy/*.go` for formatting.
- `goimports -w corsproxy/*.go` to fix imports (if installed).
- `golangci-lint run ./...` for linting (available in `flake.nix`).
- `go vet ./...` for basic static checks.

## Tests
- `go test ./...` runs all tests (none exist yet).
- Single package: `go test ./corsproxy -run TestName -count=1`.
- Strict regex: `go test ./corsproxy -run '^TestName$' -count=1 -v`.
- Go does not run tests by file name; use `-run` to filter.
- Add tests under `corsproxy/*_test.go` when introducing new behavior.

## Docker and nginx
- `docker compose up -d nginx corsproxy` runs the services.
- `docker compose logs -f corsproxy` tails proxy logs.
- `docker compose exec nginx nginx -s reload` reloads nginx.
- Cert issuance example (from `README.md`):
  `docker compose run --rm --entrypoint certbot certbot certonly --webroot -w /var/www/html -d corsproxy.budabit.club --email you@example.com --agree-tos --no-eff-email --non-interactive`
- ACME challenges are served from `/.well-known/acme-challenge/`.

## Runtime configuration
- `ALLOWED_ORIGINS` is a comma-separated allowlist.
- Use `*` to allow all origins only for trusted scenarios.
- `localhost` and `127.0.0.1` are always accepted by the proxy.

## Code style (Go)
- Use `gofmt` for formatting; do not hand-align spacing.
- Tabs are standard; keep line lengths reasonable.
- Imports should be grouped by `goimports`: standard, third-party, local.
- Keep functions small; prefer early returns for error paths.
- Use `mixedCaps` for identifiers; capitalize initialisms (URL, HTTP, ID).
- File names should be simple and lower-case; keep `main.go` as the entry point.
- Avoid package-level state except configuration like `allowedOrigins`.
- Use explicit types when it improves clarity; rely on inference for locals.
- Use `const` for repeated header names or tokens.
- Only add comments when logic is non-obvious.

## Error handling
- Check every error; return a clear status and message.
- Use `http.Error` for client-facing failures.
- Log unexpected errors; avoid `panic`.
- In `main`, `log.Fatal` is acceptable for missing required config.

## Logging
- Use `log.Printf`/`log.Println` for operational logs.
- Never log secrets (Authorization headers, tokens).
- Keep logs single-line and concise.

## HTTP and proxy behavior
- Preserve request headers unless explicitly filtering.
- Add CORS headers for allowed origins and preflight responses.
- Handle `OPTIONS` by returning 200 without proxying upstream.
- Do not auto-follow redirects; rewrite `Location` to proxy paths.
- Do not forward upstream `Content-Length`; let Go compute it.

## Naming and structure
- Use descriptive helper names (for example: `allowCorsForOrigin`).
- Use `req`/`resp` for HTTP objects and `w` for `ResponseWriter`.
- Keep `main` focused on wiring and configuration.

## Security and correctness
- Treat the proxy as trusted but validate inputs.
- Return 400 on malformed paths or missing targets.
- Keep CORS allowlist logic explicit and easy to audit.
- Be careful when changing URL rewriting or redirect behavior.

## Formatting, tools, and CI
- There is no CI configuration yet; keep local checks green.
- If adding lint or format configs, update this file.

## Cursor and Copilot rules
- No `.cursor/rules/`, `.cursorrules`, or `.github/copilot-instructions.md` found.
- If added later, include them verbatim in this file.

## Agent workflow
- Read `README.md` for deployment and TLS steps.
- Prefer editing `corsproxy/main.go` for proxy behavior changes.
- Update `nginx/conf.d/corsproxy.conf` when hostnames change.
- Keep docker-compose service names stable (`corsproxy`, `nginx`, `certbot`).

## Suggested change checklist
- Run `gofmt` on modified Go files.
- Run `go test ./...` if tests exist.
- Run `golangci-lint run ./...` when available.
- Rebuild the docker image if runtime behavior changed.

## Notes for single-test runs
- `go test ./corsproxy -run TestName -count=1` runs one test.
- `go test ./corsproxy -run 'TestName|TestOther' -count=1` runs multiple by regex.
- Use `-v` for verbose output when debugging.

## Common commands (copy/paste)
- `nix develop`
- `export ALLOWED_ORIGINS="localhost"`
- `go run ./corsproxy`
- `curl "http://localhost:8080/github.com/owner/repo"`
- `go build -o corsproxy ./corsproxy`
- `gofmt -w corsproxy/*.go`
- `golangci-lint run ./...`
- `go test ./...`
- `docker compose up -d nginx corsproxy`
- `docker compose logs -f corsproxy`

## File references
- `corsproxy/main.go` contains the proxy server.
- `corsproxy/Dockerfile` builds the Go binary.
- `docker-compose.yml` wires nginx/corsproxy/certbot.
- `nginx/conf.d/corsproxy.conf` defines TLS and proxying.
- `README.md` documents deployment steps.

## Conventions for new code
- Prefer standard library packages when possible.
- Add new deps to `go.mod`/`go.sum` (not present yet); keep them minimal.
- If introducing config, source from env vars and document in README.
- Keep ASCII-only content unless a file already uses Unicode.

## When adding tests
- Put tests in `corsproxy/*_test.go` with package `main`.
- Use table-driven tests for request/response cases.
- Avoid network calls; use `httptest` servers.
- Cover CORS allowlist behavior and redirect rewriting.

## Maintenance
- Update this file when build/lint/test commands change.
- Keep instructions accurate for agentic tools.
- If repository structure changes, update the layout section.

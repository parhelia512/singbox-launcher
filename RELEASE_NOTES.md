# Release Notes
See [docs/release_notes/0-7-0.md](docs/release_notes/0-7-0.md) for detailed release notes 

## Testing
- Validate FREE VPN feature (dialog, links insertion)

## CI / CD
- Updated CI/CD logic: added cross-platform Go build cache (GOCACHE and module cache), skipped `go mod tidy` in CI, and standardized action versions (`actions/checkout@v6`, `actions/setup-go@v6`) to speed up and stabilize builds.
- Added Windows-friendly cache paths for Go build and modules as per `actions/cache` examples.



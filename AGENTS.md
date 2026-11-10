# SNClient

SNClient (Secure Naemon Client) is a general-purpose monitoring agent for multiple platforms.
It is designed to replace NRPE and NSClient++.
It supports Naemon/Nagios plugins and includes exporters for Prometheus.

## Basic Information

- **Language:** Written in Go.
- **Supported Platforms:** Linux, Windows, FreeBSD, and macOS.
- **Protocols Supported:** Prometheus HTTP(S), NRPE (v2/v4), and the NSCP REST API via HTTP(S).
- **Checks/Plugins:** Includes system checks for CPU,
  memory, disks, network, processes, and services. It can also run external scripts.
  Located in `pkg/snclient/check_*.go`.
- **Documentation:** The official documentation is in `docs/`.
  Use `make docs` to generate the files in `docs/checks/commands/` and `docs/checks/plugins/`.

## Rules

- Write all new or updated documentation and text in STE80 style (80% of the way to ASD-STE100).
  Use plain words, active voice, and one idea per sentence.
  Aim for at most 20 words per instruction and 25 words per descriptive sentence.
  Preserve technical facts, uncertainty, identifiers, commands, and configuration values.
  Keep existing single-line `.ini` descriptions on one line.
- Use `make fmt` instead of `go fmt`.
- Use `make test` instead of `go test`.
- Use `make racetest` instead of `go test -race`.
- Use `make golangci` after making code changes and fix the linter errors.
- Use `make docs` instead of editing `docs/checks/commands/*.md` or `docs/checks/plugins/`.
- Do not create additional Markdown files unless explicitly requested.
- Keep comments short. Do not comment the obvious. Only comment exceptional non-obvious things.
- Avoid duplicate code.
- Do not add unnecessary tests.
- Do not use "t.Parallel()". It makes test output less readable.
- Existing metrics / performancedata must not change.
- Follow clean code principles. Avoid bad practices such as `goto`.
- The final acceptance test is `make citest`.

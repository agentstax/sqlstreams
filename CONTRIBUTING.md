# Contributing

Setup is in [DEVELOPING.md](DEVELOPING.md). Code rules are in
[CONVENTIONS.md](CONVENTIONS.md) and are enforced in review. Vulkan is
pre-v1, so the public API can change between releases.

## Bugs

Open an issue with the Vulkan and Postgres versions and the error or log
line.

## Changes

Open an issue before writing code, using the
[Bug](https://github.com/agentstax/vulkan/issues/new?template=bug.yml) or
[Feature](https://github.com/agentstax/vulkan/issues/new?template=feature.yml)
form. The maintainer reads it within a week or two, asks for anything
missing, and labels it. Ideally, do not start coding work until the label lands:

| Label | Meaning |
| --- | --- |
| `accepted` | go ahead |
| `help wanted` | accepted and designed, nobody is working on it |
| `roadmap` | not planned yet; tracked in [docs/ROADMAP.md](docs/ROADMAP.md) |
| closed | no |

### Fixing a bug

1. Open a [bug issue](https://github.com/agentstax/vulkan/issues/new?template=bug.yml).
2. Wait for `accepted`.
3. Open a pull request with a test that fails before the fix.

### Adding a feature

1. Search [docs/DECISIONS.md](docs/DECISIONS.md) and the rejected line in
   [docs/DECISION_MAP.md](docs/DECISION_MAP.md). Rejected ideas need new
   evidence to be reconsidered.
2. Open a [feature issue](https://github.com/agentstax/vulkan/issues/new?template=feature.yml)
   describing the problem, not a solution.
3. Wait for `accepted`.
4. The maintainer writes the decision record.
5. Create/Open the implementation pull request.

## Pull requests

The pull request template's checklist is the two lists below.

### Every pull request

- One change per pull request.
- `go fmt ./...` and `just verify` pass.
- Changed behavior has a test. Tests call the real datastore methods,
  never a copy of their SQL.
- Commits are signed off (`git commit -s`, the
  [DCO](https://developercertificate.org/)). There is no CLA.
- Do not edit decision records, HISTORY.md, or ROADMAP.md. The maintainer
  updates those at merge.

### Review

- AI tools are fine, but you have to understand and be able to explain
  everything you submit.
- Expect a first review within a week or two.

## License

Apache-2.0 ([LICENSE](LICENSE)). Contributions are licensed the same.

# Releasing

Releases are automated via GoReleaser and GitHub Actions. Pushing a tag triggers the release pipeline.

## Steps

1. Make sure all changes are merged to `main`.

2. Choose a version following [semver](https://semver.org/) (e.g. `v1.2.3`).

3. Tag and push:

   ```bash
   git tag cli/v1.2.3
   git push origin cli/v1.2.3
   ```

4. GitHub Actions runs the `CLI Release` workflow, which:
   - Builds binaries for `darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`
   - Creates archives: `qfex_v1.2.3_darwin_arm64.tar.gz`, etc.
   - Generates `checksums.txt`
   - Publishes a GitHub Release with all assets attached

5. Monitor the run at `https://github.com/QFEX-org/cli/actions`.

## Artifacts

Each release produces:

| File | Description |
|------|-------------|
| `qfex_v1.2.3_darwin_amd64.tar.gz` | macOS Intel |
| `qfex_v1.2.3_darwin_arm64.tar.gz` | macOS Apple Silicon |
| `qfex_v1.2.3_linux_amd64.tar.gz` | Linux x86-64 |
| `qfex_v1.2.3_linux_arm64.tar.gz` | Linux ARM64 |
| `checksums.txt` | SHA256 checksums for all archives |

## Fixing a bad release

Tags cannot be re-used. To re-release the same logical version:

1. Delete the tag locally and remotely:
   ```bash
   git tag -d cli/v1.2.3
   git push origin :refs/tags/cli/v1.2.3
   ```
2. Delete the draft/published GitHub Release if one was created.
3. Re-tag with a patch bump (e.g. `cli/v1.2.4`) instead.

## Local dry run

To test the GoReleaser config without publishing:

```bash
cd cli
goreleaser release --snapshot --clean
```

Artifacts are written to `cli/dist/`. Requires [GoReleaser](https://goreleaser.com/install/) installed locally.

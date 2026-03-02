---
name: release
description: Create a tagged release with a Keep a Changelog entry and trigger the GoReleaser workflow.
---

Your job is to guide the user through creating a release for this project. Releases use annotated git tags with a `v` prefix (e.g. `v1.1.0`) and are built by the GoReleaser GitHub Actions workflow. Release notes are set explicitly via `gh release edit` — do NOT rely on the workflow to populate them.

## Rules

- Always create an **annotated** tag (`git tag -a`), never a lightweight tag.
- Tag format is `vMAJOR.MINOR.PATCH` following [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
- The changelog MUST follow [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/) format.
- Valid changelog categories: Added, Changed, Deprecated, Removed, Fixed, Security.
- DO NOT push the tag until the user confirms.
- DO NOT skip asking the user for the version number, tag annotation message, and changelog review.

## Steps

1. **Gather context**
   - Check the current branch and ensure it is clean (`git status`).
   - List existing tags (`git tag --sort=-v:refname`) to determine the next version.
   - Collect the commit log since the last tag (or all commits if this is the first release).

2. **Ask the user**
   - What version number to use.
   - What the annotated tag message should say.
   - Whether the auto-generated changelog looks correct or needs edits.

3. **Categorize commits** into Keep a Changelog sections:
   - `Added` — new features and capabilities.
   - `Changed` — changes to existing functionality.
   - `Deprecated` — features marked for removal.
   - `Removed` — features that were removed.
   - `Fixed` — bug fixes.
   - `Security` — vulnerability fixes.
   - Exclude commits prefixed with `docs:`, `test:`, `ci:`, or merge commits.

4. **Update CHANGELOG.md**
   - Prepend the new release section under the `# Changelog` header.
   - Add a reference link at the bottom of the file using the Keep a Changelog format:
     - First release: `[1.0.0]: https://github.com/jrswab/axe/releases/tag/v1.0.0`
     - Subsequent releases: `[1.1.0]: https://github.com/jrswab/axe/compare/v1.0.0...v1.1.0`
   - Commit the changelog update before tagging.

5. **Create the annotated tag** on the changelog commit.

6. **Push** the commit and tag to origin only after the user confirms. The `v*` tag push triggers the GoReleaser workflow which builds binaries and creates the GitHub release.

7. **Set release notes via `gh`**
   - After the tag is pushed, extract the version's section from `CHANGELOG.md` (excluding the heading and reference link footer) and write it to a temporary file.
   - Run `gh release edit vX.Y.Z --notes-file <tmp-file>` to set the release description.
   - This is the authoritative source for release notes — do NOT rely on the workflow's awk extraction.

## Reference

- GoReleaser config: `.goreleaser.yml` (auto-changelog is disabled)
- Release workflow: `.github/workflows/release.yml` (builds binaries; do NOT rely on it for release notes)
- Changelog: `CHANGELOG.md`

---
name: release-version
description: Use when preparing or publishing a new semantic version release, updating bilingual changelogs, creating Git tags, or publishing GitHub/Gitee release pages for this repository.
---

# Release Version

## Overview

Create releases with a strict `vx.y.z` tag, produce bilingual changelog entries from actual Git history, publish both the `docs` submodule update and repository tag, and create the GitHub and Gitee Release page entries in one controlled workflow.

Run the workflow from the repository root. Read [references/changelog-style.md](references/changelog-style.md) before drafting the human-facing update notes.

## Workflow

1. Validate the requested version.
2. Inspect the repository and determine the comparison range.
3. Draft bilingual changelog entries from the actual diff.
4. Commit and push the `docs` submodule.
5. Commit the parent repository update if the submodule pointer changed.
6. Create and push the annotated tag.
7. Create GitHub and Gitee Release page entries for the tag.
8. Verify both remote tags and both Release pages.

Do not skip the repository inspection step. Release notes must come from the real diff between tags, not from guesswork.

## Validate The Version

- Accept only tags that match `^v\d+\.\d+\.\d+$`.
- Reject date-style tags such as `v20260414`.
- Confirm the target tag does not already exist locally or on any configured remote.
- Prefer the latest reachable semver tag as the previous release tag.
- If no earlier semver tag exists, fall back to the latest reachable tag of any format and state that fallback in the changelog drafting notes.

Use the helper script first:

```bash
python3 .codex/skills/release-version/scripts/collect_release_context.py \
  --repo . \
  --tag v1.2.3
```

If the caller already specifies the previous tag, pass it explicitly:

```bash
python3 .codex/skills/release-version/scripts/collect_release_context.py \
  --repo . \
  --tag v1.2.3 \
  --previous-tag v1.2.2
```

If the repository-local helper is unavailable, fall back to `~/.codex/skills/release-version/scripts/collect_release_context.py`.

## Inspect The Repository

- Check `git status --short` in the parent repo.
- Check `git -C docs status --short` in the `docs` submodule.
- Read the JSON output of `collect_release_context.py`.
- Use the commit list, changed files, and insertions/deletions to decide what is user-visible.
- Prioritize behavior changes, new features, fixes, migrations, API changes, configuration changes, and documentation changes that matter to adopters.
- Ignore pure formatting churn unless it changes usage.

If the working tree contains unrelated changes that would be risky to include in the release, stop and ask the user before proceeding.

## Draft The Changelog

Update these files:

- `docs/zh/docs/changelog.md`
- `docs/en/docs/changelog.md`

Prepend a new entry using this exact structure:

```md
## ${tag} (${yyyy-MM-dd})

### 更新内容

${content}

### 发布地址

- Github: <https://github.com/huabeitech/agent-desk/releases/tag/${tag}>
- Gitee: <https://gitee.com/huabeitech/agent-desk/releases/tag/${tag}>
```

For the English file, keep the same links and heading level, but translate the section heading and content naturally:

```md
## ${tag} (${yyyy-MM-dd})

### Updates

${content}

### Release Links

- Github: <https://github.com/huabeitech/agent-desk/releases/tag/${tag}>
- Gitee: <https://gitee.com/huabeitech/agent-desk/releases/tag/${tag}>
```

Changelog writing rules:

- Write concise, user-facing summaries instead of raw commit subjects.
- Keep Chinese and English entries semantically aligned.
- Prefer 3-6 bullets unless the release is extremely small.
- Group related changes into a single bullet when that reads better.
- Mention compatibility-sensitive changes explicitly.
- If the comparison baseline is a non-semver fallback tag, note that in your private reasoning, not in the public changelog unless the user asks for it.

## Commit And Push The Docs Submodule

After editing the changelog files:

1. Run `git -C docs status --short`.
2. Review the diff with `git -C docs diff -- zh/docs/changelog.md en/docs/changelog.md`.
3. Commit inside the `docs` submodule with a focused message such as `docs: update changelog for v1.2.3`.
4. Push the `docs` submodule commit to its remote branch.

Branch rule:

- If `docs` is on a local branch, push that branch.
- If `docs` is detached, push `HEAD` to `origin/main` unless the repository clearly uses another default branch.

## Commit The Parent Repository

If the `docs` submodule pointer changed in the parent repository, commit it before tagging. Otherwise the release tag will not reference the new changelog revision.

Recommended flow:

```bash
git status --short
git add docs
git commit -m "chore: update docs submodule for v1.2.3"
```

Only include unrelated parent-repo changes if the user explicitly wants them in the release commit.

## Create And Push The Tag

Create an annotated tag after the repository state is ready:

```bash
git tag -a v1.2.3 -m "Release v1.2.3"
```

Push the commit branch first if needed, then push the tag to every configured remote that should publish releases:

```bash
git push github HEAD
git push origin HEAD
git push github v1.2.3
git push origin v1.2.3
```

Adjust the branch name if `HEAD` is not tracking the intended release branch.

## Create GitHub And Gitee Releases

Pushing tags is not enough. The release is incomplete until both Release pages exist:

- GitHub: `https://github.com/huabeitech/agent-desk/releases/tag/${tag}`
- Gitee: `https://gitee.com/huabeitech/agent-desk/releases/tag/${tag}`

Use the same concise release notes derived from the changelog. Prefer a bilingual body with Chinese first and English second.

Required credentials:

- GitHub: `GITHUB_TOKEN` or `GH_TOKEN` with access to `huabeitech/agent-desk` and permission to create releases. For a fine-grained PAT, use an organization-allowed lifetime and grant the repository at least `Contents: Read and write` plus `Metadata: Read`.
- Gitee: `GITEE_ACCESS_TOKEN` or `GITEE_TOKEN` with release write access to `huabeitech/agent-desk`.

Never print tokens in command output or final responses. If the user pastes a token into the conversation, use it only for the requested release operation and recommend rotation after use.

Build the release body from the new changelog entry, for example:

```bash
mkdir -p /tmp/agent-desk-release
awk 'BEGIN{p=0} /^## v1\.2\.3 /{p=1; next} /^## v[0-9]/{if(p) exit} p{print}' \
  docs/zh/docs/changelog.md | sed '/^### 发布地址/,$d' > /tmp/agent-desk-release/v1.2.3-zh.md
awk 'BEGIN{p=0} /^## v1\.2\.3 /{p=1; next} /^## v[0-9]/{if(p) exit} p{print}' \
  docs/en/docs/changelog.md | sed '/^### Release Links/,$d' > /tmp/agent-desk-release/v1.2.3-en.md
{
  printf '## 更新内容\n\n'
  sed '1,/^### 更新内容$/d' /tmp/agent-desk-release/v1.2.3-zh.md
  printf '\n## Updates\n\n'
  sed '1,/^### Updates$/d' /tmp/agent-desk-release/v1.2.3-en.md
} > /tmp/agent-desk-release/v1.2.3-release-body.md
```

Create the GitHub Release:

```bash
token="${GITHUB_TOKEN:-$GH_TOKEN}"
curl -sS -o /tmp/github_release_v1.2.3.json -w '%{http_code}' \
  -X POST https://api.github.com/repos/huabeitech/agent-desk/releases \
  -H "Authorization: Bearer ${token}" \
  -H 'Accept: application/vnd.github+json' \
  -H 'X-GitHub-Api-Version: 2022-11-28' \
  -H 'Content-Type: application/json' \
  -d @<(jq -n --rawfile body /tmp/agent-desk-release/v1.2.3-release-body.md \
    '{tag_name:"v1.2.3", target_commitish:"main", name:"v1.2.3", body:$body, draft:false, prerelease:false}')
```

Create the Gitee Release:

```bash
token="${GITEE_ACCESS_TOKEN:-$GITEE_TOKEN}"
curl -sS -o /tmp/gitee_release_v1.2.3.json -w '%{http_code}' \
  -X POST https://gitee.com/api/v5/repos/huabeitech/agent-desk/releases \
  -H 'Content-Type: application/json' \
  -d @<(jq -n --rawfile body /tmp/agent-desk-release/v1.2.3-release-body.md --arg token "${token}" \
    '{access_token:$token, tag_name:"v1.2.3", target_commitish:"main", name:"v1.2.3", body:$body, prerelease:false}')
```

If creation returns `422`/already exists, fetch the existing release and verify it references the target tag before treating it as complete. If GitHub returns `Resource not accessible by personal access token`, inspect the API message and ask for a token that satisfies the organization policy and repository permissions.

## Final Verification

- Confirm `git rev-parse v1.2.3^{tag}` succeeds.
- Confirm `git ls-remote --tags github v1.2.3` and `git ls-remote --tags origin v1.2.3` show the new tag.
- Confirm `curl -sS -o /tmp/github_release_verify.json -w '%{http_code}' https://api.github.com/repos/huabeitech/agent-desk/releases/tags/v1.2.3` returns `200`.
- Confirm `curl -sS -o /tmp/gitee_release_verify.json -w '%{http_code}' https://gitee.com/api/v5/repos/huabeitech/agent-desk/releases/tags/v1.2.3` returns `200`.
- Confirm the `docs` submodule remote contains the changelog commit.
- Confirm both parent and `docs` working trees are clean.
- Summarize the previous tag used for comparison, the files updated, the commit hashes created, the remotes pushed, and the GitHub/Gitee Release URLs.

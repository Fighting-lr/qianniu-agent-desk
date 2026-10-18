# Changelog Style

Use this guide when drafting `docs/zh/changelog.md` and `docs/en/changelog.md`.

## Goal

Turn Git history into short release notes that explain what changed for adopters.

## Keep

- New user-facing features.
- Bug fixes with clear impact.
- Breaking or compatibility-sensitive changes.
- API, configuration, deployment, model, workflow, or schema changes that affect usage.
- Important documentation updates when they unlock new workflows.

## Drop Or Compress

- Pure refactors with no visible effect.
- Formatting-only changes.
- Internal rename churn.
- Mechanical dependency updates unless they fix a real issue.

## Chinese Style

- Use concise bullets.
- Prefer product or workflow language over commit jargon.
- Start with the effect, then mention the area if needed.
- Keep terms consistent across bullets.

Example:

```md
- 优化会话列表查询与筛选逻辑，减少后台定位问题时的人工排查成本。
- 修复消息发送链路中的异常处理，避免部分失败场景下页面状态不同步。
```

## English Style

- Mirror the Chinese meaning instead of translating word by word.
- Use direct release-note phrasing.
- Prefer active wording and concrete impact.

Example:

```md
- Improved conversation list querying and filtering so operators can locate problem cases faster.
- Fixed error handling in the message send flow to prevent UI state from drifting after partial failures.
```

## Grouping Heuristics

- Merge multiple commits into one bullet when they deliver one outcome.
- Separate bullets when the audience or impact differs.
- Keep both language versions aligned in bullet count when possible.

## Before Finalizing

- Re-check that every bullet is supported by the diff.
- Remove statements that depend on assumptions you cannot verify from code, tests, or commits.
- Keep the notes short enough to scan in under a minute.

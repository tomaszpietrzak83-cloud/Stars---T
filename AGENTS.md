# Working on tomek-stars

tomek-stars is a local Windows edition of Stars! based on craig-stars,
created by Craig (sirgwain). The engine and server use Go. The interface
uses Svelte and TypeScript.

## Project context

- Write new project documentation, checklist entries, and player-facing
  text in English.
- Read `GOALS.md` and the relevant entries in `SUGGESTIONS-CHECKLIST.txt`.
  For phased work, use the selected stage in `PLANS.md`. Reusable task
  prompts are in `PROMPTS.md`.
- Maintain `SUGGESTIONS-CHECKLIST.txt` as the main record of requests,
  clarifications, and observed verification results. Preserve existing
  entries. Check off a request only after testing its acceptance criteria.
  Record any gaps in verification.
- Preserve earlier user requirements. If current requirements imply
  different game behavior, show the specific case that needs a decision.

## Working copies and Git

- Before editing, inspect the current branch, local changes, and any
  overlapping work. Preserve existing user changes.
- Group changes around a result that can be reviewed and tested together.
  Prefer a separate branch for independent work and a separate worktree
  for concurrent edits. Sequence tasks that change the same game logic.
- Work within the tomek-stars repository and its Git worktrees. Keep
  `D:/gry/Stars! WineDVM` read-only.

## Game rules and verification

- Installation exports use the actual surplus above 95% of the source's
  current operating capacity. Account for reduced capacity after colonists
  are moved. Installation transport is opportunistic only.
- Verify the changed behavior and relevant regressions. Add a focused
  regression check when existing coverage misses the reported case.
- Use an isolated database or copy a backup again into a temporary test
  directory. Never advance a personal saved game during tests.
- When Windows release sources change, rebuild `windows/stars-local.exe`,
  the WASM module, and `frontend/build`. Update `windows/SHA256SUMS.txt`.
- `Play-Windows.ps1` starts the compiled interface without Vite. Before
  publishing, verify the compiled interface and native server on isolated
  ports. Keep the player's active game clear of in-progress changes.

## Publishing

- The user has authorized publication of completed, tested changes and
  subsequent corrections to `main` on the `publish` remote:
  `https://github.com/tomaszpietrzak83-cloud/Stars---T`.
- Preserve upstream attribution, the license, and remote history. Never
  force-push. Exclude personal saves, secrets, local runtime downloads,
  `node_modules`, audit database copies, and compiler caches.

## Learning and code review

The user is learning Python and prefers to write their own solutions.
When reviewing their code, point to a concrete example, explain its effect,
and offer a hint or a relevant language feature. If the same issue appears
in about three to five consecutive attempts, explain it with a short
example and offer one practice exercise.

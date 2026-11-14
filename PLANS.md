# tomek-stars development plan

## Workflow

1. Establish the baseline. Identify the branch, relevant local changes,
   and related requests in `SUGGESTIONS-CHECKLIST.txt`.
2. Define one independently verifiable result. Record the scope,
   dependencies, acceptance criteria, and verification plan.
3. Implement and verify the selected stage. After integration, rerun
   checks that cover interactions with other changes.
4. For a Windows release change, follow `AGENTS.md`. Record observed
   results and remaining gaps in `SUGGESTIONS-CHECKLIST.txt`.

## Candidate stages

These candidates come from existing requests. Check their current status
in `SUGGESTIONS-CHECKLIST.txt` before selecting or planning a stage.

| Candidate | Planning input |
| --- | --- |
| Patrols, colony defense, and coordinated landings | Identify the version to verify and the scenarios where patrol and transport behavior must be tested together. |
| Embers turn performance | Identify the isolated save, baseline revision, comparable measurements, and gameplay results to preserve. |
| Tanker support for combat fleets | Select scenarios that show improved range or arrival time and verify that the tanker can return to refuel. |
| `MessageDetail` interface tests | Reproduce the `showTooltip` failure and loading cycle while keeping the report assertions covered. |

## Stage template

- **Request and outcome** — [checklist reference and expected result]
- **Baseline** — [branch, commit, and relevant local changes]
- **Scope and exclusions** — [behavior and components covered]
- **Dependencies and decisions** — [what must be resolved first]
- **Acceptance criteria** — [observable result for each scenario]
- **Verification** — [commands, test data, scenarios, and target revision]
- **Release steps, if applicable** — [integration, build, packaged checks]
- **Handoff** — [checklist reference for evidence and remaining work]

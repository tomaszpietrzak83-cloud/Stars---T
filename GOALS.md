# tomek-stars project goals

## Purpose

Develop a local Windows edition of Stars! with a playable single-player
experience and preserve upstream attribution and licensing.

## Desired outcomes

- Players can start the packaged game with its native server and compiled
  interface without building the source or signing in manually.
- Gameplay and turn automation follow agreed rules. Changes preserve
  existing campaign behavior outside their approved scope.
- Program updates preserve personal saves. Development tests use isolated
  data so a failed test cannot advance a player's campaign.
- Completed changes have evidence tied to the scenario and version tested.

## Acceptance criteria

- Each change satisfies the selected stage and the relevant requests in
  `SUGGESTIONS-CHECKLIST.txt`.
- A bug fix demonstrates the expected behavior in the reported scenario.
  A performance change includes a comparable measurement and checks
  gameplay behavior against the agreed baseline.
- For Windows release changes, the packaged components work together
  and the recorded checksums match the delivered files.
- Verification gaps remain visible in `SUGGESTIONS-CHECKLIST.txt`.

## Scope and open decisions

Development covers the Go engine and server, the Svelte/TypeScript
interface, and the packaged Windows edition. The original game
installation and personal saves are outside development and testing scope.
Operational constraints are recorded in `AGENTS.md`.

The order of open work depends on user priorities and technical
dependencies. Record unresolved gameplay decisions in the stage
that depends on them.

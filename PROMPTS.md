# Reusable prompts for tomek-stars

Replace the bracketed fields before use. Read `AGENTS.md`, `GOALS.md`,
`PLANS.md`, and the relevant entries in `SUGGESTIONS-CHECKLIST.txt`.
Start by inspecting the branch, local changes, and scope in read-only mode.

Before writing, summarize the proposed changes, affected files, and
verification plan. Work within the approved scope and obtain approval for
new work outside it. Check existing authorization before commits, pushes,
deployments, credential use, or external writes. The standing publication
authorization is recorded in `AGENTS.md`. Stop at the selected stage.

## 1. Repository orientation

Read `README.md` as well. For [topic], identify the entry point, existing
tests, and related requests. Report the branch and relevant local changes.
Distinguish findings from questions that need investigation. Use read-only
inspection.

## 2. Prepare a stage

Draft one stage for [request] using the template in `PLANS.md`. Include
existing work, dependencies, acceptance criteria, and verification.
Identify decisions I need to make. Present the draft in chat for approval.

## 3. Execute an approved stage

Implement [approved stage] within its recorded scope. For a regression,
where practical, demonstrate the failure first, make the test pass, then
clean up the affected code and rerun relevant checks. Report observed
results and any acceptance criteria still unverified.

## 4. Verify and gather evidence

Verify [stage] on [branch or commit] against its acceptance criteria.
Use isolated test data as described in `AGENTS.md`. Report the scenario
or command, tested revision, and observed outcome for each criterion.
Run the checks without changing source files.

## 5. Review or remediate

Review [diff or files] against [stage]. Identify concrete defects,
regression risks, and gaps in coverage. Give the location and likely
effect of each finding. If fixes are included in my request, apply them
within scope and rerun relevant checks. If I ask for learning support,
give me a hint and let me attempt the correction.

## 6. Close and hand off a stage

Compare [stage] with its acceptance criteria. Update the relevant
entries in `SUGGESTIONS-CHECKLIST.txt` within the approved scope.
Report the tested revision, observed results, open issues, and decisions
needed in the next chat. Finish after handing off this stage.

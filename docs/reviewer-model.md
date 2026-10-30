# Reviewer model

## Decision

Reviewers run on `claude-sonnet-5-5` (see the role table in the hub's plan). An epic can switch its reviewers to Opus in its own config if Sonnet misses problems.

## Why this and not a measured result

No reviewer run has been measured yet. The first epic has not run through the hub, so this is the plan's default, not evidence that Sonnet is strict enough. The plan lists it as an open question.

## How to check it

Record these for each reviewer verdict on the first real epic:

- verdict (PASS, REVISE, SPLIT, ESCALATE) and the reason it gave
- whether a later human review or a failing check found a problem the reviewer passed

Then decide:
- If Sonnet passes work that a human or a check later rejects, switch that epic's reviewers to Opus and note it here.
- If the REVISE rate is high and the REVISE reasons are not real, look at the prompt before changing the model.

Keep this file's decision in step with the role table.

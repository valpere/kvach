---
name: project_pr_workflow
description: Required PR workflow including Copilot review gate
type: project
---

# PR Workflow

Every change must follow this exact sequence:

1. **Create branch** — `git checkout -b <type>/<description>` (feat/, fix/, docs/, refactor/, test/)
2. **Stage and commit** — `git add` + `git commit -m "type(scope): message"`
3. **Push to remote** — `git push -u origin <branch>`
4. **Create PR** — `gh pr create --title "..." --body "..."`
5. **Review PR** — check diff, mergeable status, any CI checks
6. **Wait for Copilot review** — `gh pr view <n> --json reviews,statusCheckRollup` — do NOT merge until Copilot has reviewed
7. **Fix Copilot comments** — if Copilot leaves review comments, fix them, commit, push, resolve
8. **Merge** — only after Copilot review is clean: `gh pr merge <n> --merge`

After merge: `git checkout main && git pull origin main`

## Key rules

- Never merge without waiting for Copilot review
- Never skip step 6 even if PR is immediately mergeable
- If Copilot requests changes, fix all comments before merging
- Use `gh pr view <n> --json reviews` to check review status
- Force-add `.kvach/` and `.github/` files if global gitignore blocks them

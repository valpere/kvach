---
name: project_pr_workflow
description: Required PR workflow with Copilot review gate
type: project
---

# PR Workflow for Code Generator

When your implementation is complete and all checks pass:

1. `git checkout -b feat/<description>` or `fix/<description>`
2. `git add -A` (force-add `.kvach/` and `.github/` if needed)
3. `git commit -m "feat(scope): description"` — must pass commitlint
4. `git push -u origin <branch>`
5. `gh pr create --title "..." --body "..."` with Summary section
6. `gh pr view <n> --json state,mergeable,reviews,statusCheckRollup`
7. **WAIT** for Copilot to review — do NOT merge immediately
8. If Copilot comments: read, fix, commit, push, resolve
9. `gh pr merge <n> --merge` only when Copilot review is clean
10. `git checkout main && git pull origin main`

## Critical rules

- Step 7 is mandatory — never skip the Copilot review wait
- Global gitignore blocks `.*` — use `git add -f .kvach/ .github/` for dotfiles
- Commit must pass commitlint (scope from commitlint.config.js)
- All four checks must pass before commit: gofmt, build, vet, test

---
name: shell
---

## Shell command rules

- Always run build/compile commands after writing code to verify it works
- Run tests after implementing features — passing tests = done
- Use working_dir to run from the correct project directory
- Commands time out after 30 seconds — if a command hangs, abort and report
- Never run destructive commands: rm -rf /, DROP TABLE, mkfs, format

## Verify-before-done pattern

1. Write the code
2. run_command("go build ./...") or equivalent
3. If error: read output carefully, fix the specific issue, run again
4. run_command("go test ./...") or equivalent
5. Only report done when build AND tests pass
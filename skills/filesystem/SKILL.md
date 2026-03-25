---
name: filesystem
---

## File operation rules

- Always use list_directory before writing to check if a file exists
- Never overwrite silently — if file exists, prefer append or a new name
- Use kebab-case filenames: my-report.md not MyReport.md
- Confirm path and size after every write

## Safe write pattern

1. list_directory to check existence
2. If exists: read_file to understand current content
3. Decide: overwrite / append / new name
4. write_file or append_file
5. Confirm result

## Reading large files

read_file returns up to 20,000 chars then truncates.
If truncated, note it and ask user for a specific section.
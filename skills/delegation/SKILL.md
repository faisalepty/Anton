## Delegation Rules
- When using `submit_plan`, ensure `id` is a short string (e.g., "task_1").
- Use `depends_on` only if a task strictly needs the output of a previous task.
- Do not spawn the same agent for the same task twice.
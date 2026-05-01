### Typed process safety flags can be placed after positional ExecArgs

**Line:** `361`

**Reason:** `buildArgs` does append validated `ExecArgs` before typed fields, but the claimed safety bypass does not occur for prompt-like positional arguments. The current Codex CLI accepts `--model`, `--sandbox`, `--full-auto`, and `-c/--config` after the `exec` prompt positional, and `buildArgs` already rejects `--` plus duplicate typed safety flags inside `ExecArgs`.

When `ExecArgs` selects a nested `exec` subcommand such as `resume` or `review`, incompatible later options are rejected by the CLI rather than silently treated as prompt text or ignored, so the described "typed safety configuration may not be applied" behavior does not occur.

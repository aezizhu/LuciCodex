# LuciCodex Fastpath Architecture (0.6.3 plan)

Goal: make responses as fast as possible while keeping safety.

## Modes
- **Fast mode (default for simple asks):**
  - One LLM call, no history, no summaries.
  - Non-stream execute for 1–2 commands.
  - Tight timeouts (30–45s) and max_commands=5.
- **Full mode (multi-command/risky):**
  - Streaming + optional summaries.
  - Timeout up to 60s, summaries on-demand.

## LLM payload discipline
- No chat history for one-shot prompts.
- Hard cap context length before sending to LLM.
- Minimal templates for common status checks (wifi status, ifstatus, logread tail).
- Small/fast model by default; heavier model only on user selection.

## Execution policy
- Non-stream for short plans by default.
- Stream only when >2 commands or long-running tasks.
- If streaming fails or stalls, fall back to non-stream and still show output.
- Hide approve/reject when there are zero commands.

## Timeouts and retries
- Fast mode: 30–45s LLM timeout, single retry on transient 5xx.
- Full mode: 60s max, single retry.

## Summaries
- Off by default in fast mode.
- On-demand in full mode; summarize only the last output chunk.

## UI guidelines
- Fail fast on LLM/HTTP errors; don’t show execution UI when plan fails.
- Ensure live console shows output; auto-switch to non-stream if stream stalls.
- Keep prompts short in the UI for status checks (“wifi status only”).

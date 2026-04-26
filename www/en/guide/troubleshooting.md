---
title: Troubleshooting
description: Common NeoCode issues and practical checks for startup, provider auth, gateway, and long-session quality.
---

# Troubleshooting

This page follows: symptom -> likely causes -> 3-step checks.

## 1) `neocode` command not found

### Symptom

- Terminal says `command not found` or executable is missing.

### Likely causes

- Install script did not complete.
- Binary path is not in `PATH`.

### 3-step checks

1. Run `neocode version` and `neocode --help`.
2. If missing, rerun the install steps in [Install and Run](./install).
3. Open a new terminal session and run `neocode version` again.

## 2) API key is set but auth still fails

### Symptom

- Requests fail with `unauthorized` or invalid API key errors.

### Likely causes

- Env vars were set in a different shell session.
- Current provider does not match the env var you set.

### 3-step checks

1. Use `/provider` to confirm the active provider.
2. Check the env var mapping in [Configuration](./configuration).
3. Restart terminal, then launch `neocode` again.

## 3) Provider or model switch does not apply

### Symptom

- You switched provider or model, but behavior still looks unchanged.

### Likely causes

- Current session still carries previous context.
- Target model is unavailable under that provider.

### 3-step checks

1. Reconfirm the current selection with `/provider` and `/model`.
2. Run `/compact` to reduce stale context impact in the current session.
3. Start a new session and retry the same prompt.

## 4) Too many permission prompts

### Symptom

- Frequent approval prompts for file edits or commands.

### Likely causes

- Current policy is still `Ask`.
- Slight parameter changes make requests non-identical.

### 3-step checks

1. Read [Tools and Permissions](./tools-permissions) decision table.
2. Use `Allow` for stable, trusted repetitive actions.
3. Keep `Ask` for unknown repos or risky operations.

## 5) Gateway connection or URL dispatch fails

### Symptom

- Gateway is running but external request flow still fails.

### Likely causes

- Gateway not ready or listening mismatch.
- Auth token mismatch.
- Origin/source policy blocks request.

### 3-step checks

1. Start `neocode gateway` separately and confirm the process is running.
2. Verify minimal local path first on `127.0.0.1:8080`.
3. Review auth and origin limits in [Gateway usage](./gateway).

## 6) Long sessions drift or degrade

### Symptom

- Answers become repetitive, miss context, or quality drops.

### Likely causes

- Prompt budget is too tight for current context.
- Session history contains too much noise.

### 3-step checks

1. Trigger manual compaction with `/compact`.
2. Tune `context.budget.*` and `context.compact.*` in config.
3. Start a new session for unrelated tasks.

## Still blocked?

- Return to [Getting Started](./) for a minimal path
- Daily operations: [Daily use](./daily-use)
- Gateway issues: [Gateway usage](./gateway)

---
name: handoff
description: Execute a complex task end to end after the user explicitly grants full autonomy with wording such as "full autonomy", "you decide and do it", or "tự quyết và làm đến cùng". Do not use for ordinary implementation requests or ambiguous delegation.
---

# Handoff

Treat the user's explicit autonomy grant as permission to make decisions only inside the stated task scope. It does not authorize destructive actions, external publication, secret access, or unrelated refactors.

1. Restate the granted scope and its success criteria.
2. For a consequential or ambiguous design choice, spawn up to three independent proposal agents, then one read-only reviewer to select an approach using simplicity, repository fit, reversibility, and test surface. Skip the panel when the task is straightforward or cannot benefit from independent proposals.
3. Turn the selected approach into a short working plan and implement it without asking for routine confirmations.
4. Run the narrowest relevant checks, followed by the repository baseline when proportionate.
5. Report the decision, implementation, verification evidence, and any remaining human action.

Stop and ask the user when the work would require a destructive action, exceed the granted scope, create an unresolved decision tie, expose secrets, or mutate an external system without prior authorization.

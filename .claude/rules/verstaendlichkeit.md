---
paths:
  - "frontend/src/**"
  - "backend/templates/email/**"
  - "backend/email/**"
---

# Verständlichkeit — Build for the Worst Plausible Reading

**RULE: Every user-visible change must pass the checklist below before it is called done.** What can be misunderstood will be misunderstood. A screen is finished when the least favourable plausible reading still leads the person to the right action.

Scope: everything a school user sees — tenant portal (staff), parents portal, PyrePortal kiosk, e-mails, notifications, the in-app help guide. **Out of scope:** operator portal (internal team), logs, developer docs, code comments, backend-only changes with no visible effect.

## Why this rule exists

Support load at the pilot schools comes from misunderstandings, not from bugs. Three cases from Schule am Berg (August 2026) are the negative patterns this rule encodes:

| Case | What the user read | What was true | Issue |
|---|---|---|---|
| „Betreuungszeiten" next to „Betreuungsangebote" | two names for the same thing, so one of them must be wrong | two different concepts, no visible boundary between them | #2295 |
| Block „AGs und Gruppen" in the parents portal | a place to sign my child up for an AG | a read-only list; enrolment happens at the school | #2296 |
| Push notifications switched on, nothing arrives | „I turned it on, so it works" | needs the app installed on the home screen first | #2297 |

None of these needed a code fix at the point of failure. All three needed a name, a boundary, or a precondition stated where the person was looking.

## The checklist (verifiable, not aspirational)

Run this against the screen you changed, not against the diff.

- [ ] **One-sentence purpose.** For every visible block (card, section, tab, list, banner): a parent or care worker with no product knowledge can say what it is for in one sentence, from the screen alone. If not — rename it, merge it into the neighbouring block, or make it hideable per school (a setting; see `.claude/rules/settings-system.md`).
- [ ] **Displays look like displays.** Nothing read-only may carry an affordance it does not have: no button styling, no chevron, no hover/press feedback, no cursor pointer, no wording that reads as an invitation („Angebote wählen"). A read-only list says so in one line — for example „Nur zur Information. Anmeldung läuft über die Schule."
- [ ] **Preconditions live in the product.** A function that only works after another step (installed app, activated device, released phase, assigned group) states that precondition where the person switches it on, together with the concrete next step. The help guide may repeat it; it must not be the only place that carries it.
- [ ] **No twin headings.** Two visible labels in the same portal may not share a word stem (Betreuungs-, Anmelde-, Gruppen-) unless the screen itself shows how they differ: one line under the heading, distinct icons and position, or one of them renamed. Search the portal for the stem before shipping a new name.
- [ ] **State without action is explained.** Where a person cannot change what they see (waiting for approval, decided by the school, outside the booking window), the screen says who acts next and by when. Never a dead end, never a greyed-out control with no reason.
- [ ] **Texts follow `moto-einfache-sprache`.** Binding text standard (`.claude/skills/moto-einfache-sprache/SKILL.md`): short sentences, no technical vocabulary, name the action, German with Umlauten, Sie-Form. Load the skill for any new or changed copy.
- [ ] **Guide in sync.** If the flow is documented, the help guide changes in the same PR (`.claude/rules/help-guide-sync.md`).
- [ ] **Looked at the real screen.** The check runs against the running app, not the code — same requirement as the visual check in `.claude/rules/frontend-ui-kit.md`.

## How to apply it in practice

1. Open the changed screen in the running app.
2. Read every visible label out loud as somebody who does not know the product. Write down the reading you would get. That is the „worst plausible reading".
3. If that reading leads to a wrong action (a click that does nothing, a wait for something that never comes, a form filled at the wrong place), fix the screen — not the help text.
4. Record the result in the PR description: **Missverständnis-Check** with the blocks you checked and what you changed. „nicht user-sichtbar" is a valid entry when it is true.

The fix hierarchy, cheapest first: **rename** → **explain in one line** → **restructure or merge** → **make hideable per school** → **remove**. Adding explanatory text is the third-best option, not the first; text that changes no decision belongs nowhere (`moto-einfache-sprache`, rule 8).

## Relation to the other rules

- `frontend-ui-kit.md` governs how a screen looks. This rule governs whether it can be read wrongly. Both apply; neither substitutes for the other.
- `moto-einfache-sprache` governs the wording of a text. This rule governs whether the block should carry that text at all.
- `help-guide-sync.md` keeps the manual truthful. This rule keeps the product understandable without the manual.

## When to deviate

Only with explicit reviewer approval recorded in the PR description, naming the checklist item and the reason. „Die Schulen kennen das schon" is not a reason — new staff and new parents start every August.

---
name: help-guide-sync
description: Use when adding or substantially changing a user-facing feature flow that the in-app help documents, or when editing help articles. Triggers on help-content.ts, HELP_TOPICS, /help pages, Hilfe, Anleitung, HelpTopic, the help sidebar, or "update the docs/guide".
metadata:
  author: moto-nrw
  version: "2.0.0"
---

# In-App Help Sync

Keeps the school-facing help (`/help`) in sync with the app. The full reference is `.claude/rules/help-guide-sync.md` — this skill points you at the right files and the update flow.

## When to Use

- You added a **new user-facing feature flow** (a school user does something new)
- You **substantially changed** a documented flow (renamed or moved controls, changed step order, new screen)
- You're editing help articles directly
- A documented setting or toggle changed its meaning or default

Skip for backend-only changes, operator-portal-only changes, and pure styling tweaks. The rule has the full WHEN / WHEN-NOT list.

## The Shape of the Help

One article per question, one page per article, four role sidebars over one content set. `/help` asks who is reading; `/help/<topic>` is an article; `/help/gruppe/<group>` is a category.

The three long guide pages are gone (#2229). `/help/setup`, `/help/features` and `/help/nfc` redirect to `/help`. `/help/nfc/erste-schritte` is **not** an article — it is the printed onepager for the tablet box and the only remaining PDF.

## Files to Read First

```
frontend/src/components/help/help-content.ts    # THE CONTENT — every article (edit this)
frontend/src/lib/help-topics.ts                 # HELP_TOPICS ids + app-path mapping
frontend/src/components/help/help-view.tsx      # the shell; GROUP_ORDER_BY_ROLE lives here
frontend/public/help/screens/                   # app screenshots (also the UI visual reference)
```

## Where Does It Belong?

| The change is about… | Put it under `group` | Visible to `audience` |
|---|---|---|
| Getting in at all (invitation, login, app install) | `einstieg` | usually `"all"` |
| The care day (day plan, supervision, check-in) | `tagesplanung`, `kinder`, `gruppen` | `caregiver` (often + `lead`) |
| Setting the OGS up once (rooms, groups, staff, children) | `einrichten` | `lead` |
| Staff records, rights, hours, payroll | `personal` | `lead` |
| The NFC tablet and wristbands | `nfc` | `["caregiver", "lead"]` |
| A parent in the parents portal | `mein-kind`, `nachrichten`, `anmeldung` | `parent` |
| A teacher in moto schule | `klasse`, `aufsicht` | `teacher` |

A category also needs an entry in `HELP_GROUP_LABELS` and in `GROUP_ORDER_BY_ROLE` for every role that sees it.

## Process

1. **Register the id** in `HELP_TOPICS` (`src/lib/help-topics.ts`) — camelCase key, URL slug as the value. A typo becomes a compile error.
2. **Write the article** in `help-content.ts` as `function <name>Topic(): HelpTopic`, then add the call to the role list in `getHelpTopics` **at a position its prerequisites allow** — order inside a category is array order, not alphabetical.
3. **Read the labels off the running screen**, not from memory, and put each one in backticks exactly as it is spelled there.
4. **Write German** per `moto-einfache-sprache`: Sie-Form, one thought per sentence, no technical vocabulary.
5. **Pick the right section.** `differences` = the screen looks different; `troubleshootingDetails` = something is missing or broken; `notes` = an optional extra that changes no step.
6. **Verify**:
   ```bash
   cd frontend && pnpm run check
   ```

## Common Mistakes

| Mistake | Consequence |
|---------|-------------|
| Ship a UI change, skip `help-content.ts` | The help lies to schools → support tickets |
| `related` points at an article the reader's role cannot see | The link vanishes silently — no error, no warning |
| New article placed before its prerequisite | The reader is told to pick a `Gruppenleitung` that does not exist yet |
| New category without `GROUP_ORDER_BY_ROLE` entry | Its articles are unreachable in the sidebar |
| A label quoted from memory | Reader hunts for a button that is named something else |
| Write help text in English | The help UI is German-only; it sticks out immediately |
| Add a component or color for the help | `help-view.tsx` renders every section already; follow `frontend-ui-kit.md` |
| Break the `/help/nfc/erste-schritte` render | CI PDF generation (`generate:guides`) fails on `MIN_PDF_BYTES` |

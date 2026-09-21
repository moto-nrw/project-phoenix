---
paths:
  - "frontend/**"
---

# In-App Help — Keep Docs in Sync With Features

**RULE: When you add a user-facing feature flow, or substantially change a flow that is already documented, update the in-app help in the SAME PR.** The help is a living asset — it drifts the moment a screen, sidebar area, or step changes and nobody touches it.

The help is the public, school-facing manual served under `/help`. When it lies, support tickets follow.

---

## What the Help System Is

**One article per question, one page per article.** The reader picks a role once, then walks a sidebar of short articles; there is no long chapter to scroll. This replaced the three long guide pages (`/help/setup`, `/help/features`, `/help/nfc`) in #2229 — those URLs now redirect to `/help` (`frontend/next.config.js`).

| Address | What it shows |
|---|---|
| `/help` | Asks who is reading, then that role's article list |
| `/help/<topic>` | One article. The slug is the topic's `HELP_TOPICS` value, so it survives re-sorting |
| `/help/gruppe/<group>` | One category, as cards of its articles |
| `/help/nfc/erste-schritte` | **Not an article.** The printed onepager for the tablet box, and the only PDF left |

Four roles: `caregiver`, `lead`, `parent`, `teacher`. Each has its own sidebar over **one** shared content set (ADR 0028).

Five optional URL parameters carry context; none of them is an access check:

- `role` — which sidebar and which articles. The app sets it; a direct visit to `/help` is asked
- `nfc_enabled`, `presence_mode`, `group_mode` — the three tenant settings that change instructions (`docs/research/help-guide-configuration-impact.md`). A missing or unknown value produces no variant
- `return_to` — internal path for "Zurück zur App". External targets and other help pages are rejected

## File Map

| File | Role |
|------|------|
| `frontend/src/components/help/help-content.ts` | **The content.** Every article, in German. This is what you edit. |
| `frontend/src/components/help/help-view.tsx` | The shell: sidebar, search, "Auf dieser Seite", article rendering, category order |
| `frontend/src/components/help/help-entry.tsx` | The role question on a bare `/help` |
| `frontend/src/lib/help-topics.ts` | `HELP_TOPICS` (the id registry) and the app-path → topic maps |
| `frontend/src/components/help/context-help-link.tsx` | The `?` in the app header; builds the URL with role and settings |
| `frontend/src/app/help/[[...topic]]/page.tsx` | The single route behind all three address shapes above |
| `frontend/src/app/help/nfc/erste-schritte/page.tsx` | The printed onepager (+ `onepager-header.tsx`) |
| `frontend/public/help/screens/*.webp` | App screenshots, kept from the old guide. Articles currently set no `image`; the files are also the visual reference named in `frontend-ui-kit.md` |
| `frontend/public/help/pdfs/` | **Generated** (gitignored). Only `nfc-erste-schritte.pdf`, built in CI |

## Data Model

One flat list of articles. The authoritative shape is `HelpTopic` in `help-content.ts` — trust it over this copy:

```ts
interface HelpTopic {
  id: HelpTopicId;                 // from HELP_TOPICS — also the URL slug
  title: string;
  question: string;                // "Wie lege ich Kinder an?" — how the sidebar lists it
  summary: string;                 // one sentence under the title
  group: HelpTopicGroup;           // the category it appears under
  audience: "all" | HelpRole | readonly HelpRole[];
  icon: PhosphorIcon;              // from "@phosphor-icons/react/ssr"; unique within each tile overview (tested)
  requirements?: readonly string[];      // "Das brauchen Sie"
  steps: readonly string[];              // "So geht es", when there is one path
  instructionGroups?: readonly {         // several named paths instead of one list
    title: string; description?: string; steps: readonly string[]; ordered?: boolean;
  }[];
  result?: string;                       // "Danach"
  differences?: readonly string[];       // "Wenn es anders aussieht" — the screen deviates
  notes?: readonly string[];             // "Tipp" — optional extras, nothing required
  troubleshooting?: HelpTopicId;         // link to a whole troubleshooting article
  troubleshootingDetails?: readonly string[]; // "Wenn es nicht klappt"
  image?: string; imageAlt?: string;     // "So sieht es aus"
  related: readonly HelpTopicId[];       // "Weitere Themen" (required, may be empty)
}
```

The three "something is wrong" sections are **not** interchangeable, and mixing them is the most common content mistake:

- `differences` — the screen looks different than described (another setting, a phone, a permission)
- `troubleshootingDetails` — something is missing or does not work
- `notes` — an optional extra that changes no step

Three more things that bite:

- **Order inside a category is array order** in `getHelpTopics`, not alphabetical. Put a prerequisite before what needs it.
- **`related` passes the same role filter as the sidebar.** A link to an article the reader's role cannot see disappears silently.
- **Category order per role** lives in `GROUP_ORDER_BY_ROLE` (`help-view.tsx`), separately from the article list. A new category needs an entry there *and* in `HELP_GROUP_LABELS`.

Settings-driven filtering uses three id sets in `help-content.ts`: `NFC_ONLY_TOPIC_IDS`, `DETAILED_ONLY_TOPIC_IDS`, `FIXED_GROUPS_ONLY_TOPIC_IDS`. An article listed there disappears for tenants the setting does not apply to — that is a hard filter, unlike the URL parameters above.

All visible strings are **German** — the help UI is German-only.

---

## WHEN To Update

Update the help when the change is **visible to a school user**:

- **New user-facing feature flow** → register an id, then add an article.
- **Substantial UI change to a documented flow** → fix the affected steps. A renamed button, a moved control, a changed order all qualify.
- **Renamed or moved sidebar area / page** → fix every step that names it, and the path map in `help-topics.ts`.
- **A documented setting or toggle changes meaning or default** → update the article, and check whether it belongs in one of the three id sets.
- **A new page under `(protected)`** → map it in `help-topics.ts`, or accept that it shows no `?`. The baseline in `help-topics.test.ts` is shrink-only.

**Do NOT** update the help for:
- Backend-only changes with no user-visible effect
- Operator-portal-only changes (the help covers the tenant, parents and school portals)
- Pure styling tweaks that don't change what a step instructs

When unsure: if a school admin or supervisor would *do something differently* after your change, the help needs a line.

## HOW To Update

1. **Register the id** in `HELP_TOPICS` (`src/lib/help-topics.ts`): the key is the camelCase id, the value the URL slug. A typo is then a compile error, not a dead link.
2. **Write the article** in `help-content.ts` as a `function <name>Topic(): HelpTopic`, and add the call to the right role list in `getHelpTopics`, at a position its prerequisites allow.
3. **Write German** per `moto-einfache-sprache` (Sie-Form, short sentences, no technical vocabulary). Put every label the reader clicks in backticks, spelled exactly as the screen spells it. **Open the real screen and read the labels off it** — a remembered label is a wrong label.
4. **Check the reverse direction:** does an existing article now contradict your change? Search `help-content.ts` for the label you renamed.
5. **Reuse the kit.** The help is frontend, so `.claude/rules/frontend-ui-kit.md` applies. `help-view.tsx` already renders every section — do not invent a new block.
6. **Verify:**
   ```bash
   cd frontend && pnpm run check    # zero warnings policy
   ```
   `help-topics.test.ts` guards that every registered id resolves in every settings combination, that no category is empty, that category ids never collide with article slugs, and that every `(protected)` page is mapped or on the shrink-only baseline. It also pins the per-role article counts and the first articles of each role's sidebar: a new article means updating those numbers, which is expected — not a test to weaken.
7. **Don't break the onepager PDF.** CI renders `/help/nfc/erste-schritte` with
   ```bash
   cd frontend && pnpm run generate:guides   # Playwright, playwright.guides.config.ts
   ```
   `MIN_PDF_BYTES` fails the build if the render comes out blank. Only that one page is rendered: there is no PDF per topic and no PDF of the help area.

---

## Scope Boundary

This rule covers the **in-app, school-facing help only**. It does NOT govern: developer docs (`CLAUDE.md`, `.claude/rules/*`), cross-repo docs (PyrePortal/balenaOS), or generated API route docs (`./main gendoc`). Those are separate concerns; don't bundle them here unless explicitly asked.

## Background

`docs/hilfebereich-umbau-plan.md` holds the rework plan and the decisions behind this structure; ADR 0027 (format and platform: typed TS, no docs framework) and ADR 0028 (audiences: separate entries, one content set) are the binding ones.

## Paired Skill

`help-guide-sync` walks through the files above when you're doing help work. This rule is the reference; the skill is the workflow.

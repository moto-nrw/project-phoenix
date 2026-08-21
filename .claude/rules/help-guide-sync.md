# In-App Help Guide — Keep Docs in Sync With Features

**RULE: When you add a user-facing feature flow, or substantially change a flow that is already documented, update the in-app help guide in the SAME PR.** The help guide is a living asset — it drifts the moment a screen, sidebar area, or step changes and nobody touches it.

The help guide is the public, school-facing manual served under `/help`. Schools read it in the browser AND receive it as a printable PDF (generated in CI, see below). When it lies, support tickets follow.

---

## What the Help System Is

Three guides plus a landing page, all rendered from one content file:

| Guide | Page | Chapter set (in `guide-data.ts`) | Purpose |
|-------|------|----------------------------------|---------|
| Landing | `/help` | `guideEntryPoints` | Three entry cards linking to the guides |
| Ersteinrichtung | `/help/setup` | `setupChapters` | Empty system → first real care day |
| Die App im Alltag | `/help/features` | `appChapters` | Every app area explained (the feature reference) |
| NFC & Tablets | `/help/nfc` | `nfcChapters` | Tablet/NFC manual — mostly the **initial setup** of NFC kiosks, plus daily check-in/out and troubleshooting |

## File Map

| File | Role |
|------|------|
| `frontend/src/components/help/guide-data.ts` | **The content.** All chapters, steps, callouts, screenshot captions. German text. This is what you edit. |
| `frontend/src/components/help/guide-components.tsx` | Rendering: `GuideShell`, `HelpHeader`, `EntryPointCard` |
| `frontend/src/components/help/guide-search.ts` + `help-search.tsx` | Guide search — the index derives from `guide-data.ts`; `help-search-sync.test.ts` guards the sync |
| `frontend/src/components/help/guide-pdf-button.tsx` | Download button for the rendered PDF |
| `frontend/src/components/help/help-back-button.tsx` | "Zurück" — `router.back()`, falls back to `/`. Lets kiosk/app users return after opening Hilfe. |
| `frontend/src/app/help/{page,setup/page,features/page,nfc/page}.tsx` | The four pages; each wires one chapter set into `GuideShell` |
| `frontend/public/help/screens/*.webp` | App screenshots, referenced by a step's `image:` field |
| `frontend/public/help/pdfs/` | **Generated** PDFs (gitignored). Built in CI, shipped in the Docker image. |

## Data Model

Content is a tree: `GuideChapter` → `GuideStep`. Defined and exported from `guide-data.ts`:

```ts
interface GuideChapter {
  id: string;
  title: string;
  description: string;
  icon: LucideIcon;              // from lucide-react
  tone: GuideTone;              // "blue" | "green" | "orange" | "red" | "purple" | "gray"
  steps: readonly GuideStep[];
}

interface GuideStep {
  id: string;
  title: string;
  summary: string;
  steps?: readonly string[];     // ordered actions (optional — a card may be checklist-only)
  checklist?: readonly string[]; // rendered as a checklist block
  callout?: GuideCallout;        // { title, body, tone? } — highlighted hint
  screenshot: string;            // caption / alt text describing the supporting image (ALWAYS set)
  image?: string;                // path under /public, e.g. "/help/screens/kindersuche.webp"
  gallery?: readonly { image; caption }[]; // captioned grid instead of one image (NFC tablet states)
  icon?: LucideIcon;             // shown instead of a number badge on reference pages
  printCompact?: boolean;        // tighter print spacing + keeps the card on one PDF page
}
```

(The authoritative shape is the interface in `guide-data.ts` — trust it over this copy.)

Notes:
- `screenshot` is the **caption/intent** and is always present; `image` is the actual file and is optional (a step with no `image` renders no placeholder).
- `gallery` is used by the NFC manual to show every tablet state, not just one.
- All visible strings are **German** — the help UI is German-only.

---

## WHEN To Update

Update the guide when the change is **visible to a school user**:

- **New user-facing feature flow** → add a `GuideStep` (or a new `GuideChapter`) to the matching chapter set: setup-related → `setupChapters`, an app area → `appChapters`, NFC/tablet → `nfcChapters`.
- **Substantial UI change to an already-documented flow** → update the affected step(s) **and re-capture the screenshot** so the image matches what the user sees. A renamed button, a moved control, a changed step order all qualify.
- **Renamed or moved sidebar area / page** → fix the step title and, on the landing page, the relevant `guideEntryPoint`.
- **NFC/tablet flow change** (especially first-time setup, but also daily check-in/out or troubleshooting) → update `nfcChapters`, including any `gallery` screens.
- **A documented setting/toggle changes meaning or default** → update the step that explains it.

**Do NOT** update the guide for:
- Backend-only changes with no user-visible effect (repos, services, migrations, internal refactors)
- Operator/parents-portal-only changes (the guide documents the **tenant/staff** app)
- Pure styling tweaks that don't change what a step instructs

When unsure: if a school admin or supervisor would *do something differently* after your change, the guide needs a line.

## HOW To Update

1. **Find the right chapter set** in `guide-data.ts` (`setupChapters` / `appChapters` / `nfcChapters`) and the step whose `id`/`title` matches the flow.
2. **Edit content in German**, reusing the existing `GuideStep` shape. Keep `steps` imperative and concrete ("`Räume` öffnen, `Neuer Raum` klicken …"), mirror the tone of neighboring steps. Reuse an existing `lucide-react` icon already imported in the file before adding a new import.
3. **Screenshots**: if the screen changed, replace the `.webp` under `frontend/public/help/screens/` (same filename to avoid touching `image:` paths) or add a new one and point `image:` at it. Always keep `screenshot:` (the caption) accurate even when there's no image.
4. **Brand colors / reuse**: the guide is part of the frontend — follow the existing component and brand-color rules in `CLAUDE.md` and `frontend/CLAUDE.md`. Don't invent new UI; `GuideShell` already renders everything.
5. **Don't break the PDF render.** The PDFs are produced in CI by:
   ```bash
   cd frontend && pnpm run generate:guides   # Playwright, playwright.guides.config.ts
   ```
   It renders the public `/help` pages to PDF (`public/help/pdfs/`, gitignored) and ships them in the image. Anything that breaks the `/help` build breaks the PDFs. The pages are host-agnostic, so a local render only needs placeholder hostnames.
6. **Verify** before you call it done:
   ```bash
   cd frontend && pnpm run check          # zero warnings policy
   ```
   Optionally render the PDFs locally (`pnpm run generate:guides`) if you touched layout-affecting structure.

---

## Scope Boundary

This rule covers the **in-app, school-facing help guide only**. It does NOT govern: developer docs (`CLAUDE.md`, `.claude/rules/*`), cross-repo docs (PyrePortal/balenaOS), or generated API route docs (`./main gendoc`). Those are separate concerns; don't bundle them here unless explicitly asked.

## Paired Skill

`help-guide-sync` walks through the files above when you're doing guide work. This rule is the reference; the skill is the workflow.

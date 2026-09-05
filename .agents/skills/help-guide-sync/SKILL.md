---
name: help-guide-sync
description: Use when adding or substantially changing a user-facing feature flow that the in-app help guide documents, or when editing the help guide content, screenshots, or PDF export. Triggers on guide-data.ts, /help pages, Hilfe, Anleitung, guide chapters, GuideStep/GuideChapter, or "update the docs/guide".
metadata:
  author: moto-nrw
  version: "1.0.0"
---

# In-App Help Guide Sync

Keeps the school-facing help guide (`/help`) in sync with the app. The full reference is in `.claude/rules/help-guide-sync.md` — this skill points you at the right files and the update flow.

## When to Use

- You added a **new user-facing feature flow** (a school user does something new)
- You **substantially changed** a flow that is already documented (renamed/moved controls, changed step order, new screen)
- You're editing the help content, a screenshot, or the PDF export directly
- A documented setting/toggle changed its meaning or default

Skip for backend-only changes, operator/parents-portal-only changes, and pure styling tweaks. See the rule for the full WHEN/WHEN-NOT list.

## Files to Read First

```
frontend/src/components/help/guide-data.ts          # THE CONTENT — chapters, steps, captions (edit this)
frontend/src/components/help/guide-components.tsx    # GuideShell rendering (rarely edited)
frontend/src/app/help/{,setup,features,nfc}/page.tsx # which page wires which chapter set
frontend/public/help/screens/                        # screenshots referenced by step `image:`
```

## Which Chapter Set?

All three live in `guide-data.ts`:

| Change is about… | Edit |
|------------------|------|
| First-time setup (empty system → first care day) | `setupChapters` (`/help/setup`) |
| An app area / everyday feature | `appChapters` (`/help/features`) |
| NFC tablet / kiosk (setup, check-in/out, troubleshooting) | `nfcChapters` (`/help/nfc`) |
| Landing-page entry cards | `guideEntryPoints` (`/help`) |

## Process

1. **Locate** the chapter set and the `GuideStep` whose `id`/`title` matches the flow.
2. **Edit in German**, reusing the existing `GuideStep` shape (`title`, `summary`, `steps`, `checklist`, `callout`, `screenshot`, `image`, `gallery`, `icon`). Match the tone of neighboring steps; reuse an already-imported `lucide-react` icon before adding one.
3. **Screenshot**: if the screen changed, replace the `.webp` in `frontend/public/help/screens/` (keep the filename) or add a new file and point `image:` at it. Keep the `screenshot:` caption accurate even with no image.
4. **Don't break the PDF render** — the public `/help` build feeds the CI PDF export:
   ```bash
   cd frontend && pnpm run generate:guides   # Playwright → public/help/pdfs/ (gitignored)
   ```
5. **Verify**:
   ```bash
   cd frontend && pnpm run check
   ```

## Common Mistakes

| Mistake | Consequence |
|---------|-------------|
| Ship a UI change, skip `guide-data.ts` | Guide + PDF lie to schools → support tickets |
| Change a screen but reuse the old screenshot | Image no longer matches the instructions |
| Write guide text in English | Help UI is German-only; sticks out immediately |
| Add a new component/color for the guide | `GuideShell` already renders everything; follow brand-color/reuse rules |
| Break the `/help` build | CI PDF generation (`generate:guides`) fails |

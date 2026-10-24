---
paths:
  - "backend/**"
  - "frontend/src/**"
  - "error-registry.json"
---

# Error path: the code is the identity

Decision and sources: [ADR 0006](../../docs/adr/0006-fehleridentitaet-ist-der-code.md).
Registry mechanics: [docs/agents/error-registry.md](../../docs/agents/error-registry.md).

## Rules

1. **The code is the identity.** Every error response carries a code from
   `error-registry.json` (`bereich.fehlername`). Programs branch on the code or
   the HTTP status, never on a message text. The backend text travels in
   `detail` and is developer diagnostics; the user reads the catalog text for
   the code (`errorCatalog` in `frontend/src/i18n/messages/*.json`).
2. **One envelope.** The backend answers every error with the RFC 9457 body:
   `type`, `title`, `status` (the HTTP status number), `detail`, `instance`,
   plus `code`, `errors[]` and `details{}`. A handler without its own domain
   code gets the code of its error class (`general.*`). Every value the text
   mentions also travels structured in `details`.
3. **One display path.** The frontend turns a failure into an `ApiError`
   (`frontend/src/lib/api-error.ts`) and shows it through the hooks in
   `frontend/src/contexts/ToastContext.tsx`:
   - `useApiErrorDisplay` — a toast for an action as a whole: `show(error)`
     for a failed API call, `showMessage(text)` for a failure the screen
     found itself (copy failed, file too large).
   - `useApiFormError` — `FormErrorAlert` at the top of the form for its save
     or validation failure.
   - `useApiLoadError` — `LoadErrorAlert` or the `TenantPage` error for data
     that failed to load.
   A modal only where the user has to decide something. A toast always follows
   a user action. Every `catch` has a visible result or a comment that names
   why it stays silent.
4. **Error toasts stay.** An error toast stays until the user closes it and
   carries `role="alert"`; a success toast fades after four seconds with
   `role="status"`. The text names the affected object („Die Gruppe konnte
   nicht gespeichert werden“).
5. **Field errors at the field.** `errors[]` entries go to their field with
   `aria-invalid` and `aria-describedby` (the kit control's `error` prop, or
   `FieldErrorHint` under a control without one); focus moves to the first
   invalid field.
6. **Codes are additive.** Add a new code for a new failure. A registered code
   keeps its name and stays in the registry. Each new code gets a text in
   every locale of `backend/modules/delivery/internal/adapters/localization/locales.json`. A code the parent
   portal can show gets a real translation; a staff-only code may carry its
   class text outside German.

## Enforcement

The checks run without exception lists. When one of them fails, move the code
onto the path.

- `pnpm run check` runs `scripts/verify-error-catalog.mjs` (every code has a
  text in every locale) and the oxlint `error-path/*` rules (error display
  outside the path, raw error toasts, hand-written `role="alert"` elements,
  showing or matching an error's own text).
- `TestErrorPathsAnswerWithRegisteredCodes` and
  `TestErrorWritesUseTheSharedEnvelope` (`backend/api/common`) fail on an
  unregistered code and on an error write that bypasses the envelope; the
  envelope check in `TestFullProductionRouterGolden` (`backend/api`) proves
  the wire shape on every route.

# Parent Portal Internationalization

Project Phoenix localizes only parent-facing paths in the first stage:

- `/parents/*`
- public enrollment at `/anmeldung/{phaseId}`
- authenticated parent enrollment at `/parents/anmeldung/{slug}/{phaseId}`

The staff/tenant portal and operator portal stay German. Shared components must
therefore opt into localized copy explicitly. For example,
`EnrollmentForm` keeps German chrome by default and parent/public callers pass
`localizedCopy`.

## Scope: Static Translation Only

Only **code-authored app chrome** is translated (Tier A). School-authored
content and legal text stay German on purpose — see below. There is **no
machine translation** (no DeepL, no runtime translation service). This keeps the
feature free of an external paid processor and the GDPR/AVV obligations that
come with sending content to a third party.

## Locale Source Of Truth

Supported locales live in `backend/localization/locales.json`.

Both Go and TypeScript read that same file:

- Backend: `backend/localization/locales.go`
- Frontend: `frontend/src/i18n/locales.generated.ts` (mirror of the backend
  JSON, regenerated via `pnpm run generate:locales` and guarded by
  `pnpm run check` through `scripts/verify-locales.mjs`)

Adding a language requires:

1. Add one locale entry (`code`, `label`, optional `fallback`) to
   `backend/localization/locales.json`.
2. Run `pnpm run generate:locales` to update the frontend mirror.
3. Add a message catalog in `frontend/src/i18n/messages/{code}.json`.

Do not add a second hardcoded locale list for switchers or validation.

## Text Tiers

Tier A — app chrome (translated)

Code-authored strings use `next-intl` message catalogs. German (`de`) is the
fallback. Locale is resolved from the parent locale cookie or `Accept-Language`;
there is no locale URL prefix because tenant and portal routing already use the
path and subdomain shape.

Tier B — school-authored content (NOT translated)

School-defined display text — phase names, care offering
names/descriptions/groups, custom form labels/descriptions/options — is shown
**as the school entered it (German)**. A German OGS authors this content; there
is no machine translation step. Parent form answers are likewise never altered.

Tier C — legal text (NOT translated)

Tenant legal settings such as AGB, DSGVO, photo consent, and email contact text
remain German. Parent forms show a localized notice that these legal texts are
binding in German.

## Persistence

Anonymous enrollment uses the `phoenix_parent_locale` cookie.

Authenticated parents use `users.guardian_profiles.portal_locale` through:

- `GET /api/parent/me/profile` → `{ "portal_locale": "en" | … | null }`
- `PUT /api/parent/me/profile` → `{ "portal_locale": "en" }`

`portal_locale` is a dedicated nullable column. `NULL` means the guardian has
never picked a language in the portal. It is deliberately **separate** from
`language_preference`, which records the guardian's contact/spoken language for
the school (fed by the student import, written `'de'` on creation everywhere) —
overloading that field would make `NULL` meaningless.

On first authenticated load the portal reads the profile:

- `portal_locale = NULL` → keep the anonymous locale the parent arrived with
  (their pre-login switcher cookie, or the browser's `Accept-Language`) instead
  of snapping to German, and persist it so it sticks.
- `portal_locale = <locale>` → that explicit choice is the source of truth; if
  it differs from the rendered locale the portal does a soft `router.refresh()`
  (no full reload) to re-render the server tree in it.

The parent language switcher writes to the profile when authenticated and to the
cookie otherwise, then soft-refreshes. This selects which Tier A catalog renders
— it does not translate school content.

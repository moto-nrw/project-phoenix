# Task: Wire up Cloudflare Turnstile widget on the public enrollment form

## Context

The parent-enrollment public form (`/{tenant}/(public)/enroll`) ships with
end-to-end captcha verification on the **backend** but no widget on the
**frontend**. The form sends an empty `captcha_token`, so submissions
only succeed when a tenant disables captcha via
`enrollment.require_captcha=false` (which is the dev/test default).

Your job is to integrate the Cloudflare Turnstile widget so production
tenants can keep `enrollment.require_captcha=true` and still let parents
submit.

## What already exists

- **Backend verifier**: `backend/services/enrollment/captcha_service.go`
  posts the submitted token to
  `https://challenges.cloudflare.com/turnstile/v0/siteverify` together
  with the secret from `enrollment.captcha_secret_key` and the parent's
  IP. Wired into `submitEnrollment` in
  `backend/api/enrollment/submission_handlers.go`.
- **Settings**: `enrollment.captcha_site_key` (public),
  `enrollment.captcha_secret_key` (admin-only), and
  `enrollment.require_captcha` (boolean) are registered in
  `backend/services/config/defaults/enrollment.go` and editable via the
  enrollment settings tab.
- **Form payload field**: `captcha_token` is already in
  `SubmitEnrollmentPayload` (`frontend/src/lib/enrollment-submission-api.ts`)
  and in the form state as `captchaToken`
  (`frontend/src/components/enrollment/enrollment-form.tsx:77`).
- **Backend behaviour when `require_captcha=true`**: missing/invalid
  `captcha_token` → HTTP 400 with `ErrInvalidSubmission`. The Next.js
  proxy surfaces the German error to the user.

## What's missing

1. The Turnstile JS bundle is not loaded on the page.
2. The widget is not rendered in the form.
3. `captchaToken` state is never written by the widget callback — the
   form posts an empty string.
4. Site key plumbing: the form needs to read
   `enrollment.captcha_site_key` for the current tenant. There is no
   public read-only endpoint for it yet — see "Open question" below.

## Files to touch

- `frontend/src/components/enrollment/enrollment-form.tsx`
  - Render the Turnstile widget.
  - Wire its success callback to `setCaptchaToken`.
  - Wire its expired/error callbacks to clear it and surface a German
    error message.
  - Disable the submit button until `captchaToken` is set when captcha
    is enabled.
- `frontend/src/app/[tenant]/(public)/enroll/page.tsx`
  - Inject the Turnstile script tag (Next.js `<Script>`) — preferably
    only when the site key is non-empty.
- A new public endpoint or schema-payload extension to expose
  `enrollment.captcha_site_key` + `enrollment.require_captcha` to the
  unauthenticated form. See "Open question."

## Open question (resolve before writing code)

The public form has no current way to read the captcha site key. Pick
one of these two and surface the choice in your PR description:

- **Option A**: Extend the existing public care-offerings response
  (`GET /api/enrollment/care-offerings/public/{tenantSlug}`) with a
  `captcha` block (`{ site_key, required }`). Cheap, reuses an
  existing public endpoint, but couples care offerings to captcha.
- **Option B**: Add a dedicated `GET /api/enrollment/{tenantSlug}/config`
  that returns the public-form bootstrap (site key, required flag,
  grade max, calendar period name). Cleaner, less coupling, but adds
  an endpoint.

Either way: NEVER expose `enrollment.captcha_secret_key` over a public
endpoint. Filter it server-side.

## Acceptance criteria

1. Visiting `/{tenant}/(public)/enroll` on a tenant with
   `enrollment.require_captcha=true` and a valid
   `enrollment.captcha_site_key` renders the Turnstile widget inline
   above the submit button.
2. Submit is disabled until the widget produces a token.
3. The widget's expired/error/timeout callback clears the token and
   surfaces a German error ("Sicherheitsprüfung fehlgeschlagen, bitte
   erneut versuchen.").
4. Successful submit posts the live `captcha_token` and the backend
   accepts it.
5. Tenants with `enrollment.require_captcha=false` (or empty site key)
   skip rendering the widget entirely — the form submits without it.
6. No backend route exposes `enrollment.captcha_secret_key`. The
   admin-only setting stays admin-only.
7. `pnpm run check` passes (zero warnings) and any new `useEffect`
   cleanups handle the widget lifecycle (Next.js Strict Mode mounts
   twice in dev — verify the widget doesn't end up duplicated).

## Test plan

1. Unit/integration test the new public config endpoint (if you go
   with Option B) or the extended care-offerings response (Option A) —
   assert the secret key is never present.
2. Manual test in dev: set `enrollment.captcha_site_key` to the
   Cloudflare-provided test site key
   (`1x00000000000000000000AA` always passes,
   `2x00000000000000000000AB` always blocks) and walk through the
   enrollment form on `{slug}.localhost:3000/enroll`.
3. Negative test: with `require_captcha=true` and the always-blocks
   key, submit must fail with the German error visible to the parent.
4. Negative test: backend route must reject submissions where the
   widget was bypassed (e.g., a curl call without `captcha_token`)
   when `require_captcha=true` is on. Already covered by
   `TestRequestService_Submit_RejectsInvalidSubmission` style; add an
   API-level test if missing.

## Out of scope

- Replacing Turnstile with hCaptcha or reCAPTCHA — settings + verifier
  are Turnstile-specific. If a tenant needs a different provider,
  that's a separate RFC.
- Admin-side captcha (e.g., on the admin login form). This task is
  parent-enrollment only.

## References

- Cloudflare Turnstile docs: <https://developers.cloudflare.com/turnstile/>
- Backend verifier: `backend/services/enrollment/captcha_service.go`
- Settings registration: `backend/services/config/defaults/enrollment.go`
- Public submission flow: PR 7 commits on `feat/enrollment`
  (`2bfd3ae84` … `95add29e7`)

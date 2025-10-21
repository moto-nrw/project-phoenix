# Email Templates Guide

This document describes how Project Phoenix renders transactional emails (password reset and invitations), the available template variables, and how to customize branding safely.

## Overview

- **Renderer**: `email.Message.parse()` uses Go `html/template` combined with `go-premailer` to inline CSS for broad email client support. A plain-text alternative is generated automatically from the rendered HTML.
- **Location**: All templates live in `backend/templates/email/`.
- **Delivery**: The SMTP mailer is initialised once in `services/factory.go`. Missing SMTP credentials fall back to the mock mailer, which logs redacted messages to STDOUT.
- **Configuration**: Email behaviour is controlled via:
  - `EMAIL_SMTP_HOST`, `EMAIL_SMTP_PORT`, `EMAIL_SMTP_USER`, `EMAIL_SMTP_PASSWORD`
  - `EMAIL_FROM_NAME`, `EMAIL_FROM_ADDRESS`
  - `FRONTEND_URL` (must be HTTPS in production)
  - `INVITATION_TOKEN_EXPIRY_HOURS` (default 48)
  - `PASSWORD_RESET_TOKEN_EXPIRY_MINUTES` (default 30)

## Template Structure

```
backend/templates/email/
├── header.html       # Shared DOCTYPE/head/start of body
├── footer.html       # Shared footer with moto signature
├── styles.html       # Centralised CSS (buttons, typography, layout)
├── invitation.html   # Invitation-specific content block
└── password-reset.html # Password reset-specific content block
```

Both feature templates wrap their content with `{{template "header" .}}` and `{{template "footer" .}}`. When you add a new template, follow the same pattern so shared branding and inline styles stay consistent.

## Shared Layout & Styling

- **Brand Colours**: Buttons use moto blue `#5080d8`; emphasis/highlight text uses moto green `#83cd2d`.
- **Logo Placement**: The header expects a `LogoURL` data field pointing to a PNG (typically `${FRONTEND_URL}/images/moto_transparent.png`).
- **Typography**: Styles favour sans-serif fonts with generous spacing; edit `styles.html` if you need to adjust fonts globally.

## Invitation Email (`invitation.html`)

The invitation template expects the following keys on the `Content` map passed to `email.Message`:

| Key              | Type           | Description                                                    |
|------------------|----------------|----------------------------------------------------------------|
| `LogoURL`        | `string`       | Absolute URL to the moto logo                                 |
| `InvitationURL`  | `string`       | Acceptance link (`{FRONTEND_URL}/invite?token=...`)           |
| `ExpiryHours`    | `int`          | Time-to-live displayed to the recipient (typically 48)        |
| `FirstName`      | `*string`      | Optional; greets the invitee by name                          |
| `LastName`       | `*string`      | Optional; stored with the invitation but not rendered directly |
| `RoleName`       | `string`       | Human-readable role (e.g., “Lehrkraft”)                       |

Default sender name/address are pulled from `EMAIL_FROM_NAME` / `EMAIL_FROM_ADDRESS`.

### Sentences & Links

- The CTA button and fallback text both use `InvitationURL`.
- A short disclaimer reminds recipients to ignore the email if unexpected.

## Password Reset Email (`password-reset.html`)

Expected payload keys:

| Key              | Type     | Description                                                          |
|------------------|----------|----------------------------------------------------------------------|
| `LogoURL`        | `string` | Absolute URL to the moto logo                                       |
| `ResetURL`       | `string` | Password reset link (`{FRONTEND_URL}/reset-password?token=...`)     |
| `ExpiryMinutes`  | `int`    | Expiry reminder shown to the recipient (default 30)                 |

The footer includes a security notice clarifying that passwords remain unchanged until the link is used.

## Customising Branding

1. **Logo**: Replace `frontend/public/images/moto_transparent.png` or update the `LogoURL` passed via services.
2. **Colours**: Update the CSS variables in `styles.html`. Keep accessibility in mind (minimum contrast ratio 4.5:1 for body text).
3. **Copy**: Edit the paragraph text inside `invitation.html` or `password-reset.html`. Avoid removing the security disclaimer in the password reset template.
4. **Internationalisation**: Templates currently render German copy. If you add translations, consider duplicating templates per locale and selecting them in the service layer.

## Adding a New Template

1. Create a new file in `backend/templates/email/` (e.g., `welcome.html`) and wrap its markup with `{{define "welcome.html"}} … {{end}}`.
2. Reuse shared header/footer with `{{template "header" .}}` and `{{template "footer" .}}`.
3. Add the template name to the mail-sending code (e.g., set `Message.Template = "welcome.html"`).
4. Pass all required data via `Message.Content`. Keep keys snake_case or camelCase; Go templates will handle either.
5. Update documentation and tests as needed.

## Testing & Debugging

- **Mock Mailer**: Without SMTP credentials, the application automatically logs emails (recipient, subject, template) so you can verify data without delivering messages.
- **Local SMTP**: Point `EMAIL_SMTP_HOST` to a local catcher (e.g., [MailHog](https://github.com/mailhog/MailHog)) to preview rendered HTML.
- **Premailer Output**: If you modify CSS, run a password reset or invitation locally and inspect the logged HTML to ensure styles inline correctly.

## Example: Dispatching a Password Reset Email

```go
frontend := strings.TrimRight(factory.FrontendURL, \"/\")
message := email.Message{
    From:     factory.DefaultFrom,
    To:       email.NewEmail(\"\", account.Email),
    Subject:  \"Password Reset Request\",
    Template: \"password-reset.html\",
    Content: map[string]any{
        \"ResetURL\":      fmt.Sprintf(\"%s/reset-password?token=%s\", frontend, token.Token),
        \"ExpiryMinutes\": int(factory.PasswordResetExpiry.Minutes()),
        \"LogoURL\":       fmt.Sprintf(\"%s/images/moto_transparent.png\", frontend),
    },
}
go func() {
    if err := factory.Mailer.Send(message); err != nil {
        log.Printf(\"Failed to send password reset email: %v\", err)
    }
}()
```

Use this pattern as a reference when emitting additional transactional emails.


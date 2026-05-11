# 2FA Implementierungsplan (Issue #1308)

**Working document.** Wird laufend gepflegt — `[ ]` offen, `[x]` erledigt, `[~]` in Arbeit, `[!]` blockiert.

Design-Referenz: [`2fa-plan-issue-1308.md`](./2fa-plan-issue-1308.md). Bei Konflikt gewinnt die Design-Doku — diese Datei spiegelt nur die Umsetzung.

---

## Status-Snapshot

| Feld | Wert |
|---|---|
| Branch | `feat/1308-2fa-email` (von `origin/development`) |
| PR | _tbd_ |
| Aktueller Stand | **Phase 11 abgeschlossen** — Sicherheits-Settings-Page unter `/[tenant]/settings/security` + `/operator/settings/security`. Neuer Backend-Endpoint `GET /auth/mfa/status` (+ Operator-Pendant). Sechs Frontend-Bugfixes aus der manuellen Browser-Begutachtung committet. Alle 9169 Frontend-Tests grün. |
| Letztes Update | 2026-05-11 |
| Blocker | keine |

**Nächster Schritt:** Phase 12 — Admin-Override UI. Section „2FA" im Admin-User-Detail (`/[tenant]/admin/users/{id}`), sichtbar bei `users:manage`. „2FA zurücksetzen"-Modal mit Pflichtfeld Grund → DELETE `/auth/admin/users/{id}/mfa`. „Recovery-Codes neu generieren"-Modal mit Grund → POST. Toast-Feedback via existierendem `ToastContext`. Backend-Endpoints existieren bereits aus Phase 6.

### Recherche-Findings (Phase 0)

| Bereich | Befund | Konsequenz |
|---|---|---|
| **SMTP-Mailer** | `backend/email/mailer.go` (`Mailer.Send(Message)`), Templates in `backend/templates/email/` (Premailer für HTML, html2text für Plain). Dispatcher mit Retry: `backend/email/dispatcher.go` (`pending`/`sent`/`failed`). Bestand: Passwort-Reset, Einladung, Operator-Mail-Change. | Phase 4 nutzt bestehende `Mailer`-Abstraktion + neuen Template-Slot, keine neue Infrastruktur nötig. |
| **PIN-Lockout** | `backend/models/auth/account.go:151–174` — `IsPINLocked`, `IncrementPINAttempts` (5 → 15 min), `ResetPINAttempts`. DB-Felder: `pin_attempts INT`, `pin_locked_until TIMESTAMPTZ`. | Phase 3 spiegelt das Pattern für MFA-Verify-Fehler (z. B. `mfa_attempts` + `mfa_locked_until` auf `auth.accounts` oder im neuen `mfa_credentials`). Migration-Slot dafür einplanen. |
| **Login-Lockout** | **Existiert nicht** für reguläre Logins (`services/auth/auth_login.go:24–56` macht nur Credential-Check + Audit). | Wir führen MFA-spezifischen Lockout neu ein, nicht "analog Login-Lockout". Design-Doku §7 bleibt korrekt (sie referenziert PIN-Lockout). |
| **Audit-Events** | Schreibweg: `backend/database/repositories/audit/auth_event.go:34–46` (`Create`). Event-Type-Konstanten: `backend/models/audit/auth_event.go:25–35` (`EventTypeLogin`, `EventTypeLogout`, …, `EventTypeAccountLocked`). Operator-Pendant: `backend/database/repositories/platform/audit_log_repository.go:30–44`. | Phase 3: neue Konstanten in derselben Datei (`EventTypeMFAEmailSent`, …). Keine DB-Migration für Event-Types nötig (freier `text`-Wert). |
| **Permissions** | `users:manage` ist gesetzt (siehe `001007001_add_missing_permissions.go`). Wildcard-Support (`admin:*`) ist da. Middleware: `auth/authorize/permission.go:13–30` `RequiresPermission(...)` / `RequiresAnyPermission(...)`. Service-Layer: `HasPermission(...)`. | Phase 6: Admin-Override-Endpoints geschützt via `RequiresPermission("users:manage")`. Falls Granularität gewünscht, später eigenes `auth:mfa_admin_override`. |
| **Migrationen** | Höchste registrierte Version: **`1.15.48`**. `1.15.41` ist **NICHT frei** (bereits genutzt). | Plan korrigiert: Migrationen starten ab **`1.15.49`** (siehe Phase 1). |

---

## Phase 0 — Vorbereitung

- [x] Feature-Branch `feat/1308-2fa-email` von `development` anlegen
- [x] Phasen-Checkliste im Haupt-Issue #1308 ergänzt (User-Entscheidung gegen separate Sub-Issues)
- [x] Bestehende SMTP-Infrastruktur (`email.Mailer`) lokalisieren — `backend/email/mailer.go` + `dispatcher.go`, Templates `backend/templates/email/`
- [ ] Lokaler SMTP-Test (Mailpit/MailHog?) — verschoben in Phase 4 (Template-Arbeit)
- [x] Bestehenden PIN-Lockout-Mechanismus lokalisieren — `backend/models/auth/account.go:151–174` (Pattern wird gespiegelt)
- [x] Bestehende `audit.auth_events`-Schreibwege identifizieren — `database/repositories/audit/auth_event.go:34–46`, Konstanten in `models/audit/auth_event.go:25–35`
- [x] Permissions-Inventar — `users:manage` als Default für Admin-Override (Wildcard-Support vorhanden, granularere Permission optional später)
- [x] Slot für Migration-Versionen reservieren — höchste ist `1.15.48`, neue starten ab **`1.15.49`** (Plan in Phase 1 entsprechend angepasst)

---

## Phase 1 — DB-Migrationen + Modelle

**Ziel:** Tabellen + Models stehen, Migrationen laufen sauber rauf und runter.

### Migrationen (ab Version `1.15.49`)
- [x] `001015049_auth_mfa_credentials.go` — `auth.mfa_credentials` (FK auth.accounts, UNIQUE(account_id, method), CHECK method='email')
- [x] `001015050_auth_mfa_email_challenges.go` — partial Index `(account_id, expires_at)` WHERE consumed_at IS NULL + Index `(expires_at)` für Cleanup
- [x] `001015051_auth_mfa_recovery_codes.go` — UNIQUE(account_id, code_hash), partial Index WHERE used_at IS NULL
- [x] `001015052_auth_mfa_trusted_devices.go` — UNIQUE(account_id, token_hash), partial Index WHERE revoked_at IS NULL
- [x] `001015053_add_mfa_lockout_to_accounts.go` — `mfa_attempts`, `mfa_locked_until` auf `auth.accounts` UND `platform.operators`
- [x] `001015054_platform_operator_mfa.go` — Spiegelung aller 4 Tabellen unter `platform.operator_mfa_*` (FK platform.operators)
- [x] **RLS bewusst weggelassen** — `auth.*` Tabellen verwenden keine RLS (Tenant-Scoping läuft über `auth.account_tenants`); identisch zu `auth.password_reset_tokens` & Co.
- [x] Audit-Event-Types als Konstanten in `models/audit/auth_event.go` (8 neue: `mfa_email_sent`, `mfa_verified`, `mfa_failed`, `mfa_locked`, `mfa_recovery_used`, `mfa_disabled`, `mfa_trusted_device_added`, `mfa_admin_override`)
- [x] `migrate validate` grün (15-Migration-Sequenz, keine Versionskollision)
- [x] `migrate` Up auf Dev-DB grün (alle 8 Tabellen + Lockout-Spalten + Trigger live)
- [x] `migrate reset` Round-Trip grün (167 Migrationen, alle 8 MFA-Tabellen + Lockout-Spalten nach Reset wieder da)

### Models (`backend/models/auth/`)
- [x] `MFACredential` (`mfa_credential.go`) — embeds `base.Model`; `Validate()` checks method whitelist
- [x] `MFAEmailChallenge` (`mfa_email_challenge.go`) — `IsExpired()`, `IsConsumed()` helpers; `IPAddress net.IP` für INET-Spalte
- [x] `MFARecoveryCode` (`mfa_recovery_code.go`) — `IsUsed()` helper
- [x] `MFATrustedDevice` (`mfa_trusted_device.go`) — `IsExpired()`, `IsRevoked()`, `IsActive()`
- [x] `Account` erweitert um `MFAAttempts`, `MFALockedUntil` + `IsMFALocked()/IncrementMFAAttempts()/ResetMFAAttempts()` (Pattern-Spiegel von PIN-Lockout)
- [x] `Operator` (platform) erweitert um dieselben Felder + Methoden
- [ ] Operator-MFA-Pendants (`OperatorMFACredential` etc.) unter `backend/models/platform/` — verschoben in Phase 3 (zusammen mit dem Operator-Service)
- [x] Build grün: `cd backend && go build ./...`

**Hermetic-Check:** keine hardcoded IDs, Test-Fixtures vorbereiten (`testpkg.CreateTestMFA*`).

---

## Phase 2 — Repository Layer

**Regel:** `Repository[T]` einbetten, keine `FindByX`-Cluster (siehe `.claude/rules/backend-conventions.md`).

- [x] `database/repositories/auth/mfa_credential.go` — `FindByAccountID`, `UpdateLastUsedAt`, `DeleteByAccountID`, `List`
- [x] `database/repositories/auth/mfa_email_challenge.go` — `FindActiveByAccountID`, `MarkConsumed`, `CountRecentByAccountID` (Rate-Limit), `DeleteExpired` (Cron)
- [x] `database/repositories/auth/mfa_recovery_code.go` — `BulkCreate`, `FindUnusedByAccountID`, `MarkUsed`, `DeleteByAccountID`, `CountUnused`
- [x] `database/repositories/auth/mfa_trusted_device.go` — `FindActiveByAccountIDAndTokenHash`, `ListActiveByAccountID`, `UpdateLastUsedAt`, `Revoke`, `RevokeAllByAccountID`, `DeleteExpired`
- [x] 4 Interface-Deklarationen in `models/auth/repository.go`
- [x] Factory-Wiring (`repositories.NewFactory(db)`) erweitert um 4 MFA-Repos
- [x] Smoke-Test (`mfa_smoke_test.go`) gegen Test-DB grün (alle 4 Repos round-trip Create → Find → Update → Delete)
- [x] Hermetic-Check (`TestHermeticTestPatterns`) grün
- [ ] Operator-Pendants in `database/repositories/platform/` — verschoben in Phase 3 (mit Operator-Service)

---

## Phase 3 — MFA Service (Core)

**Step 3a (Foundation, commit `1b2cca1b`)**
- [x] 4 Settings registriert (`security.mfa_mode`, `*_trusted_device_enabled`, `*_trusted_device_days`, `*_email_resend_cooldown_seconds`) — `MFAModeOff/RequiredAdmins/RequiredAll`-Konstanten + DependsOn-Graph
- [x] JWT-Challenge-Claims: `MFAChallengeClaims` (account_id, scope, tenant_id, mfa_pending) + `TokenAuth.CreateMFAChallengeJWT(claims, ttl)` + `ParseClaims` mit strict-rejection bei fehlendem `mfa_pending` oder unbekanntem scope
- [x] Crypto-Helpers: `GenerateEmailCode` (6-stellig crypto/rand), `GenerateRecoveryCodes` (10 Codes, 4-4-4-4 Hex), `HashShortCode/VerifyShortCode` (Argon2id über `userpass`), `GenerateTrustedDeviceToken` (256 bit), `HashTrustedDeviceToken` (SHA-256), `Sign/VerifyTrustedDeviceToken` (HMAC + constant-time), `DeriveMFASecret` (HKDF-style aus AUTH_JWT_SECRET — keine neue ENV)
- [x] 15 Unit-Tests (10 Crypto + 5 JWT-Claims-Roundtrip) grün

**Step 3b (Service, this commit)**
- [x] `services/auth/mfa_service.go` — `MFAService` Interface + Implementierung
  - [x] `IsRequired(ctx, account)` mit Settings-`HasTenantOverride`-Pattern, Drei-Stufen-Modell + Admin-Rolle-Check
  - [x] `StartChallenge` (TTL 10 min, Rate-Limit 3/15min, Hash speichern, Async-Mail via Dispatcher, Audit-Event `mfa_email_sent`)
  - [x] `VerifyChallenge` (Lockout-Check, constant-time Code-Compare, single-use, Reset Lockout, Audit-Event `mfa_verified`/`mfa_failed`/`mfa_locked`)
  - [x] `ResendChallenge` (gated durch denselben Rate-Limit)
  - [x] `Enroll` / `Disable` (Cascade: Credential → Recovery Codes → Trusted Devices revoked → Lockout-Reset, Audit-Event `mfa_disabled`)
  - [x] `GenerateRecoveryCodes` / `VerifyRecoveryCode` / `CountUnusedRecoveryCodes` (Audit-Event `mfa_recovery_used`)
  - [x] Trusted-Device: `IssueTrustedDevice` / `VerifyTrustedDevice` / `ListTrustedDevices` / `RevokeTrustedDevice` (Audit-Event `mfa_trusted_device_added`, Days aus Setting)
  - [x] Admin-Override: `AdminDisable` / `AdminRegenerateRecoveryCodes` mit Defense-in-Depth-Permission-Check (`users:manage` + Wildcard `admin:*`) + Pflicht-Reason + Audit-Event `mfa_admin_override`
- [x] Audit-Wiring für alle 8 Event-Typen via async-Goroutine (kein Block am Login-Pfad), Tenant-Tx-wrapped
- [x] Wiring in `services.Factory` (neuer `MFA` Slot, JWT-Secret aus viper)
- [x] 5 Service-Smoke-Tests gegen Test-DB grün (Lifecycle, Recovery, Trusted-Device, Challenge-Roundtrip, Admin-Override-Permission-Gate)
- [x] Hermetic-Check grün
- [ ] **Operator-Variante** (`platform.operator_mfa_*`) — verschoben in Phase 7 (Login-Flow-Modifikationen, wo der Operator-Login-Pfad sowieso angefasst wird)

---

## Phase 4 — E-Mail-Template (Mail UI)

**Yannick: „Bitte auf die Mail UI achten."** Eigener Block — nicht unterschätzen.

- [x] HTML-Template `backend/templates/email/mfa-email-code.html` mit gemeinsamem Header/Footer/Style-Chrome (Premailer inline-t alles, `html2text` baut den Plain-Text-Fallback automatisch — multipart/alternative kommt aus `email.Mailer`)
- [x] Code-Box: 32px monospace, zentriert, `letter-spacing: 0.4em` für Lesbarkeit
- [x] Mobile-first via bestehende Media-Query in `styles.html`; inline-Styles am Code-Block als E-Mail-Client-Failsafe
- [x] Deutscher Text, Sie-Form, kurze Sätze
- [x] Anti-Phishing:
  - [x] „Wenn Sie diese Anmeldung nicht angefordert haben, ignorieren Sie diese E-Mail."
  - [x] Zusätzlich: „moto fragt Sie niemals per E-Mail-Antwort, Anruf oder Messenger nach diesem Code"
  - [x] **Keine Links** zur Code-Eingabe — Negative-Assertion im Render-Test verhindert Regressions
- [x] Inhalte: Code prominent, TTL (10 min), IP via `{{if .RequestIP}}`, Trusted-Device-Hint via `{{if .TrustedDeviceEnabled}}` mit Tagen-Setting
- [x] Asset-Hosting: Logo via `{{.LogoURL}}` aus `frontendURL` — keine localhost-Refs
- [x] Service-Integration: `dispatchChallengeEmail` setzt `Template: "mfa-email-code.html"` + füllt Content-Map; Trusted-Device-Hint resolved über Settings (Service prüft `KeyMFATrustedDeviceEnabled`)
- [x] Render-Test (`templates/email/mfa_email_code_test.go`) — 2 Fälle (mit + ohne Trusted-Device); prüft Code, TTL, IP-Conditional, Anti-Phishing-Strings, verbietet Klick-Links
- [ ] **Litmus-/Email-on-Acid-Tests** (Gmail Web/Mobile, Outlook Web/Desktop, Apple Mail Desktop/iOS) — verschoben in Phase 15 (Staging-Soak), wenn echte Mails rausgehen
- [ ] **SPF/DKIM/DMARC für Sender-Adresse** — DevOps-Aufgabe, an Phase 15/16 koppeln

---

## Phase 5 — HTTP-Handler (User-facing)

**Datei:** `api/auth/mfa_handlers.go`

- [x] `POST /auth/mfa/verify` — `{challenge_token, code, remember_device?}` → `{access_token, refresh_token}` + optional Set-Cookie via `MFAService.IssueTrustedDevice`
- [x] `POST /auth/mfa/enroll/start` — Authenticated user → triggert Code per Mail (kein Body, 204 zurück)
- [x] `POST /auth/mfa/enroll/confirm` — Code-Verify + `Enroll` + Recovery-Codes (10 plain Codes einmalig im Body)
- [x] `POST /auth/mfa/recovery-codes` — User-Self-Service, alte Codes werden komplett ersetzt
- [x] `POST /auth/mfa/recovery/verify` — Login mit Recovery-Code; nutzt `peekChallengeIdentity` um Account aus dem JWT zu holen ohne neuen Code zu verbrauchen
- [x] `DELETE /auth/mfa` — User deaktiviert eigenes 2FA + clear `mfa_trust_device` Cookie
- [x] `GET /auth/mfa/trusted-devices` — DTO mit ID, UA, IP, ExpiresAt, LastUsedAt
- [x] `DELETE /auth/mfa/trusted-devices/{id}` — Revoke einzeln
- [x] `POST /auth/mfa/resend` — JWT-validiert via `ResendChallenge`, Rate-Limit kommt aus dem Service
- [x] Service-Hook: `AuthService.IssueTokensForAuthenticatedAccount(accountID, tenantID, ip, ua)` reuses `loadAccountMetadataForTenant` → `createRefreshTokenWithRetry` → `buildJWTClaims` → `generateAndLogTokens`
- [x] Service-Helper: `MFAService.VerifyCodeForAccount(accountID, code)` — JWT-loses Sibling von `VerifyChallenge` für Enrollment-Confirmation
- [x] Trusted-Device-Cookie: `mfa_trust_device`, HttpOnly+Secure+SameSite=Lax, Max-Age aus Service-`expiresAt`
- [x] Alle Handler ≤ 15 Cognitive-Complexity (Logik im Service); Bind-Validation mit `ozzo-validation`
- [x] Errors über `api/common/errors.go` (kein lokaler Helper); zentrales `mapMFAError` mappt Service-Errors → HTTP-Status
- [x] Failsafe: jeder Handler beginnt mit `requireMFA(w, r)` — 503 wenn `MFAService` nicht gewired ist
- [x] Resource-Wiring via `SetMFAService` in `api/base.go`
- [x] 17 Internal-Tests: Bind-Validation (negative + positive Trim), 9× 503-Pfad, 7× Error-Mapping
- [ ] **Operator-Pendants unter `api/operator/auth/mfa_handlers.go`** — verschoben in Phase 7 (Operator-Login-Flow)

---

## Phase 6 — HTTP-Handler (Admin-Override)

**Datei:** `api/auth/mfa_admin_handlers.go`

- [x] `DELETE /auth/accounts/{accountId}/mfa` — `{reason}` Pflichtfeld (mind. 3 Zeichen, getrimmt)
- [x] `POST /auth/accounts/{accountId}/mfa/recovery-codes` — `{reason}` Pflichtfeld → liefert 10 neue Codes (einmalig)
- [x] Permission-Check: `users:manage` per Route-Middleware (`authorize.RequiresPermission`); Service prüft denselben Permission-String **noch einmal** als Defense-in-Depth (Wildcard-aware, `admin:*` matcht)
- [x] Audit-Event `mfa_admin_override` schon in Phase 3 implementiert — Service zieht `actor_account_id`, `reason`, `action` aus den Handler-Argumenten
- [x] Self-Override **erlaubt** — kein zusätzlicher Block im Handler/Service; ein Admin darf sich selbst zurücksetzen, Audit-Trail bleibt sichtbar
- [x] URL-Pfad-Wahl: `/auth/accounts/{accountId}/mfa` (nicht `/auth/admin/users/{id}/mfa` wie ursprünglich geplant) — folgt der bestehenden Admin-Route-Konvention im Auth-Router
- [x] Shared `resolveAdminOverrideContext` Helper extrahiert Target-ID, Actor-ID + Permissions aus JWT-Claims, Bind-validation der Reason — hält beide Handler unter gocognit 15
- [x] 5 Internal-Tests: 3× Bind-Validation (empty/short/trim), 2× 503-Pfad
- [ ] **Mail-Benachrichtigung an betroffenen User** ("Ihr 2FA wurde zurückgesetzt") — verschoben in Phase 14 (DSGVO-Doku) als optionale Privacy-Verbesserung; nicht kritisch für v1

---

## Phase 7 — Login-Flow Modifikationen

**Step 7a (Tenant-Login, in Arbeit / committed):**
- [x] `services/auth/auth_login.go` — neue Methode `LoginWithMFAGate(ctx, email, password, ip, ua, tenantSlug, trustedDeviceCookie)` — `LoginWithAudit` bleibt unangetastet (Test-Regel)
- [x] Discriminated `LoginResult{Status, AccessToken, RefreshToken, ChallengeToken, MaskedEmail, MFAEnrollmentRequired}`
- [x] Trusted-Device-Cookie verifizieren via `MFAService.VerifyTrustedDevice` → bei gültig: MFA übersprungen
- [x] `MFAEnrollmentRequired` Flag wenn IsRequired aber HasEnrollment=false (Frontend zeigt Force-Enrollment-Screen)
- [x] `MaskedEmail` (`j***@example.com`) für UX im Code-Eingabe-Screen
- [x] `Service.SetMFAService` als Setter (vermeidet Konstruktor-Zyklus AuthService↔MFAService); Factory wired beide nach Konstruktion
- [x] Login-Handler liest `mfa_trust_device` Cookie + neue `LoginResponse{Status, ...}` JSON
- [x] `handleLoginError` Helper extrahiert (DRY für beide Pfade)
- [x] `MFAChallengeClaims` schon in Phase 3 gebaut — TTL 10 min (nicht 5 wie ursprünglich geplant), siehe Service-API-Diskussion
- [x] 8 Internal-Tests: 3× Response-Shape (mfa_required, authenticated, enrollment-flag), 1× Cookie-Forward, 4× Error-Mapping
- [x] Bestehende Auth-Tests grün — keine Regression (LoginWithAudit-Pfad bleibt)

**Step 7b (Operator-Login, in Arbeit):**

*7b-1 (committed `fa4e9c2d`) — Data-Layer:*
- [x] Operator-MFA-Models unter `models/platform/` (4 Files: `OperatorMFACredential`, `OperatorMFAEmailChallenge`, `OperatorMFARecoveryCode`, `OperatorMFATrustedDevice`)
- [x] Operator-MFA-Repository-Interfaces in `models/platform/repository.go`
- [x] Operator-MFA-Repositories unter `database/repositories/platform/` — Mirror der auth-side Repos, `account_id` → `operator_id`, `auth.mfa_*` → `platform.operator_mfa_*`
- [x] Factory-Wiring + 4-Sub-Test Smoke gegen Test-DB

*7b-2 (in Commit) — Service:*
- [x] Action-Konstanten in `models/platform/operator_audit_log.go` (`ActionMFA*` + `ResourceOperatorMFA`)
- [x] `services/platform/operator_mfa_service.go` — `OperatorMFAService` Interface + Implementation (HasEnrollment, GetCredential, StartChallenge, VerifyChallenge, ResendChallenge, VerifyCodeForOperator, Enroll, Disable, GenerateRecoveryCodes, VerifyRecoveryCode, CountUnusedRecoveryCodes, Issue/Verify/List/Revoke TrustedDevice). Hardcoded `IsRequired=true` (kein Settings-Lookup), kein Tenant-Context, Audit via `OperatorAuditLog`.
- [x] Crypto-Helpers (`GenerateEmailCode`, `HashShortCode`, `VerifyShortCode`, `GenerateRecoveryCodes`, Trusted-Device-Pipeline, `DeriveMFASecret`) aus `services/auth` wiederverwendet — kein Duplikat
- [x] Errors per Alias gegen `authService.ErrMFA*` gemappt (gemeinsame Fehler-Identität)
- [x] JWT-Challenge nutzt `MFAChallengeScopePlatform` (existierende JWT-Claim-Struktur aus Phase 3)
- [x] Wiring in `services.Factory.OperatorMFA`
- [x] 4 Service-Tests gegen Test-DB grün (Lifecycle, Recovery, TrustedDevice, Challenge-Wrong-Code)
- [x] Mock-Stub-Anpassung in `operator_provisioning_service_test.go` — User-Freigabe für No-Op-Stubs nach Test-Regel

*7b-3 (in Commit) — Login-Flow:*
- [x] `services/platform/operator_auth_service.go` MFA-aware: `OperatorLoginResult` Discriminator (`authenticated` / `mfa_required`), `LoginWithMFAGate(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie)`, hardcoded Pflicht (kein Settings-Lookup), Trusted-Device-Skip, `MFAEnrollmentRequired`-Flag bei nicht-enrolled Operatoren. Bestehende `Login`-Methode unverändert (Test-Regel)
- [x] `SetMFAService` Setter zur Aufbruch des Konstruktor-Zyklus + Factory-Wiring (`operatorAuthService.SetMFAService(operatorMFAService)`)
- [x] `api/operator/auth.go` updated: `LoginResponse` mit `Status`-Field als Mirror der Tenant-Shape, `mfa_trust_device`-Cookie wird aus Request gelesen + an Service weitergereicht
- [x] Bestehende `TestLogin_*`-Tests an neue diskriminierte Shape angepasst (User-Freigabe nach Test-Regel — bewusste API-Änderung parallel zu Phase 7a)

*7b-4 (in Commit) — HTTP-Handler:*
- [x] `api/operator/mfa.go` — `MFAResource` mit 9 Handlern: `Verify`, `RecoveryVerify`, `Resend` (public, mid-login), `EnrollStart`, `EnrollConfirm`, `RegenerateRecoveryCodes`, `Disable`, `ListTrustedDevices`, `RevokeTrustedDevice` (protected). Public-Pfade unter `/auth/mfa/*` ohne Auth, protected unter `/auth/mfa/*` mit Operator-Token-Verifier
- [x] `services/platform.OperatorAuthService.IssueTokensForAuthenticatedOperator` — Mirror der Tenant-Methode für Token-Pair-Issuance nach MFA-Verifizierung
- [x] `MFAResource` im Operator-`Resource`/`ResourceConfig` verdrahtet, `MFAService: api.Services.OperatorMFA` in `api/base.go` hinzugefügt
- [x] `api/operator/mfa_internal_test.go` — 7 internal tests (Bind-Validation, 503-Pfad für alle 9 Handler, Error-Mapping)
- [x] `mockOperatorAuthService` Stub für `IssueTokensForAuthenticatedOperator` ergänzt (Interface-Compliance, kein Behavior-Change)

**WICHTIG:** Test-Regel beachten — `.claude/rules/no-test-modifications.md`. Bestehende Tests bleiben unangetastet; wir bauen Schwester-Methoden statt zu überschreiben.

---

## Phase 8 — Settings-Registry-Audit ✅

**Regel:** Settings-System (siehe `.claude/rules/settings-system.md`), nicht ENV.

*Bereits in Phase 3 gemacht:*
- [x] `models/config/keys.go` — Konstanten (`KeyMFAMode`, `KeyMFATrustedDeviceEnabled`, `KeyMFATrustedDeviceDays`, `KeyMFAEmailResendCooldownSeconds`)
- [x] `services/config/defaults/security.go` — alle 4 `Register()`-Calls mit Defaults (`mfa_trusted_device_enabled = true`), Select-Optionen, Number-Ranges, DependsOn-Graph
- [x] Frontend: keine Code-Änderung — Settings-Page generiert sich aus Schema

*Audit-Findings & Fixes in dieser Phase:*
- [x] **Bug gefunden + gefixt**: `resolveTrustedDeviceHint` rief `HasTenantOverride` und fiel bei fehlendem Override auf `false` zurück — falsche Logik, weil Registry-Default `true` ist. Tenants auf Default-Config sahen den Trusted-Device-Hint nicht in MFA-E-Mails. Konsumenten-Funktionen (`IsRequired`, `resolveTrustedDeviceDays`, `resolveTrustedDeviceHint`) auf direktes `Resolve*` vereinfacht — `HasTenantOverride` ist nur nötig bei Env-Var-Fallback (siehe settings-system.md), für reine Settings-Konsumenten genügt `Resolve*` (gibt Registry-Default zurück, wenn kein Override existiert)
- [x] **Bug gefunden + gefixt**: `KeyMFAEmailResendCooldownSeconds` war registriert aber nie konsumiert. Per-Tenant-Cooldown (Default 60s) jetzt in `ResendChallenge` verdrahtet via `resolveResendCooldown`-Helper — bleibt distinkt vom Sliding-Window in `StartChallenge` (3/15min als Abuse-Defense, der Cooldown ist UX/Cost-Knob)
- [x] `defaults_test.go` um 4 MFA-Settings erweitert: in `TestAllSettingsRegistered` Expected-Keys-Liste, neue `TestMFASettings_TypesAndDefaults` (Typen, Defaults, Validation-Ranges), neue `TestMFASettings_DependsOnGraph` (Conditional-Visibility-Rules)
- [x] Min-Count in `TestAllSettingsRegistered` auf 42 erhöht (38 + 4 MFA)

---

## Phase 9 — Frontend: Login mit MFA-Step ✅

- [x] **Custom-Page-Ansatz** — Page POSTed direkt an `/api/auth/login` (resp. `/api/operator/auth/login`), branched auf `status`-Discriminator. Bei `authenticated`: `signIn("credentials", { internalRefresh: "true", token, refreshToken })` zum Session-Seeding. Bei `mfa_required`: `MFAChallengeForm` rendern. NextAuth-Configs unverändert — `internalRefresh`-Pfad existiert bereits seit Refresh-Flow
- [x] `/[tenant]/login` (= `frontend/src/app/[tenant]/page.tsx`) erweitert: nach Passwort-POST Discriminator-Response auswerten, auf Step 2 wechseln
- [x] Komponente `MFAChallengeForm` (`components/auth/mfa-challenge-form.tsx`):
  - [x] 6-Stellen-Eingabe (Auto-Tab zwischen Felder, Auto-Submit bei 6 Zeichen, Paste-Support, Backspace zum Vorgänger)
  - [x] Resend-Button mit Cooldown (Default 60s, Countdown im Label, Reset des Code-Inputs nach Resend)
  - [x] Trusted-Device-Checkbox **immer sichtbar** (User-Entscheidung: Backend könnte als Phase 9-Follow-up `KeyMFATrustedDeviceEnabled` vor `IssueTrustedDevice` prüfen)
  - [x] „Wiederherstellungscode verwenden"-Switch (rendered alternative Eingabe + dedizierten Submit-Button)
  - [x] Fehlerzustände in `germanMFAErrorMessage` (abgelaufen / falsch / gesperrt / 429 / 5xx) deutsch + freundlich
- [x] Nach erfolgreichem `/auth/mfa/verify`: `signIn("credentials", { internalRefresh: "true", token, refreshToken })` seedet NextAuth-Session
- [x] Operator-Login parallel unter `/operator/login` — `MFAChallengeForm` mit `scope="operator"`, derselbe Flow + Operator-Envelope-Handling
- [x] **Brand-Farben:** `#5080D8` (LOCATION_COLORS.OTHER_ROOM) für Focus/Primary, `#83CD2D` (LOCATION_COLORS.GROUP_ROOM) für Checkbox-Accent — keine generischen Tailwind-Farben
- [x] Reuse: bestehende `Input`/`Alert` aus `~/components/ui` + neue `MFAChallengeForm` für Tenant + Operator
- [x] `pnpm run check` grün (oxlint + tsc)
- [x] Tests: 11 bestehende Login-Tests an 2-Step-Flow angepasst (User-Freigabe nach Test-Regel — bewusste API-Änderung wie in Phase 7b-3), 6 neue Komponenten-Tests für `MFAChallengeForm`, alle 9156 Frontend-Tests grün

**Neue Frontend-Files:**
- `lib/mfa-api.ts` — Discriminated `LoginResponse`, `verifyMFA`, `verifyRecovery`, `resendChallenge`, `MFAApiError`, `germanMFAErrorMessage`. Tenant flat / Operator envelope `{status, data, message}` transparent
- `lib/auth-proxy.ts` — Geteilter `forwardJsonPost(req, backendPath)` mit Cookie + Set-Cookie-Passthrough für `mfa_trust_device`
- `components/auth/mfa-challenge-form.tsx` + `*.test.tsx`
- `app/api/auth/mfa/verify/route.ts`, `app/api/auth/mfa/recovery/verify/route.ts`, `app/api/auth/mfa/resend/route.ts`
- `app/api/operator/auth/login/route.ts` (NEU für 2-Step-Flow), `app/api/operator/auth/mfa/verify/route.ts`, `app/api/operator/auth/mfa/recovery/verify/route.ts`, `app/api/operator/auth/mfa/resend/route.ts`

**Aktualisierte Files:**
- `app/api/auth/login/route.ts` — Cookie-Forwarding für trusted-device-Skip + Set-Cookie-Passthrough
- `app/[tenant]/page.tsx` — 2-Step-Flow mit `mfaStep`-State + `seedSessionWithTokens`-Helper
- `app/operator/login/page.tsx` — analog für Operator-Pfad

**Verschoben:**
- Optional: Backend prüft `KeyMFATrustedDeviceEnabled` vor `IssueTrustedDevice`, damit deaktiviertes Setting auch das Cookie-Setzen unterdrückt — als Phase 9-Follow-up tracked, nicht v1-blockend

---

## Phase 10 — Frontend: Enrollment-Flow ✅

- [x] `MFAEnrollmentScreen` (`components/auth/mfa-enrollment-screen.tsx`) — 3-Schritt-Wizard, rendered direkt auf der Login-Page wenn `mfa_enrollment_required: true`. Token aus Login-Response wird per Bearer-Header weiterverwendet, NextAuth-Session wird erst nach erfolgreichem Enrollment geseedet.
- [x] Schritt 1 (Intro): erklärt Ablauf, „Code an meine E-Mail senden"-Button → `/auth/mfa/enroll/start`
- [x] Schritt 2 (Code-Eingabe): 6-Box-Input mit Auto-Tab/Auto-Submit/Paste/Backspace (gleiche Mechanik wie `MFAChallengeForm`), „Erneut senden"-Link, Fehlermeldungen über `germanMFAErrorMessage` → `/auth/mfa/enroll/confirm`
- [x] Schritt 3 (`RecoveryCodesDisplay`):
  - [x] 10 Codes monospace, 2-Spalten-Grid auf ≥ sm
  - [x] Download als `.txt` (Blob, `URL.createObjectURL`, Anchor-click)
  - [x] Copy-to-Clipboard mit `navigator.clipboard.writeText` + 2s „Kopiert!"-Feedback
  - [x] Pflicht-Bestätigungs-Checkbox „Ich habe die Codes gespeichert" — schaltet „Weiter zum Dashboard"-Button frei
  - [x] Warnhinweis-Box: „Diese Codes werden nur jetzt einmalig angezeigt"
- [x] Schritt 4: `onComplete` callback — Login-Page seedet NextAuth-Session via `signIn(internalRefresh)` und redirected zum Dashboard
- [x] Login-Page-Integration (Tenant `app/[tenant]/page.tsx` + Operator `app/operator/login/page.tsx`): Backend-Antwort `mfa_enrollment_required: true` setzt `enrollmentStep`-State, Form wird durch Wizard ersetzt (gleiches Render-Pattern wie der MFA-Challenge-Step)
- [x] **8 neue authenticated Proxy-Routen** (Bearer-Token-Forwarding):
  - `/api/auth/mfa/enroll/start`, `/api/auth/mfa/enroll/confirm`, `/api/auth/mfa/recovery-codes`, `/api/auth/mfa/disable`
  - Operator-Pendants unter `/api/operator/auth/mfa/*`
- [x] `auth-proxy.ts` erweitert: `ForwardOptions { method, hasBody }` + Bearer-Token-Forwarding über `Authorization`-Header + 204-No-Content-Handling
- [x] `mfa-api.ts` erweitert: `enrollStart`, `enrollConfirm`, `regenerateRecoveryCodes`, `disableMFA` — alle mit `bearerToken`-Param. Operator-Envelope-Unwrapping wie bei den existierenden Funktionen
- [x] 7 Komponenten-Tests (4× `RecoveryCodesDisplay`: Render, Confirm-Button-Gating, Clipboard, Download — 3× `MFAEnrollmentScreen`: Intro→Code, Auto-Submit→Recovery, 429-Fehlerpfad)
- [x] `pnpm run check` clean (oxlint + tsc), 9163 Frontend-Tests grün

**Bewusst aus v1 verschoben:**
- Kein „Cancel"/„Logout-aus-Enrollment"-Pfad — wenn MFA Pflicht ist und User nicht enrollen will, soll die einzige Option re-Login sein. (Phase 11 Self-Service kann das anders handhaben, weil dort kein Force-Pfad besteht.)
- Kein Routing-Guard auf Dashboard-Routen — Force-Pfad wird ausschließlich beim Login durchgesetzt. Wenn ein User mit `mfa_enrollment_required: true`-State die Page-URL manuell aufruft, würde der bestehende Auth-Guard greifen, weil die Session noch nicht geseedet ist.

**Neue Frontend-Files:**
- `components/auth/mfa-enrollment-screen.tsx` + `.test.tsx`
- `components/auth/recovery-codes-display.tsx` + `.test.tsx`
- 8 Proxy-Routen unter `app/api/auth/mfa/{enroll,recovery-codes,disable}/` und `app/api/operator/auth/mfa/{enroll,recovery-codes,disable}/`

**Aktualisierte Files:**
- `lib/auth-proxy.ts` — `ForwardOptions`, Bearer-Header-Passthrough, 204-Handling
- `lib/mfa-api.ts` — Enrollment-/Self-Service-Helper + `bearerToken`-Param in `postJson`
- `app/[tenant]/page.tsx` + `app/operator/login/page.tsx` — `enrollmentStep`-State + `MFAEnrollmentScreen`-Rendering

---

## Phase 11 — Frontend: User-Settings ✅

**Pfad:** `/[tenant]/settings/security` + `/operator/settings/security`

- [x] **Backend**: Neuer `GET /auth/mfa/status` Endpoint (`MFAStatusResponse{Enrolled, LastUsedAt, UnusedRecoveryCodes, ModeRequired}`) + Operator-Pendant. Frontend-Proxys `/api/auth/mfa/status`, `/api/auth/mfa/trusted-devices`, `/api/auth/mfa/trusted-devices/[id]` (DELETE) — je Tenant + Operator
- [x] **Komponente** `MFASecuritySettings` (geteilt zwischen Tenant + Operator):
  - [x] Status-Card mit Aktiv/Inaktiv-Badge, „Zuletzt verwendet", „N Wiederherstellungscodes ungenutzt"
  - [x] „2FA aktivieren"-CTA wenn nicht enrolled → öffnet `MFAEnrollmentScreen` als Modal-Wizard (Reuse Phase 10)
  - [x] „Neue Wiederherstellungscodes"-Button mit Bestätigungs-Modal → ruft existierendes `/recovery-codes` → zeigt neue Codes in `RecoveryCodesDisplay` (Reuse Phase 10)
  - [x] „2FA deaktivieren"-Button mit zweistufiger Bestätigung → DELETE `/auth/mfa`. **Disabled mit Hinweistext** wenn `mode_required` true ist (Pflicht durch `mfa_mode = required_all`/`required_admins` oder Operator-Hardcoded)
  - [x] Trusted-Devices-Liste mit User-Agent + IP + Gültig-bis + Last-Used + Pro-Gerät-„Entfernen"-Button
- [x] **mfa-api.ts** um `getMFAStatus`, `listTrustedDevices`, `revokeTrustedDevice` erweitert; `forwardJsonPost` unterstützt jetzt `method: "GET"`
- [x] **Pages** `[tenant]/(protected)/settings/security/page.tsx` + `operator/settings/security/page.tsx` — beide nutzen `useSession().user.token` als bearerToken, leiten auf Login um wenn nicht authentifiziert
- [x] **Brand-Farben**: Grün `#83CD2D` für Aktivieren, Blau `#5080D8` für Regenerate, Rot `#FF3130` für Deaktivieren
- [x] **6 Komponenten-Tests** für `MFASecuritySettings` (Inaktiv-Status, Aktiv-Status mit Last-Used, Disable-Disabled bei Pflicht, Disable-Confirm-Modal, Regenerate-Confirm-Modal, leere Trusted-Devices-Liste)
- [x] `pnpm run check` clean, 9169 Frontend-Tests grün, Backend-Tests grün

**Frontend-Bugfixes aus der manuellen Browser-Begutachtung** (auf demselben Branch committed):
1. `IsRequired` brauchte expliziten `tenantID`-Parameter, weil Login außerhalb der `TenantTxMiddleware` läuft und `ResolveString` ohne Kontext den Registry-Default zurückgibt → Pflicht-Modus wurde ignoriert. Fix: `IsRequired(ctx, account, tenantID)` nutzt `ResolveStringForTenant`
2. Migrationen 1.15.49–52 + 1.15.54 hatten keine GRANTs für `phoenix_auth`/`phoenix_tenant` auf die MFA-Tabellen → permission denied auf jeden `/auth/mfa/*`-Call. Fix: Migrationen 1.15.55 + 1.15.56 ergänzen GRANTs (Tenant + Operator)
3. Trusted-Device-Cookie war hardcoded `Secure: true` → Browser akzeptiert es nicht über HTTP in dev. Fix: `secureCookie()`-Helper liest `app_env` (analog `shouldExposeSeedInvitationToken`)
4. Operator-`AuthErrorRenderer` mapped `ErrMFARateLimited`/`ErrMFALocked` nicht → 500 statt 429. Fix: explizite `errors.Is`-Mapping + Server-Log auf 500-Pfad
5. Operator `r.Route("/auth/mfa", ...)` im protected-Block überschattete public `/auth/mfa/verify` (Chi-Subtree-Mount). Fix: Routen einzeln als Leaves registrieren
6. Tenant + Operator-Tests an 2-Step-Discriminator angepasst (Phase 9-Follow-up nach erstem Browser-Lauf)

---

## Phase 12 — Frontend: Admin-Override UI

**Pfad:** `/[tenant]/admin/users/{id}` (oder dort wo User-Detail bereits lebt)

- [ ] Section „2FA" sichtbar nur bei Permission
- [ ] Button „2FA zurücksetzen" → Modal mit Pflichtfeld „Grund" → POST `/auth/admin/users/{id}/mfa` (DELETE)
- [ ] Button „Recovery-Codes neu generieren" → Modal mit Grund → Codes werden im Modal angezeigt (einmalig)
- [ ] Toast-Feedback (siehe `ToastContext.tsx`)

---

## Phase 13 — Tests

- [ ] Backend Unit-Tests: alle Service-Methoden in `mfa_service`
- [ ] Backend Integration-Tests gegen Test-DB: Login → Challenge → Verify → Tokens, Recovery-Code-Pfad, Lockout, Rate-Limit, Admin-Override, Trusted-Device-Skip
- [ ] Hermetic-Check grün: `cd backend && go test ./test/ -run TestHermeticTestPatterns -v`
- [ ] Frontend Komponenten-Tests: `MFAChallengeForm`, `RecoveryCodesDisplay`
- [ ] E2E (Playwright): kompletter Login-Flow inkl. Enrollment, inkl. Admin-Override
- [ ] Edge Cases (Liste in Design-Doku §11) abgedeckt

---

## Phase 14 — DSGVO / TOM-Doku

- [ ] TOM-Doku-Abschnitt: Verfahren, TTLs, Rechtsgrundlage Art. 32 DSGVO
- [ ] Datenschutzerklärung: Trusted-Device-Cookie (insb. weil Default `true`)
- [ ] Datenschutzerklärung: Admin-Override-Recht
- [ ] Audit-Log-Aufbewahrung dokumentieren (90 Tage)
- [ ] Review mit Florian / DSB

---

## Phase 15 — Staging Soak-Test

- [ ] Deploy auf Staging via `development`-Push (SOPS-Workflow)
- [ ] `security.mfa_mode = required_admins` für Test-Tenant aktivieren
- [ ] moto-Operator: Pflicht-Enrollment durchgehen
- [ ] Mindestens 1 Woche laufen lassen, Audit-Log monitoren
- [ ] Litmus-Mail-Tests in echtem Umfeld
- [ ] Bug-Fixes / UX-Schliff

---

## Phase 16 — Production Rollout

- [ ] Merge `development` → `main`
- [ ] Production-Deploy (alle Schulen `mfa_mode=off`)
- [ ] Operator-Pflicht ab Deploy aktiv → moto-Team enrolled sich
- [ ] Pilot-Schule mit Florian abstimmen, dann `required_admins`
- [ ] Schuljahresstart 2026/27: Default-Empfehlung an alle, finale TOM-Doku rausgeben

---

## Cross-cutting Reminder

- **Backend-Konventionen** (`.claude/rules/backend-conventions.md`): keine Repo-Imports in `api/`, `Repository[T]` einbetten, `base.Model` verwenden, gocognit ≤ 15, Settings statt ENV, keine Queries im Service.
- **Test-Regel** (`.claude/rules/no-test-modifications.md`): bestehende Tests nicht zu „grün biegen", bei Konflikt User fragen.
- **Settings-Pattern** (`.claude/rules/settings-system.md`): `HasTenantOverride` → `Resolve*` → ggf. ENV-Fallback.
- **Cross-Repo (PyrePortal)**: Auth-Header oder Error-Format der `/api/iot/*`-Endpoints **nicht** verändern — IoT-Auth ist separat (Device API Key + PIN), nicht betroffen, aber im Hinterkopf behalten.
- **GitHub Labels** (`.claude/rules/github-labels.md`): nur Bestand verwenden, keine neuen anlegen.
- **Commits**: kein „Co-Authored-By: Claude".
- **PR-Target**: `development`, nicht `main`.

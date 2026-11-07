"use client";

import { Check } from "lucide-react";
import { EmptyState } from "~/components/ui/empty-state";
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- redirect targets root login, not tenant route
import { useRouter } from "next/navigation";
import { signOut } from "next-auth/react";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FieldLabel } from "~/components/ui/field-label";
import { Input } from "~/components/ui/input";
import {
  credentialError,
  INVITATION_REFUSALS,
} from "~/components/auth/credential-error";
import { PasswordRequirements } from "~/components/auth/password-requirements";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { useApiFormError } from "~/contexts/ToastContext";
import { getRoleDisplayName } from "~/lib/auth-helpers";
import { acceptInvitation } from "~/lib/invitation-api";
import type { InvitationValidation } from "~/lib/invitation-helpers";
import { PASSWORD_RULES } from "~/lib/password-rules";
import { createLogger } from "~/lib/logger";
import { Button } from "~/components/ui/button";
import { InvitationSummary } from "~/components/auth/invitation-summary";
import { InsetPanel } from "~/components/ui/inset-panel";

const logger = createLogger({ component: "InvitationAccept" });

interface InvitationAcceptFormProps {
  readonly token: string;
  readonly invitation: InvitationValidation;
  /**
   * Fixed post-accept redirect path on the CURRENT host. When set, the
   * tenant-subdomain redirect is skipped entirely — the school portal
   * (#2207) sends freshly created Lehrkraft accounts to its own login
   * instead of the tenant portal.
   */
  readonly redirectToPath?: string;
}

/** Prüfung vor dem Senden. Schlüssel sind die Feldnamen des Backends. */
function acceptFieldErrors(
  firstName: string,
  lastName: string,
  passwordOk: boolean,
  passwordsMatch: boolean,
): Record<string, string> {
  const fields: Record<string, string> = {};
  if (!firstName.trim())
    fields.first_name = "Bitte geben Sie Ihren Vornamen an.";
  if (!lastName.trim())
    fields.last_name = "Bitte geben Sie Ihren Nachnamen an.";
  if (!passwordOk) {
    fields.password = "Das Passwort erfüllt noch nicht alle Anforderungen.";
  } else if (!passwordsMatch) {
    fields.confirm_password = "Die Passwörter stimmen nicht überein.";
  }
  return fields;
}

export function InvitationAcceptForm({
  token,
  invitation,
  redirectToPath,
}: InvitationAcceptFormProps) {
  const router = useRouter();
  const [firstName, setFirstName] = useState(invitation.firstName ?? "");
  const [lastName, setLastName] = useState(invitation.lastName ?? "");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  // Fehler über den gemeinsamen Weg (#2517): Katalogtext im Fehlerkasten,
  // Feldfehler am Feld. Vor der Anmeldung heißt 401 "abgelehnt".
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const latestSubmitRef = useRef<() => void>(() => undefined);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isAccepted, setIsAccepted] = useState(false);
  const [signOutFailed, setSignOutFailed] = useState(false);
  const [signOutRetryFailed, setSignOutRetryFailed] = useState(false);
  const [tenantRedirectUrl, setTenantRedirectUrl] = useState<string | null>(
    null,
  );

  useEffect(() => {
    setFirstName(invitation.firstName ?? "");
    setLastName(invitation.lastName ?? "");
  }, [invitation.firstName, invitation.lastName]);

  const requirementStatus = useMemo(
    () =>
      PASSWORD_RULES.map(({ label, test }) => ({
        label,
        met: test(password),
      })),
    [password],
  );

  const allRequirementsMet = useMemo(
    () => requirementStatus.every((requirement) => requirement.met),
    [requirementStatus],
  );

  const submit = async () => {
    const fields = acceptFieldErrors(
      firstName,
      lastName,
      allRequirementsMet,
      password === confirmPassword,
    );
    if (Object.keys(fields).length > 0) {
      formErrors.invalid("Bitte prüfen Sie die markierten Felder.", fields);
      return;
    }
    formErrors.clear();

    try {
      setIsSubmitting(true);
      const result = await acceptInvitation(token, {
        firstName: firstName.trim(),
        lastName: lastName.trim(),
        password,
        confirmPassword,
      });
      // Build redirect URL before signOut (need window.location)
      let redirectUrl: string | null = null;
      if (redirectToPath) {
        // Portal-fixed target (school portal): stay on this host.
        redirectUrl = null;
      } else if (result.tenantSubdomain && globalThis.window !== undefined) {
        const tenantDomain = process.env.NEXT_PUBLIC_TENANT_DOMAIN;
        if (tenantDomain) {
          const { protocol, port: locationPort } = globalThis.window.location;
          const port = locationPort ? `:${locationPort}` : "";
          redirectUrl = `${protocol}//${result.tenantSubdomain}.${tenantDomain}${port}/`;
        }
      }

      // Clear any existing session before redirecting to login.
      // If signOut fails, we must NOT auto-redirect: the tenant login
      // page auto-redirects authenticated sessions, so the user would
      // bounce back as the old account instead of being able to log in
      // as the newly created one.
      try {
        await signOut({ redirect: false });
      } catch (signOutError) {
        logger.warn("sign_out_after_invite_failed", {
          error:
            signOutError instanceof Error
              ? signOutError.message
              : String(signOutError),
        });
        setSignOutFailed(true);
        setTenantRedirectUrl(redirectUrl);
        setIsAccepted(true);
        return;
      }

      setIsAccepted(true);

      setTimeout(() => {
        if (redirectUrl) {
          globalThis.window.location.href = redirectUrl;
          return;
        }
        router.push(redirectToPath ?? "/");
      }, 1500);
    } catch (err) {
      logger.error("invitation_accept_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      // Ohne Netz kommt general.unavailable mit Wiederholen.
      void formErrors.show(credentialError(err, INVITATION_REFUSALS), {
        object: "die Einladung",
        retry: () => latestSubmitRef.current(),
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    void submit();
  };

  // „Wiederholen“ sendet den Stand, der dann im Formular steht.
  useLayoutEffect(() => {
    latestSubmitRef.current = () => void submit();
  });

  const fieldError = formErrors.fieldError;

  const handleManualRedirect = async () => {
    try {
      await signOut({ redirect: false });
    } catch {
      // signOut failed again — don't redirect, the old session would
      // cause the login page to bounce the user back as the old account.
      setSignOutFailed(true);
      setSignOutRetryFailed(true);
      return;
    }
    // signOut succeeded this time — safe to navigate
    if (tenantRedirectUrl) {
      globalThis.window.location.href = tenantRedirectUrl;
    } else {
      router.push(redirectToPath ?? "/");
    }
  };

  if (isAccepted) {
    return (
      <EmptyState
        tone="success"
        icon={<Check aria-hidden="true" />}
        titleAs="h3"
        title="Konto erstellt"
        description={
          signOutRetryFailed
            ? "Die vorherige Sitzung konnte nicht beendet werden. Bitte löschen Sie die Websitedaten in Ihren Browsereinstellungen oder versuchen Sie es später erneut."
            : signOutFailed
              ? "Die vorherige Sitzung konnte nicht automatisch beendet werden."
              : "Bitte melden Sie sich mit Ihren neuen Zugangsdaten an."
        }
        action={
          signOutFailed && !signOutRetryFailed ? (
            <Button
              type="button"
              variant="primary"
              size="field"
              onClick={handleManualRedirect}
            >
              Zur Anmeldung
            </Button>
          ) : undefined
        }
        progress={
          signOutFailed || signOutRetryFailed ? undefined : (
            <>
              <div className="h-1 w-16 overflow-hidden rounded-full bg-gray-100">
                <div
                  className="h-full rounded-full bg-gray-900"
                  style={{
                    animation: "progressFill 1.5s ease-in-out forwards",
                  }}
                />
              </div>
              <style>{`
                @keyframes progressFill {
                  from { width: 0%; }
                  to { width: 100%; }
                }
              `}</style>
            </>
          )
        }
      />
    );
  }

  return (
    <form
      ref={formRef}
      onSubmit={handleSubmit}
      noValidate
      className="space-y-6"
    >
      <FormErrorAlert message={formErrors.error} />

      <InvitationSummary
        email={invitation.email}
        role={getRoleDisplayName(invitation.roleName)}
      />

      <InsetPanel framed className="space-y-2">
        <div>
          <span className="block text-xs font-medium text-gray-600">
            Gültig bis
          </span>
          <p className="mt-0.5 text-sm font-semibold text-gray-900">
            {new Date(invitation.expiresAt).toLocaleDateString("de-DE", {
              timeZone: "Europe/Berlin",
              day: "2-digit",
              month: "2-digit",
              year: "numeric",
              hour: "2-digit",
              minute: "2-digit",
            })}
          </p>
        </div>
        {invitation.position && (
          <div>
            <span className="block text-xs font-medium text-gray-600">
              Zugewiesene Position
            </span>
            <p className="mt-0.5 text-sm font-semibold text-gray-900">
              {invitation.position}
            </p>
          </div>
        )}
      </InsetPanel>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div>
          <Input
            id="firstName"
            label="Vorname"
            name="first_name"
            controlSize="compact"
            data-testid="input-firstName"
            value={firstName}
            onChange={(event) => setFirstName(event.target.value)}
            disabled={isSubmitting}
            autoComplete="given-name"
            required
            error={fieldError("first_name")}
          />
        </div>
        <div>
          <Input
            id="lastName"
            label="Nachname"
            name="last_name"
            controlSize="compact"
            data-testid="input-lastName"
            value={lastName}
            onChange={(event) => setLastName(event.target.value)}
            disabled={isSubmitting}
            autoComplete="family-name"
            required
            error={fieldError("last_name")}
          />
        </div>
      </div>

      <div>
        <FieldLabel
          htmlFor="password"
          required
          invalid={Boolean(fieldError("password"))}
        >
          Passwort
        </FieldLabel>
        <Input
          id="password"
          name="password"
          type="password"
          controlSize="compact"
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          disabled={isSubmitting}
          autoComplete="new-password"
          aria-invalid={Boolean(fieldError("password"))}
          aria-describedby={
            fieldError("password") ? "password-error" : undefined
          }
          required
          revealable
        />
        <FieldErrorHint id="password-error" message={fieldError("password")} />
      </div>

      <div>
        <FieldLabel
          htmlFor="confirmPassword"
          required
          invalid={Boolean(fieldError("confirm_password"))}
        >
          Passwort bestätigen
        </FieldLabel>
        <Input
          id="confirmPassword"
          name="confirm_password"
          type="password"
          controlSize="compact"
          value={confirmPassword}
          onChange={(event) => setConfirmPassword(event.target.value)}
          disabled={isSubmitting}
          autoComplete="new-password"
          aria-invalid={Boolean(fieldError("confirm_password"))}
          aria-describedby={
            fieldError("confirm_password") ? "confirmPassword-error" : undefined
          }
          required
          revealable
        />
        <FieldErrorHint
          id="confirmPassword-error"
          message={fieldError("confirm_password")}
        />
      </div>

      <PasswordRequirements requirements={requirementStatus} />

      <Button
        type="submit"
        disabled={isSubmitting}
        variant="primary"
        size="field"
        className="w-full"
      >
        {isSubmitting ? "Wird übernommen..." : "Einladung akzeptieren"}
      </Button>
    </form>
  );
}

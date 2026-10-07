"use client";

import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- redirect targets root login, not tenant route
import { useRouter } from "next/navigation";
import { Check, Circle } from "lucide-react";
import { signOut } from "next-auth/react";
import {
  AuthFieldError,
  authInputClass,
  authPrimaryButtonClassName,
} from "~/components/auth/auth-shell";
import { credentialError } from "~/components/auth/credential-error";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { useApiFormError } from "~/contexts/ToastContext";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import { getRoleDisplayName } from "~/lib/auth-helpers";
import { acceptInvitation } from "~/lib/invitation-api";
import type { InvitationValidation } from "~/lib/invitation-helpers";
import { PASSWORD_RULES } from "~/lib/password-rules";
import { createLogger } from "~/lib/logger";

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
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
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
      void formErrors.show(credentialError(err), {
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
      <div className="flex flex-col items-center py-12">
        <div className="mb-6 flex h-12 w-12 items-center justify-center rounded-full bg-gray-900">
          <svg
            className="h-6 w-6 text-white"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2.5}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M5 13l4 4L19 7"
            />
          </svg>
        </div>
        <h3 className="mb-1 text-base font-semibold text-gray-900">
          Konto erstellt
        </h3>
        {signOutRetryFailed ? (
          <p className="mb-4 text-sm text-gray-500">
            Die vorherige Sitzung konnte nicht beendet werden. Bitte löschen Sie
            die Websitedaten in Ihren Browsereinstellungen oder versuchen Sie es
            später erneut.
          </p>
        ) : signOutFailed ? (
          <>
            <p className="mb-4 text-sm text-gray-500">
              Die vorherige Sitzung konnte nicht automatisch beendet werden.
            </p>
            <button
              type="button"
              onClick={handleManualRedirect}
              className="rounded-xl bg-gray-900 px-6 py-2.5 text-sm font-semibold text-white transition-all duration-200 hover:bg-gray-800"
            >
              Zur Anmeldung
            </button>
          </>
        ) : (
          <>
            <p className="mb-6 text-sm text-gray-500">
              Bitte melden Sie sich mit Ihren neuen Zugangsdaten an.
            </p>
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
        )}
      </div>
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

      <div className="mb-4">
        <p className="text-sm text-gray-600">
          Einladung für{" "}
          <span className="font-medium text-gray-900">{invitation.email}</span>{" "}
          als{" "}
          <span className="font-medium text-gray-900">
            {getRoleDisplayName(invitation.roleName)}
          </span>
        </p>
      </div>

      <div className="space-y-2 rounded-lg border border-gray-200 bg-gray-50 p-3">
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
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div>
          <label
            htmlFor="firstName"
            className={`mb-1 block text-sm font-medium ${fieldError("first_name") ? "text-moto-red" : "text-gray-700"}`}
          >
            Vorname
          </label>
          <input
            id="firstName"
            name="first_name"
            data-testid="input-firstName"
            value={firstName}
            onChange={(event) => setFirstName(event.target.value)}
            disabled={isSubmitting}
            autoComplete="given-name"
            required
            aria-invalid={Boolean(fieldError("first_name"))}
            aria-describedby={
              fieldError("first_name") ? "firstName-error" : undefined
            }
            className={authInputClass(Boolean(fieldError("first_name")))}
          />
          <AuthFieldError
            id="firstName-error"
            message={fieldError("first_name")}
          />
        </div>
        <div>
          <label
            htmlFor="lastName"
            className={`mb-1 block text-sm font-medium ${fieldError("last_name") ? "text-moto-red" : "text-gray-700"}`}
          >
            Nachname
          </label>
          <input
            id="lastName"
            name="last_name"
            data-testid="input-lastName"
            value={lastName}
            onChange={(event) => setLastName(event.target.value)}
            disabled={isSubmitting}
            autoComplete="family-name"
            required
            aria-invalid={Boolean(fieldError("last_name"))}
            aria-describedby={
              fieldError("last_name") ? "lastName-error" : undefined
            }
            className={authInputClass(Boolean(fieldError("last_name")))}
          />
          <AuthFieldError
            id="lastName-error"
            message={fieldError("last_name")}
          />
        </div>
      </div>

      <div>
        <label
          htmlFor="password"
          className={`mb-1 block text-sm font-medium ${fieldError("password") ? "text-moto-red" : "text-gray-700"}`}
        >
          Passwort
        </label>
        <div className="relative">
          <input
            id="password"
            name="password"
            type={showPassword ? "text" : "password"}
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            disabled={isSubmitting}
            autoComplete="new-password"
            aria-invalid={Boolean(fieldError("password"))}
            aria-describedby={
              fieldError("password") ? "password-error" : undefined
            }
            className={`${authInputClass(Boolean(fieldError("password")))} pr-10`}
            required
          />
          <PasswordToggleButton
            showPassword={showPassword}
            onToggle={() => setShowPassword(!showPassword)}
          />
        </div>
        <AuthFieldError id="password-error" message={fieldError("password")} />
      </div>

      <div>
        <label
          htmlFor="confirmPassword"
          className={`mb-1 block text-sm font-medium ${fieldError("confirm_password") ? "text-moto-red" : "text-gray-700"}`}
        >
          Passwort bestätigen
        </label>
        <div className="relative">
          <input
            id="confirmPassword"
            name="confirm_password"
            type={showConfirmPassword ? "text" : "password"}
            value={confirmPassword}
            onChange={(event) => setConfirmPassword(event.target.value)}
            disabled={isSubmitting}
            autoComplete="new-password"
            aria-invalid={Boolean(fieldError("confirm_password"))}
            aria-describedby={
              fieldError("confirm_password")
                ? "confirmPassword-error"
                : undefined
            }
            className={`${authInputClass(Boolean(fieldError("confirm_password")))} pr-10`}
            required
          />
          <PasswordToggleButton
            showPassword={showConfirmPassword}
            onToggle={() => setShowConfirmPassword(!showConfirmPassword)}
          />
        </div>
        <AuthFieldError
          id="confirmPassword-error"
          message={fieldError("confirm_password")}
        />
      </div>

      <div className="rounded-lg border border-gray-200 bg-gray-50 p-3">
        <p className="mb-2 text-xs font-medium text-gray-700">
          Passwortanforderungen
        </p>
        <div className="grid grid-cols-2 gap-x-3 gap-y-1.5">
          {requirementStatus.map((requirement) => (
            <div
              key={requirement.label}
              className="flex items-center gap-1.5 text-xs"
            >
              <span
                className={`flex h-4 w-4 flex-shrink-0 items-center justify-center rounded-full border ${
                  requirement.met
                    ? "border-moto-green bg-moto-green/10 text-moto-green-strong"
                    : "border-gray-300 bg-white text-gray-400"
                }`}
                aria-hidden="true"
              >
                {requirement.met ? (
                  <Check className="h-3 w-3" />
                ) : (
                  <Circle className="h-3 w-3" />
                )}
              </span>
              <span
                className={requirement.met ? "text-gray-700" : "text-gray-500"}
              >
                {requirement.label}
              </span>
            </div>
          ))}
        </div>
      </div>

      <button
        type="submit"
        disabled={isSubmitting}
        className={authPrimaryButtonClassName}
      >
        {isSubmitting ? "Wird übernommen..." : "Einladung akzeptieren"}
      </button>
    </form>
  );
}

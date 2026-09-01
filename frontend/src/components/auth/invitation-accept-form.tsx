"use client";

import { useEffect, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- redirect targets root login, not tenant route
import { useRouter } from "next/navigation";
import { Check, Circle } from "lucide-react";
import { useScrollToError } from "~/lib/hooks/use-scroll-to-error";
import { signOut } from "next-auth/react";
import {
  authInputClassName,
  authPrimaryButtonClassName,
} from "~/components/auth/auth-shell";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import { getRoleDisplayName } from "~/lib/auth-helpers";
import { acceptInvitation } from "~/lib/invitation-api";
import type { InvitationValidation } from "~/lib/invitation-helpers";
import type { ApiError } from "~/lib/auth-api";
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

// Helper to map API error status to user-friendly message
const getInvitationErrorMessage = (
  apiError: ApiError | undefined,
  err: unknown,
): string => {
  if (apiError?.status === 410) {
    return "Diese Einladung ist nicht mehr gültig. Bitte fordere eine neue Einladung an.";
  }
  if (apiError?.status === 404) {
    return "Einladung wurde nicht gefunden.";
  }
  if (apiError?.status === 409) {
    return "Für diese E-Mail existiert bereits ein Konto. Bitte melde dich direkt an oder kontaktiere den Support.";
  }
  if (apiError?.status === 400) {
    return (
      apiError.message ?? "Ungültige Eingaben. Bitte überprüfe das Formular."
    );
  }
  const generic =
    apiError?.message ?? (err instanceof Error ? err.message : undefined);
  return generic ?? "Beim Annehmen der Einladung ist ein Fehler aufgetreten.";
};

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
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isAccepted, setIsAccepted] = useState(false);
  const [signOutFailed, setSignOutFailed] = useState(false);
  const [signOutRetryFailed, setSignOutRetryFailed] = useState(false);
  const [tenantRedirectUrl, setTenantRedirectUrl] = useState<string | null>(
    null,
  );
  const errorRef = useScrollToError(error);
  const [errorFieldName, setErrorFieldName] = useState<string | null>(null);

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

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setError(null);
    setErrorFieldName(null);

    if (!firstName.trim() || !lastName.trim()) {
      setError("Bitte gib Vor- und Nachname an.");
      setErrorFieldName(!firstName.trim() ? "firstName" : "lastName");
      return;
    }

    if (!allRequirementsMet) {
      setError(
        "Das Passwort erfüllt noch nicht alle Sicherheitsanforderungen.",
      );
      setErrorFieldName("password");
      return;
    }

    if (password !== confirmPassword) {
      setError("Die Passwörter stimmen nicht überein.");
      setErrorFieldName("confirmPassword");
      return;
    }

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
      // Distinguish network/offline from HTTP errors
      if (typeof navigator !== "undefined" && !navigator.onLine) {
        logger.warn("invitation_accept_offline", {
          error: "no_network_connection",
        });
        setError(
          "Keine Netzwerkverbindung. Bitte überprüfe deine Internetverbindung und versuche es erneut.",
        );
        setIsSubmitting(false);
        return;
      }
      logger.error("invitation_accept_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      const apiError = err as ApiError | undefined;
      const errorMessage = getInvitationErrorMessage(apiError, err);
      setError(errorMessage);
    } finally {
      setIsSubmitting(false);
    }
  };

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
            Die vorherige Sitzung konnte nicht beendet werden. Bitte lösche die
            Websitedaten in deinen Browsereinstellungen oder versuche es später
            erneut.
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
              Bitte melde dich mit deinen neuen Zugangsdaten an.
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
    <form onSubmit={handleSubmit} noValidate className="space-y-6">
      {error && (
        <div
          ref={errorRef}
          className="border-moto-red/10 bg-moto-red-soft/50 rounded-xl border p-4"
        >
          <div className="flex items-start gap-3">
            <svg
              className="text-moto-red mt-0.5 h-5 w-5 flex-shrink-0"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
            <p className="text-moto-red-strong text-sm">{error}</p>
          </div>
        </div>
      )}
      {/* Success toast handled globally */}

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
            className={`mb-1 block text-sm font-medium ${errorFieldName === "firstName" ? "text-moto-red" : "text-gray-700"}`}
          >
            Vorname
          </label>
          <input
            id="firstName"
            name="firstName"
            data-testid="input-firstName"
            value={firstName}
            onChange={(event) => setFirstName(event.target.value)}
            disabled={isSubmitting}
            autoComplete="given-name"
            required
            className={`${authInputClassName} ${errorFieldName === "firstName" ? "ring-moto-red/30 ring-2" : ""}`}
          />
        </div>
        <div>
          <label
            htmlFor="lastName"
            className={`mb-1 block text-sm font-medium ${errorFieldName === "lastName" ? "text-moto-red" : "text-gray-700"}`}
          >
            Nachname
          </label>
          <input
            id="lastName"
            name="lastName"
            data-testid="input-lastName"
            value={lastName}
            onChange={(event) => setLastName(event.target.value)}
            disabled={isSubmitting}
            autoComplete="family-name"
            required
            className={`${authInputClassName} ${errorFieldName === "lastName" ? "ring-moto-red/30 ring-2" : ""}`}
          />
        </div>
      </div>

      <div>
        <label
          htmlFor="password"
          className={`mb-1 block text-sm font-medium ${errorFieldName === "password" ? "text-moto-red" : "text-gray-700"}`}
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
            className={`${authInputClassName} pr-10 ${errorFieldName === "password" ? "ring-moto-red/30 ring-2" : ""}`}
            required
          />
          <PasswordToggleButton
            showPassword={showPassword}
            onToggle={() => setShowPassword(!showPassword)}
          />
        </div>
      </div>

      <div>
        <label
          htmlFor="confirmPassword"
          className={`mb-1 block text-sm font-medium ${errorFieldName === "confirmPassword" ? "text-moto-red" : "text-gray-700"}`}
        >
          Passwort bestätigen
        </label>
        <div className="relative">
          <input
            id="confirmPassword"
            name="confirmPassword"
            type={showConfirmPassword ? "text" : "password"}
            value={confirmPassword}
            onChange={(event) => setConfirmPassword(event.target.value)}
            disabled={isSubmitting}
            autoComplete="new-password"
            className={`${authInputClassName} pr-10 ${errorFieldName === "confirmPassword" ? "ring-moto-red/30 ring-2" : ""}`}
            required
          />
          <PasswordToggleButton
            showPassword={showConfirmPassword}
            onToggle={() => setShowConfirmPassword(!showConfirmPassword)}
          />
        </div>
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

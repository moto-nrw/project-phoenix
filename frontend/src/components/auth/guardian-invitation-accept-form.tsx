"use client";

import { useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- redirects target tenant root, not a tenant route helper
import { useRouter } from "next/navigation";
import { useScrollToError } from "~/lib/hooks/use-scroll-to-error";
import { Input } from "~/components/ui";
import {
  acceptGuardianInvitation,
  type GuardianInvitationValidation,
} from "~/lib/guardian-invitation-api";
import type { ApiError } from "~/lib/auth-api";
import { PASSWORD_RULES } from "~/lib/password-rules";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "GuardianInvitationAccept" });

interface Props {
  readonly token: string;
  readonly invitation: GuardianInvitationValidation;
}

const getErrorMessage = (
  apiError: ApiError | undefined,
  err: unknown,
): string => {
  if (apiError?.status === 410) {
    return "Diese Einladung ist nicht mehr gültig. Bitte fordere eine neue Einladung bei der Schule an.";
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
  return (
    apiError?.message ??
    (err instanceof Error ? err.message : undefined) ??
    "Beim Annehmen der Einladung ist ein Fehler aufgetreten."
  );
};

export function GuardianInvitationAcceptForm({ token, invitation }: Props) {
  const router = useRouter();
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [errorFieldName, setErrorFieldName] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isAccepted, setIsAccepted] = useState(false);
  const errorRef = useScrollToError(error);

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

  const guardianFullName = [invitation.firstName, invitation.lastName]
    .filter((value) => value && value.trim().length > 0)
    .join(" ")
    .trim();

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setError(null);
    setErrorFieldName(null);

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
      await acceptGuardianInvitation(token, {
        password,
        confirmPassword,
      });

      // Redirect to the parents-portal login. The accept-invite page
      // is unauth, so there's no NextAuth session to clear here — the
      // earlier signOut() round-trip occasionally left the cookie in
      // an in-between state where the next signIn looked stale and
      // the dashboard stuck on its loading skeleton until the user
      // logged in a second time. Just navigate.
      let redirectUrl: string | null = null;
      if (globalThis.window !== undefined) {
        const parentsHostname = process.env.NEXT_PUBLIC_PARENTS_HOSTNAME;
        if (parentsHostname) {
          const { protocol } = globalThis.window.location;
          redirectUrl = `${protocol}//${parentsHostname}/login`;
        }
      }

      setIsAccepted(true);
      setTimeout(() => {
        if (redirectUrl) {
          globalThis.window.location.href = redirectUrl;
          return;
        }
        router.push("/");
      }, 1500);
    } catch (err) {
      if (typeof navigator !== "undefined" && !navigator.onLine) {
        logger.warn("guardian_invitation_accept_offline", {
          error: "no_network_connection",
        });
        setError(
          "Keine Netzwerkverbindung. Bitte überprüfe deine Internetverbindung und versuche es erneut.",
        );
        setIsSubmitting(false);
        return;
      }
      logger.error("guardian_invitation_accept_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      const apiError = err as ApiError | undefined;
      setError(getErrorMessage(apiError, err));
    } finally {
      setIsSubmitting(false);
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
            aria-hidden="true"
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
        <p className="mb-6 text-sm text-gray-500">
          Bitte melde dich mit deinen neuen Zugangsdaten an.
        </p>
        <div className="h-1 w-16 overflow-hidden rounded-full bg-gray-100">
          <div
            className="h-full rounded-full bg-gray-900"
            style={{
              animation: "guardianProgressFill 1.5s ease-in-out forwards",
            }}
          />
        </div>
        <style>{`
          @keyframes guardianProgressFill {
            from { width: 0%; }
            to { width: 100%; }
          }
        `}</style>
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit} noValidate className="space-y-6">
      {error && (
        <div
          ref={errorRef}
          className="rounded-xl border border-red-200/50 bg-red-50/50 p-4"
        >
          <p className="text-sm text-red-700">{error}</p>
        </div>
      )}

      <div className="mb-4">
        <p className="text-sm text-gray-600">
          Einladung für{" "}
          <span className="font-medium text-gray-900">{invitation.email}</span>
          {guardianFullName && (
            <>
              {" "}
              als{" "}
              <span className="font-medium text-gray-900">
                {guardianFullName}
              </span>
            </>
          )}
        </p>
      </div>

      <div className="space-y-2 rounded-lg border border-gray-200 bg-gray-50 p-3">
        <div>
          <span className="block text-xs font-medium text-gray-600">
            Gültig bis
          </span>
          <p className="mt-0.5 text-sm font-semibold text-gray-900">
            {new Date(invitation.expiresAt).toLocaleDateString("de-DE", {
              day: "2-digit",
              month: "2-digit",
              year: "numeric",
              hour: "2-digit",
              minute: "2-digit",
            })}
          </p>
        </div>
      </div>

      <div>
        <label
          htmlFor="password"
          className={`mb-1 block text-sm font-medium ${errorFieldName === "password" ? "text-red-600" : "text-gray-700"}`}
        >
          Passwort
        </label>
        <div className="relative">
          <Input
            id="password"
            name="password"
            type={showPassword ? "text" : "password"}
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            disabled={isSubmitting}
            autoComplete="new-password"
            className={`w-full pr-10 ${errorFieldName === "password" ? "ring-red-400" : ""}`}
            required
          />
          <button
            type="button"
            onClick={() => setShowPassword(!showPassword)}
            className="absolute top-1/2 right-3 -translate-y-1/2 text-gray-500 transition-colors hover:text-gray-700"
            aria-label={
              showPassword ? "Passwort verbergen" : "Passwort anzeigen"
            }
          >
            {showPassword ? "verbergen" : "anzeigen"}
          </button>
        </div>
      </div>

      <div>
        <label
          htmlFor="confirmPassword"
          className={`mb-1 block text-sm font-medium ${errorFieldName === "confirmPassword" ? "text-red-600" : "text-gray-700"}`}
        >
          Passwort bestätigen
        </label>
        <div className="relative">
          <Input
            id="confirmPassword"
            name="confirmPassword"
            type={showConfirmPassword ? "text" : "password"}
            value={confirmPassword}
            onChange={(event) => setConfirmPassword(event.target.value)}
            disabled={isSubmitting}
            autoComplete="new-password"
            className={`w-full pr-10 ${errorFieldName === "confirmPassword" ? "ring-red-400" : ""}`}
            required
          />
          <button
            type="button"
            onClick={() => setShowConfirmPassword(!showConfirmPassword)}
            className="absolute top-1/2 right-3 -translate-y-1/2 text-gray-500 transition-colors hover:text-gray-700"
            aria-label={
              showConfirmPassword ? "Passwort verbergen" : "Passwort anzeigen"
            }
          >
            {showConfirmPassword ? "verbergen" : "anzeigen"}
          </button>
        </div>
      </div>

      <div className="rounded-xl border border-gray-100 bg-gray-50/60 p-3">
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
                    ? "border-green-400 bg-green-100 text-green-700"
                    : "border-gray-300 bg-white text-gray-400"
                }`}
                aria-hidden="true"
              >
                {requirement.met ? "✓" : ""}
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
        className="w-full rounded-xl bg-gray-900 py-3 text-sm font-semibold text-white shadow-lg transition-all duration-200 hover:bg-gray-800 hover:shadow-xl disabled:cursor-not-allowed disabled:bg-gray-400"
      >
        {isSubmitting ? "Wird übernommen..." : "Einladung akzeptieren"}
      </button>
    </form>
  );
}

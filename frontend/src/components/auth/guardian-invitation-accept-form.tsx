"use client";

import { useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- redirects target tenant root, not a tenant route helper
import { useRouter } from "next/navigation";
import { AlertCircle, Check, Circle } from "lucide-react";
import { useScrollToError } from "~/lib/hooks/use-scroll-to-error";
import { Input } from "~/components/ui";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
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

      // Redirect to the parents portal login. The accept invite page
      // is unauth, so there is no NextAuth session to clear here.
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
    <form onSubmit={handleSubmit} noValidate className="space-y-5 sm:space-y-6">
      {error && (
        <div
          ref={errorRef}
          className="rounded-xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-4"
        >
          <div className="flex items-start gap-3">
            <AlertCircle
              className="mt-0.5 h-5 w-5 shrink-0 text-[#CC2626]"
              aria-hidden="true"
            />
            <p className="text-sm text-[#CC2626]">{error}</p>
          </div>
        </div>
      )}

      <section className="rounded-xl border border-gray-200 bg-gray-50/70 px-3.5 py-3 text-sm lg:hidden">
        <p className="font-medium text-gray-700">
          Einladung für{" "}
          <span className="font-semibold text-gray-900">
            {guardianFullName || invitation.email}
          </span>
        </p>
        {guardianFullName && (
          <p className="mt-1 text-gray-500">{invitation.email}</p>
        )}
      </section>

      <div>
        <label
          htmlFor="password"
          className={`mb-2 block text-sm font-medium ${errorFieldName === "password" ? "text-[#CC2626]" : "text-gray-700"}`}
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
            className={`w-full pr-12 ${errorFieldName === "password" ? "ring-[#FF3130]" : ""}`}
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
          className={`mb-2 block text-sm font-medium ${errorFieldName === "confirmPassword" ? "text-[#CC2626]" : "text-gray-700"}`}
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
            className={`w-full pr-12 ${errorFieldName === "confirmPassword" ? "ring-[#FF3130]" : ""}`}
            required
          />
          <PasswordToggleButton
            showPassword={showConfirmPassword}
            onToggle={() => setShowConfirmPassword(!showConfirmPassword)}
          />
        </div>
      </div>

      <div className="rounded-2xl border border-gray-200 bg-gray-50/70 p-4">
        <p className="text-sm font-semibold text-gray-900">
          Passwortanforderungen
        </p>
        <div className="mt-3 grid gap-1.5 sm:grid-cols-2 sm:gap-2">
          {requirementStatus.map((requirement) => (
            <div
              key={requirement.label}
              className="flex items-center gap-2 text-sm"
            >
              <span
                className={`flex h-5 w-5 shrink-0 items-center justify-center rounded-full border ${
                  requirement.met
                    ? "border-[#83CD2D] bg-[#83CD2D]/10 text-[#5A8B1F]"
                    : "border-gray-300 bg-white text-gray-400"
                }`}
                aria-hidden="true"
              >
                {requirement.met ? (
                  <Check className="h-3.5 w-3.5" />
                ) : (
                  <Circle className="h-3.5 w-3.5" />
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
        className="h-11 w-full rounded-xl bg-gray-900 px-4 text-sm font-semibold text-white shadow-lg transition-all duration-200 hover:bg-gray-800 hover:shadow-xl focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-gray-400 sm:h-12"
      >
        {isSubmitting ? "Wird übernommen..." : "Einladung akzeptieren"}
      </button>
    </form>
  );
}

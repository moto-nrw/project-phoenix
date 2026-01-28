"use client";

import { useEffect, useMemo, useState } from "react";
import { signOut } from "next-auth/react";
import { useToast } from "~/contexts/ToastContext";
import { useRouter } from "next/navigation";
import { Input } from "~/components/ui";
import { getRoleDisplayName } from "~/lib/auth-helpers";
import { acceptInvitation } from "~/lib/invitation-api";
import type { InvitationValidation } from "~/lib/invitation-helpers";
import type { ApiError } from "~/lib/auth-api";

interface InvitationAcceptFormProps {
  readonly token: string;
  readonly invitation: InvitationValidation;
}

const PASSWORD_REQUIREMENTS: Array<{
  label: string;
  test: (value: string) => boolean;
}> = [
  { label: "Mindestens 8 Zeichen", test: (value) => value.length >= 8 },
  { label: "Ein Großbuchstabe", test: (value) => /[A-Z]/.test(value) },
  { label: "Ein Kleinbuchstabe", test: (value) => /[a-z]/.test(value) },
  { label: "Eine Zahl", test: (value) => /\d/.test(value) },
  { label: "Ein Sonderzeichen", test: (value) => /[^A-Za-z0-9]/.test(value) },
];

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
}: InvitationAcceptFormProps) {
  const router = useRouter();
  const [firstName, setFirstName] = useState(invitation.firstName ?? "");
  const [lastName, setLastName] = useState(invitation.lastName ?? "");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const { success: toastSuccess } = useToast();
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    setFirstName(invitation.firstName ?? "");
    setLastName(invitation.lastName ?? "");
  }, [invitation.firstName, invitation.lastName]);

  const requirementStatus = useMemo(
    () =>
      PASSWORD_REQUIREMENTS.map(({ label, test }) => ({
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

    if (!firstName.trim() || !lastName.trim()) {
      setError("Bitte gib Vor- und Nachname an.");
      return;
    }

    if (!allRequirementsMet) {
      setError(
        "Das Passwort erfüllt noch nicht alle Sicherheitsanforderungen.",
      );
      return;
    }

    if (password !== confirmPassword) {
      setError("Die Passwörter stimmen nicht überein.");
      return;
    }

    try {
      setIsSubmitting(true);
      await acceptInvitation(token, {
        firstName: firstName.trim(),
        lastName: lastName.trim(),
        password,
        confirmPassword,
      });
      toastSuccess(
        "Einladung erfolgreich angenommen! Du wirst zur Anmeldung weitergeleitet.",
      );

      // Logout any existing session before redirecting to login
      await signOut({ redirect: false });

      setTimeout(() => {
        router.push("/");
      }, 2500);
    } catch (err) {
      // Distinguish network/offline from HTTP errors
      if (typeof navigator !== "undefined" && !navigator.onLine) {
        setError(
          "Keine Netzwerkverbindung. Bitte überprüfe deine Internetverbindung und versuche es erneut.",
        );
        setIsSubmitting(false);
        return;
      }
      const apiError = err as ApiError | undefined;
      const errorMessage = getInvitationErrorMessage(apiError, err);
      setError(errorMessage);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} noValidate className="space-y-6">
      {error && (
        <div className="rounded-xl border border-red-200/50 bg-red-50/50 p-4">
          <div className="flex items-start gap-3">
            <svg
              className="mt-0.5 h-5 w-5 flex-shrink-0 text-red-600"
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
            <p className="text-sm text-red-700">{error}</p>
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
        <Input
          id="firstName"
          name="firstName"
          label="Vorname"
          value={firstName}
          onChange={(event) => setFirstName(event.target.value)}
          disabled={isSubmitting}
          autoComplete="given-name"
          required
        />
        <Input
          id="lastName"
          name="lastName"
          label="Nachname"
          value={lastName}
          onChange={(event) => setLastName(event.target.value)}
          disabled={isSubmitting}
          autoComplete="family-name"
          required
        />
      </div>

      <div>
        <label
          htmlFor="password"
          className="mb-1 block text-sm font-medium text-gray-700"
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
            className="w-full pr-10"
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
            {showPassword ? (
              <svg
                className="h-5 w-5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21"
                />
              </svg>
            ) : (
              <svg
                className="h-5 w-5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                />
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
                />
              </svg>
            )}
          </button>
        </div>
      </div>

      <div>
        <label
          htmlFor="confirmPassword"
          className="mb-1 block text-sm font-medium text-gray-700"
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
            className="w-full pr-10"
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
            {showConfirmPassword ? (
              <svg
                className="h-5 w-5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21"
                />
              </svg>
            ) : (
              <svg
                className="h-5 w-5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                />
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
                />
              </svg>
            )}
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

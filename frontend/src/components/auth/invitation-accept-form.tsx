"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { Input } from "~/components/ui";
import { acceptInvitation } from "~/lib/invitation-api";
import type { InvitationValidation } from "~/lib/invitation-helpers";
import type { ApiError } from "~/lib/auth-api";

interface InvitationAcceptFormProps {
  token: string;
  invitation: InvitationValidation;
}

const PASSWORD_REQUIREMENTS: Array<{ label: string; test: (value: string) => boolean }> = [
  { label: "Mindestens 8 Zeichen", test: (value) => value.length >= 8 },
  { label: "Ein Großbuchstabe", test: (value) => /[A-Z]/.test(value) },
  { label: "Ein Kleinbuchstabe", test: (value) => /[a-z]/.test(value) },
  { label: "Eine Zahl", test: (value) => /[0-9]/.test(value) },
  { label: "Ein Sonderzeichen", test: (value) => /[^A-Za-z0-9]/.test(value) },
];

export function InvitationAcceptForm({ token, invitation }: InvitationAcceptFormProps) {
  const router = useRouter();
  const [firstName, setFirstName] = useState(invitation.firstName ?? "");
  const [lastName, setLastName] = useState(invitation.lastName ?? "");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    setFirstName(invitation.firstName ?? "");
    setLastName(invitation.lastName ?? "");
  }, [invitation.firstName, invitation.lastName]);

  const requirementStatus = useMemo(
    () => PASSWORD_REQUIREMENTS.map(({ label, test }) => ({ label, met: test(password) })),
    [password]
  );

  const allRequirementsMet = useMemo(
    () => requirementStatus.every((requirement) => requirement.met),
    [requirementStatus]
  );

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setError(null);
    setSuccessMessage(null);

    if (!firstName.trim() || !lastName.trim()) {
      setError("Bitte gib Vor- und Nachname an.");
      return;
    }

    if (!allRequirementsMet) {
      setError("Das Passwort erfüllt noch nicht alle Sicherheitsanforderungen.");
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

      setSuccessMessage("Einladung erfolgreich angenommen! Du wirst zur Anmeldung weitergeleitet.");
      setTimeout(() => {
        router.push("/");
      }, 2500);
    } catch (err) {
      // Distinguish network/offline from HTTP errors
      if (typeof navigator !== "undefined" && (navigator as any).onLine === false) {
        setError("Keine Netzwerkverbindung. Bitte überprüfe deine Internetverbindung und versuche es erneut.");
        return;
      }
      const apiError = err as ApiError | undefined;
      if (apiError?.status === 410) {
        setError("Diese Einladung ist nicht mehr gültig. Bitte fordere eine neue Einladung an.");
      } else if (apiError?.status === 404) {
        setError("Einladung wurde nicht gefunden.");
      } else if (apiError?.status === 409) {
        setError("Für diese E-Mail existiert bereits ein Konto. Bitte melde dich direkt an oder kontaktiere den Support.");
      } else if (apiError?.status === 400) {
        setError(apiError.message ?? "Ungültige Eingaben. Bitte überprüfe das Formular.");
      } else {
        const generic = apiError?.message ?? (err instanceof Error ? err.message : undefined);
        setError(generic ?? "Beim Annehmen der Einladung ist ein Fehler aufgetreten.");
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      {error && (
        <div className="rounded-xl border border-red-200/50 bg-red-50/50 p-4">
          <div className="flex items-start gap-3">
            <svg className="h-5 w-5 text-red-600 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
            <p className="text-sm text-red-700">{error}</p>
          </div>
        </div>
      )}
      {successMessage && (
        <div className="rounded-xl border border-green-200/50 bg-green-50/50 p-4">
          <div className="flex items-start gap-3">
            <svg className="h-5 w-5 text-green-600 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <p className="text-sm text-green-700">{successMessage}</p>
          </div>
        </div>
      )}

      <div className="space-y-2">
        <p className="text-sm text-gray-600">
          Einladung für <span className="font-medium text-gray-900">{invitation.email}</span> als
          {" "}
          <span className="font-medium text-gray-900">{invitation.roleName}</span>
        </p>
        <p className="text-xs text-gray-500">
          Die Einladung ist gültig bis {new Date(invitation.expiresAt).toLocaleString("de-DE")}
        </p>
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

      <Input
        id="password"
        name="password"
        type="password"
        label="Passwort"
        value={password}
        onChange={(event) => setPassword(event.target.value)}
        disabled={isSubmitting}
        autoComplete="new-password"
        required
      />
      <Input
        id="confirmPassword"
        name="confirmPassword"
        type="password"
        label="Passwort bestätigen"
        value={confirmPassword}
        onChange={(event) => setConfirmPassword(event.target.value)}
        disabled={isSubmitting}
        autoComplete="new-password"
        required
      />

      <div className="rounded-xl border border-gray-100 bg-gray-50/60 p-4">
        <p className="text-sm font-medium text-gray-700 mb-3">Passwortanforderungen</p>
        <ul className="space-y-2">
          {requirementStatus.map((requirement) => (
            <li key={requirement.label} className="flex items-center gap-2 text-sm">
              <span
                className={`flex h-5 w-5 items-center justify-center rounded-full border text-xs ${
                  requirement.met
                    ? "border-green-300 bg-green-100 text-green-700"
                    : "border-gray-300 bg-white text-gray-400"
                }`}
                aria-hidden="true"
              >
                {requirement.met ? "✓" : ""}
              </span>
              <span className={requirement.met ? "text-gray-600" : "text-gray-500"}>{requirement.label}</span>
            </li>
          ))}
        </ul>
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

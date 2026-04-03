"use client";

import { useState, useMemo } from "react";
import { useRouter } from "next/navigation";
import { createLogger } from "~/lib/logger";
import {
  acceptOperatorInvitation,
  type OperatorInvitationValidation,
} from "~/lib/operator/invitation-api";
import { OperatorApiError } from "~/lib/operator/api-helpers";

const logger = createLogger({ component: "OperatorInvitationAcceptForm" });

interface PasswordRequirement {
  label: string;
  test: (pw: string) => boolean;
}

const PASSWORD_REQUIREMENTS: PasswordRequirement[] = [
  { label: "Mindestens 8 Zeichen", test: (pw) => pw.length >= 8 },
  { label: "Mindestens ein Großbuchstabe", test: (pw) => /[A-Z]/.test(pw) },
  { label: "Mindestens ein Kleinbuchstabe", test: (pw) => /[a-z]/.test(pw) },
  { label: "Mindestens eine Zahl", test: (pw) => /\d/.test(pw) },
  {
    label: "Mindestens ein Sonderzeichen",
    test: (pw) => /[^A-Za-z0-9]/.test(pw),
  },
];

interface OperatorInvitationAcceptFormProps {
  readonly token: string;
  readonly invitation: OperatorInvitationValidation;
}

export function OperatorInvitationAcceptForm({
  token,
  invitation,
}: OperatorInvitationAcceptFormProps) {
  const router = useRouter();
  const [displayName, setDisplayName] = useState(invitation.display_name ?? "");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const passwordValid = useMemo(
    () => PASSWORD_REQUIREMENTS.every((req) => req.test(password)),
    [password],
  );
  const passwordsMatch = password === confirmPassword && confirmPassword !== "";
  const formValid =
    displayName.trim() !== "" && passwordValid && passwordsMatch;

  const expiresAt = new Date(invitation.expires_at);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!formValid || loading) return;

    setError(null);
    setLoading(true);

    try {
      await acceptOperatorInvitation({
        token,
        display_name: displayName.trim(),
        password,
      });
      setSuccess(true);
      logger.info("operator_invitation_accepted");
      setTimeout(() => {
        router.push("/operator/login");
      }, 2500);
    } catch (err) {
      if (err instanceof OperatorApiError) {
        setError(err.message);
      } else {
        setError("Ein unerwarteter Fehler ist aufgetreten");
        logger.error("accept_invitation_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      }
    } finally {
      setLoading(false);
    }
  };

  if (success) {
    return (
      <div className="w-full max-w-md rounded-xl bg-white p-8 text-center shadow-lg">
        <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-green-100">
          <svg
            className="h-6 w-6 text-green-600"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M5 13l4 4L19 7"
            />
          </svg>
        </div>
        <h2 className="mb-2 text-lg font-semibold text-gray-900">
          Konto erstellt
        </h2>
        <p className="text-sm text-gray-600">
          Dein Operator-Konto wurde erfolgreich erstellt. Du wirst zum Login
          weitergeleitet...
        </p>
      </div>
    );
  }

  return (
    <div className="w-full max-w-md rounded-xl bg-white p-8 shadow-lg">
      <div className="mb-6 text-center">
        <h1 className="text-xl font-semibold text-gray-900">
          Einladung annehmen
        </h1>
        <p className="mt-2 text-sm text-gray-500">
          Du wurdest von{" "}
          <span className="font-medium text-gray-700">
            {invitation.inviter_name}
          </span>{" "}
          als Operator eingeladen.
        </p>
        <p className="mt-1 text-xs text-gray-400">
          Gültig bis {expiresAt.toLocaleDateString("de-DE")},{" "}
          {expiresAt.toLocaleTimeString("de-DE", {
            hour: "2-digit",
            minute: "2-digit",
          })}
        </p>
      </div>

      <form onSubmit={(e) => void handleSubmit(e)} className="space-y-4">
        {/* Email (read-only) */}
        <div>
          <label
            htmlFor="accept-email"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            E-Mail
          </label>
          <input
            id="accept-email"
            type="email"
            value={invitation.email}
            disabled
            className="w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-500"
          />
        </div>

        {/* Display Name */}
        <div>
          <label
            htmlFor="accept-display-name"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Anzeigename *
          </label>
          <input
            id="accept-display-name"
            type="text"
            required
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
            placeholder="Dein Name"
            disabled={loading}
            autoFocus
          />
        </div>

        {/* Password */}
        <div>
          <label
            htmlFor="accept-password"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Passwort *
          </label>
          <div className="relative">
            <input
              id="accept-password"
              type={showPassword ? "text" : "password"}
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 pr-10 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
              placeholder="Sicheres Passwort"
              disabled={loading}
            />
            <button
              type="button"
              onClick={() => setShowPassword(!showPassword)}
              className="absolute inset-y-0 right-0 flex items-center pr-3 text-gray-400 hover:text-gray-600"
              tabIndex={-1}
            >
              {showPassword ? (
                <svg
                  className="h-4 w-4"
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
                  className="h-4 w-4"
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

          {/* Password Requirements */}
          {password.length > 0 && (
            <ul className="mt-2 space-y-1">
              {PASSWORD_REQUIREMENTS.map((req) => {
                const met = req.test(password);
                return (
                  <li
                    key={req.label}
                    className={`flex items-center gap-1.5 text-xs ${met ? "text-green-600" : "text-gray-400"}`}
                  >
                    {met ? (
                      <svg
                        className="h-3 w-3"
                        fill="currentColor"
                        viewBox="0 0 20 20"
                      >
                        <path
                          fillRule="evenodd"
                          d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                          clipRule="evenodd"
                        />
                      </svg>
                    ) : (
                      <svg
                        className="h-3 w-3"
                        fill="currentColor"
                        viewBox="0 0 20 20"
                      >
                        <circle cx="10" cy="10" r="3" />
                      </svg>
                    )}
                    {req.label}
                  </li>
                );
              })}
            </ul>
          )}
        </div>

        {/* Confirm Password */}
        <div>
          <label
            htmlFor="accept-confirm-password"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Passwort bestätigen *
          </label>
          <input
            id="accept-confirm-password"
            type={showPassword ? "text" : "password"}
            required
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            className={`w-full rounded-lg border px-3 py-2 text-sm focus:ring-1 focus:outline-none ${
              confirmPassword && !passwordsMatch
                ? "border-red-300 focus:border-red-500 focus:ring-1 focus:ring-red-500 focus:outline-none"
                : "border-gray-300 focus:border-blue-500 focus:ring-blue-500"
            }`}
            placeholder="Passwort wiederholen"
            disabled={loading}
          />
          {confirmPassword && !passwordsMatch && (
            <p className="mt-1 text-xs text-red-500">
              Passwörter stimmen nicht überein
            </p>
          )}
        </div>

        {error && (
          <div className="rounded-lg bg-red-50 p-3 text-sm text-red-700">
            {error}
          </div>
        )}

        <button
          type="submit"
          disabled={loading || !formValid}
          className="w-full rounded-lg bg-gray-900 px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:opacity-50"
        >
          {loading ? "Konto wird erstellt..." : "Konto erstellen"}
        </button>
      </form>
    </div>
  );
}

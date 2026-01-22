"use client";

import { useCallback, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import {
  signupWithOrganization,
  SignupWithOrgException,
} from "~/lib/auth-client";
import { useToast } from "~/contexts/ToastContext";
import { Input } from "~/components/ui";
import {
  validateSlug,
  normalizeSlug,
  generateSlugFromName,
} from "~/lib/slug-validation";

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

export function SignupForm() {
  const router = useRouter();
  const { success: toastSuccess } = useToast();

  // User account fields
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);

  // Organization fields
  const [orgName, setOrgName] = useState("");
  const [slug, setSlug] = useState("");
  const [slugTouched, setSlugTouched] = useState(false);

  // Form state
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Auto-generate slug from org name if not manually edited
  const handleOrgNameChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const newName = e.target.value;
      setOrgName(newName);

      // Only auto-generate slug if user hasn't manually edited it
      if (!slugTouched) {
        setSlug(generateSlugFromName(newName));
      }
    },
    [slugTouched],
  );

  const handleSlugChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      setSlugTouched(true);
      setSlug(normalizeSlug(e.target.value));
    },
    [],
  );

  // Validate slug
  const slugValidation = useMemo(() => validateSlug(slug), [slug]);

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

    // Validate name
    if (!name.trim()) {
      setError("Bitte gib deinen Namen an.");
      return;
    }

    // Validate email
    if (!email.trim() || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      setError("Bitte gib eine gültige E-Mail-Adresse an.");
      return;
    }

    // Validate password requirements
    if (!allRequirementsMet) {
      setError(
        "Das Passwort erfüllt noch nicht alle Sicherheitsanforderungen.",
      );
      return;
    }

    // Validate password confirmation
    if (password !== confirmPassword) {
      setError("Die Passwörter stimmen nicht überein.");
      return;
    }

    // Validate organization name
    if (!orgName.trim()) {
      setError("Bitte gib den Namen deiner Organisation an.");
      return;
    }

    // Validate slug
    if (!slugValidation.valid) {
      setError(slugValidation.error ?? "Ungültige Subdomain.");
      return;
    }

    try {
      setIsSubmitting(true);

      // Atomic signup: Creates user + organization in a single transaction
      // If slug is taken or email registered, nothing is created (avoids orphaned users)
      await signupWithOrganization({
        name: name.trim(),
        email: email.trim(),
        password,
        orgName: orgName.trim(),
        orgSlug: normalizeSlug(slug),
      });

      toastSuccess(
        "Registrierung erfolgreich! Deine Organisation wird geprüft.",
      );

      // Redirect to pending approval page
      setTimeout(() => {
        router.push("/signup/pending");
        router.refresh();
      }, 1500);
    } catch (err) {
      // Handle network/offline errors
      if (typeof navigator !== "undefined" && !navigator.onLine) {
        setError(
          "Keine Netzwerkverbindung. Bitte überprüfe deine Internetverbindung und versuche es erneut.",
        );
        return;
      }

      // Handle specific signup errors
      if (err instanceof SignupWithOrgException) {
        if (err.code === "USER_ALREADY_EXISTS") {
          setError(
            "Diese E-Mail-Adresse ist bereits registriert. Bitte melde dich an oder verwende eine andere E-Mail.",
          );
        } else if (err.code === "SLUG_ALREADY_EXISTS") {
          setError(
            "Diese Subdomain ist bereits vergeben. Bitte wähle eine andere.",
          );
        } else {
          setError(err.message);
        }
        return;
      }

      const errorMessage =
        err instanceof Error
          ? err.message
          : "Bei der Registrierung ist ein Fehler aufgetreten.";
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

      {/* Section: Personal Information */}
      <div className="space-y-4">
        <h3 className="text-sm font-medium text-gray-900">Deine Daten</h3>

        {/* Name Input */}
        <Input
          id="name"
          name="name"
          type="text"
          label="Name"
          value={name}
          onChange={(event) => setName(event.target.value)}
          disabled={isSubmitting}
          autoComplete="name"
          placeholder="Max Mustermann"
          required
        />

        {/* Email Input */}
        <Input
          id="email"
          name="email"
          type="email"
          label="E-Mail-Adresse"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          disabled={isSubmitting}
          autoComplete="email"
          placeholder="max@example.com"
          required
        />

        {/* Password Input */}
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

        {/* Confirm Password Input */}
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

        {/* Password Requirements */}
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
                  className={
                    requirement.met ? "text-gray-700" : "text-gray-500"
                  }
                >
                  {requirement.label}
                </span>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Section: Organization Information */}
      <div className="space-y-4 border-t border-gray-100 pt-6">
        <h3 className="text-sm font-medium text-gray-900">
          Deine Organisation
        </h3>

        {/* Organization Name Input */}
        <Input
          id="orgName"
          name="orgName"
          type="text"
          label="Name der Organisation"
          value={orgName}
          onChange={handleOrgNameChange}
          disabled={isSubmitting}
          placeholder="OGS Musterstadt"
          required
        />

        {/* Subdomain Input */}
        <div>
          <label
            htmlFor="slug"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Subdomain
          </label>
          <div className="flex items-center gap-2">
            <Input
              id="slug"
              name="slug"
              type="text"
              value={slug}
              onChange={handleSlugChange}
              disabled={isSubmitting}
              placeholder="ogs-musterstadt"
              className="flex-1"
              required
            />
            <span className="text-sm text-gray-500">.moto-app.de</span>
          </div>
          {slug && !slugValidation.valid && (
            <p className="mt-1 text-xs text-red-600">{slugValidation.error}</p>
          )}
          {slug && slugValidation.valid && (
            <p className="mt-1 text-xs text-green-600">
              Deine Organisation wird unter{" "}
              <span className="font-medium">{slug}.moto-app.de</span> erreichbar
              sein.
            </p>
          )}
        </div>
      </div>

      {/* Submit Button */}
      <button
        type="submit"
        disabled={isSubmitting}
        className="w-full rounded-xl bg-gray-900 py-3 text-sm font-semibold text-white shadow-lg transition-all duration-200 hover:bg-gray-800 hover:shadow-xl disabled:cursor-not-allowed disabled:bg-gray-400"
      >
        {isSubmitting ? "Wird registriert..." : "Organisation registrieren"}
      </button>

      {/* Info about approval */}
      <div className="rounded-xl border border-blue-100 bg-blue-50/50 p-4">
        <div className="flex items-start gap-3">
          <svg
            className="mt-0.5 h-5 w-5 flex-shrink-0 text-blue-600"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
          <p className="text-sm text-blue-700">
            Nach der Registrierung wird deine Organisation von unserem Team
            geprüft. Du erhältst eine E-Mail, sobald sie freigeschaltet wurde.
          </p>
        </div>
      </div>

      {/* Link to Login */}
      <div className="text-center text-sm text-gray-600">
        <p>
          Bereits ein Konto?{" "}
          <Link
            href="/"
            className="font-medium text-gray-900 underline hover:text-gray-700"
          >
            Zur Anmeldung
          </Link>
        </p>
      </div>
    </form>
  );
}

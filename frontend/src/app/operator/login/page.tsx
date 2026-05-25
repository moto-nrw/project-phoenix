"use client";

import { useState, useEffect, useRef } from "react";
// eslint-disable-next-line no-restricted-imports -- operator routes are not tenant-scoped
import { useRouter } from "next/navigation";
import { signIn, signOut, useSession } from "next-auth/react";
import { Alert } from "~/components/ui";
import {
  AuthShell,
  OperatorBrand,
  authInputClassName,
  authPrimaryButtonClassName,
} from "~/components/auth/auth-shell";
import { Loading } from "~/components/ui/loading";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import { operatorPath } from "~/lib/operator-url";

export default function OperatorLoginPage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const router = useRouter();
  const { data: session, status } = useSession();
  // Ref prevents re-triggering signOut (not in effect deps → no loop).
  // Separate state controls the loading spinner for the UI.
  const cleanupStartedRef = useRef(false);
  const [isCleaningUp, setIsCleaningUp] = useState(false);

  // Redirect if already authenticated as operator, or clear stale sessions
  useEffect(() => {
    const check = async () => {
      // Clear stale sessions with errors before checking redirect.
      // Uses a ref guard so a failed signOut doesn't re-trigger the effect.
      if (
        status === "authenticated" &&
        session?.error &&
        !cleanupStartedRef.current
      ) {
        cleanupStartedRef.current = true;
        setIsCleaningUp(true);
        try {
          await signOut({ redirect: false });
        } catch {
          cleanupStartedRef.current = false;
        }
        setIsCleaningUp(false);
        return;
      }

      if (
        status === "authenticated" &&
        session?.user?.scope === "platform" &&
        session?.user?.token
      ) {
        router.push(operatorPath("/operator/suggestions"));
      }
    };
    void check();
  }, [status, session, router]);

  // Show loading while checking auth or cleaning stale session
  if (
    status === "loading" ||
    isCleaningUp ||
    (status === "authenticated" &&
      session?.user?.scope === "platform" &&
      session?.user?.token)
  ) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <Loading />
      </div>
    );
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError("");

    try {
      const result = await signIn("operator-credentials", {
        redirect: false,
        email,
        password,
      });

      if (result?.error) {
        const errorMessages: Record<string, string> = {
          account_inactive:
            "Ihr Konto ist deaktiviert. Bitte kontaktieren Sie den Administrator.",
          rate_limited:
            "Zu viele Anmeldeversuche. Bitte versuchen Sie es später erneut.",
          invalid_credentials: "Ungültige Anmeldedaten",
        };
        setError(errorMessages[result.code ?? ""] ?? "Ungültige Anmeldedaten");
        return;
      }

      router.push(operatorPath("/operator/suggestions"));
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Anmeldefehler. Bitte versuchen Sie es erneut.",
      );
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <AuthShell
      eyebrow="Operator"
      eyebrowClassName="text-gray-700"
      title="Operator Dashboard"
      subtitle="Melden Sie sich an, um Plattformbetrieb, Träger und Schulen zu verwalten."
      variant="operator"
      brand={<OperatorBrand />}
    >
      <form onSubmit={handleSubmit} noValidate className="space-y-6">
        {error && <Alert type="error" message={error} />}

        <div className="space-y-4">
          <div className="text-left">
            <label
              htmlFor="operator-email"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              E-Mail-Adresse
            </label>
            <input
              id="operator-email"
              name="email"
              type="email"
              autoComplete="username"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className={authInputClassName}
            />
          </div>

          <div className="text-left">
            <label
              htmlFor="operator-password"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Passwort
            </label>
            <div className="relative">
              <input
                id="operator-password"
                name="password"
                type={showPassword ? "text" : "password"}
                autoComplete="current-password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className={`${authInputClassName} pr-10`}
              />
              <PasswordToggleButton
                showPassword={showPassword}
                onToggle={() => setShowPassword(!showPassword)}
              />
            </div>
          </div>
        </div>

        <button
          type="submit"
          disabled={isLoading}
          className={authPrimaryButtonClassName}
        >
          <span className="relative z-10">
            {isLoading ? "Anmeldung läuft..." : "Anmelden"}
          </span>
        </button>
      </form>
    </AuthShell>
  );
}

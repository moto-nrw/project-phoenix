"use client";

import { useState, useEffect, useRef } from "react";
// eslint-disable-next-line no-restricted-imports -- parent routes are not tenant-scoped
import { useRouter } from "next/navigation";
import { signIn, signOut, useSession } from "next-auth/react";
import { Alert } from "~/components/ui";
import {
  AuthShell,
  authInputClassName,
  authPrimaryButtonClassName,
} from "~/components/auth/auth-shell";
import { Loading } from "~/components/ui/loading";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import { parentPath } from "~/lib/parent-url";

export default function ParentLoginPage() {
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

  // Redirect if already authenticated as parent, or clear stale sessions.
  useEffect(() => {
    const check = async () => {
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
        session?.user?.scope === "parent" &&
        session?.user?.token
      ) {
        router.push(parentPath("/parents"));
      }
    };
    void check();
  }, [status, session, router]);

  if (
    status === "loading" ||
    isCleaningUp ||
    (status === "authenticated" &&
      session?.user?.scope === "parent" &&
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
      const result = await signIn("parent-credentials", {
        redirect: false,
        email,
        password,
      });

      if (result?.error) {
        const errorMessages: Record<string, string> = {
          account_inactive:
            "Ihr Konto ist deaktiviert. Bitte kontaktieren Sie die Schule.",
          rate_limited:
            "Zu viele Anmeldeversuche. Bitte versuchen Sie es später erneut.",
          // ErrAccountNoGuardianRole: backend sends 403 which CredentialsProvider
          // surfaces here with code "invalid_credentials" by default. A future
          // refinement could plumb a separate code; for now the German copy
          // covers both "wrong password" and "not a parent" cases without
          // leaking which one applies (account-enumeration mask).
          invalid_credentials:
            "Anmeldung nicht möglich. Bitte prüfen Sie Ihre Zugangsdaten oder verwenden Sie das Schul-Login, falls Sie zum Personal gehören.",
        };
        setError(errorMessages[result.code ?? ""] ?? "Ungültige Anmeldedaten");
        return;
      }

      router.push(parentPath("/parents"));
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
      eyebrow="Elternportal"
      eyebrowClassName="text-[#83CD2D]"
      title="Willkommen im Eltern-Portal"
      subtitle="Melden Sie sich an, um alles Wichtige zur Betreuung Ihres Kindes im Blick zu behalten."
      variant="parents"
    >
      <form onSubmit={handleSubmit} noValidate className="space-y-6">
        {error && <Alert type="error" message={error} />}

        <div className="space-y-4">
          <div className="text-left">
            <label
              htmlFor="parent-email"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              E-Mail-Adresse
            </label>
            <input
              id="parent-email"
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
              htmlFor="parent-password"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Passwort
            </label>
            <div className="relative">
              <input
                id="parent-password"
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

        <p className="text-center text-sm leading-6 text-gray-500">
          Passwort vergessen? Bitte wenden Sie sich an Ihre OGS.
        </p>
      </form>
    </AuthShell>
  );
}

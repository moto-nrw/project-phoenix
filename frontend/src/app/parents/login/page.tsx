"use client";

import { useState, useEffect, useRef } from "react";
// eslint-disable-next-line no-restricted-imports -- parent routes are not tenant-scoped
import { useRouter } from "next/navigation";
import Image from "next/image";
import { signIn, signOut, useSession } from "next-auth/react";
import { Input, Alert } from "~/components/ui";
import { Loading } from "~/components/ui/loading";
import { launchConfetti, clearConfetti } from "~/lib/confetti";
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
      launchConfetti();

      const result = await signIn("parent-credentials", {
        redirect: false,
        email,
        password,
      });

      if (result?.error) {
        clearConfetti();
        const errorMessages: Record<string, string> = {
          account_inactive:
            "Ihr Konto ist deaktiviert. Bitte kontaktieren Sie die Schule.",
          rate_limited:
            "Zu viele Anmeldeversuche. Bitte versuchen Sie es später erneut.",
          // ErrAccountNoGuardianRole — backend sends 403 which CredentialsProvider
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
      clearConfetti();

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
    <div className="flex min-h-screen flex-col items-center justify-center p-4">
      <div className="relative mx-auto w-full max-w-2xl rounded-2xl bg-white/80 p-6 text-center shadow-xl backdrop-blur-md transition-all duration-300 hover:bg-white/90 hover:shadow-2xl sm:p-10">
        <div className="mb-8 flex justify-center">
          <Image
            src="/images/moto_transparent.png"
            alt="MOTO Logo"
            width={200}
            height={80}
            priority
          />
        </div>

        <h1
          className="mb-2 text-4xl font-bold md:text-5xl"
          style={{
            background: "linear-gradient(135deg, #5080d8, #83cd2d)",
            WebkitBackgroundClip: "text",
            backgroundClip: "text",
            WebkitTextFillColor: "transparent",
          }}
        >
          Willkommen im Eltern-Portal
        </h1>
        <p className="mb-6 text-3xl font-semibold tracking-wide text-gray-900 sm:mb-10">
          Anmeldung
        </p>

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
              <Input
                id="parent-email"
                name="email"
                type="email"
                autoComplete="username"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="w-full"
                label=""
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
                <Input
                  id="parent-password"
                  name="password"
                  type={showPassword ? "text" : "password"}
                  autoComplete="current-password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full pr-10"
                  label=""
                />
                <PasswordToggleButton
                  showPassword={showPassword}
                  onToggle={() => setShowPassword(!showPassword)}
                />
              </div>
            </div>
          </div>

          <div className="mt-2 flex justify-center">
            <button
              type="submit"
              disabled={isLoading}
              className="group relative overflow-hidden rounded-xl bg-gray-900 px-8 py-2.5 text-sm font-semibold text-white transition-all duration-200 hover:bg-gray-800 focus:outline-none active:scale-95 disabled:cursor-not-allowed disabled:opacity-50"
            >
              <span className="relative z-10">
                {isLoading ? "Anmeldung läuft..." : "Anmelden"}
              </span>
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

"use client";

import { useState, useEffect, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Image from "next/image";
import { Input, Alert } from "~/components/ui";
import { useSession, signIn } from "~/lib/auth-client";
import { Loading } from "~/components/ui/loading";

function ConsoleLoginForm() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [checkingAuth, setCheckingAuth] = useState(true);
  const [showPassword, setShowPassword] = useState(false);
  const router = useRouter();
  const searchParams = useSearchParams();
  const { data: session, isPending: isSessionLoading } = useSession();

  // Check for existing valid session
  useEffect(() => {
    const checkSession = async () => {
      if (isSessionLoading) return;

      if (session?.user) {
        // Already logged in - verify SaaS admin status
        try {
          const response = await fetch("/api/auth/check-saas-admin");
          const data = (await response.json()) as { isSaasAdmin: boolean };

          if (data.isSaasAdmin) {
            router.push("/console");
          } else {
            setError(
              "Zugriff verweigert. Nur Plattform-Administratoren können sich hier anmelden.",
            );
            setCheckingAuth(false);
          }
        } catch {
          setCheckingAuth(false);
        }
      } else {
        setCheckingAuth(false);
      }
    };

    void checkSession();
  }, [isSessionLoading, session, router]);

  // Check for error messages in URL
  useEffect(() => {
    const urlError = searchParams.get("error");
    if (urlError === "Unauthorized") {
      setError(
        "Zugriff verweigert. Nur Plattform-Administratoren können auf die Konsole zugreifen.",
      );
    }
  }, [searchParams]);

  if (checkingAuth || isSessionLoading) {
    return (
      <div className="flex min-h-dvh flex-col items-center justify-center p-4">
        <Loading />
      </div>
    );
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError("");

    try {
      const result = await signIn.email({
        email,
        password,
      });

      if (result.error) {
        setError(result.error.message ?? "Ungültige E-Mail oder Passwort");
        setIsLoading(false);
        return;
      }

      // Verify SaaS admin status after login
      const response = await fetch("/api/auth/check-saas-admin");
      const data = (await response.json()) as { isSaasAdmin: boolean };

      if (data.isSaasAdmin) {
        router.push("/console");
      } else {
        setError(
          "Zugriff verweigert. Nur Plattform-Administratoren können sich hier anmelden.",
        );
        setIsLoading(false);
      }
    } catch {
      setError("Anmeldefehler. Bitte versuchen Sie es erneut.");
      setIsLoading(false);
    }
  };

  return (
    <div className="flex min-h-dvh flex-col items-center justify-center bg-gray-50 p-4">
      <div className="mx-auto w-full max-w-md rounded-2xl bg-white p-8 shadow-lg">
        {/* Logo */}
        <div className="mb-6 flex justify-center">
          <Image
            src="/images/moto_transparent.png"
            alt="MOTO Logo"
            width={120}
            height={48}
            priority
          />
        </div>

        {/* Header */}
        <div className="mb-8 text-center">
          <h1 className="text-2xl font-bold text-gray-900">
            Plattform-Konsole
          </h1>
          <p className="mt-2 text-sm text-gray-600">
            Anmeldung für Administratoren
          </p>
        </div>

        {/* Login Form */}
        <form onSubmit={handleSubmit} noValidate className="space-y-5">
          {error && <Alert type="error" message={error} />}

          <div className="space-y-4">
            <div>
              <label
                htmlFor="email"
                className="mb-1.5 block text-sm font-medium text-gray-700"
              >
                E-Mail-Adresse
              </label>
              <Input
                id="email"
                name="email"
                type="email"
                autoComplete="username"
                required
                autoFocus
                spellCheck={false}
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="w-full"
                label=""
              />
            </div>

            <div>
              <label
                htmlFor="password"
                className="mb-1.5 block text-sm font-medium text-gray-700"
              >
                Passwort
              </label>
              <div className="relative">
                <Input
                  id="password"
                  name="password"
                  type={showPassword ? "text" : "password"}
                  autoComplete="current-password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full pr-10"
                  label=""
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  tabIndex={-1}
                  className="absolute top-1/2 right-3 -translate-y-1/2 text-gray-500 transition-colors hover:text-gray-700"
                  aria-label={
                    showPassword ? "Passwort verbergen" : "Passwort anzeigen"
                  }
                >
                  {showPassword ? (
                    <svg
                      className="size-5"
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
                      className="size-5"
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
          </div>

          <button
            type="submit"
            disabled={isLoading}
            className="w-full rounded-lg bg-gray-900 px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-gray-800 focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {isLoading ? "Anmeldung läuft..." : "Anmelden"}
          </button>
        </form>

        {/* Footer */}
        <p className="mt-8 text-center text-xs text-gray-500">
          Nur für autorisierte Plattform-Administratoren
        </p>
      </div>
    </div>
  );
}

export default function ConsoleLoginPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-dvh flex-col items-center justify-center p-4">
          <Loading />
        </div>
      }
    >
      <ConsoleLoginForm />
    </Suspense>
  );
}

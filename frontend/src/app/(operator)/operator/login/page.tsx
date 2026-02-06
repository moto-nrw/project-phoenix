"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import Image from "next/image";
import { Input, Alert, HelpButton } from "~/components/ui";
import { Loading } from "~/components/ui/loading";
import { useOperatorAuth } from "~/lib/operator/auth-context";
import { launchConfetti, clearConfetti } from "~/lib/confetti";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import { LoginHelpContent } from "~/components/shared/login-help-content";

export default function OperatorLoginPage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const router = useRouter();
  const { login, isAuthenticated, isLoading: authLoading } = useOperatorAuth();

  // Redirect if already authenticated
  useEffect(() => {
    if (isAuthenticated && !authLoading) {
      router.push("/operator/suggestions");
    }
  }, [isAuthenticated, authLoading, router]);

  // Show loading while checking auth
  if (authLoading || isAuthenticated) {
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

      await login(email, password);
      // login() in auth context handles redirect
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
      <div className="mx-auto w-full max-w-2xl rounded-2xl bg-white/80 p-10 text-center shadow-xl backdrop-blur-md transition-all duration-300 hover:bg-white/90 hover:shadow-2xl">
        {/* Help Button */}
        <div className="absolute top-4 right-4">
          <HelpButton
            title="Hilfe"
            content={
              <LoginHelpContent
                accountType="Operator-Account"
                emailLabel="Ihre Operator E-Mail-Adresse"
                passwordLabel="Ihr Operator-Passwort"
              />
            }
          />
        </div>

        {/* Logo Section */}
        <div className="mb-8 flex justify-center">
          <Image
            src="/images/moto_transparent.png"
            alt="MOTO Logo"
            width={200}
            height={80}
            priority
          />
        </div>

        {/* Welcome Text */}
        <h1
          className="mb-2 text-4xl font-bold md:text-5xl"
          style={{
            background: "linear-gradient(135deg, #5080d8, #83cd2d)",
            WebkitBackgroundClip: "text",
            backgroundClip: "text",
            WebkitTextFillColor: "transparent",
          }}
        >
          Willkommen bei moto
        </h1>
        <p className="mb-10 text-xl text-gray-700">Operator Dashboard</p>

        {/* Login Form */}
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
              <Input
                id="operator-email"
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
                htmlFor="operator-password"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                Passwort
              </label>
              <div className="relative">
                <Input
                  id="operator-password"
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
              className="group relative overflow-hidden rounded-xl bg-gray-900 px-8 py-2.5 text-sm font-semibold text-white transition-all duration-200 hover:bg-gray-800 focus:ring-2 focus:ring-gray-900 focus:ring-offset-2 focus:outline-none active:scale-95 disabled:cursor-not-allowed disabled:opacity-50"
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

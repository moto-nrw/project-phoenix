"use client";

import { useState, useEffect, useRef } from "react";
// eslint-disable-next-line no-restricted-imports -- operator routes are not tenant-scoped
import { useRouter } from "next/navigation";
import Image from "next/image";
import { signIn, signOut, useSession } from "next-auth/react";
import { Input, Alert } from "~/components/ui";
import { Loading } from "~/components/ui/loading";
import { launchConfetti, clearConfetti } from "~/lib/confetti";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import { operatorPath } from "~/lib/operator-url";
import { MFAChallengeForm } from "~/components/auth/mfa-challenge-form";
import { MFAEnrollmentScreen } from "~/components/auth/mfa-enrollment-screen";
import {
  login as loginApi,
  germanMFAErrorMessage,
  MFAApiError,
  type MFATokenResponse,
} from "~/lib/mfa-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OperatorLoginPage" });

interface MFAStep {
  challengeToken: string;
  maskedEmail: string;
}

interface MFAEnrollmentStep {
  accessToken: string;
  refreshToken: string;
  email: string;
}

export default function OperatorLoginPage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [mfaStep, setMfaStep] = useState<MFAStep | null>(null);
  const [enrollmentStep, setEnrollmentStep] =
    useState<MFAEnrollmentStep | null>(null);
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

  const seedSessionWithTokens = async (tokens: MFATokenResponse) => {
    const result = await signIn("operator-credentials", {
      redirect: false,
      internalRefresh: "true",
      token: tokens.access_token,
      refreshToken: tokens.refresh_token,
    });
    if (result?.error) {
      clearConfetti();
      setError("Anmeldung fehlgeschlagen. Bitte versuchen Sie es erneut.");
      logger.error("operator_session_seed_failed", { error: result.error });
      return;
    }
    router.push(operatorPath("/operator/suggestions"));
  };

  const handleMFASuccess = async (tokens: MFATokenResponse) => {
    setMfaStep(null);
    await seedSessionWithTokens(tokens);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError("");

    try {
      launchConfetti();

      const response = await loginApi("operator", { email, password });

      if (response.status === "mfa_required") {
        setMfaStep({
          challengeToken: response.challenge_token,
          maskedEmail: response.masked_email,
        });
        return;
      }

      if (response.mfa_enrollment_required) {
        setEnrollmentStep({
          accessToken: response.access_token,
          refreshToken: response.refresh_token,
          email,
        });
        return;
      }

      await seedSessionWithTokens({
        access_token: response.access_token,
        refresh_token: response.refresh_token,
      });
    } catch (err) {
      clearConfetti();
      if (err instanceof MFAApiError) {
        if (err.status === 403) {
          setError(
            "Ihr Konto ist deaktiviert. Bitte kontaktieren Sie den Administrator.",
          );
        } else if (err.status === 429) {
          setError(
            "Zu viele Anmeldeversuche. Bitte versuchen Sie es später erneut.",
          );
        } else if (err.status === 401) {
          setError("Ungültige Anmeldedaten");
        } else {
          setError(germanMFAErrorMessage(err));
        }
      } else {
        setError(
          err instanceof Error
            ? err.message
            : "Anmeldefehler. Bitte versuchen Sie es erneut.",
        );
      }
      logger.error("operator_login_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen flex-col items-center justify-center p-4">
      <div className="relative mx-auto w-full max-w-2xl rounded-2xl bg-white/80 p-6 text-center shadow-xl backdrop-blur-md transition-all duration-300 hover:bg-white/90 hover:shadow-2xl sm:p-10">
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
        <p className="mb-6 text-3xl font-semibold tracking-wide text-gray-900 sm:mb-10">
          Operator Dashboard
        </p>

        {mfaStep && (
          <MFAChallengeForm
            scope="operator"
            challengeToken={mfaStep.challengeToken}
            maskedEmail={mfaStep.maskedEmail}
            onSuccess={handleMFASuccess}
            onCancel={() => {
              setMfaStep(null);
              setError("");
              setPassword("");
            }}
          />
        )}

        {enrollmentStep && (
          <MFAEnrollmentScreen
            scope="operator"
            bearerToken={enrollmentStep.accessToken}
            userEmail={enrollmentStep.email}
            onComplete={async () => {
              const tokens = {
                access_token: enrollmentStep.accessToken,
                refresh_token: enrollmentStep.refreshToken,
              };
              setEnrollmentStep(null);
              await seedSessionWithTokens(tokens);
            }}
          />
        )}

        {/* Login Form */}
        <form
          onSubmit={handleSubmit}
          noValidate
          className={`space-y-6 ${mfaStep || enrollmentStep ? "hidden" : ""}`}
        >
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

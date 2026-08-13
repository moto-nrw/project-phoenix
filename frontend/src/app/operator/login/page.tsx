"use client";

import { useState, useEffect, useRef } from "react";
// eslint-disable-next-line no-restricted-imports -- operator routes are not tenant-scoped
import { redirect, useRouter } from "next/navigation";
import { signIn, signOut, useSession } from "next-auth/react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { Alert } from "~/components/ui/alert";
import {
  AuthShell,
  OperatorBrand,
  authInputClassName,
  authPrimaryButtonClassName,
} from "~/components/auth/auth-shell";
import { Loading } from "~/components/ui/loading";
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
import {
  isPasskeySupported,
  isPasskeyCeremonyIncompleteError,
  loginWithPasskey,
  PasskeyApiError,
} from "~/lib/passkey-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OperatorLoginPage" });

interface MFAStep {
  challengeToken: string;
  maskedEmail: string;
  trustedDeviceEnabled: boolean;
  trustedDeviceDays: number;
}

interface MFAEnrollmentStep {
  // enrollmentToken is the narrow-scope JWT issued by operator login when
  // MFA enrollment is required. Only authorizes /operator/auth/mfa/enroll/*.
  // A full session pair is minted by the confirm endpoint.
  enrollmentToken: string;
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
  const [passkeySupported, setPasskeySupported] = useState(false);
  const router = useRouter();
  const { data: session, status } = useSession();
  // Ref prevents re-triggering signOut (not in effect deps → no loop).
  // Separate state controls the loading spinner for the UI.
  const cleanupStartedRef = useRef(false);
  const [isCleaningUp, setIsCleaningUp] = useState(false);

  useEffect(() => {
    setPasskeySupported(isPasskeySupported());
  }, []);

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
    };
    void check();
  }, [status, session]);

  if (
    status === "authenticated" &&
    session?.user?.scope === "platform" &&
    session.user.token &&
    session.error === undefined
  ) {
    redirect(operatorPath("/operator/suggestions"));
  }

  // Show loading while checking auth or cleaning stale session
  if (
    status === "loading" ||
    isCleaningUp ||
    (status === "authenticated" && session?.error !== undefined)
  ) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <Loading />
      </div>
    );
  }

  // seedSessionWithTokens hands the already-minted access/refresh pair to
  // NextAuth via the internalRefresh credential path (the same provider
  // development uses for password login, just bypassing the credential
  // check because MFA/verify already authenticated the operator).
  const seedSessionWithTokens = async (tokens: MFATokenResponse) => {
    const result = await signIn("operator-credentials", {
      redirect: false,
      internalRefresh: "true",
      token: tokens.access_token,
      refreshToken: tokens.refresh_token,
    });
    if (result?.error) {
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

  // Map login failures to a German UI message. Kept outside the
  // handler to keep its cognitive complexity below the linter cap.
  const operatorLoginErrorMessage = (err: unknown): string => {
    if (err instanceof MFAApiError) {
      if (err.status === 403) {
        return "Ihr Konto ist deaktiviert. Bitte kontaktieren Sie den Administrator.";
      }
      if (err.status === 429) {
        return "Zu viele Anmeldeversuche. Bitte versuchen Sie es später erneut.";
      }
      if (err.status === 401) {
        return "Ungültige Anmeldedaten";
      }
      return germanMFAErrorMessage(err);
    }
    return err instanceof Error
      ? err.message
      : "Anmeldefehler. Bitte versuchen Sie es erneut.";
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError("");

    try {
      const response = await loginApi("operator", { email, password });

      if (response.status === "mfa_required") {
        setMfaStep({
          challengeToken: response.challenge_token,
          maskedEmail: response.masked_email,
          trustedDeviceEnabled: response.trusted_device_enabled ?? true,
          trustedDeviceDays: response.trusted_device_days ?? 90,
        });
        return;
      }

      if (response.status === "mfa_enrollment_required") {
        // Post-#1430: response carries an enrollment-scoped JWT in
        // access_token (no refresh_token). It only authorizes the
        // operator enrollment routes; the real session is minted by
        // /operator/auth/mfa/enroll/confirm.
        setEnrollmentStep({
          enrollmentToken: response.access_token,
          email,
        });
        return;
      }

      await seedSessionWithTokens({
        access_token: response.access_token,
        refresh_token: response.refresh_token,
      });
    } catch (err) {
      setError(operatorLoginErrorMessage(err));
      logger.error("operator_login_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setIsLoading(false);
    }
  };

  const handlePasskeyLogin = async () => {
    setIsLoading(true);
    setError("");
    try {
      const response = await loginWithPasskey("operator");
      await seedSessionWithTokens({
        access_token: response.access_token,
        refresh_token: response.refresh_token,
      });
    } catch (err) {
      if (isPasskeyCeremonyIncompleteError(err)) {
        logger.info("operator_passkey_login_not_completed", {
          error: err instanceof Error ? err.message : String(err),
        });
        return;
      }
      if (err instanceof PasskeyApiError && err.status === 401) {
        setError("Passkey-Anmeldung fehlgeschlagen.");
      } else {
        setError(
          err instanceof Error
            ? err.message
            : "Passkey-Anmeldung fehlgeschlagen.",
        );
      }
      logger.error("operator_passkey_login_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
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
      {mfaStep ? (
        <MFAChallengeForm
          scope="operator"
          challengeToken={mfaStep.challengeToken}
          maskedEmail={mfaStep.maskedEmail}
          trustedDeviceEnabled={mfaStep.trustedDeviceEnabled}
          trustedDeviceDays={mfaStep.trustedDeviceDays}
          onSuccess={handleMFASuccess}
          onCancel={() => {
            setMfaStep(null);
            setError("");
            setPassword("");
          }}
        />
      ) : enrollmentStep ? (
        <MFAEnrollmentScreen
          scope="operator"
          bearerToken={enrollmentStep.enrollmentToken}
          userEmail={enrollmentStep.email}
          onExit={() => {
            setEnrollmentStep(null);
            setError("");
            setPassword("");
          }}
          onComplete={async (tokens) => {
            // Post-#1430: confirm() returns a fresh access/refresh pair.
            // The enrollment-scoped token never had session privileges,
            // so dropping the enrollment step is the only cleanup needed.
            setEnrollmentStep(null);
            await seedSessionWithTokens({
              access_token: tokens.access_token,
              refresh_token: tokens.refresh_token,
            });
          }}
        />
      ) : (
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
          {passkeySupported && (
            <button
              type="button"
              disabled={isLoading}
              onClick={handlePasskeyLogin}
              className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-4 text-sm font-semibold text-gray-900 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-400"
            >
              <MotoConceptIcon concept="passkeys" size={16} />
              <span>Mit Passkey anmelden</span>
            </button>
          )}
        </form>
      )}
    </AuthShell>
  );
}

"use client";

import { useState, useEffect, useRef } from "react";
// eslint-disable-next-line no-restricted-imports -- operator routes are not tenant-scoped
import { redirect, useRouter } from "next/navigation";
import { useSession } from "next-auth/react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { Alert } from "~/components/ui/alert";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import {
  credentialError,
  LOGIN_REFUSALS,
  PASSKEY_LOGIN_REFUSALS,
} from "~/components/auth/credential-error";
import { useApiFormError } from "~/contexts/ToastContext";
import { signOutStaleSession } from "~/lib/login-session-notice";
import type { MFAStep } from "~/lib/sign-in-answer";
import { AuthShell, OperatorBrand } from "~/components/auth/auth-shell";
import { FieldLabel } from "~/components/ui/field-label";
import { Input } from "~/components/ui/input";
import { Loading } from "~/components/ui/loading";
import { hasSessionAccess } from "~/lib/session-access";
import { operatorPath } from "~/lib/operator-url";
import { MFAChallengeForm } from "~/components/auth/mfa-challenge-form";
import { MFAEnrollmentScreen } from "~/components/auth/mfa-enrollment-screen";
import { login as loginApi } from "~/lib/mfa-api";
import {
  isPasskeySupported,
  isPasskeyCeremonyIncompleteError,
  loginWithPasskey,
} from "~/lib/passkey-api";
import { createLogger } from "~/lib/logger";
import { passkeysConcept } from "~/lib/concepts/passkeys";
import { Button } from "~/components/ui/button";

const logger = createLogger({ component: "OperatorLoginPage" });

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
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { show: showLoginError, clear: clearLoginError } = formErrors;
  const [sessionExpired, setSessionExpired] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [mfaStep, setMfaStep] = useState<MFAStep | null>(null);
  const [enrollmentStep, setEnrollmentStep] =
    useState<MFAEnrollmentStep | null>(null);
  const [passkeySupported, setPasskeySupported] = useState(false);
  const router = useRouter();
  const { data: session, status, update } = useSession();
  const cleanupStartedRef = useRef(false);
  const [isCleaningUp, setIsCleaningUp] = useState(false);

  useEffect(() => {
    setPasskeySupported(isPasskeySupported());
  }, []);

  useEffect(() => {
    const url = new URL(window.location.href);
    if (url.searchParams.get("error") !== "SessionExpired") return;
    setSessionExpired(true);
    url.searchParams.delete("error");
    window.history.replaceState(
      {},
      "",
      `${url.pathname}${url.search}${url.hash}`,
    );
  }, []);

  useEffect(() => {
    const cleanup = signOutStaleSession({
      status,
      session,
      started: cleanupStartedRef,
      logger,
    });
    if (!cleanup) return;
    setIsCleaningUp(true);
    void cleanup.then(() => setIsCleaningUp(false));
  }, [status, session]);

  // No browser session carries a token (#4991, #5005); a stripped session
  // carries an error and is cleared above instead of redirected.
  if (
    status === "authenticated" &&
    !enrollmentStep &&
    session?.user?.scope === "platform" &&
    hasSessionAccess(session)
  ) {
    redirect(operatorPath("/operator/organizations"));
  }

  // Show loading while checking auth or cleaning stale session
  if (
    status === "loading" ||
    isCleaningUp ||
    (status === "authenticated" && session?.error !== undefined)
  ) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <Loading fullPage />
      </div>
    );
  }

  // The BFF has installed the cookie. Refresh only the safe provider snapshot.
  const completeLogin = async () => {
    await update();
    router.push(operatorPath("/operator/organizations"));
  };

  const handleMFASuccess = async () => {
    await completeLogin();
    setMfaStep(null);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    clearLoginError();
    setSessionExpired(false);

    try {
      const response = await loginApi("operator", { email, password });

      if (response.status === "mfa_required") {
        setMfaStep(response);
        return;
      }

      if (response.status === "mfa_enrollment_required") {
        setEnrollmentStep({
          enrollmentToken: response.enrollmentToken,
          email,
        });
        return;
      }

      await completeLogin();
    } catch (err) {
      logger.warn("operator_login_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      // Vor der Anmeldung heißt 401 "abgelehnt", nicht "Sitzung abgelaufen".
      void showLoginError(credentialError(err, LOGIN_REFUSALS), {
        object: "die Anmeldung",
      });
    } finally {
      setIsLoading(false);
    }
  };

  const handlePasskeyLogin = async () => {
    setIsLoading(true);
    clearLoginError();
    setSessionExpired(false);
    try {
      await loginWithPasskey("operator");
      await completeLogin();
    } catch (err) {
      if (isPasskeyCeremonyIncompleteError(err)) {
        logger.info("operator_passkey_login_not_completed", {
          error: err instanceof Error ? err.message : String(err),
        });
        return;
      }
      logger.warn("operator_passkey_login_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showLoginError(credentialError(err, PASSKEY_LOGIN_REFUSALS), {
        object: "die Anmeldung mit Passkey",
      });
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <AuthShell
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
            clearLoginError();
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
            clearLoginError();
            setPassword("");
          }}
          onComplete={async () => {
            // The BFF has installed the session; discard the limited proof
            // only after the browser-safe session read succeeds.
            await completeLogin();
            setEnrollmentStep(null);
          }}
        />
      ) : (
        <form
          ref={formRef}
          onSubmit={handleSubmit}
          noValidate
          className="space-y-6"
        >
          {formErrors.error ? (
            <FormErrorAlert message={formErrors.error} />
          ) : sessionExpired ? (
            <Alert
              type="info"
              message="Ihre Anmeldung ist abgelaufen. Bitte melden Sie sich erneut an."
            />
          ) : null}

          <div className="space-y-4">
            <div className="text-left">
              <Input
                id="operator-email"
                name="email"
                type="email"
                label="E-Mail-Adresse"
                autoComplete="username"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                controlSize="compact"
              />
            </div>

            <div className="text-left">
              <FieldLabel htmlFor="operator-password" required>
                Passwort
              </FieldLabel>
              <Input
                id="operator-password"
                name="password"
                type="password"
                autoComplete="current-password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                controlSize="compact"
                revealable
              />
            </div>
          </div>

          <Button
            type="submit"
            disabled={isLoading}
            variant="primary"
            size="field"
            className="w-full"
          >
            <span className="relative z-10">
              {isLoading ? "Anmeldung läuft..." : "Anmelden"}
            </span>
          </Button>
          {passkeySupported && (
            <Button
              type="button"
              variant="outline"
              size="field"
              className="w-full gap-2"
              disabled={isLoading}
              onClick={handlePasskeyLogin}
            >
              <MotoConceptIcon concept={passkeysConcept} size={16} />
              <span>Mit Passkey anmelden</span>
            </Button>
          )}
        </form>
      )}
    </AuthShell>
  );
}

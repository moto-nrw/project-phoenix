"use client";

// Login des Schul-Portals ("moto schule", #2207). Spiegelt den Aufbau des
// Tenant-Logins (MFA-Kette inklusive Enrollment), aber ohne Tenant-Kontext,
// Passkeys und Eltern-Weiche: der Backend-Login pinnt selbst die Schule, an
// der das Konto eine Schul-Portal-Rolle hält.

import { useEffect, useRef, useState, Suspense } from "react";
import { signIn, signOut, useSession } from "next-auth/react";
import { useSearchParams } from "next/navigation";
import {
  AuthShell,
  MotoBrand,
  authInputClassName,
  authPrimaryButtonClassName,
} from "~/components/auth/auth-shell";
import { MFAChallengeForm } from "~/components/auth/mfa-challenge-form";
import { MFAEnrollmentScreen } from "~/components/auth/mfa-enrollment-screen";
import { PasswordResetModal } from "~/components/ui/password-reset-modal";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import { Alert } from "~/components/ui/alert";
import { requestSchoolPasswordReset } from "~/lib/auth-api";
import { schoolPath } from "~/lib/school-url";
import { DELIBERATE_LOGOUT_KEY } from "~/lib/session-cache";
import {
  login as loginApi,
  germanMFAErrorMessage,
  MFAApiError,
  type MFATokenResponse,
} from "~/lib/mfa-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "SchoolLoginPage" });

interface MFAStep {
  challengeToken: string;
  maskedEmail: string;
  trustedDeviceEnabled: boolean;
  trustedDeviceDays: number;
}

interface MFAEnrollmentStep {
  enrollmentToken: string;
  email: string;
}

function clearSessionErrorFromUrl() {
  const url = new URL(window.location.href);
  const hadSessionError =
    url.searchParams.get("error") === "SessionRequired" ||
    url.searchParams.get("error") === "SessionExpired";

  if (!hadSessionError) return;

  url.searchParams.delete("error");
  url.searchParams.delete("callbackUrl");

  const nextUrl = `${url.pathname}${url.search}${url.hash}`;
  window.history.replaceState({}, "", nextUrl);
}

function LoginForm() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [noPortalRole, setNoPortalRole] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [checkingAuth, setCheckingAuth] = useState(true);
  const [showPassword, setShowPassword] = useState(false);
  const [awaitingRedirect, setAwaitingRedirect] = useState(false);
  const [isResetModalOpen, setIsResetModalOpen] = useState(false);
  const [mfaStep, setMfaStep] = useState<MFAStep | null>(null);
  const [enrollmentStep, setEnrollmentStep] =
    useState<MFAEnrollmentStep | null>(null);
  const searchParams = useSearchParams();
  const { data: session, status } = useSession();

  // Guard against calling signOut multiple times during stale session cleanup
  const isCleaningSessionRef = useRef(false);

  useEffect(() => {
    const checkAndRedirect = async () => {
      // Irrecoverable session error (refresh token rejected): clear the
      // stale cookie so a fresh login isn't shadowed by retry loops.
      if (
        status === "authenticated" &&
        session?.error &&
        !isCleaningSessionRef.current
      ) {
        isCleaningSessionRef.current = true;
        logger.debug("clearing_stale_session", { error: session.error });
        try {
          await signOut({ redirect: false });
        } catch (err) {
          isCleaningSessionRef.current = false;
          logger.warn("signout_during_cleanup_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
        }
        setCheckingAuth(false);
        return;
      }

      if (status === "authenticated" && session?.user?.token) {
        window.location.href = schoolPath("/school");
        return;
      }

      if (status !== "loading") {
        setCheckingAuth(false);
      }
    };

    void checkAndRedirect();
  }, [status, session]);

  // Session-abgelaufen-Hinweis, außer nach bewusstem Logout.
  useEffect(() => {
    const urlError = searchParams.get("error");
    if (urlError === "SessionRequired" || urlError === "SessionExpired") {
      let deliberate = false;
      try {
        deliberate = sessionStorage.getItem(DELIBERATE_LOGOUT_KEY) === "1";
        sessionStorage.removeItem(DELIBERATE_LOGOUT_KEY);
      } catch {
        // sessionStorage unavailable
      }
      if (!deliberate) {
        setError(
          "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.",
        );
      }
      clearSessionErrorFromUrl();
    }
  }, [searchParams]);

  const isCheckingAuth = checkingAuth || status === "loading";
  const isSubmitting = isLoading || awaitingRedirect;

  // seedSessionWithTokens übergibt ein bereits gemintetes school-Token-Paar
  // an NextAuth (internalRefresh-Pfad) — nach MFA-Verify/Enroll oder einem
  // Login ohne zweite Stufe.
  const seedSessionWithTokens = async (tokens: MFATokenResponse) => {
    clearSessionErrorFromUrl();
    const result = await signIn("school-credentials", {
      redirect: false,
      internalRefresh: "true",
      token: tokens.access_token,
      refreshToken: tokens.refresh_token,
    });
    if (result?.error) {
      setError("Anmeldung fehlgeschlagen. Bitte versuchen Sie es erneut.");
      logger.error("session_seed_failed", { error: result.error });
      return;
    }
    setAwaitingRedirect(true);
    window.location.href = schoolPath("/school");
  };

  const handleMFASuccess = async (tokens: MFATokenResponse) => {
    await seedSessionWithTokens(tokens);
    setMfaStep(null);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError("");
    setNoPortalRole(false);

    try {
      const response = await loginApi("school", { email, password });

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
      // Konto ohne Schul-Portal-Rolle: das Passwort war korrekt, die
      // Ablehnung kommt von der Rolle — die spezifische Meldung verrät
      // also nichts über fremde Konten und erspart Passwort-Reset-Schleifen.
      if (err instanceof MFAApiError && err.code === "no_school_portal_role") {
        setNoPortalRole(true);
        return;
      }

      if (err instanceof MFAApiError && err.status === 401) {
        setError("Ungültige E-Mail oder Passwort");
      } else {
        setError(germanMFAErrorMessage(err));
      }
      logger.error("school login failed", {
        error: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <>
      <AuthShell
        eyebrow="Schulportal"
        eyebrowClassName="text-[#5080D8]"
        title="Willkommen im Schulportal"
        subtitle="Melden Sie sich mit Ihrem Lehrkraft-Konto an."
        variant="tenant"
        brand={<MotoBrand />}
      >
        {isCheckingAuth && (
          <div className="flex items-center justify-center py-12">
            <div className="flex flex-col items-center gap-4">
              <div className="h-10 w-10 animate-spin rounded-full border-2 border-gray-200 border-t-gray-950" />
              <p className="text-sm text-gray-500">Sitzung wird überprüft...</p>
            </div>
          </div>
        )}

        <div
          className={`transition-opacity duration-300 ${isCheckingAuth ? "pointer-events-none hidden" : "opacity-100"}`}
        >
          {mfaStep ? (
            <MFAChallengeForm
              scope="school"
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
              scope="school"
              bearerToken={enrollmentStep.enrollmentToken}
              userEmail={enrollmentStep.email}
              onExit={() => {
                setEnrollmentStep(null);
                setError("");
                setPassword("");
              }}
              onComplete={async (tokens) => {
                setEnrollmentStep(null);
                await seedSessionWithTokens({
                  access_token: tokens.access_token,
                  refresh_token: tokens.refresh_token,
                });
              }}
            />
          ) : (
            <form onSubmit={handleSubmit} noValidate className="space-y-6">
              {noPortalRole ? (
                <Alert
                  type="info"
                  message="Dieses Konto hat keinen Zugang zum Schul-Portal. Bitte wenden Sie sich an Ihre OGS-Verwaltung."
                />
              ) : (
                error && <Alert type="error" message={error} />
              )}

              <div className="space-y-4">
                <div className="text-left">
                  <label
                    htmlFor="email"
                    className="mb-1 block text-sm font-medium text-gray-700"
                  >
                    E-Mail-Adresse
                  </label>
                  <input
                    id="email"
                    name="email"
                    type="email"
                    data-testid="input-email"
                    autoComplete="username"
                    required
                    disabled={isSubmitting}
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    className={authInputClassName}
                  />
                </div>

                <div className="text-left">
                  <label
                    htmlFor="password"
                    className="mb-1 block text-sm font-medium text-gray-700"
                  >
                    Passwort
                  </label>
                  <div className="relative">
                    <input
                      id="password"
                      name="password"
                      type={showPassword ? "text" : "password"}
                      data-testid="input-password"
                      autoComplete="current-password"
                      required
                      disabled={isSubmitting}
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

                <div className="text-center">
                  <button
                    type="button"
                    disabled={isSubmitting}
                    onClick={() => setIsResetModalOpen(true)}
                    className="text-sm text-gray-600 transition-colors hover:text-gray-800 hover:underline focus:underline focus:outline-none disabled:cursor-not-allowed disabled:text-gray-400"
                  >
                    Passwort vergessen?
                  </button>
                </div>
              </div>

              <div className="mt-2">
                <button
                  type="submit"
                  disabled={isSubmitting}
                  className={authPrimaryButtonClassName}
                >
                  <span className="relative z-10">
                    {isSubmitting ? "Anmeldung läuft..." : "Anmelden"}
                  </span>
                </button>
              </div>
            </form>
          )}
        </div>
      </AuthShell>

      <PasswordResetModal
        isOpen={isResetModalOpen}
        onClose={() => setIsResetModalOpen(false)}
        onRequestReset={requestSchoolPasswordReset}
      />
    </>
  );
}

export default function SchoolLoginPage() {
  return (
    <Suspense
      fallback={
        <AuthShell
          eyebrow="Schulportal"
          eyebrowClassName="text-[#5080D8]"
          title="Willkommen im Schulportal"
          subtitle="Melden Sie sich mit Ihrem Lehrkraft-Konto an."
          variant="tenant"
          brand={<MotoBrand />}
        >
          <div className="flex flex-col items-center gap-4 py-8">
            <div className="h-10 w-10 animate-spin rounded-full border-2 border-gray-200 border-t-gray-950" />
            <p className="text-sm text-gray-500">Laden...</p>
          </div>
        </AuthShell>
      }
    >
      <LoginForm />
    </Suspense>
  );
}

// [tenant]/page.tsx, tenant-scoped login page
"use client";

import { useState, useEffect, useRef, Suspense } from "react";
import { signIn, signOut, useSession } from "next-auth/react";
import { useSearchParams } from "next/navigation";
import Image from "next/image";
import { KeyRound } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { refreshToken } from "~/lib/auth-api";
import { trackEvent } from "~/lib/analytics";
import { SmartRedirect } from "~/components/auth/smart-redirect";
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
import { useTenant } from "~/lib/tenant-context";
import { loginImageSrc } from "~/lib/tenant-api";
import { useTenantRouter } from "~/lib/tenant-router";
import { DELIBERATE_LOGOUT_KEY } from "~/lib/session-cache";
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

const logger = createLogger({ component: "TenantLoginPage" });

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

interface MFAStep {
  challengeToken: string;
  maskedEmail: string;
  trustedDeviceEnabled: boolean;
  trustedDeviceDays: number;
}

interface MFAEnrollmentStep {
  // enrollmentToken is the narrow-scope JWT issued by login when MFA is
  // required and the user has no credential yet. It only authorizes
  // /auth/mfa/enroll/* — a full session pair is minted by the confirm
  // endpoint and seeded via seedSessionWithTokens once enrollment succeeds.
  enrollmentToken: string;
  email: string;
}

function LoginForm() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [checkingAuth, setCheckingAuth] = useState(true);
  const [showPassword, setShowPassword] = useState(false);
  const [awaitingRedirect, setAwaitingRedirect] = useState(false);
  const [isResetModalOpen, setIsResetModalOpen] = useState(false);
  const [mfaStep, setMfaStep] = useState<MFAStep | null>(null);
  const [enrollmentStep, setEnrollmentStep] =
    useState<MFAEnrollmentStep | null>(null);
  const [passkeySupported, setPasskeySupported] = useState(false);
  const router = useTenantRouter();
  const { tenantSlug, tenant } = useTenant();
  const searchParams = useSearchParams();
  const { data: session, status } = useSession();
  const loginTitle = tenant?.name?.trim() ? tenant.name : "Willkommen bei moto";
  const tenantLoginImageUrl = tenant?.settings?.loginImageUrl;
  const hasTenantLoginLogo = Boolean(tenantLoginImageUrl);

  // Guard against calling signOut multiple times during stale session cleanup
  const isCleaningSessionRef = useRef(false);

  // Check for valid session
  useEffect(() => {
    setPasskeySupported(isPasskeySupported());
  }, []);

  useEffect(() => {
    const checkAndRedirect = async () => {
      // If the session has an irrecoverable error (refresh token was rejected
      // by the backend), clear the stale session cookie so the user can log in
      // fresh. Without this, the JWT callback keeps retrying the failed refresh
      // in the background, and the stale session competes with new login attempts.
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

      // If we have a valid session with access token, set up for redirect
      if (status === "authenticated" && session?.user?.token) {
        logger.debug("valid session found, preparing smart redirect");
        setAwaitingRedirect(true);
        setCheckingAuth(false);
        return;
      }

      // If session is expired but we have a refresh token, try to refresh
      if (
        status === "authenticated" &&
        session?.user?.refreshToken &&
        !session?.user?.token
      ) {
        logger.debug(
          "session expired but refresh token available, attempting refresh",
        );
        try {
          const newTokens = await refreshToken();
          if (newTokens) {
            // Update session with new tokens
            const result = await signIn("credentials", {
              redirect: false,
              internalRefresh: true,
              token: newTokens.access_token,
              refreshToken: newTokens.refresh_token,
            });

            if (!result?.error) {
              logger.debug("token refreshed successfully");
              setAwaitingRedirect(true);
              setCheckingAuth(false);
              return;
            }
          }
        } catch (error) {
          logger.error("failed to refresh token", {
            error: error instanceof Error ? error.message : String(error),
          });
        }
      }

      // Only show login form if not authenticated
      if (status !== "loading") {
        setCheckingAuth(false);
      }
    };

    void checkAndRedirect();
  }, [status, session]);

  // Check for session errors in URL, but suppress after a deliberate logout.
  // NextAuth's useSession({ required: true }) races the logout navigation and
  // can redirect here with ?error=SessionRequired before the page unloads.
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
      if (deliberate) {
        // Clean up NextAuth's error/callbackUrl params from the URL
        clearSessionErrorFromUrl();
      } else {
        setError(
          "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.",
        );
        clearSessionErrorFromUrl();
      }
    }
  }, [searchParams]);

  const isCheckingAuth = checkingAuth || status === "loading";
  const isSmartRedirecting =
    awaitingRedirect &&
    status === "authenticated" &&
    Boolean(session?.user?.token);
  const isSubmitting = isLoading || awaitingRedirect;

  // seedSessionWithTokens hands an already-minted access/refresh pair to
  // NextAuth via the internalRefresh credential path. Used after a
  // successful MFA verify/enroll OR a non-MFA login — in all cases the
  // backend has already authenticated the account, so we only seed the
  // session.
  const seedSessionWithTokens = async (tokens: MFATokenResponse) => {
    clearSessionErrorFromUrl();
    const result = await signIn("credentials", {
      redirect: false,
      internalRefresh: "true",
      token: tokens.access_token,
      refreshToken: tokens.refresh_token,
    });
    if (result?.error) {
      setError("Anmeldung fehlgeschlagen. Bitte versuchen Sie es erneut.");
      logger.error("session_seed_failed", { error: result.error });
      trackEvent("login_failed", { reason: "error" });
      return;
    }
    trackEvent("login_success");
    setAwaitingRedirect(true);
    router.refresh();
  };

  const handleMFASuccess = async (tokens: MFATokenResponse) => {
    await seedSessionWithTokens(tokens);
    setMfaStep(null);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError("");

    try {
      const response = await loginApi("tenant", {
        email,
        password,
        tenantSlug,
      });

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
        // Post-#1430: the response carries an enrollment-scoped JWT in
        // access_token (no refresh_token). It only authorizes
        // /auth/mfa/enroll/* — the real session is minted by confirm.
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
      if (err instanceof MFAApiError && err.status === 401) {
        setError("Ungültige E-Mail oder Passwort");
        trackEvent("login_failed", { reason: "invalid_credentials" });
      } else {
        setError(germanMFAErrorMessage(err));
        trackEvent("login_failed", { reason: "error" });
      }
      logger.error("login failed", {
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
      const response = await loginWithPasskey("tenant", { tenantSlug });
      await seedSessionWithTokens({
        access_token: response.access_token,
        refresh_token: response.refresh_token,
      });
    } catch (err) {
      if (isPasskeyCeremonyIncompleteError(err)) {
        logger.info("passkey login not completed", {
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
      logger.error("passkey login failed", {
        error: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <>
      <AuthShell
        eyebrow="Ihr OGS Portal"
        eyebrowClassName="text-[#83CD2D]"
        title={loginTitle}
        subtitle="Melden Sie sich mit Ihrem Konto an."
        variant="tenant"
        showMotoAttribution={hasTenantLoginLogo}
        brand={
          tenantLoginImageUrl ? (
            <Image
              src={loginImageSrc(tenantLoginImageUrl)}
              alt={`${tenant?.name ?? "Einrichtung"} Logo`}
              width={180}
              height={104}
              className="max-h-[104px] w-auto object-contain"
              priority
              unoptimized
            />
          ) : (
            <MotoBrand />
          )
        }
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
          {isSmartRedirecting ? (
            <div className="flex items-center justify-center py-12">
              <div className="flex flex-col items-center gap-4 text-center">
                <div className="h-10 w-10 animate-spin rounded-full border-2 border-gray-200 border-t-gray-950" />
                <div className="space-y-1">
                  <p className="text-sm font-medium text-gray-900">
                    Sie sind bereits angemeldet.
                  </p>
                  <p className="text-sm text-gray-500">
                    moto öffnet Ihre OGS...
                  </p>
                </div>
              </div>
            </div>
          ) : mfaStep ? (
            <MFAChallengeForm
              scope="tenant"
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
              scope="tenant"
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
              {error && <Alert type="error" message={error} />}

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

                {/* Forgot Password Link */}
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
              {passkeySupported && (
                <button
                  type="button"
                  disabled={isSubmitting}
                  onClick={handlePasskeyLogin}
                  className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-4 text-sm font-semibold text-gray-900 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-400"
                >
                  <KeyRound className="h-4 w-4" aria-hidden="true" />
                  <span>Mit Passkey anmelden</span>
                </button>
              )}
            </form>
          )}
        </div>

        {/* Smart redirect for authenticated users */}
        {awaitingRedirect &&
          status === "authenticated" &&
          session?.user?.token && (
            <SmartRedirect
              onRedirect={(path) => {
                logger.info("redirecting based on user permissions", { path });
                router.push(path);
              }}
            />
          )}
      </AuthShell>

      {/* Password Reset Modal */}
      <PasswordResetModal
        isOpen={isResetModalOpen}
        onClose={() => setIsResetModalOpen(false)}
      />
    </>
  );
}

export default function HomePage() {
  return (
    <Suspense
      fallback={
        <AuthShell
          eyebrow="Ihr OGS Portal"
          eyebrowClassName="text-[#83CD2D]"
          title="Willkommen"
          subtitle="Melden Sie sich mit Ihrem Konto an."
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

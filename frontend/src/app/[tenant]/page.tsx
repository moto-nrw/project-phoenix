// [tenant]/page.tsx, tenant-scoped login page
"use client";

import type { ErrorCode } from "~/lib/error-codes.generated";
import { useState, useEffect, useLayoutEffect, useRef, Suspense } from "react";
import { signIn, signOut, useSession } from "next-auth/react";
import { useSearchParams } from "next/navigation";
import Image from "next/image";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { Alert } from "~/components/ui/alert";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { credentialError } from "~/components/auth/credential-error";
import { useApiFormError } from "~/contexts/ToastContext";
import { wireErrorCode } from "~/lib/api-error";
import { refreshToken } from "~/lib/auth-api";
import { trackTenantEvent } from "~/lib/analytics";
import { SmartRedirect } from "~/components/auth/smart-redirect";
import {
  AuthFieldError,
  AuthShell,
  MotoBrand,
  authInputClass,
  authPrimaryButtonClassName,
} from "~/components/auth/auth-shell";
import { MFAChallengeForm } from "~/components/auth/mfa-challenge-form";
import { MFAEnrollmentScreen } from "~/components/auth/mfa-enrollment-screen";
import { TenantPasswordResetModal } from "~/components/auth/tenant-password-reset-modal";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import { useTenant } from "~/lib/tenant-context";
import { loginImageSrc } from "~/lib/tenant-api";
import { useTenantRouter } from "~/lib/tenant-router";
import { parentsPortalLoginUrl } from "~/lib/parent-url";
import { schoolPortalLoginUrl } from "~/lib/school-url";
import { isSchoolPortalHandoffPath } from "~/lib/redirect-utils";
import { DELIBERATE_LOGOUT_KEY } from "~/lib/session-cache";
import {
  login as loginApi,
  MFAApiError,
  type MFATokenResponse,
} from "~/lib/mfa-api";
import {
  isPasskeySupported,
  isPasskeyCeremonyIncompleteError,
  loginWithPasskey,
  PasskeyApiError,
} from "~/lib/passkey-api";

import { errorStatus } from "~/lib/expected-failure";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "TenantLoginPage" });

/**
 * Grace period before an account is handed over to the portal it belongs
 * to. Long enough to read why the page is about to change, short enough
 * that nobody starts wondering whether the login worked.
 */
const WRONG_PORTAL_REDIRECT_MS = 2000;

/** The analytics reason of a refused login, by code; anything else is "error". */
const LOGIN_FAILURE_REASONS: Partial<Record<ErrorCode, string>> = {
  "identity.invalid_credentials": "invalid_credentials",
  "identity.session_account_inactive": "account_inactive",
};

/**
 * Set when the backend refused a login on PORTAL grounds: the credentials
 * were correct, but this account belongs to the parents portal
 * ("identity.use_parent_portal") or to moto schule ("identity.use_school_portal", #2207).
 *
 * redirectUrl is null only if the portal's hostname env var is missing at
 * runtime (env.js rejects that at build time). We then still explain the
 * situation, just without a link or an automatic jump.
 */
interface WrongPortalHint {
  portal: "parents" | "school";
  redirectUrl: string | null;
}

/** What the hint says, per portal. */
const WRONG_PORTAL_COPY = {
  parents: {
    // Kurz halten: das Ziel steht schon auf dem Link daneben, sonst
    // quetscht sich die Meldung im Kartenlayout.
    redirecting: "Dieses Konto ist ein Elternkonto. Sie werden weitergeleitet.",
    manual:
      "Dieses Konto ist ein Elternkonto. Bitte melden Sie sich im Elternportal an.",
    action: "Jetzt zum Elternportal",
  },
  school: {
    redirecting:
      "Dieses Konto ist ein Lehrkraft-Konto. Sie werden weitergeleitet.",
    manual:
      "Dieses Konto ist ein Lehrkraft-Konto. Bitte melden Sie sich bei moto schule an.",
    action: "Jetzt zu moto schule",
  },
} as const;

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
  // Ein Hinweis, kein Fehler: die Sitzung ist abgelaufen (aus der URL).
  const [sessionExpired, setSessionExpired] = useState(false);
  // Fehler laufen über den gemeinsamen Weg (#2517): Katalogtext je Code im
  // Fehlerkasten des Formulars, Feldfehler am Feld.
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearFormError } = formErrors;
  const latestSubmitRef = useRef<() => void>(() => undefined);
  const latestPasskeyRef = useRef<() => void>(() => undefined);
  const [isLoading, setIsLoading] = useState(false);
  const [checkingAuth, setCheckingAuth] = useState(true);
  const [showPassword, setShowPassword] = useState(false);
  const [awaitingRedirect, setAwaitingRedirect] = useState(false);
  const [isResetModalOpen, setIsResetModalOpen] = useState(false);
  const [mfaStep, setMfaStep] = useState<MFAStep | null>(null);
  const [enrollmentStep, setEnrollmentStep] =
    useState<MFAEnrollmentStep | null>(null);
  const [wrongPortalHint, setWrongPortalHint] =
    useState<WrongPortalHint | null>(null);
  const [passkeySupported, setPasskeySupported] = useState(false);
  const router = useTenantRouter();
  const { tenantSlug, tenant } = useTenant();
  const searchParams = useSearchParams();
  const { data: session, status } = useSession();
  const loginTitle = tenant?.name?.trim() ? tenant.name : "Willkommen bei moto";
  const tenantLoginImageUrl = tenant?.settings?.loginImageUrl;
  const hasTenantLoginLogo = Boolean(tenantLoginImageUrl);
  const analyticsSchoolId = tenant?.tenantId?.toString() ?? null;

  const trackLoginEvent = (
    event: "login_failed",
    props?: Record<string, string | number | boolean>,
  ) => {
    if (!analyticsSchoolId) return;
    trackTenantEvent(event, analyticsSchoolId, props);
  };

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
        setSessionExpired(true);
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
      logger.error("session_seed_failed", { error: result.error });
      trackLoginEvent("login_failed", { reason: "error" });
      // Kein ApiError: der Weg zeigt den allgemeinen Text für die Anmeldung.
      void formErrors.show(new Error(result.error), {
        object: "die Anmeldung",
      });
      return;
    }
    // login_success comes from the backend once it mints the session (#3602).
    setAwaitingRedirect(true);
    router.refresh();
  };

  // Hand the account over to the portal it belongs to. Done in an effect
  // rather than a setTimeout inside the submit handler so React cancels the
  // pending jump if the user navigates away first.
  useEffect(() => {
    const target = wrongPortalHint?.redirectUrl;
    if (!target) return;

    const timer = setTimeout(() => {
      window.location.href = target;
    }, WRONG_PORTAL_REDIRECT_MS);

    return () => clearTimeout(timer);
  }, [wrongPortalHint]);

  const showWrongPortalHint = (code: ErrorCode | undefined): boolean => {
    if (
      code !== "identity.use_parent_portal" &&
      code !== "identity.use_school_portal"
    ) {
      return false;
    }

    const portal = code === "identity.use_parent_portal" ? "parents" : "school";
    let redirectUrl: string | null = null;
    try {
      redirectUrl =
        portal === "parents"
          ? parentsPortalLoginUrl("?from=staff")
          : schoolPortalLoginUrl(
              `?from=staff&tenant=${encodeURIComponent(tenantSlug)}`,
            );
    } catch (urlErr) {
      logger.error("portal_url_unavailable", {
        portal,
        error: urlErr instanceof Error ? urlErr.message : String(urlErr),
      });
    }
    setWrongPortalHint({ portal, redirectUrl });
    // The analytics reason keeps its name from before the code rename (#2506).
    trackLoginEvent("login_failed", {
      reason: portal === "parents" ? "use_parent_portal" : "use_school_portal",
    });
    return true;
  };

  const handleMFASuccess = async (tokens: MFATokenResponse) => {
    await seedSessionWithTokens(tokens);
    setMfaStep(null);
  };

  const submitLogin = async () => {
    setIsLoading(true);
    formErrors.clear();
    setSessionExpired(false);
    setWrongPortalHint(null);

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
      // Wrong portal at the staff login. The backend accepted the password
      // and refused on role, so being specific leaks nothing — and the
      // generic "Anmeldung fehlgeschlagen" reads like a password problem,
      // which is what sent parents into repeated reset loops.
      if (err instanceof MFAApiError && showWrongPortalHint(err.code)) {
        return;
      }

      // Ein ausgeschaltetes Konto hat einen eigenen Katalogtext: sonst liest
      // sich die Ablehnung wie ein falsches Passwort, und niemand kommt über
      // "Passwort vergessen" wieder hinein (#3376).
      const code =
        err instanceof MFAApiError ? wireErrorCode(err.code) : undefined;
      trackLoginEvent("login_failed", {
        reason: (code && LOGIN_FAILURE_REASONS[code]) ?? "error",
      });
      // Vor der Anmeldung heißt 401 "abgelehnt", nicht "Sitzung abgelaufen".
      void formErrors.show(credentialError(err), {
        object: "die Anmeldung",
        retry: () => latestSubmitRef.current(),
      });
      // A 4xx is the backend rejecting the login (wrong password, no access
      // to this school), not a defect, so it stays out of Sentry issues.
      const status = errorStatus(err);
      const logContext = {
        error: err instanceof Error ? err.message : String(err),
        status,
      };
      if (status !== undefined && status >= 400 && status < 500) {
        logger.warn("login rejected", logContext);
      } else {
        logger.error("login failed", logContext);
      }
    } finally {
      setIsLoading(false);
    }
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    void submitLogin();
  };

  const handlePasskeyLogin = async () => {
    setIsLoading(true);
    formErrors.clear();
    setSessionExpired(false);
    setWrongPortalHint(null);
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
      if (err instanceof PasskeyApiError && showWrongPortalHint(err.code)) {
        return;
      }
      logger.error("passkey login failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(credentialError(err), {
        object: "die Anmeldung mit Passkey",
        retry: () => latestPasskeyRef.current(),
      });
    } finally {
      setIsLoading(false);
    }
  };

  // „Wiederholen“ sendet den Stand, der dann im Formular steht.
  useLayoutEffect(() => {
    latestSubmitRef.current = () => void submitLogin();
    latestPasskeyRef.current = () => void handlePasskeyLogin();
  });

  return (
    <>
      <AuthShell
        eyebrow="Ihr OGS Portal"
        eyebrowClassName="text-moto-green"
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
              onError={(mfaError) => {
                if (
                  mfaError instanceof MFAApiError &&
                  showWrongPortalHint(mfaError.code)
                ) {
                  setMfaStep(null);
                  return true;
                }
                return false;
              }}
              onCancel={() => {
                setMfaStep(null);
                clearFormError();
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
                clearFormError();
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
            <form
              ref={formRef}
              onSubmit={handleSubmit}
              noValidate
              className="space-y-6"
            >
              {wrongPortalHint ? (
                <Alert
                  type="info"
                  message={
                    wrongPortalHint.redirectUrl
                      ? WRONG_PORTAL_COPY[wrongPortalHint.portal].redirecting
                      : WRONG_PORTAL_COPY[wrongPortalHint.portal].manual
                  }
                  action={
                    wrongPortalHint.redirectUrl ? (
                      <a
                        href={wrongPortalHint.redirectUrl}
                        className="font-medium whitespace-nowrap underline underline-offset-2"
                      >
                        {WRONG_PORTAL_COPY[wrongPortalHint.portal].action}
                      </a>
                    ) : undefined
                  }
                />
              ) : formErrors.error ? (
                <FormErrorAlert message={formErrors.error} />
              ) : sessionExpired ? (
                <Alert
                  type="info"
                  message="Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an."
                />
              ) : null}

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
                    aria-invalid={Boolean(formErrors.fieldError("email"))}
                    aria-describedby={
                      formErrors.fieldError("email") ? "email-error" : undefined
                    }
                    className={authInputClass(
                      Boolean(formErrors.fieldError("email")),
                    )}
                  />
                  <AuthFieldError
                    id="email-error"
                    message={formErrors.fieldError("email")}
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
                      aria-invalid={Boolean(formErrors.fieldError("password"))}
                      aria-describedby={
                        formErrors.fieldError("password")
                          ? "password-error"
                          : undefined
                      }
                      className={`${authInputClass(Boolean(formErrors.fieldError("password")))} pr-10`}
                    />
                    <PasswordToggleButton
                      showPassword={showPassword}
                      onToggle={() => setShowPassword(!showPassword)}
                    />
                  </div>
                  <AuthFieldError
                    id="password-error"
                    message={formErrors.fieldError("password")}
                  />
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
                  <MotoConceptIcon concept="passkeys" size={16} />
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
                if (isSchoolPortalHandoffPath(path)) {
                  window.location.href = schoolPortalLoginUrl(
                    `?from=staff&tenant=${encodeURIComponent(tenantSlug)}`,
                  );
                  return;
                }
                router.push(path);
              }}
            />
          )}
      </AuthShell>

      {/* Password Reset Modal */}
      <TenantPasswordResetModal
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
          eyebrowClassName="text-moto-green"
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

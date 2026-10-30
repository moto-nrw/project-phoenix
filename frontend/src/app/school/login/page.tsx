"use client";

// Login des Schul-Portals ("moto schule", #2207). Spiegelt den Aufbau des
// Tenant-Logins (MFA-Kette inklusive Enrollment), aber ohne Tenant-Kontext,
// Passkeys und Eltern-Weiche: der Backend-Login pinnt selbst die Schule, an
// der das Konto eine Schul-Portal-Rolle hält.

import { useEffect, useLayoutEffect, useRef, useState, Suspense } from "react";
import { signOut, useSession } from "next-auth/react";
import { useSearchParams } from "next/navigation";
import {
  AuthFieldError,
  AuthShell,
  MotoBrand,
  authPrimaryButtonClassName,
  authTextButtonClassName,
} from "~/components/auth/auth-shell";
import {
  credentialError,
  LOGIN_REFUSALS,
} from "~/components/auth/credential-error";
import { MFAChallengeForm } from "~/components/auth/mfa-challenge-form";
import { MFAEnrollmentScreen } from "~/components/auth/mfa-enrollment-screen";
import { PasswordResetModal } from "~/components/ui/password-reset-modal";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import { Alert } from "~/components/ui/alert";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { FieldLabel } from "~/components/ui/field-label";
import { Input } from "~/components/ui/input";
import { useApiFormError } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { portalNotBookedExplanation } from "~/lib/portal-not-booked";
import { hasSessionAccess } from "~/lib/session-access";
import { requestSchoolPasswordReset } from "~/lib/auth-api";
import { schoolPath } from "~/lib/school-url";
import { DELIBERATE_LOGOUT_KEY } from "~/lib/session-cache";
import { login as loginApi } from "~/lib/mfa-api";
import { errorStatus } from "~/lib/expected-failure";
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
  // Ein Hinweis, kein Fehler: die Sitzung ist abgelaufen (aus der URL).
  const [sessionExpired, setSessionExpired] = useState(false);
  // Fehler laufen über den gemeinsamen Weg: Katalogtext je Code im
  // Fehlerkasten des Formulars, Feldfehler am Feld.
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearFormError } = formErrors;
  const latestSubmitRef = useRef<() => void>(() => undefined);
  const resetFormRef = useRef<HTMLFormElement>(null);
  const resetErrors = useApiFormError(resetFormRef);
  const [noPortalRole, setNoPortalRole] = useState(false);
  const [portalNotBooked, setPortalNotBooked] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [checkingAuth, setCheckingAuth] = useState(true);
  const [showPassword, setShowPassword] = useState(false);
  const [awaitingRedirect, setAwaitingRedirect] = useState(false);
  const [isResetModalOpen, setIsResetModalOpen] = useState(false);
  const [mfaStep, setMfaStep] = useState<MFAStep | null>(null);
  const [enrollmentStep, setEnrollmentStep] =
    useState<MFAEnrollmentStep | null>(null);
  const searchParams = useSearchParams();
  const targetTenantSlug = searchParams.get("tenant") ?? undefined;
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

      // The layout's server snapshot carries no token (#4991) and redirects
      // too; a stripped session (empty token) stays on the form.
      if (
        status === "authenticated" &&
        hasSessionAccess(session) &&
        !enrollmentStep
      ) {
        window.location.href = schoolPath("/school");
        return;
      }

      if (status !== "loading") {
        setCheckingAuth(false);
      }
    };

    void checkAndRedirect();
  }, [status, session, enrollmentStep]);

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
        setSessionExpired(true);
      }
      clearSessionErrorFromUrl();
    }
  }, [searchParams]);

  const isCheckingAuth = checkingAuth || status === "loading";
  const isSubmitting = isLoading || awaitingRedirect;

  const completeLogin = async () => {
    clearSessionErrorFromUrl();
    setAwaitingRedirect(true);
    window.location.href = schoolPath("/school");
  };

  const handleMFASuccess = async () => {
    await completeLogin();
    setMfaStep(null);
  };

  const submit = async () => {
    setIsLoading(true);
    formErrors.clear();
    setSessionExpired(false);
    setNoPortalRole(false);
    setPortalNotBooked(false);

    try {
      const response = await loginApi("school", {
        email,
        password,
        tenantSlug: targetTenantSlug,
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
        setEnrollmentStep({
          enrollmentToken: response.access_token,
          email,
        });
        return;
      }

      await completeLogin();
    } catch (err) {
      // Konto ohne Schul-Portal-Rolle: das Passwort war korrekt, die
      // Ablehnung kommt von der Rolle — die spezifische Meldung verrät
      // also nichts über fremde Konten und erspart Passwort-Reset-Schleifen.
      if (
        err instanceof ApiError &&
        err.code === "school.no_school_portal_role"
      ) {
        setNoPortalRole(true);
        return;
      }
      // Die Schule nutzt moto schule nicht (#4895): erklärt, kein Fehler.
      if (err instanceof ApiError && err.code === "module.not_booked") {
        setPortalNotBooked(true);
        return;
      }

      // Ein falsches Passwort, ein ausgeschaltetes Konto oder zu viele
      // Versuche haben eigene Katalogtexte (#3376). Vor der Anmeldung heißt
      // 401 "abgelehnt", nicht "Sitzung abgelaufen".
      void formErrors.show(credentialError(err, LOGIN_REFUSALS), {
        object: "die Anmeldung",
        retry: () => latestSubmitRef.current(),
      });
      logger.error("school login failed", {
        error: err instanceof Error ? err.message : String(err),
        status: errorStatus(err),
      });
    } finally {
      setIsLoading(false);
    }
  };

  // „Wiederholen“ sendet den Stand, der dann im Formular steht.
  useLayoutEffect(() => {
    latestSubmitRef.current = () => void submit();
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    void submit();
  };

  return (
    <>
      <AuthShell
        eyebrow="Schulportal"
        eyebrowClassName="text-moto-blue"
        title="Willkommen im Schulportal"
        subtitle="Melden Sie sich mit Ihrem Konto für moto schule an."
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
                clearFormError();
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
                clearFormError();
                setPassword("");
              }}
              onComplete={async () => {
                setEnrollmentStep(null);
                await completeLogin();
              }}
            />
          ) : (
            <form
              ref={formRef}
              onSubmit={handleSubmit}
              noValidate
              className="space-y-6"
            >
              {portalNotBooked ? (
                <Alert
                  type="info"
                  message={portalNotBookedExplanation("school", "de")}
                />
              ) : noPortalRole ? (
                <Alert
                  type="info"
                  message="Dieses Konto hat keinen Zugang zum Schul-Portal. Bitte wenden Sie sich an Ihre OGS-Verwaltung."
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
                  <Input
                    id="email"
                    name="email"
                    type="email"
                    label="E-Mail-Adresse"
                    controlSize="compact"
                    data-testid="input-email"
                    autoComplete="username"
                    required
                    disabled={isSubmitting}
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    error={formErrors.fieldError("email")}
                  />
                </div>

                <div className="text-left">
                  <FieldLabel
                    htmlFor="password"
                    required
                    invalid={Boolean(formErrors.fieldError("password"))}
                  >
                    Passwort
                  </FieldLabel>
                  <div className="relative">
                    <Input
                      id="password"
                      name="password"
                      type={showPassword ? "text" : "password"}
                      controlSize="compact"
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
                      className="pr-10"
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

                <div className="text-center">
                  <button
                    type="button"
                    disabled={isSubmitting}
                    onClick={() => setIsResetModalOpen(true)}
                    className={authTextButtonClassName}
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
        errorPath={resetErrors}
        formRef={resetFormRef}
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
          eyebrowClassName="text-moto-blue"
          title="Willkommen im Schulportal"
          subtitle="Melden Sie sich mit Ihrem Konto für moto schule an."
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

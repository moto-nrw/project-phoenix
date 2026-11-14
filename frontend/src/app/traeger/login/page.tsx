"use client";

// Login des Träger-Portals ("moto träger", #4649). Spiegelt den Aufbau des
// Schul-Logins mit einem Zwischenschritt: Wer zu mehreren Trägern gehört,
// wählt den Träger aus. Danach ist der zweite Faktor immer Pflicht (#4651):
// ein Code per E-Mail, beim ersten Mal mit Einrichtung. Ein Gerät merkt sich
// das Träger-Portal nicht. „Passwort vergessen?“ schickt den Link auf den
// Träger-Host (#5149).

import {
  Suspense,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";
import { signOut, useSession } from "next-auth/react";
import { useSearchParams } from "next/navigation";
import { AuthShell, MotoBrand } from "~/components/auth/auth-shell";
import {
  credentialError,
  LOGIN_REFUSALS,
} from "~/components/auth/credential-error";
import { MFAChallengeForm } from "~/components/auth/mfa-challenge-form";
import { MFAEnrollmentScreen } from "~/components/auth/mfa-enrollment-screen";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { FieldLabel } from "~/components/ui/field-label";
import { Input } from "~/components/ui/input";
import { Loading } from "~/components/ui/loading";
import { PasswordResetModal } from "~/components/auth/password-reset-modal";
import { Radio } from "~/components/ui/radio";
import { useApiFormError } from "~/contexts/ToastContext";
import { requestTraegerPasswordReset } from "~/lib/auth-api";
import { hasSessionAccess } from "~/lib/session-access";
import { errorStatus } from "~/lib/expected-failure";
import { createLogger } from "~/lib/logger";
import {
  clearSessionErrorFromUrl,
  useSessionExpiredNotice,
} from "~/lib/login-session-notice";
import { endTraegerSession } from "~/lib/shell-auth-context";
import {
  clearTraegerSignInWithCode,
  isTraegerSignInWithCodeRequested,
  requestTraegerSignInWithCode,
  serverTraegerSignInWithCode,
  subscribeTraegerSignInWithCode,
} from "~/lib/traeger-sign-in-with-code";
import { traegerLogin, type TraegerChoice } from "~/lib/traeger-api";
import {
  TRAEGER_FORGOT_PASSWORD_PARAM,
  TRAEGER_SIGN_IN_WITH_CODE_PARAM,
} from "~/lib/traeger-login-params";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerLoginPage" });

const SHELL_COPY = {
  title: "Willkommen im Träger-Portal",
  subtitle: "Melden Sie sich mit Ihrem moto-Konto an.",
} as const;

interface MFAStep {
  readonly challengeToken: string;
  readonly maskedEmail: string;
  readonly organizationId: string;
  readonly trustedDeviceEnabled: boolean;
  readonly trustedDeviceDays?: number;
}

interface EnrollmentStep {
  readonly enrollmentToken: string;
  readonly organizationId: string;
}

function LoginForm() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  // A school asked for a sign-in with code (#6955): the next sign-in in
  // this tab ignores the remembered device. Read, never consumed, while
  // rendering; the server renders no hint.
  const withCode = useSyncExternalStore(
    subscribeTraegerSignInWithCode,
    isTraegerSignInWithCodeRequested,
    serverTraegerSignInWithCode,
  );
  const [isLoading, setIsLoading] = useState(false);
  const [awaitingRedirect, setAwaitingRedirect] = useState(false);
  const [choices, setChoices] = useState<readonly TraegerChoice[] | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [mfaStep, setMfaStep] = useState<MFAStep | null>(null);
  const [enrollmentStep, setEnrollmentStep] = useState<EnrollmentStep | null>(
    null,
  );
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const [isResetModalOpen, setIsResetModalOpen] = useState(false);
  const resetFormRef = useRef<HTMLFormElement>(null);
  const resetErrors = useApiFormError(resetFormRef);
  // Ending a session a sign-in with code is to replace (#6955).
  const endErrors = useApiFormError();
  const [endingSession, setEndingSession] = useState(false);
  const latestSubmitRef = useRef<() => void>(() => undefined);
  const isCleaningSessionRef = useRef(false);
  const searchParams = useSearchParams();
  const [sessionExpired, setSessionExpired] =
    useSessionExpiredNotice(searchParams);
  const { data: session, status } = useSession();

  // A school tab sends the person here to sign in with code (#6955): the
  // request holds for this tab, and the address forgets it. Declared before
  // the session effect below, which reads it in the same commit.
  useEffect(() => {
    if (searchParams.get(TRAEGER_SIGN_IN_WITH_CODE_PARAM) !== "1") return;
    requestTraegerSignInWithCode();
    const url = new URL(window.location.href);
    url.searchParams.delete(TRAEGER_SIGN_IN_WITH_CODE_PARAM);
    window.history.replaceState(
      {},
      "",
      `${url.pathname}${url.search}${url.hash}`,
    );
  }, [searchParams]);

  // An existing Träger session goes straight to the school list; a session
  // the backend no longer refreshes is cleared so a new login is not
  // shadowed by it. A session a sign-in with code is to replace stays until
  // the person ends it with the button below: a page visit alone never
  // signs out (#6955). The request is read from storage here, since the
  // effect above may have set it in this same commit.
  useEffect(() => {
    if (status !== "authenticated") return;
    if (session?.error && !isCleaningSessionRef.current) {
      isCleaningSessionRef.current = true;
      void signOut({ redirect: false }).catch((err: unknown) => {
        isCleaningSessionRef.current = false;
        logger.warn("signout_during_cleanup_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
      return;
    }
    if (hasSessionAccess(session) && isTraegerSignInWithCodeRequested()) {
      return;
    }
    // The layout's server snapshot carries no token (#4991) and redirects
    // too; a stripped session (empty token) stays on the form.
    if (
      !enrollmentStep &&
      hasSessionAccess(session) &&
      session.user.scope === "org"
    ) {
      window.location.href = traegerPath("/traeger");
    }
  }, [status, session, enrollmentStep]);

  // A reset link without token leads here to ask for a new link: the dialog
  // opens at once, and the address forgets the request.
  useEffect(() => {
    if (searchParams.get(TRAEGER_FORGOT_PASSWORD_PARAM) !== "1") return;
    setIsResetModalOpen(true);
    const url = new URL(window.location.href);
    url.searchParams.delete(TRAEGER_FORGOT_PASSWORD_PARAM);
    window.history.replaceState(
      {},
      "",
      `${url.pathname}${url.search}${url.hash}`,
    );
  }, [searchParams]);

  const isSubmitting = isLoading || awaitingRedirect;

  const completeLogin = async () => {
    // The sign-in is complete: the next one may use the device again.
    clearTraegerSignInWithCode();
    clearSessionErrorFromUrl();
    setAwaitingRedirect(true);
    window.location.href = traegerPath("/traeger");
  };

  const submit = async () => {
    formErrors.clear();
    // An empty field is named before anything is sent: „prüfen Sie Ihre
    // Angaben“ would not say what is missing.
    if (!choices) {
      const missing: Record<string, string> = {};
      if (email.trim() === "")
        missing.email = "Bitte geben Sie Ihre E-Mail-Adresse ein.";
      if (password === "")
        missing.password = "Bitte geben Sie Ihr Passwort ein.";
      const texts = Object.values(missing);
      if (texts.length > 0) {
        formErrors.invalid(
          texts.length > 1
            ? "Bitte geben Sie E-Mail-Adresse und Passwort ein."
            : (texts[0] ?? ""),
          missing,
        );
        return;
      }
    }
    setIsLoading(true);
    setSessionExpired(false);
    try {
      const ignoreTrustedDevice = isTraegerSignInWithCodeRequested();
      const result = await traegerLogin({
        email,
        password,
        organizationId: choices ? (selectedId ?? undefined) : undefined,
        ...(ignoreTrustedDevice && { ignoreTrustedDevice: true }),
      });
      if (result.status === "organization_selection_required") {
        setChoices(result.organizations);
        setSelectedId(result.organizations[0]?.id ?? null);
        return;
      }
      if (result.status === "mfa_required") {
        setMfaStep({
          challengeToken: result.challengeToken,
          maskedEmail: result.maskedEmail,
          organizationId: result.organizationId,
          trustedDeviceEnabled: result.trustedDeviceEnabled,
          trustedDeviceDays: result.trustedDeviceDays,
        });
        return;
      }
      if (result.status === "mfa_enrollment_required") {
        setEnrollmentStep({
          enrollmentToken: result.enrollmentToken,
          organizationId: result.organizationId,
        });
        return;
      }
      await completeLogin();
    } catch (err) {
      // Ein falsches Passwort und ein ausgeschaltetes Konto haben eigene
      // Katalogtexte; vor der Anmeldung heißt 401 "abgelehnt". Ein Konto
      // ohne Zugang zum Träger-Portal kommt als traeger.no_membership.
      void formErrors.show(credentialError(err, LOGIN_REFUSALS), {
        object: "die Anmeldung",
        retry: () => latestSubmitRef.current(),
      });
      logger.error("traeger login failed", {
        error: err instanceof Error ? err.message : String(err),
        status: errorStatus(err),
      });
    } finally {
      setIsLoading(false);
    }
  };

  useLayoutEffect(() => {
    latestSubmitRef.current = () => void submit();
  });

  const backToCredentials = () => {
    setChoices(null);
    setSelectedId(null);
    setMfaStep(null);
    setEnrollmentStep(null);
    setPassword("");
    formErrors.clear();
  };

  if (status === "loading") {
    return (
      <AuthShell {...SHELL_COPY} variant="tenant" brand={<MotoBrand />}>
        <Loading message="Sitzung wird überprüft …" />
      </AuthShell>
    );
  }

  // A school asked for a sign-in with code while a Träger session is still
  // open (#6955). Only the person ends it: the button runs the portal's
  // logout (revoke, clear, barrier) and opens this page again without it.
  if (withCode && hasSessionAccess(session) && session.user.scope === "org") {
    const endSession = async () => {
      setEndingSession(true);
      endErrors.clear();
      try {
        await endTraegerSession();
      } catch (err) {
        setEndingSession(false);
        void endErrors.show(err, {
          object: "die Abmeldung",
          retry: () => void endSession(),
        });
      }
    };
    return (
      <AuthShell {...SHELL_COPY} variant="tenant" brand={<MotoBrand />}>
        <div className="space-y-6">
          {endErrors.error ? (
            <FormErrorAlert message={endErrors.error} />
          ) : null}
          <Alert
            type="info"
            message="Die Schule verlangt eine Anmeldung mit Code. Melden Sie sich dafür zuerst ab."
          />
          <Button
            type="button"
            variant="primary"
            size="field"
            className="w-full"
            isLoading={endingSession}
            loadingText="Sie werden abgemeldet …"
            tooltip="Meldet Sie ab. Danach melden Sie sich mit dem Code aus Ihrer E-Mail an."
            onClick={() => void endSession()}
          >
            Abmelden und mit Code anmelden
          </Button>
        </div>
      </AuthShell>
    );
  }

  if (mfaStep) {
    return (
      <AuthShell {...SHELL_COPY} variant="tenant" brand={<MotoBrand />}>
        <MFAChallengeForm
          scope="traeger"
          challengeToken={mfaStep.challengeToken}
          maskedEmail={mfaStep.maskedEmail}
          organizationId={mfaStep.organizationId}
          trustedDeviceEnabled={mfaStep.trustedDeviceEnabled}
          trustedDeviceDays={mfaStep.trustedDeviceDays}
          onSuccess={completeLogin}
          onCancel={backToCredentials}
        />
      </AuthShell>
    );
  }

  if (enrollmentStep) {
    return (
      <AuthShell {...SHELL_COPY} variant="tenant" brand={<MotoBrand />}>
        <MFAEnrollmentScreen
          scope="traeger"
          bearerToken={enrollmentStep.enrollmentToken}
          userEmail={email}
          organizationId={enrollmentStep.organizationId}
          onExit={backToCredentials}
          onComplete={completeLogin}
        />
      </AuthShell>
    );
  }

  return (
    <>
      <AuthShell {...SHELL_COPY} variant="tenant" brand={<MotoBrand />}>
        <form
          ref={formRef}
          noValidate
          className="space-y-6"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          {formErrors.error ? (
            <FormErrorAlert message={formErrors.error} />
          ) : sessionExpired ? (
            <Alert
              type="info"
              message="Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an."
            />
          ) : withCode ? (
            <Alert
              type="info"
              message="Die Schule verlangt eine Anmeldung mit Code. Geben Sie nach dem Passwort den Code aus Ihrer E-Mail ein."
            />
          ) : null}

          {choices ? (
            <fieldset className="space-y-3 border-0 p-0">
              <legend className="mb-2 text-sm font-medium text-gray-700">
                Sie gehören zu mehreren Trägern. Welchen Träger möchten Sie
                öffnen?
              </legend>
              {choices.map((choice) => (
                <ChoiceTile
                  key={choice.id}
                  tone="green"
                  selected={selectedId === choice.id}
                  className="p-3"
                >
                  <Radio
                    name="organization"
                    value={choice.id}
                    checked={selectedId === choice.id}
                    disabled={isSubmitting}
                    onChange={() => setSelectedId(choice.id)}
                  />
                  <span>{choice.name}</span>
                </ChoiceTile>
              ))}
            </fieldset>
          ) : (
            <div className="space-y-4">
              <div className="text-left">
                <Input
                  id="email"
                  name="email"
                  type="email"
                  label="E-Mail-Adresse"
                  controlSize="touch"
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
                <Input
                  id="password"
                  name="password"
                  type="password"
                  controlSize="touch"
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
                  revealable
                />
                <FieldErrorHint
                  id="password-error"
                  message={formErrors.fieldError("password")}
                />
              </div>

              <div className="text-center">
                <Button
                  type="button"
                  disabled={isSubmitting}
                  onClick={() => setIsResetModalOpen(true)}
                  variant="link"
                  size="md"
                >
                  Passwort vergessen?
                </Button>
              </div>
            </div>
          )}

          <div className="mt-2 space-y-3">
            <Button
              type="submit"
              disabled={isSubmitting || (choices !== null && !selectedId)}
              variant="primary"
              size="field"
              className="w-full"
            >
              <span className="relative z-10">
                {isSubmitting
                  ? "Anmeldung läuft …"
                  : choices
                    ? "Träger öffnen"
                    : "Anmelden"}
              </span>
            </Button>
            {choices ? (
              <Button
                type="button"
                variant="ghost"
                size="field"
                disabled={isSubmitting}
                onClick={backToCredentials}
                className="w-full"
              >
                Mit einem anderen Konto anmelden
              </Button>
            ) : null}
          </div>
        </form>
      </AuthShell>

      <PasswordResetModal
        isOpen={isResetModalOpen}
        onClose={() => setIsResetModalOpen(false)}
        onRequestReset={requestTraegerPasswordReset}
        inputControlSize="compact"
        errorPath={resetErrors}
        formRef={resetFormRef}
      />
    </>
  );
}

export default function TraegerLoginPage() {
  return (
    <Suspense
      fallback={
        <AuthShell {...SHELL_COPY} variant="tenant" brand={<MotoBrand />}>
          <Loading message="Laden …" />
        </AuthShell>
      }
    >
      <LoginForm />
    </Suspense>
  );
}

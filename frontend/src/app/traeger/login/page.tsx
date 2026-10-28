"use client";

// Login des Träger-Portals ("moto träger", #4649). Spiegelt den Aufbau des
// Schul-Logins mit einem Zwischenschritt: Wer zu mehreren Trägern gehört,
// wählt den Träger aus. Danach ist der zweite Faktor immer Pflicht (#4651):
// ein Code per E-Mail, beim ersten Mal mit Einrichtung. Ein Gerät merkt sich
// das Träger-Portal nicht.

import { Suspense, useEffect, useLayoutEffect, useRef, useState } from "react";
import { signIn, signOut, useSession } from "next-auth/react";
import { useSearchParams } from "next/navigation";
import {
  AuthFieldError,
  AuthShell,
  MotoBrand,
  authPrimaryButtonClassName,
} from "~/components/auth/auth-shell";
import {
  credentialError,
  LOGIN_REFUSALS,
} from "~/components/auth/credential-error";
import { MFAChallengeForm } from "~/components/auth/mfa-challenge-form";
import { MFAEnrollmentScreen } from "~/components/auth/mfa-enrollment-screen";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { FieldLabel } from "~/components/ui/field-label";
import { Input } from "~/components/ui/input";
import { Loading } from "~/components/ui/loading";
import { Radio } from "~/components/ui/radio";
import { useApiFormError } from "~/contexts/ToastContext";
import { hasSessionAccess } from "~/lib/session-access";
import { errorStatus } from "~/lib/expected-failure";
import { createLogger } from "~/lib/logger";
import type { MFATokenResponse } from "~/lib/mfa-api";
import { DELIBERATE_LOGOUT_KEY } from "~/lib/session-cache";
import { traegerLogin, type TraegerChoice } from "~/lib/traeger-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerLoginPage" });

const SHELL_COPY = {
  eyebrow: "Träger-Portal",
  // Akzentfarbe wie die Eyebrow des Schul-Logins (#5145).
  eyebrowClassName: "text-moto-blue",
  title: "Willkommen im Träger-Portal",
  subtitle: "Melden Sie sich mit Ihrem moto-Konto an.",
} as const;

function clearSessionErrorFromUrl() {
  const url = new URL(window.location.href);
  const error = url.searchParams.get("error");
  if (error !== "SessionRequired" && error !== "SessionExpired") return;
  url.searchParams.delete("error");
  url.searchParams.delete("callbackUrl");
  window.history.replaceState(
    {},
    "",
    `${url.pathname}${url.search}${url.hash}`,
  );
}

interface MFAStep {
  readonly challengeToken: string;
  readonly maskedEmail: string;
  readonly organizationId: string;
}

interface EnrollmentStep {
  readonly enrollmentToken: string;
  readonly organizationId: string;
}

function LoginForm() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [sessionExpired, setSessionExpired] = useState(false);
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
  const latestSubmitRef = useRef<() => void>(() => undefined);
  const isCleaningSessionRef = useRef(false);
  const searchParams = useSearchParams();
  const { data: session, status } = useSession();

  // An existing Träger session goes straight to the school list; a session
  // the backend no longer refreshes is cleared so a new login is not
  // shadowed by it.
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
    // The layout's server snapshot carries no token (#4991) and redirects
    // too; a stripped session (empty token) stays on the form.
    if (hasSessionAccess(session) && session.user.scope === "org") {
      window.location.href = traegerPath("/traeger");
    }
  }, [status, session]);

  useEffect(() => {
    const urlError = searchParams.get("error");
    if (urlError !== "SessionRequired" && urlError !== "SessionExpired") {
      return;
    }
    let deliberate = false;
    try {
      deliberate = sessionStorage.getItem(DELIBERATE_LOGOUT_KEY) === "1";
      sessionStorage.removeItem(DELIBERATE_LOGOUT_KEY);
    } catch {
      // sessionStorage unavailable: show the note, it does no harm.
    }
    if (!deliberate) setSessionExpired(true);
    clearSessionErrorFromUrl();
  }, [searchParams]);

  const isSubmitting = isLoading || awaitingRedirect;

  // Übergibt ein fertiges Token-Paar an NextAuth: nach dem Code oder nach
  // der Einrichtung des zweiten Faktors.
  const seedSessionWithTokens = async (tokens: MFATokenResponse) => {
    clearSessionErrorFromUrl();
    const seeded = await signIn("traeger-credentials", {
      redirect: false,
      internalRefresh: "true",
      token: tokens.access_token,
      refreshToken: tokens.refresh_token,
    });
    if (seeded?.error) {
      logger.error("session_seed_failed", { error: seeded.error });
      // Zurück zum Formular, damit der Fehlerkasten sichtbar ist.
      setMfaStep(null);
      setEnrollmentStep(null);
      // Kein ApiError: der Weg zeigt den allgemeinen Text der Anmeldung.
      void formErrors.show(new Error(seeded.error), {
        object: "die Anmeldung",
      });
      return;
    }
    setAwaitingRedirect(true);
    window.location.href = traegerPath("/traeger");
  };

  const submit = async () => {
    setIsLoading(true);
    formErrors.clear();
    setSessionExpired(false);
    try {
      const result = await traegerLogin({
        email,
        password,
        organizationId: choices ? (selectedId ?? undefined) : undefined,
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
      await seedSessionWithTokens({
        access_token: result.accessToken,
        refresh_token: result.refreshToken,
      });
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
        <Loading fullPage={false} message="Sitzung wird überprüft …" />
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
          trustedDeviceEnabled={false}
          onSuccess={seedSessionWithTokens}
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
          onComplete={seedSessionWithTokens}
        />
      </AuthShell>
    );
  }

  return (
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
          </div>
        )}

        <div className="mt-2 space-y-3">
          <button
            type="submit"
            disabled={isSubmitting || (choices !== null && !selectedId)}
            className={authPrimaryButtonClassName}
          >
            <span className="relative z-10">
              {isSubmitting
                ? "Anmeldung läuft …"
                : choices
                  ? "Träger öffnen"
                  : "Anmelden"}
            </span>
          </button>
          {choices ? (
            <Button
              type="button"
              variant="ghost"
              size="compact"
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
  );
}

export default function TraegerLoginPage() {
  return (
    <Suspense
      fallback={
        <AuthShell {...SHELL_COPY} variant="tenant" brand={<MotoBrand />}>
          <Loading fullPage={false} message="Laden …" />
        </AuthShell>
      }
    >
      <LoginForm />
    </Suspense>
  );
}

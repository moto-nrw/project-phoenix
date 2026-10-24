"use client";

// Login des Träger-Portals ("moto träger", #4649). Spiegelt den Aufbau des
// Schul-Logins, aber ohne zweiten Faktor (folgt mit #4651) und mit einem
// Zwischenschritt: Wer zu mehreren Trägern gehört, wählt den Träger aus.

import { Suspense, useEffect, useLayoutEffect, useRef, useState } from "react";
import { signIn, signOut, useSession } from "next-auth/react";
import { useSearchParams } from "next/navigation";
import {
  AuthFieldError,
  AuthShell,
  MotoBrand,
  authInputClass,
  authPrimaryButtonClassName,
} from "~/components/auth/auth-shell";
import {
  credentialError,
  LOGIN_REFUSALS,
} from "~/components/auth/credential-error";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import { Alert } from "~/components/ui/alert";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Radio } from "~/components/ui/radio";
import { useApiFormError } from "~/contexts/ToastContext";
import { errorStatus } from "~/lib/expected-failure";
import { createLogger } from "~/lib/logger";
import { DELIBERATE_LOGOUT_KEY } from "~/lib/session-cache";
import { traegerLogin, type TraegerChoice } from "~/lib/traeger-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerLoginPage" });

const SHELL_COPY = {
  eyebrow: "Träger-Portal",
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

function Spinner({ label }: { readonly label: string }) {
  return (
    <div className="flex flex-col items-center gap-4 py-10">
      <div className="h-10 w-10 animate-spin rounded-full border-2 border-gray-200 border-t-gray-950" />
      <p className="text-sm text-gray-500">{label}</p>
    </div>
  );
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
    if (session?.user?.token && session.user.scope === "org") {
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
      clearSessionErrorFromUrl();
      const seeded = await signIn("traeger-credentials", {
        redirect: false,
        internalRefresh: "true",
        token: result.accessToken,
        refreshToken: result.refreshToken,
      });
      if (seeded?.error) {
        logger.error("session_seed_failed", { error: seeded.error });
        // Kein ApiError: der Weg zeigt den allgemeinen Text der Anmeldung.
        void formErrors.show(new Error(seeded.error), {
          object: "die Anmeldung",
        });
        return;
      }
      setAwaitingRedirect(true);
      window.location.href = traegerPath("/traeger");
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
    setPassword("");
    formErrors.clear();
  };

  if (status === "loading") {
    return (
      <AuthShell {...SHELL_COPY} variant="tenant" brand={<MotoBrand />}>
        <Spinner label="Sitzung wird überprüft …" />
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
            <button
              type="button"
              disabled={isSubmitting}
              onClick={backToCredentials}
              className="w-full text-sm text-gray-600 transition-colors hover:text-gray-800 hover:underline focus:underline focus:outline-none disabled:cursor-not-allowed disabled:text-gray-400"
            >
              Mit einem anderen Konto anmelden
            </button>
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
          <Spinner label="Laden …" />
        </AuthShell>
      }
    >
      <LoginForm />
    </Suspense>
  );
}

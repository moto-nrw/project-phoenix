"use client";

// Login of the caterer portal (ADR 0077). Like the school login without the
// second factor: the backend checks e-mail and password, refuses every other
// kind of account like a wrong password, and the BFF installs the session.

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
import { PasswordResetModal } from "~/components/ui/password-reset-modal";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import { Alert } from "~/components/ui/alert";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { FieldLabel } from "~/components/ui/field-label";
import { Input } from "~/components/ui/input";
import { useApiFormError } from "~/contexts/ToastContext";
import { hasSessionAccess } from "~/lib/session-access";
import { requestCatererPasswordReset } from "~/lib/auth-api";
import { loginCaterer } from "~/lib/caterer-api";
import { catererPath } from "~/lib/caterer-url";
import { DELIBERATE_LOGOUT_KEY } from "~/lib/session-cache";
import { errorStatus } from "~/lib/expected-failure";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CatererLoginPage" });

// Wie die anderen Anmeldungen: der Portalname klein darüber, die Begrüßung
// als Titel.
const SHELL_COPY = {
  eyebrow: "Caterer-Portal",
  eyebrowClassName: "text-moto-blue",
  title: "Willkommen im Caterer-Portal",
  subtitle: "Melden Sie sich mit Ihrem Caterer-Konto an.",
} as const;

function sessionErrorInUrl(): boolean {
  const url = new URL(window.location.href);
  const error = url.searchParams.get("error");
  if (error !== "SessionRequired" && error !== "SessionExpired") return false;
  url.searchParams.delete("error");
  url.searchParams.delete("callbackUrl");
  window.history.replaceState(
    {},
    "",
    `${url.pathname}${url.search}${url.hash}`,
  );
  return true;
}

function LoginForm() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [sessionExpired, setSessionExpired] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [checkingAuth, setCheckingAuth] = useState(true);
  const [awaitingRedirect, setAwaitingRedirect] = useState(false);
  const [isResetModalOpen, setIsResetModalOpen] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const resetFormRef = useRef<HTMLFormElement>(null);
  const resetErrors = useApiFormError(resetFormRef);
  const latestSubmitRef = useRef<() => void>(() => undefined);
  const searchParams = useSearchParams();
  const { data: session, status } = useSession();
  const cleaningRef = useRef(false);

  useEffect(() => {
    const check = async () => {
      // A refresh the backend refused leaves a stale cookie behind.
      if (
        status === "authenticated" &&
        session?.error &&
        !cleaningRef.current
      ) {
        cleaningRef.current = true;
        try {
          await signOut({ redirect: false });
        } catch (cause) {
          cleaningRef.current = false;
          logger.warn("signout_during_cleanup_failed", {
            error: cause instanceof Error ? cause.message : String(cause),
          });
        }
        setCheckingAuth(false);
        return;
      }
      // The layout's server snapshot carries no token (#4991) and redirects
      // too; a stripped session (empty token) stays on the form.
      if (status === "authenticated" && hasSessionAccess(session)) {
        window.location.href = catererPath("/caterer");
        return;
      }
      if (status !== "loading") setCheckingAuth(false);
    };
    void check();
  }, [status, session]);

  useEffect(() => {
    const error = searchParams.get("error");
    if (error !== "SessionRequired" && error !== "SessionExpired") return;
    let deliberate = false;
    try {
      deliberate = sessionStorage.getItem(DELIBERATE_LOGOUT_KEY) === "1";
      sessionStorage.removeItem(DELIBERATE_LOGOUT_KEY);
    } catch {
      // sessionStorage unavailable: show the notice.
    }
    setSessionExpired(!deliberate);
    sessionErrorInUrl();
  }, [searchParams]);

  const isCheckingAuth = checkingAuth || status === "loading";
  const isSubmitting = isLoading || awaitingRedirect;

  const submit = async () => {
    setIsLoading(true);
    formErrors.clear();
    setSessionExpired(false);
    try {
      await loginCaterer(email, password);
      setAwaitingRedirect(true);
      window.location.href = catererPath("/caterer");
    } catch (cause) {
      // A wrong password, a switched-off account and too many tries have
      // texts of their own; before the login a 401 means "refused".
      void formErrors.show(credentialError(cause, LOGIN_REFUSALS), {
        object: "die Anmeldung",
        retry: () => latestSubmitRef.current(),
      });
      logger.error("caterer_login_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
        status: errorStatus(cause),
      });
    } finally {
      setIsLoading(false);
    }
  };

  useLayoutEffect(() => {
    latestSubmitRef.current = () => void submit();
  });

  const emailError = formErrors.fieldError("email");
  const passwordError = formErrors.fieldError("password");

  return (
    <>
      <AuthShell {...SHELL_COPY} variant="tenant" brand={<MotoBrand />}>
        {isCheckingAuth ? (
          <div className="flex flex-col items-center gap-4 py-12">
            <div className="h-10 w-10 animate-spin rounded-full border-2 border-gray-200 border-t-gray-950" />
            <p className="text-sm text-gray-500">Anmeldung wird geprüft …</p>
          </div>
        ) : (
          <form
            ref={formRef}
            onSubmit={(event) => {
              event.preventDefault();
              void submit();
            }}
            noValidate
            className="space-y-6"
          >
            {formErrors.error ? (
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
                  autoComplete="username"
                  required
                  disabled={isSubmitting}
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  error={emailError}
                />
              </div>

              <div className="text-left">
                <FieldLabel
                  htmlFor="password"
                  required
                  invalid={Boolean(passwordError)}
                >
                  Passwort
                </FieldLabel>
                <div className="relative">
                  <Input
                    id="password"
                    name="password"
                    type={showPassword ? "text" : "password"}
                    controlSize="compact"
                    autoComplete="current-password"
                    required
                    disabled={isSubmitting}
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                    aria-invalid={Boolean(passwordError)}
                    aria-describedby={
                      passwordError ? "password-error" : undefined
                    }
                    className="pr-10"
                  />
                  <PasswordToggleButton
                    showPassword={showPassword}
                    onToggle={() => setShowPassword(!showPassword)}
                  />
                </div>
                <AuthFieldError id="password-error" message={passwordError} />
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
                  {isSubmitting ? "Anmeldung läuft …" : "Anmelden"}
                </span>
              </button>
            </div>
          </form>
        )}
      </AuthShell>

      <PasswordResetModal
        isOpen={isResetModalOpen}
        onClose={() => setIsResetModalOpen(false)}
        onRequestReset={requestCatererPasswordReset}
        errorPath={resetErrors}
        formRef={resetFormRef}
      />
    </>
  );
}

export default function CatererLoginPage() {
  return (
    <Suspense
      fallback={
        <AuthShell {...SHELL_COPY} variant="tenant" brand={<MotoBrand />}>
          <div className="flex flex-col items-center gap-4 py-8">
            <div className="h-10 w-10 animate-spin rounded-full border-2 border-gray-200 border-t-gray-950" />
            <p className="text-sm text-gray-500">Wird geladen …</p>
          </div>
        </AuthShell>
      }
    >
      <LoginForm />
    </Suspense>
  );
}

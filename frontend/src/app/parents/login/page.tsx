"use client";

import {
  Suspense,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
// eslint-disable-next-line no-restricted-imports -- parent routes are not tenant-scoped
import { redirect, useSearchParams } from "next/navigation";
import { signOut, useSession } from "next-auth/react";
import { useLocale, useTranslations } from "next-intl";
import { Alert } from "~/components/ui/alert";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { FieldLabel } from "~/components/ui/field-label";
import { Input } from "~/components/ui/input";
import {
  AuthShell,
  AuthShellSkeleton,
  MotoBrand,
} from "~/components/auth/auth-shell";
import { buildParentAuthShellCopy } from "~/components/auth/parent-auth-shell-copy";
import { PasswordResetModal } from "~/components/auth/password-reset-modal";
import { LanguageSwitcher } from "~/components/parent/language-switcher";
import { useApiFormError } from "~/contexts/ToastContext";
import { ApiError, unavailableApiError } from "~/lib/api-error";
import { credentialError } from "~/components/auth/credential-error";
import {
  MFAChallengeForm,
  type MFAChallengeCopy,
} from "~/components/auth/mfa-challenge-form";
import { portalPasswordLogin } from "~/lib/portal-password-login";
import { createLogger } from "~/lib/logger";
import { requestParentPasswordReset } from "~/lib/auth-api";
import { parentPath } from "~/lib/parent-url";
import { holidayRegistrationReturnPath } from "~/lib/parent-holiday-registration";
import { portalNotBookedExplanation } from "~/lib/portal-not-booked";
import { normalizeLocale } from "~/i18n/locales";
import { clientEnv } from "~/env.client";
import { Button } from "~/components/ui/button";

const logger = createLogger({ component: "ParentLoginPage" });

/**
 * Absolute URL der Schul-Anmeldung. Anders als das Elternportal liegt das
 * Personal-Portal pro Schule auf einer eigenen Subdomain, und welche Schule
 * gemeint ist, weiss das Elternportal nicht. Deshalb zeigt der Link auf die
 * Einrichtungs-Auswahl unter der blanken Domain (src/app/page.tsx).
 *
 * NEXT_PUBLIC_TENANT_DOMAIN traegt keinen Port, der kommt aus dem aktuellen
 * Fenster — dieselbe Konstruktion wie in src/app/page.tsx:52-58.
 */
function staffLoginUrl(): string {
  const portSuffix = window.location.port ? `:${window.location.port}` : "";
  return `${window.location.protocol}//${clientEnv.NEXT_PUBLIC_TENANT_DOMAIN}${portSuffix}/`;
}

/**
 * The 401 refusals of the parents' password and code steps: a wrong password
 * names the care code the page has always shown (portalauth ParentLoginRules).
 */
const PARENT_LOGIN_REFUSALS = [
  "care.invalid_credentials",
  "identity.invalid_credentials",
  "identity.session_account_inactive",
] as const;

type ParentSignInTranslate = ReturnType<typeof useTranslations<"parentSignIn">>;

/** The code step in the parent's language (#7051). */
function challengeCopy(t: ParentSignInTranslate): MFAChallengeCopy {
  return {
    back: t("challenge.back"),
    backLabel: t("challenge.backLabel"),
    title: t("challenge.title"),
    sentTo: (email) =>
      t.rich("challenge.sentTo", {
        email: "",
        b: () => <span className="font-medium text-gray-800">{email}</span>,
      }),
    codeLabel: t("challenge.codeLabel"),
    digitLabel: (position) => t("challenge.digitLabel", { position }),
    checking: t("challenge.checking"),
    autoCheck: t("challenge.autoCheck"),
    question: t("challenge.question"),
    remember: t("challenge.remember"),
    rememberHint: (days) => t("challenge.rememberHint", { days }),
    once: t("challenge.once"),
    onceHint: t("challenge.onceHint"),
    enterCodeFirst: t("challenge.enterCodeFirst"),
    removeDevicesHint: t("challenge.removeDevicesHint"),
    resend: t("challenge.resend"),
    resendIn: (seconds) => t("challenge.resendIn", { seconds }),
    resending: t("challenge.resending"),
    verifyObject: t("challenge.verifyObject"),
    resendObject: t("challenge.resendObject"),
  };
}

interface MFAStep {
  readonly challengeToken: string;
  readonly maskedEmail: string;
  readonly trustedDeviceEnabled: boolean;
  readonly trustedDeviceDays?: number;
}

export default function ParentLoginPage() {
  // useSearchParams verlangt eine Suspense-Grenze (siehe frontend/CLAUDE.md).
  return (
    <Suspense fallback={<AuthShellSkeleton />}>
      <ParentLoginForm />
    </Suspense>
  );
}

function ParentLoginForm() {
  const t = useTranslations("parentLogin");
  const tErrorCatalog = useTranslations("errorCatalog");
  const tAuthShell = useTranslations("parentAuthShell");
  const tReset = useTranslations("parentPasswordResetModal");
  const tSignIn = useTranslations("parentSignIn");
  const mfaCopy = useMemo(() => challengeCopy(tSignIn), [tSignIn]);
  // Where the account has the code by e-mail switched on (#7051), the
  // password leads to this second step.
  const [mfaStep, setMfaStep] = useState<MFAStep | null>(null);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  // Fehler laufen über den gemeinsamen Weg (#2518): Katalogtext je Code im
  // Fehlerkasten des Formulars, in der Sprache der Eltern.
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const latestSubmitRef = useRef<() => void>(() => undefined);
  const resetFormRef = useRef<HTMLFormElement>(null);
  const resetErrors = useApiFormError(resetFormRef);
  // Ein Personal-Konto: kein Fehler der Eingabe, sondern der falsche Ort. Der
  // Hinweis bringt den Link zur Schul-Anmeldung mit.
  const [isStaffAccount, setIsStaffAccount] = useState(false);
  // Keine Schule des Kontos nutzt das Elternportal (#4895): kein Fehler der
  // Eingabe, die Schule entscheidet.
  const [portalNotBooked, setPortalNotBooked] = useState(false);
  const locale = normalizeLocale(useLocale());
  const [isLoading, setIsLoading] = useState(false);
  const [isResetModalOpen, setIsResetModalOpen] = useState(false);
  // Gesetzt, wenn die Personal-Anmeldung ein Elternkonto hierher geschickt hat.
  const searchParams = useSearchParams();
  const cameFromStaffLogin = searchParams.get("from") === "staff";
  const sessionExpired = searchParams.get("error") === "SessionExpired";
  // Die Ferienanmeldung schickt Eltern mit Konto hierher und will sie nach
  // dem Login wieder im Formular haben (#6671). Nur dieses eine Ziel gilt.
  const returnPath = holidayRegistrationReturnPath(searchParams.get("next"));
  // Erst nach dem Mount, weil die URL window.location.port braucht und die
  // Seite serverseitig vorgerendert wird.
  const [staffUrl, setStaffUrl] = useState<string | null>(null);
  useEffect(() => setStaffUrl(staffLoginUrl()), []);
  // Gesetzt, sobald die BFF die Sitzung installiert hat: Die Seite wechselt
  // in die App und bleibt bis dahin gesperrt.
  const [awaitingRedirect, setAwaitingRedirect] = useState(false);
  const { data: session, status } = useSession();
  // Ref prevents re-triggering signOut (not in effect deps → no loop).
  // Separate state controls the loading spinner for the UI.
  const cleanupStartedRef = useRef(false);
  const [cleanupAttempt, setCleanupAttempt] = useState(0);
  const [isCleaningUp, setIsCleaningUp] = useState(false);
  const { error: formError, show: showFormError } = formErrors;
  const hasError = Boolean(formError) || isStaffAccount || portalNotBooked;
  const testimonialPanelCopy = useMemo(
    () => buildParentAuthShellCopy(tAuthShell),
    [tAuthShell],
  );
  // A session the callback stripped (no usable access token) carries an
  // error and stays on the form. No browser session carries a token
  // (#4990, #5005); the server checks the cookie, so a signed-in parent
  // opening /login is still sent to the portal.
  const isRedirectingToParent =
    status === "authenticated" &&
    session?.user?.scope === "parent" &&
    session.error === undefined;
  const isSessionSettling =
    status === "loading" ||
    isCleaningUp ||
    isRedirectingToParent ||
    (status === "authenticated" && session?.error !== undefined);

  // Redirect if already authenticated as parent, or clear stale sessions.
  useEffect(() => {
    const check = async () => {
      if (
        status === "authenticated" &&
        session?.error &&
        !cleanupStartedRef.current
      ) {
        cleanupStartedRef.current = true;
        setIsCleaningUp(true);
        try {
          await signOut({ redirect: false });
        } catch (err) {
          cleanupStartedRef.current = false;
          logger.warn("parent_login_stale_session_cleanup_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          // The stale session keeps the form locked: say so, and let
          // Wiederholen run the cleanup again.
          void showFormError(unavailableApiError(err), {
            object: t("errorObject"),
            retry: () => setCleanupAttempt((current) => current + 1),
          });
        }
        setIsCleaningUp(false);
        return;
      }
    };
    void check();
  }, [status, session, cleanupAttempt, showFormError, t]);

  // Ein Elternteil, das bereits angemeldet ist und /login direkt aufruft,
  // geht in die App. Nach einer Anmeldung hier lädt completeLogin die App
  // neu; die Weiterleitung greift dann nicht zusätzlich (#7051).
  if (isRedirectingToParent && !awaitingRedirect) {
    redirect(returnPath ?? parentPath("/parents"));
  }

  // Die BFF hat die Sitzung installiert: nach dem Passwort oder dem Code.
  const completeLogin = () => {
    setAwaitingRedirect(true);
    window.location.href = returnPath ?? parentPath("/parents");
  };

  const submit = async () => {
    if (isSessionSettling) return;
    setIsLoading(true);
    formErrors.clear();
    setIsStaffAccount(false);
    setPortalNotBooked(false);

    try {
      const result = await portalPasswordLogin("parent", email, password);
      if (result.status === "mfa_required") {
        setMfaStep({
          challengeToken: result.challengeToken,
          maskedEmail: result.maskedEmail,
          trustedDeviceEnabled: result.trustedDeviceEnabled,
          trustedDeviceDays: result.trustedDeviceDays,
        });
        setIsLoading(false);
        return;
      }
      // isLoading bleibt stehen: Das Formular bleibt bis zur Navigation
      // gesperrt, damit niemand ein zweites Mal absendet.
      completeLogin();
    } catch (err) {
      setIsLoading(false);
      // The 403 follows a correct password check, so naming a staff
      // account tells a stranger nothing.
      if (err instanceof ApiError && err.code === "care.not_a_guardian") {
        setIsStaffAccount(true);
        return;
      }
      if (err instanceof ApiError && err.code === "module.not_booked") {
        setPortalNotBooked(true);
        return;
      }
      if (err instanceof ApiError && err.status === 429) {
        formErrors.invalid(t("errors.rateLimited"));
        return;
      }
      void formErrors.show(
        err instanceof ApiError
          ? credentialError(err, PARENT_LOGIN_REFUSALS)
          : unavailableApiError(err),
        {
          object: t("errorObject"),
          retry: () => latestSubmitRef.current(),
        },
      );
    }
  };

  const backToCredentials = () => {
    setMfaStep(null);
    setPassword("");
    formErrors.clear();
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    void submit();
  };

  // „Wiederholen“ sendet, was dann in den Feldern steht.
  useLayoutEffect(() => {
    latestSubmitRef.current = () => void submit();
  });

  return (
    <>
      <AuthShell
        eyebrow={t("eyebrow")}
        title={t("title")}
        subtitle={t("subtitle")}
        variant="parents"
        brand={<MotoBrand />}
        footer={<LanguageSwitcher />}
        testimonialPanelCopy={testimonialPanelCopy}
      >
        {mfaStep ? (
          <MFAChallengeForm
            scope="parent"
            copy={mfaCopy}
            challengeToken={mfaStep.challengeToken}
            maskedEmail={mfaStep.maskedEmail}
            trustedDeviceEnabled={mfaStep.trustedDeviceEnabled}
            trustedDeviceDays={mfaStep.trustedDeviceDays}
            onSuccess={completeLogin}
            onCancel={backToCredentials}
          />
        ) : (
          <form
            ref={formRef}
            onSubmit={handleSubmit}
            noValidate
            className="space-y-6"
            aria-busy={isSessionSettling || undefined}
          >
            {cameFromStaffLogin && !hasError && (
              <Alert type="info" message={t("fromStaffBanner")} />
            )}
            {sessionExpired && !hasError && (
              <Alert type="info" message={tErrorCatalog("loginNotice")} />
            )}

            {portalNotBooked ? (
              <Alert
                type="info"
                message={portalNotBookedExplanation("parent", locale)}
              />
            ) : isStaffAccount ? (
              <Alert
                type="info"
                message={t("errors.notAGuardian")}
                action={
                  staffUrl ? (
                    <a
                      href={staffUrl}
                      className="font-medium whitespace-nowrap underline underline-offset-2"
                    >
                      {t("staffLoginLink")}
                    </a>
                  ) : undefined
                }
              />
            ) : (
              <FormErrorAlert message={formErrors.error} />
            )}

            <div className="space-y-4">
              <div className="text-left">
                <Input
                  id="parent-email"
                  name="email"
                  type="email"
                  label={t("emailLabel")}
                  controlSize="compact"
                  autoComplete="username"
                  required
                  disabled={isSessionSettling}
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
              </div>

              <div className="text-left">
                <FieldLabel htmlFor="parent-password" required>
                  {t("passwordLabel")}
                </FieldLabel>
                <Input
                  id="parent-password"
                  name="password"
                  type="password"
                  controlSize="compact"
                  autoComplete="current-password"
                  required
                  disabled={isSessionSettling}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  revealable
                  showPasswordLabel={t("showPassword")}
                  hidePasswordLabel={t("hidePassword")}
                />
              </div>

              <div className="text-center">
                <Button
                  type="button"
                  variant="link"
                  size="md"
                  disabled={isLoading || isSessionSettling}
                  onClick={() => setIsResetModalOpen(true)}
                >
                  {t("forgotPassword")}
                </Button>
              </div>
            </div>

            <Button
              type="submit"
              disabled={isLoading || isSessionSettling}
              variant="primary"
              size="field"
              className="w-full"
            >
              <span className="relative z-10">
                {isLoading ? t("submitting") : t("submit")}
              </span>
            </Button>
          </form>
        )}
      </AuthShell>

      <PasswordResetModal
        isOpen={isResetModalOpen}
        onClose={() => setIsResetModalOpen(false)}
        onRequestReset={requestParentPasswordReset}
        rateLimitStorageKey="parentPasswordResetRateLimitUntil"
        errorPath={resetErrors}
        formRef={resetFormRef}
        copy={{
          title: tReset("title"),
          description: tReset("description"),
          emailLabel: tReset("emailLabel"),
          invalidEmail: tReset("invalidEmail"),
          cancel: tReset("cancel"),
          submit: tReset("submit"),
          submitting: tReset("submitting"),
          successTitle: tReset("successTitle"),
          successMessage: tReset("successMessage"),
          successHint: tReset("successHint"),
          close: tReset("close"),
          errorObject: tReset("errorObject"),
          rateLimitWait: (countdown) => tReset("rateLimitWait", { countdown }),
        }}
      />
    </>
  );
}

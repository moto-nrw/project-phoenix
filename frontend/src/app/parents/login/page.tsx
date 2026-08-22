"use client";

import { Suspense, useEffect, useMemo, useRef, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- parent routes are not tenant-scoped
import { redirect, useSearchParams } from "next/navigation";
import { signIn, signOut, useSession } from "next-auth/react";
import { useTranslations } from "next-intl";
import { Alert } from "~/components/ui/alert";
import {
  AuthShell,
  AuthShellSkeleton,
  MotoBrand,
  authInputClassName,
  authPrimaryButtonClassName,
} from "~/components/auth/auth-shell";
import { buildParentAuthShellCopy } from "~/components/auth/parent-auth-shell-copy";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import { PasswordResetModal } from "~/components/ui/password-reset-modal";
import { LanguageSwitcher } from "~/components/parent/language-switcher";
import { requestParentPasswordReset } from "~/lib/auth-api";
import { parentPath } from "~/lib/parent-url";
import { env } from "~/env";

/**
 * Wie lange nach einem erfolgreichen signIn auf die publizierte Session
 * gewartet wird, bevor das Formular wieder freigegeben wird. Der Abruf geht an
 * die eigene Route (/api/parent/auth/session), zehn Sekunden sind also
 * reichlich und trotzdem kurz genug, dass niemand vor einem toten Screen sitzt.
 */
const SESSION_HANDOFF_TIMEOUT_MS = 10_000;

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
  return `${window.location.protocol}//${env.NEXT_PUBLIC_TENANT_DOMAIN}${portSuffix}/`;
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
  const tAuthShell = useTranslations("parentAuthShell");
  const tReset = useTranslations("parentPasswordResetModal");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  // Steuert nur, ob der Fehler den Link zur Schul-Anmeldung mitbekommt.
  const [isStaffAccount, setIsStaffAccount] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [isResetModalOpen, setIsResetModalOpen] = useState(false);
  // Gesetzt, wenn die Personal-Anmeldung ein Elternkonto hierher geschickt hat.
  const searchParams = useSearchParams();
  const cameFromStaffLogin = searchParams.get("from") === "staff";
  // Erst nach dem Mount, weil die URL window.location.port braucht und die
  // Seite serverseitig vorgerendert wird.
  const [staffUrl, setStaffUrl] = useState<string | null>(null);
  useEffect(() => setStaffUrl(staffLoginUrl()), []);
  // True zwischen erfolgreichem signIn und der publizierten Session. Steuert
  // den Watchdog weiter unten.
  const [awaitingSession, setAwaitingSession] = useState(false);
  const { data: session, status } = useSession();
  // Ref prevents re-triggering signOut (not in effect deps → no loop).
  // Separate state controls the loading spinner for the UI.
  const cleanupStartedRef = useRef(false);
  const [isCleaningUp, setIsCleaningUp] = useState(false);
  const testimonialPanelCopy = useMemo(
    () => buildParentAuthShellCopy(tAuthShell),
    [tAuthShell],
  );
  const isRedirectingToParent =
    status === "authenticated" &&
    session?.user?.scope === "parent" &&
    Boolean(session.user.token) &&
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
        } catch {
          cleanupStartedRef.current = false;
        }
        setIsCleaningUp(false);
        return;
      }
    };
    void check();
  }, [status, session]);

  // Watchdog für die Übergabe von signIn an die Session. signIn kann ok
  // melden, ohne dass danach je eine Session ankommt: NextAuth holt sie per
  // fetchData(), und das schluckt JEDEN Fetch-Fehler und liefert null
  // (next-auth/lib/client.js). Ein einzelner wackliger GET auf
  // /api/parent/auth/session lässt status damit auf "unauthenticated" stehen,
  // der redirect() unten feuert nie. Ohne diesen Watchdog bliebe das Formular
  // mit "Anmeldung läuft..." dauerhaft gesperrt und ohne Fehlermeldung
  // zurück, und nur ein manueller Reload käme da wieder raus.
  useEffect(() => {
    if (!awaitingSession) return;
    const timer = setTimeout(() => {
      setAwaitingSession(false);
      setIsLoading(false);
      setError(t("errors.generic"));
    }, SESSION_HANDOFF_TIMEOUT_MS);
    return () => clearTimeout(timer);
  }, [awaitingSession, t]);

  // Einzige Weiterleitung nach erfolgreichem Login. Sie greift, sobald NextAuth
  // die neue Session veröffentlicht — auch für den Fall "bereits angemeldet,
  // ruft /login direkt auf". handleSubmit darf daher NICHT zusätzlich
  // router.push() feuern: zwei gleichzeitige Navigationen auf dasselbe Ziel
  // lassen den App-Router-State als pending Thenable stehen, und das
  // bedingte `use(state)` in Next' useActionQueue rendert dann eine
  // unterschiedliche Anzahl Hooks ("Rendered more hooks than during the
  // previous render", Seite bricht ab, erst ein Reload heilt es).
  if (isRedirectingToParent) {
    redirect(parentPath("/parents"));
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (isSessionSettling) return;
    setIsLoading(true);
    setError("");
    setIsStaffAccount(false);

    try {
      const result = await signIn("parent-credentials", {
        redirect: false,
        email,
        password,
      });

      if (result?.error) {
        const errorMessages: Record<string, string> = {
          account_inactive: t("errors.accountInactive"),
          rate_limited: t("errors.rateLimited"),
          invalid_credentials: t("errors.invalidCredentials"),
          // ErrAccountNoGuardianRole: Personal-Konto im Elternportal. Der
          // Backend-Code kommt seit parent-config.ts unmaskiert an — der
          // 403 setzt eine korrekte Passwortpruefung voraus, verraet also
          // nichts ueber ein fremdes Konto.
          not_a_guardian: t("errors.notAGuardian"),
        };
        setError(errorMessages[result.code ?? ""] ?? t("errors.invalid"));
        setIsStaffAccount(result.code === "not_a_guardian");
        setIsLoading(false);
        return;
      }

      // Kein router.push hier — die Weiterleitung oben übernimmt. isLoading
      // bleibt absichtlich stehen, damit das Formular bis zur Navigation
      // gesperrt bleibt und niemand ein zweites Mal absendet; der Watchdog
      // gibt es wieder frei, falls die Session ausbleibt.
      setAwaitingSession(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("errors.generic"));
      setIsLoading(false);
    }
  };

  return (
    <>
      <AuthShell
        eyebrow={t("eyebrow")}
        eyebrowClassName="text-moto-green"
        title={t("title")}
        subtitle={t("subtitle")}
        variant="parents"
        brand={<MotoBrand />}
        footer={<LanguageSwitcher />}
        testimonialPanelCopy={testimonialPanelCopy}
      >
        <form
          onSubmit={handleSubmit}
          noValidate
          className="space-y-6"
          aria-busy={isSessionSettling || undefined}
        >
          {cameFromStaffLogin && !error && (
            <Alert type="info" message={t("fromStaffBanner")} />
          )}

          {error && (
            <Alert
              type="error"
              message={error}
              action={
                isStaffAccount && staffUrl ? (
                  <a
                    href={staffUrl}
                    className="font-medium whitespace-nowrap underline underline-offset-2"
                  >
                    {t("staffLoginLink")}
                  </a>
                ) : undefined
              }
            />
          )}

          <div className="space-y-4">
            <div className="text-left">
              <label
                htmlFor="parent-email"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                {t("emailLabel")}
              </label>
              <input
                id="parent-email"
                name="email"
                type="email"
                autoComplete="username"
                required
                disabled={isSessionSettling}
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className={authInputClassName}
              />
            </div>

            <div className="text-left">
              <label
                htmlFor="parent-password"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                {t("passwordLabel")}
              </label>
              <div className="relative">
                <input
                  id="parent-password"
                  name="password"
                  type={showPassword ? "text" : "password"}
                  autoComplete="current-password"
                  required
                  disabled={isSessionSettling}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className={`${authInputClassName} pr-10`}
                />
                <PasswordToggleButton
                  showPassword={showPassword}
                  onToggle={() => setShowPassword(!showPassword)}
                  showLabel={t("showPassword")}
                  hideLabel={t("hidePassword")}
                />
              </div>
            </div>

            <div className="text-center">
              <button
                type="button"
                disabled={isLoading || isSessionSettling}
                onClick={() => setIsResetModalOpen(true)}
                className="text-sm text-gray-600 transition-colors hover:text-gray-800 hover:underline focus:underline focus:outline-none disabled:cursor-not-allowed disabled:text-gray-400"
              >
                {t("forgotPassword")}
              </button>
            </div>
          </div>

          <button
            type="submit"
            disabled={isLoading || isSessionSettling}
            className={authPrimaryButtonClassName}
          >
            <span className="relative z-10">
              {isLoading ? t("submitting") : t("submit")}
            </span>
          </button>
        </form>
      </AuthShell>

      <PasswordResetModal
        isOpen={isResetModalOpen}
        onClose={() => setIsResetModalOpen(false)}
        onRequestReset={requestParentPasswordReset}
        rateLimitStorageKey="parentPasswordResetRateLimitUntil"
        copy={{
          title: tReset("title"),
          description: tReset("description"),
          emailLabel: tReset("emailLabel"),
          cancel: tReset("cancel"),
          submit: tReset("submit"),
          submitting: tReset("submitting"),
          successTitle: tReset("successTitle"),
          successMessage: tReset("successMessage"),
          successHint: tReset("successHint"),
          close: tReset("close"),
          rateLimitError: (countdown) =>
            tReset("errors.rateLimited", { countdown }),
          genericError: tReset("errors.generic"),
        }}
      />
    </>
  );
}

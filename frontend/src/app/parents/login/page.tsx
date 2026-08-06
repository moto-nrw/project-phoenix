"use client";

import { useEffect, useMemo, useRef, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- parent routes are not tenant-scoped
import { redirect } from "next/navigation";
import { signIn, signOut, useSession } from "next-auth/react";
import { useTranslations } from "next-intl";
import { Alert } from "~/components/ui/alert";
import {
  AuthShell,
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

export default function ParentLoginPage() {
  const t = useTranslations("parentLogin");
  const tAuthShell = useTranslations("parentAuthShell");
  const tReset = useTranslations("parentPasswordResetModal");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [isResetModalOpen, setIsResetModalOpen] = useState(false);
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
          // ErrAccountNoGuardianRole: backend sends 403 which CredentialsProvider
          // surfaces here with code "invalid_credentials" by default. A future
          // refinement could plumb a separate code; for now the German copy
          // covers both "wrong password" and "not a parent" cases without
          // leaking which one applies (account-enumeration mask).
          invalid_credentials: t("errors.invalidCredentials"),
        };
        setError(errorMessages[result.code ?? ""] ?? t("errors.invalid"));
        setIsLoading(false);
        return;
      }

      // Kein router.push hier — die Weiterleitung oben übernimmt. isLoading
      // bleibt absichtlich stehen, damit das Formular bis zur Navigation
      // gesperrt bleibt und niemand ein zweites Mal absendet.
    } catch (err) {
      setError(err instanceof Error ? err.message : t("errors.generic"));
      setIsLoading(false);
    }
  };

  return (
    <>
      <AuthShell
        eyebrow={t("eyebrow")}
        eyebrowClassName="text-[#83CD2D]"
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
          {error && <Alert type="error" message={error} />}

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

// [tenant]/page.tsx, tenant-scoped login page
"use client";

import { useState, useEffect, useRef, Suspense } from "react";
import { signIn, signOut, useSession } from "next-auth/react";
import { useSearchParams } from "next/navigation";
import Image from "next/image";
import { Alert } from "~/components/ui";
import { refreshToken } from "~/lib/auth-api";
import { SmartRedirect } from "~/components/auth/smart-redirect";
import {
  AuthShell,
  authInputClassName,
  authPrimaryButtonClassName,
} from "~/components/auth/auth-shell";
import { PasswordResetModal } from "~/components/ui/password-reset-modal";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import { useTenant } from "~/components/tenant/tenant-provider";
import { loginImageSrc } from "~/lib/tenant-api";
import { useTenantRouter } from "~/lib/tenant-router";
import { DELIBERATE_LOGOUT_KEY } from "~/lib/session-cache";

import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "TenantLoginPage" });
function LoginForm() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [checkingAuth, setCheckingAuth] = useState(true);
  const [showPassword, setShowPassword] = useState(false);
  const [awaitingRedirect, setAwaitingRedirect] = useState(false);
  const [isResetModalOpen, setIsResetModalOpen] = useState(false);
  const router = useTenantRouter();
  const { tenantSlug, tenant } = useTenant();
  const searchParams = useSearchParams();
  const { data: session, status } = useSession();
  const loginTitle = tenant?.name?.trim() ? tenant.name : "Willkommen bei moto";

  // Guard against calling signOut multiple times during stale session cleanup
  const isCleaningSessionRef = useRef(false);

  // Check for valid session
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
        window.history.replaceState({}, "", window.location.pathname);
      } else {
        setError(
          "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.",
        );
      }
    }
  }, [searchParams]);

  const isCheckingAuth = checkingAuth || status === "loading";
  const isSubmitting = isLoading || awaitingRedirect;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError("");

    try {
      const result = await signIn("credentials", {
        email,
        password,
        tenantSlug,
        redirect: false,
      });

      if (result?.error) {
        setError("Ungültige E-Mail oder Passwort");
      } else {
        // Set flag to indicate we're awaiting redirect
        setAwaitingRedirect(true);
        // Refresh the router to update session state
        router.refresh();
      }
    } catch (error) {
      setError("Anmeldefehler. Bitte versuchen Sie es erneut.");
      logger.error("login failed", {
        error: error instanceof Error ? error.message : String(error),
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
        brand={
          tenant?.settings?.loginImageUrl ? (
            <Image
              src={loginImageSrc(tenant.settings.loginImageUrl)}
              alt={`${tenant.name} Logo`}
              width={180}
              height={104}
              className="max-h-[104px] w-auto object-contain"
              priority
              unoptimized
            />
          ) : null
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
          </form>
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
          brand={null}
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

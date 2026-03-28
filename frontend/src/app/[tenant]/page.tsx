// [tenant]/page.tsx — Login page (tenant-scoped)
"use client";

import { useState, useEffect, useRef, Suspense } from "react";
import { signIn, signOut, useSession } from "next-auth/react";
import { useSearchParams } from "next/navigation";
import Image from "next/image";
import { Input, Alert } from "~/components/ui";
import { refreshToken } from "~/lib/auth-api";
import { SmartRedirect } from "~/components/auth/smart-redirect";
import { PasswordResetModal } from "~/components/ui/password-reset-modal";
import { launchConfetti, clearConfetti } from "~/lib/confetti";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import { useTenant } from "~/components/tenant/tenant-provider";
import { useTenantRouter } from "~/lib/tenant-router";
import { env } from "~/env";
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

  // Check for session errors in URL — but suppress after a deliberate logout.
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

  const isCheckingAuth =
    checkingAuth ||
    status === "loading" ||
    (awaitingRedirect && status === "authenticated");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError("");

    try {
      // Start confetti immediately when button is clicked
      // This creates a perception of instant response
      launchConfetti();

      const result = await signIn("credentials", {
        email,
        password,
        tenantSlug,
        redirect: false,
      });

      if (result?.error) {
        clearConfetti();
        setError("Ungültige E-Mail oder Passwort");
      } else {
        // Set flag to indicate we're awaiting redirect
        setAwaitingRedirect(true);
        // Refresh the router to update session state
        router.refresh();
      }
    } catch (error) {
      clearConfetti();
      setError("Anmeldefehler. Bitte versuchen Sie es erneut.");
      logger.error("login failed", {
        error: error instanceof Error ? error.message : String(error),
      });
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen flex-col items-center justify-center p-4">
      <div className="mx-auto w-full max-w-2xl rounded-2xl bg-white/80 p-10 text-center shadow-xl backdrop-blur-md transition-all duration-300 hover:bg-white/90 hover:shadow-2xl">
        {/* Top Bar: Tenant Selector (left) + Help (right) */}
        <div className="absolute top-4 right-4 left-4 flex items-center justify-between">
          <button
            type="button"
            onClick={() => {
              const tenantDomain = env.NEXT_PUBLIC_TENANT_DOMAIN;
              const portSuffix = window.location.port
                ? `:${window.location.port}`
                : "";
              window.location.href = `${window.location.protocol}//${tenantDomain}${portSuffix}/`;
            }}
            className="flex items-center gap-1.5 text-sm text-gray-500 transition-colors hover:text-gray-800"
          >
            <svg
              className="h-4 w-4"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M15 19l-7-7 7-7"
              />
            </svg>
            Einrichtung wechseln
          </button>
        </div>

        {/* Logo Section */}
        <div className="mb-8 flex justify-center">
          <Image
            src="/images/moto_transparent.png"
            alt="MOTO Logo"
            width={200}
            height={80}
            priority
          />
        </div>

        {/* Welcome Text */}
        <h1 className="mb-2 bg-gradient-to-r from-[#5080d8] to-[#83cd2d] bg-clip-text text-4xl font-bold text-transparent md:text-5xl">
          Willkommen bei moto!
        </h1>
        <p className="text-xl text-gray-700">Ganztag. Digital.</p>
        {tenant?.name && (
          <p className="mt-4 mb-10 text-2xl font-semibold text-gray-800">
            {tenant.name}
          </p>
        )}
        {!tenant?.name && <div className="mb-10" />}

        {/* Loading spinner while checking auth or awaiting redirect */}
        {isCheckingAuth && (
          <div className="flex items-center justify-center py-12">
            <div className="flex flex-col items-center gap-4">
              <div className="h-10 w-10 animate-spin rounded-full border-2 border-gray-200 border-t-[#5080D8]" />
              <p className="text-sm text-gray-500">
                {awaitingRedirect
                  ? "Sie werden weitergeleitet…"
                  : "Sitzung wird überprüft…"}
              </p>
            </div>
          </div>
        )}

        {/* Login Form — fades in after auth check */}
        <div
          className={`transition-opacity duration-300 ${isCheckingAuth ? "pointer-events-none hidden" : "opacity-100"}`}
        >
          <form
            onSubmit={handleSubmit}
            onKeyDown={(e) => {
              if (e.key === "Enter" && e.target instanceof HTMLInputElement) {
                e.preventDefault();
                e.currentTarget.requestSubmit();
              }
            }}
            noValidate
            className="space-y-6"
          >
            {error && <Alert type="error" message={error} />}

            <div className="space-y-4">
              <div className="text-left">
                <label
                  htmlFor="email"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  E-Mail-Adresse
                </label>
                <Input
                  id="email"
                  name="email"
                  type="email"
                  autoComplete="username"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="w-full"
                  label={""}
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
                  <Input
                    id="password"
                    name="password"
                    type={showPassword ? "text" : "password"}
                    autoComplete="current-password"
                    required
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className="w-full pr-10"
                    label={""}
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
                  onClick={() => setIsResetModalOpen(true)}
                  className="text-sm text-gray-600 transition-colors hover:text-gray-800 hover:underline focus:underline focus:outline-none"
                >
                  Passwort vergessen?
                </button>
              </div>
            </div>

            <div className="mt-2 flex justify-center">
              <button
                type="submit"
                disabled={isLoading}
                className="group relative overflow-hidden rounded-xl bg-gray-900 px-8 py-2.5 text-sm font-semibold text-white transition-all duration-200 hover:bg-gray-800 focus:ring-2 focus:ring-gray-900 focus:ring-offset-2 focus:outline-none active:scale-95 disabled:cursor-not-allowed disabled:opacity-50"
              >
                <span className="relative z-10">
                  {isLoading ? "Anmeldung läuft..." : "Anmelden"}
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
      </div>

      {/* Password Reset Modal */}
      <PasswordResetModal
        isOpen={isResetModalOpen}
        onClose={() => setIsResetModalOpen(false)}
      />
    </div>
  );
}

export default function HomePage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen flex-col items-center justify-center p-4">
          <div className="flex flex-col items-center gap-4">
            <div className="h-10 w-10 animate-spin rounded-full border-2 border-gray-200 border-t-[#5080D8]" />
            <p className="text-sm text-gray-500">Laden…</p>
          </div>
        </div>
      }
    >
      <LoginForm />
    </Suspense>
  );
}

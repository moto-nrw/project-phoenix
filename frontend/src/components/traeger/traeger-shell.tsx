"use client";

import { usePathname } from "next/navigation";
import { signOut, useSession } from "next-auth/react";
import { useState } from "react";
import { MotoBrand } from "~/components/auth/moto-brand";
import { Button } from "~/components/ui/button";
import { NavLink } from "~/components/ui/nav-link";
import { PortalShell } from "~/components/ui/portal-shell";
import { createLogger } from "~/lib/logger";
import { DELIBERATE_LOGOUT_KEY, clearSessionCache } from "~/lib/session-cache";
import { traegerPath } from "~/lib/traeger-url";
import { traegerHelpHref } from "./traeger-help";

const logger = createLogger({ component: "TraegerShell" });

/** The pages of the Träger portal, as paths of the Träger host. */
const TRAEGER_NAV = [
  { page: "/", label: "Schulen" },
  { page: "/einstellungen", label: "Einstellungen" },
] as const;

const NAV_LINK_CLASS =
  "rounded-md px-2.5 py-1.5 text-sm font-medium focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none";

/**
 * The shell of the Träger portal ("moto träger", #4649): the shared
 * PortalShell frame with a slim head bar. Two pages (schools, settings
 * #4652) need no sidebar; the head carries them, the help and the way out.
 */
export function TraegerShell({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  const { data: session } = useSession();
  // Null outside the app router (tests, stories): count as the school list.
  const pathname = usePathname() ?? "/";
  const currentPage = pathname.replace(/^\/traeger/, "") || "/";
  const [loggingOut, setLoggingOut] = useState(false);

  const logout = async () => {
    setLoggingOut(true);
    try {
      sessionStorage.setItem(DELIBERATE_LOGOUT_KEY, "1");
    } catch {
      // sessionStorage unavailable: the login page then shows its
      // "session expired" note, which is harmless.
    }
    // Revoke the backend refresh session first. Best-effort: logout must
    // never block on it, the local session ends either way.
    try {
      await fetch("/api/traeger/auth/logout", {
        method: "POST",
        signal: AbortSignal.timeout(5000),
      });
    } catch (err: unknown) {
      logger.warn("traeger_backend_logout_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
    }
    clearSessionCache();
    // NextAuth resolves a callbackUrl against NEXTAUTH_URL, the tenant
    // origin, and would send the person to the school chooser there. End the
    // session without its redirect and stay on the Träger host.
    await signOut({ redirect: false });
    window.location.assign(traegerPath("/traeger/login"));
  };

  const header = (
    <header className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 border-b border-gray-200/70 bg-white/95 px-8 py-4 max-md:px-4">
      <div className="flex items-center gap-3">
        <MotoBrand />
        <span className="text-sm font-semibold text-gray-600">Träger</span>
      </div>
      {/* Own row on phones, so four entries fit 320 px. */}
      <nav
        aria-label="Träger-Portal"
        className="flex items-center gap-1 max-md:order-last max-md:w-full"
      >
        {TRAEGER_NAV.map((item) => {
          const active = currentPage === item.page;
          return (
            <NavLink
              key={item.page}
              href={traegerPath(
                `/traeger${item.page === "/" ? "" : item.page}`,
              )}
              aria-current={active ? "page" : undefined}
              className={`${NAV_LINK_CLASS} ${
                active
                  ? "bg-gray-100 text-gray-900"
                  : "text-gray-700 hover:bg-gray-100"
              }`}
            >
              {item.label}
            </NavLink>
          );
        })}
      </nav>
      {/* Wraps below the brand on narrow phones (320 px) instead of overflowing. */}
      <div className="ml-auto flex items-center gap-2">
        {session?.user?.email ? (
          <span className="text-sm text-gray-600 max-md:hidden">
            {session.user.email}
          </span>
        ) : null}
        <NavLink
          href={traegerHelpHref(currentPage)}
          className={`${NAV_LINK_CLASS} text-gray-700 hover:bg-gray-100`}
        >
          Hilfe
        </NavLink>
        <Button
          type="button"
          variant="ghost"
          size="compact"
          isLoading={loggingOut}
          loadingText="Abmelden …"
          onClick={() => void logout()}
        >
          Abmelden
        </Button>
      </div>
    </header>
  );

  return (
    <PortalShell header={header} sidebar={null} bottomNav={null}>
      {children}
    </PortalShell>
  );
}

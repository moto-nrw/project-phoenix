"use client";

import { usePathname, useRouter } from "next/navigation";
import { signOut, useSession } from "next-auth/react";
import { useState } from "react";
import { MotoBrand } from "~/components/auth/moto-brand";
import { Button } from "~/components/ui/button";
import { NavLink } from "~/components/ui/nav-link";
import { NavigationTabs } from "~/components/ui/page-header/NavigationTabs";
import { PortalShell } from "~/components/ui/portal-shell";
import { hasPermission } from "~/lib/auth-utils";
import { createLogger } from "~/lib/logger";
import { DELIBERATE_LOGOUT_KEY, clearSessionCache } from "~/lib/session-cache";
import { traegerPath } from "~/lib/traeger-url";
import { TRAEGER_HELP_HREF } from "./traeger-help";

const logger = createLogger({ component: "TraegerShell" });

const NAV_ITEMS = [
  { id: "schulen", label: "Schulen", path: "/traeger" },
  { id: "ferien", label: "Ferien", path: "/traeger/ferien" },
] as const;

/**
 * The pages of the portal as header tabs. The holiday periods need the right
 * "Anmeldung verwalten"; a person without it has one page and no tabs.
 */
function TraegerNav({
  canManageEnrollment,
}: Readonly<{ canManageEnrollment: boolean }>) {
  const router = useRouter();
  const pathname = usePathname() ?? "";
  if (!canManageEnrollment) return null;
  const onHolidays =
    pathname.startsWith("/ferien") || pathname.startsWith("/traeger/ferien");
  return (
    <NavigationTabs
      items={NAV_ITEMS.map(({ id, label }) => ({ id, label }))}
      activeTab={onHolidays ? "ferien" : "schulen"}
      onTabChange={(id) => {
        const item = NAV_ITEMS.find((candidate) => candidate.id === id);
        if (item) router.push(traegerPath(item.path));
      }}
    />
  );
}

/**
 * The shell of the Träger portal ("moto träger", #4649): the shared
 * PortalShell frame with a slim head bar. The portal has few pages, so there
 * is no sidebar; the head carries the pages, the help and the way out.
 */
export function TraegerShell({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  const { data: session } = useSession();
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
      <div className="flex flex-wrap items-center gap-3">
        <MotoBrand />
        <span className="text-sm font-semibold text-gray-600">Träger</span>
        <TraegerNav
          canManageEnrollment={hasPermission(
            session ?? null,
            "traeger_enrollment:manage",
          )}
        />
      </div>
      {/* Wraps below the brand on narrow phones (320 px) instead of overflowing. */}
      <div className="ml-auto flex items-center gap-2">
        {session?.user?.email ? (
          <span className="text-sm text-gray-600 max-md:hidden">
            {session.user.email}
          </span>
        ) : null}
        <NavLink
          href={TRAEGER_HELP_HREF}
          className="rounded-md px-2.5 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-100 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
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

"use client";

import { redirect, usePathname } from "next/navigation";
import { useSession } from "next-auth/react";
import { Alert } from "~/components/ui/alert";
import { Loading } from "~/components/ui/loading";
import { portalNotBookedExplanation } from "~/lib/portal-not-booked";
import { SchoolShellProvider } from "~/lib/shell-auth-context";
import { BreadcrumbProvider } from "~/lib/breadcrumb-context";
import { SchoolShell } from "~/components/school/shell/school-shell";
import { SchoolRealtimeBridge } from "~/components/school/school-realtime-bridge";
import { ShellIntlProvider } from "~/components/dashboard/shell-intl-provider";
import { PwaInstallHint } from "~/components/tenant/pwa-install-hint";
import { PortalNotificationSetup } from "~/components/notifications/portal-notification-setup";
import { schoolPath } from "~/lib/school-url";

function FullPageLoading() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center p-4">
      <Loading fullPage message="Wird geladen …" />
    </div>
  );
}

/**
 * Client-side auth guard for school routes ("moto schule", #2207).
 * Mirrors ParentAuthGuard — the portal brings its own chrome (SchoolShell:
 * sidebar, shared header with profile menu, mobile nav), fed by
 * SchoolShellProvider.
 *
 * Reads the school session (via SchoolProviders SessionProvider) and
 * redirects non-school or unauthenticated users. Tenant, operator, and
 * parent tokens never reach the school app — host-only cookies + the proxy
 * make them invisible — so the only redirects this guard handles are
 * "session loading" → spinner and "session missing" → /school/login.
 */
const SCHOOL_PUBLIC_PAGES = [
  "/school/login",
  "/login",
  // Accept flow for Lehrkraft invitations — gated by the invitation token.
  "/school/invite",
  "/invite",
  "/school/reset-password",
  "/reset-password",
];

export function SchoolAuthGuard({
  children,
  notBooked = false,
}: {
  readonly children: React.ReactNode;
  /** The page load's session check found the portal's module not booked. */
  readonly notBooked?: boolean;
}) {
  const pathname = usePathname();
  const isPublicPage = SCHOOL_PUBLIC_PAGES.some(
    (p) => pathname === p || pathname.startsWith(`${p}/`),
  );
  const { data: session, status } = useSession();

  if (isPublicPage) {
    return <>{children}</>;
  }

  if (status === "authenticated" && session?.user?.scope !== "school") {
    redirect("/");
  }
  if (status === "unauthenticated") {
    redirect(schoolPath("/school/login"));
  }

  if (status === "loading") {
    return <FullPageLoading />;
  }

  // ShellIntlProvider: die geteilte Kopfzeile ruft
  // useTranslations("parentNav") für die Eltern-Titel auf; das deutschsprachige
  // Schul-Portal bekommt denselben minimalen Katalog wie Personal und Operator.
  return (
    <SchoolShellProvider>
      <BreadcrumbProvider>
        <ShellIntlProvider>
          <SchoolRealtimeBridge />
          <SchoolShell>
            {notBooked ? (
              <Alert
                type="info"
                message={portalNotBookedExplanation("school", "de")}
              />
            ) : (
              children
            )}
          </SchoolShell>
          <PwaInstallHint samsungOnly />
          <PortalNotificationSetup portal="school" />
        </ShellIntlProvider>
      </BreadcrumbProvider>
    </SchoolShellProvider>
  );
}

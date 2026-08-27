"use client";

import { redirect, usePathname } from "next/navigation";
import { useSession } from "next-auth/react";
import { Loading } from "~/components/ui/loading";
import { SchoolShellProvider } from "~/lib/shell-auth-context";
import { BreadcrumbProvider } from "~/lib/breadcrumb-context";
import { SchoolShell } from "~/components/school/shell/school-shell";
import { SchoolRealtimeBridge } from "~/components/school/school-realtime-bridge";
import { ShellNavIntlProvider } from "~/components/dashboard/shell-nav-intl-provider";
import { schoolPath } from "~/lib/school-url";

function FullPageLoading() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center p-4">
      <Loading message="Wird geladen …" />
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
}: {
  readonly children: React.ReactNode;
}) {
  const pathname = usePathname();
  const isPublicPage = SCHOOL_PUBLIC_PAGES.some(
    (p) => pathname === p || pathname.startsWith(`${p}/`),
  );
  const { data: session, status } = useSession();

  // Login + invite pages: render without auth guards.
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

  // ShellNavIntlProvider: die geteilte Kopfzeile ruft
  // useTranslations("parentNav") für die Eltern-Titel auf; das deutschsprachige
  // Schul-Portal bekommt denselben minimalen Katalog wie Personal und Operator.
  return (
    <SchoolShellProvider>
      <BreadcrumbProvider>
        <ShellNavIntlProvider>
          <SchoolRealtimeBridge />
          <SchoolShell>{children}</SchoolShell>
        </ShellNavIntlProvider>
      </BreadcrumbProvider>
    </SchoolShellProvider>
  );
}

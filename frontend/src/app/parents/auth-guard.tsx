"use client";

import { redirect, usePathname } from "next/navigation";
import { useSession } from "next-auth/react";
import { parentPath } from "~/lib/parent-url";
import { ParentShellProvider } from "~/lib/shell-auth-context";
import { BreadcrumbProvider } from "~/lib/breadcrumb-context";
import { ParentShell } from "~/components/parent/shell/parent-shell";
import { ParentPageSkeleton } from "~/components/parent/parent-page";
import { ParentRealtimeBridge } from "~/components/parent/parent-realtime-bridge";
import { ParentNotificationOnboarding } from "~/components/parent/parent-notification-onboarding";

/**
 * Client-side auth guard for parent routes. Mirrors OperatorAuthGuard.
 *
 * Reads the parent session (via ParentProviders SessionProvider) and
 * redirects non-parent or unauthenticated users. Tenant + operator
 * tokens never reach the parent app — host-only cookies + the proxy
 * make them invisible — so the only redirects this guard handles are
 * "session loading" → spinner and "session missing" → /parents/login.
 */
const PARENT_PUBLIC_PAGES = [
  "/invite",
  "/parents/invite",
  "/parents/login",
  "/login",
  "/parents/reset-password",
  "/reset-password",
  "/parents/email-confirm",
  "/email-confirm",
  "/parents/accept-guardian-invite",
  "/accept-guardian-invite",
  // Status page is gated by the random token — same trust model as
  // the tenant version. Email links should land here directly even
  // when the parent isn't logged in.
  "/parents/enroll/status",
  "/enroll/status",
];

export function ParentAuthGuard({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  const pathname = usePathname();
  const isPublicPage = PARENT_PUBLIC_PAGES.some(
    (p) => pathname === p || pathname.startsWith(`${p}/`),
  );
  const { data: session, status } = useSession();

  // Login page: render without auth guards.
  if (isPublicPage) {
    return <>{children}</>;
  }

  if (status === "authenticated" && session?.user?.scope !== "parent") {
    redirect("/");
  }
  if (status === "unauthenticated") {
    redirect(parentPath("/parents/login"));
  }

  const sessionLoading = status === "loading" || !session;

  // Locale handling lives in the single ParentLocaleProvider mounted by
  // ParentProviders; it picks up the now-authenticated session on its own.
  return (
    <ParentShellProvider>
      <BreadcrumbProvider>
        {!sessionLoading ? <ParentRealtimeBridge /> : null}
        <ParentShell>
          {sessionLoading ? <ParentPageSkeleton rows={2} /> : children}
          {!sessionLoading ? (
            <ParentNotificationOnboarding accountId={session.user.id} />
          ) : null}
        </ParentShell>
      </BreadcrumbProvider>
    </ParentShellProvider>
  );
}

"use client";

import { Fragment } from "react";
import { useSWRCacheKey } from "~/lib/swr";
import { operatorProvisioningKeys } from "~/lib/swr/portal-keys";
import { sessionIdentityKey } from "~/lib/session-access";
import { redirect, usePathname } from "next/navigation";
import { useSession } from "next-auth/react";
import { operatorPath } from "~/lib/operator-url";
import { OperatorShellProvider } from "~/lib/shell-auth-context";
import { BreadcrumbProvider } from "~/lib/breadcrumb-context";
import { AppShell } from "~/components/dashboard/app-shell";
import { ShellIntlProvider } from "~/components/dashboard/shell-intl-provider";
import { Loading } from "~/components/ui/loading";

function FullPageLoading() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center p-4">
      <Loading />
    </div>
  );
}

/**
 * Client-side auth guard for operator routes.
 * Reads the operator session (via OperatorProviders SessionProvider)
 * and redirects non-operator or unauthenticated users.
 */
const OPERATOR_PUBLIC_PAGES = [
  "/operator/login",
  "/login",
  "/operator/email-confirm",
  "/email-confirm",
  "/operator/invite",
  "/invite",
];

export function OperatorAuthGuard({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  const pathname = usePathname();
  const isPublicPage = OPERATOR_PUBLIC_PAGES.some(
    (p) => pathname === p || pathname.startsWith(`${p}/`),
  );
  const { data: session, status } = useSession();

  const contentScope = useSWRCacheKey()(
    operatorProvisioningKeys.organizations(),
  );

  // Login page: render without auth guards
  if (isPublicPage) {
    return <>{children}</>;
  }

  if (status === "authenticated" && session?.user?.scope !== "platform") {
    redirect("/");
  }
  if (status === "unauthenticated") {
    redirect(operatorPath("/operator/login"));
  }
  if (status === "authenticated" && session?.error !== undefined) {
    // The login page clears failed sessions and explains how to sign in again.
    redirect(operatorPath("/operator/login?error=SessionExpired"));
  }

  if (status === "loading" || !contentScope) {
    return <FullPageLoading />;
  }

  return (
    <OperatorShellProvider>
      <BreadcrumbProvider>
        <ShellIntlProvider>
          <AppShell>
            {/* Also discard local form/modal state, not only cached responses. */}
            <Fragment key={`${contentScope}:${sessionIdentityKey(session)}`}>
              {children}
            </Fragment>
          </AppShell>
        </ShellIntlProvider>
      </BreadcrumbProvider>
    </OperatorShellProvider>
  );
}

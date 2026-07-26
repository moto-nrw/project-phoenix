"use client";

import { redirect, usePathname } from "next/navigation";
import { useSession } from "next-auth/react";
import { operatorPath } from "~/lib/operator-url";
import { OperatorShellProvider } from "~/lib/shell-auth-context";
import { BreadcrumbProvider } from "~/lib/breadcrumb-context";
import { AppShell } from "~/components/dashboard/app-shell";
import { ShellNavIntlProvider } from "~/components/dashboard/shell-nav-intl-provider";
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

  if (status === "loading") {
    return <FullPageLoading />;
  }

  return (
    <OperatorShellProvider>
      <BreadcrumbProvider>
        <ShellNavIntlProvider>
          <AppShell>{children}</AppShell>
        </ShellNavIntlProvider>
      </BreadcrumbProvider>
    </OperatorShellProvider>
  );
}

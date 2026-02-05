"use client";

import { usePathname } from "next/navigation";
import { OperatorAuthProvider } from "~/lib/operator/auth-context";
import { OperatorShellProvider } from "~/lib/shell-auth-context";
import { BreadcrumbProvider } from "~/lib/breadcrumb-context";
import { AppShell } from "~/components/dashboard/app-shell";

export default function OperatorLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  const pathname = usePathname();
  const isLoginPage = pathname === "/operator/login";

  return (
    <OperatorAuthProvider>
      {isLoginPage ? (
        children
      ) : (
        <OperatorShellProvider>
          <BreadcrumbProvider>
            <AppShell>{children}</AppShell>
          </BreadcrumbProvider>
        </OperatorShellProvider>
      )}
    </OperatorAuthProvider>
  );
}

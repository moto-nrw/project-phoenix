"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useSession } from "next-auth/react";
import { OperatorShellProvider } from "~/lib/shell-auth-context";
import { BreadcrumbProvider } from "~/lib/breadcrumb-context";
import { AppShell } from "~/components/dashboard/app-shell";
import { Loading } from "~/components/ui/loading";

function FullPageLoading() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center p-4">
      <Loading />
    </div>
  );
}

export default function OperatorLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  const pathname = usePathname();
  const router = useRouter();
  const isLoginPage = pathname === "/operator/login";
  const { data: session, status } = useSession();

  // Redirect non-operator authenticated users away
  useEffect(() => {
    if (
      !isLoginPage &&
      status === "authenticated" &&
      session?.user?.scope !== "platform"
    ) {
      router.push("/");
    }
  }, [isLoginPage, status, session, router]);

  // Login page: render without auth guards
  if (isLoginPage) {
    return <>{children}</>;
  }

  // Loading, unauthenticated, or wrong account type — show loading spinner
  if (
    status === "loading" ||
    status === "unauthenticated" ||
    session?.user?.scope !== "platform"
  ) {
    if (status === "unauthenticated") {
      router.push("/operator/login");
    }
    return <FullPageLoading />;
  }

  return (
    <OperatorShellProvider>
      <BreadcrumbProvider>
        <AppShell>{children}</AppShell>
      </BreadcrumbProvider>
    </OperatorShellProvider>
  );
}

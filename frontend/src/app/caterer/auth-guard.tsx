"use client";

import { redirect, usePathname } from "next/navigation";
import { useSession } from "next-auth/react";
import { Loading } from "~/components/ui/loading";
import { catererPath } from "~/lib/caterer-url";

/** Pages of the caterer portal that need no session. */
const CATERER_PUBLIC_PAGES = [
  "/caterer/login",
  "/login",
  "/caterer/reset-password",
  "/reset-password",
  // The mailed invitation: the link itself is the proof.
  "/caterer/einladung",
  "/einladung",
];

/**
 * Client-side guard of the caterer portal (ADR 0077). Other portals' tokens
 * never reach it (host-only cookies and the proxy), so it only waits for the
 * session or sends a visitor without one to the login.
 */
export function CatererAuthGuard({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  const pathname = usePathname();
  const { data: session, status } = useSession();
  const isPublicPage = CATERER_PUBLIC_PAGES.some(
    (page) => pathname === page || pathname.startsWith(`${page}/`),
  );

  if (isPublicPage) {
    return <>{children}</>;
  }
  if (status === "authenticated" && session?.user?.scope !== "caterer") {
    redirect(catererPath("/caterer/login"));
  }
  if (status === "unauthenticated") {
    redirect(catererPath("/caterer/login"));
  }
  if (status === "loading") {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <Loading message="Wird geladen …" />
      </div>
    );
  }
  return <>{children}</>;
}

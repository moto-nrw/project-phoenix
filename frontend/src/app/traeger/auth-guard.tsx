"use client";

import { redirect, usePathname } from "next/navigation";
import { useSession } from "next-auth/react";
import { ShellIntlProvider } from "~/components/dashboard/shell-intl-provider";
import { Loading } from "~/components/ui/loading";
import { TraegerShell } from "~/components/traeger/traeger-shell";
import {
  TraegerOwnNamePublisher,
  useTraegerOwnName,
  type TraegerMembershipKey,
} from "~/components/traeger/traeger-own-name";
import { useTraegerOrganization } from "~/components/traeger/use-traeger-organization";
import { BreadcrumbProvider } from "~/lib/breadcrumb-context";
import { hasSessionAccess } from "~/lib/session-access";
import { TraegerShellProvider } from "~/lib/shell-auth-context";
import { traegerPath } from "~/lib/traeger-url";

const TRAEGER_PUBLIC_PAGES = [
  "/traeger/login",
  "/login",
  "/traeger/reset-password",
  "/reset-password",
  "/traeger/invite",
  "/invite",
];

/**
 * Client-side auth guard for Träger routes ("moto träger", #4649). Mirrors
 * SchoolAuthGuard: other portals' tokens never reach this app (host-only
 * cookies plus the proxy), so it only handles "loading" → spinner,
 * "no session" → login, and a session of another scope or a terminally
 * failed refresh → login.
 */
export function TraegerAuthGuard({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  const pathname = usePathname();
  const isPublicPage = TRAEGER_PUBLIC_PAGES.some(
    (p) => pathname === p || pathname.startsWith(`${p}/`),
  );
  const { data: session, status } = useSession();

  if (isPublicPage) {
    return <>{children}</>;
  }

  // A session of another scope, or one whose refresh ended for good (expired
  // or revoked refresh token, membership gone), goes back to the login page,
  // which clears the failed session. The layout's server snapshot carries no
  // token and stays (#4991).
  if (
    status === "authenticated" &&
    (session?.user?.scope !== "org" ||
      session.error === "RefreshTokenExpired" ||
      !hasSessionAccess(session))
  ) {
    redirect(traegerPath("/traeger/login"));
  }
  if (status === "unauthenticated") {
    redirect(traegerPath("/traeger/login"));
  }

  if (status === "loading") {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <Loading fullPage message="Wird geladen …" />
      </div>
    );
  }

  const user = session?.user;
  return (
    <TraegerPortal
      membership={user?.id ? `${user.id}:${user.orgId ?? ""}` : null}
    >
      {children}
    </TraegerPortal>
  );
}

/**
 * The signed-in portal: the shared shell providers nested like the caterer
 * portal's (#5145). The Träger is read once here, for its name next to the
 * logo and the test-system strip, and the own profile for the person's name
 * in the header (#6870).
 */
function TraegerPortal({
  children,
  membership,
}: {
  readonly children: React.ReactNode;
  readonly membership: TraegerMembershipKey | null;
}) {
  const organization = useTraegerOrganization();
  const ownName = useTraegerOwnName(membership);
  return (
    <TraegerShellProvider
      organizationName={organization?.name}
      memberName={ownName.name}
    >
      <TraegerOwnNamePublisher publish={ownName.publish}>
        <BreadcrumbProvider>
          <ShellIntlProvider>
            <TraegerShell testSystem={organization?.testSystem === true}>
              {children}
            </TraegerShell>
          </ShellIntlProvider>
        </BreadcrumbProvider>
      </TraegerOwnNamePublisher>
    </TraegerShellProvider>
  );
}

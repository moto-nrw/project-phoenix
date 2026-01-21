"use client";

import { useEffect } from "react";
import { useSession } from "~/lib/auth-client";
import { useRouter } from "next/navigation";
import { Header } from "./header";
import { Sidebar } from "./sidebar";
import { MobileBottomNav } from "./mobile-bottom-nav";

interface ResponsiveLayoutProps {
  readonly children: React.ReactNode;
  readonly pageTitle?: string;
  readonly studentName?: string; // For student detail page breadcrumbs
  readonly roomName?: string; // For room detail page breadcrumbs
  readonly activityName?: string; // For activity detail page breadcrumbs
  readonly referrerPage?: string; // Where the user came from (for contextual breadcrumbs)
  readonly activeSupervisionName?: string; // For active supervision breadcrumb (e.g., "Schulhof")
  readonly ogsGroupName?: string; // For OGS group breadcrumb (e.g., "Sonngruppe")
}

export default function ResponsiveLayout({
  children,
  pageTitle,
  studentName,
  roomName,
  activityName,
  referrerPage,
  activeSupervisionName,
  ogsGroupName,
}: ResponsiveLayoutProps) {
  const { data: session, isPending } = useSession();
  const router = useRouter();
  // eslint-disable-next-line @typescript-eslint/prefer-nullish-coalescing -- intentionally using || to treat empty strings as falsy
  const userName = session?.user?.name?.trim() || undefined;
  const userEmail = session?.user?.email ?? "";
  // BetterAuth: roles are stored differently - check the session structure
  const userRoles =
    (session?.user as { roles?: string[] } | undefined)?.roles ?? [];
  const userRole = userRoles.includes("admin") ? "Admin" : "Betreuer";

  // Check for invalid session and redirect
  useEffect(() => {
    // BetterAuth: use isPending instead of status === "loading"
    if (isPending) return;

    // BetterAuth: if not pending and no user, redirect to login
    if (!session?.user) {
      router.push("/");
    }
  }, [session, isPending, router]);

  return (
    <div className="relative min-h-screen">
      {/* Note: Modal blur overlay is now in BackgroundWrapper for global coverage */}

      {/* Header - sticky positioning */}
      <div className="sticky top-0 z-40">
        <Header
          userName={userName}
          userEmail={userEmail}
          userRole={userRole}
          customPageTitle={pageTitle}
          studentName={studentName}
          roomName={roomName}
          activityName={activityName}
          referrerPage={referrerPage}
          activeSupervisionName={activeSupervisionName}
          ogsGroupName={ogsGroupName}
        />
      </div>

      {/* Main content */}
      <div className="flex">
        {/* Desktop sidebar - only visible on md+ screens */}
        <Sidebar className="hidden lg:block" />

        {/* Main content with bottom padding on mobile for bottom navigation */}
        <main className="flex-1 p-4 pb-24 md:p-8 lg:pb-8">{children}</main>
      </div>

      {/* Mobile bottom navigation */}
      <MobileBottomNav />
    </div>
  );
}

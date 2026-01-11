"use client";

import { useEffect } from "react";
import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { Header } from "./header";
import { Sidebar } from "./sidebar";
import { MobileBottomNav } from "./mobile-bottom-nav";
import { useModal } from "./modal-context";

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
  const { data: session, status } = useSession();
  const router = useRouter();
  // eslint-disable-next-line @typescript-eslint/prefer-nullish-coalescing -- intentionally using || to treat empty strings as falsy
  const userName = session?.user?.name?.trim() || undefined;
  const userEmail = session?.user?.email ?? "";
  const userRoles = session?.user?.roles ?? [];
  const userRole = userRoles.includes("admin") ? "Admin" : "Betreuer";
  // Get modal state directly from context - much simpler than custom events
  const { isModalOpen } = useModal();

  // Check for invalid session and redirect
  useEffect(() => {
    if (status === "loading") return;

    // If session exists but token is empty, redirect to login
    if (session && !session.user?.token) {
      router.push("/");
    }
  }, [session, status, router]);

  return (
    <div className="relative min-h-screen">
      {/* Blur overlay - rendered on top when modal is open */}
      {/* Uses backdrop-blur instead of filter on content to avoid compositing bugs */}
      {/* z-50 ensures overlay is above header (z-40) but below modal (z-[9999]) */}
      <div
        className={`pointer-events-none fixed inset-0 z-50 transition-all duration-300 ${
          isModalOpen
            ? "bg-black/5 backdrop-blur-sm"
            : "bg-transparent backdrop-blur-none"
        }`}
        aria-hidden="true"
      />

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

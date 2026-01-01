// components/dashboard/header.tsx
// Refactored with extracted sub-components to reduce cognitive complexity
"use client";

import { useState, useEffect } from "react";
import { usePathname } from "next/navigation";
import { useSession } from "next-auth/react";
import { HelpButton } from "@/components/ui/help_button";
import { getHelpContent } from "@/lib/help-content";
import { LogoutModal } from "~/components/ui/logout-modal";
import { useProfile } from "~/lib/profile-context";

// Import extracted components
import { BrandLink, BreadcrumbDivider } from "./header/brand-link";
import { SessionWarning } from "./header/session-warning";
import { ProfileTrigger, ProfileDropdownMenu } from "./header/profile-dropdown";
import {
  DatabaseBreadcrumb,
  OgsGroupsBreadcrumb,
  ActiveSupervisionsBreadcrumb,
  InvitationsBreadcrumb,
  ActivityBreadcrumb,
  RoomBreadcrumb,
  StudentHistoryBreadcrumb,
  StudentDetailBreadcrumb,
  PageTitleDisplay,
} from "./header/breadcrumb-components";
import {
  getPageTitle,
  getSubPageLabel,
  getBreadcrumbLabel,
  getHistoryType,
  getPageTypeInfo,
} from "./header/breadcrumb-utils";

interface HeaderProps {
  readonly userName?: string;
  readonly userEmail?: string;
  readonly userRole?: string;
  readonly customPageTitle?: string;
  readonly studentName?: string;
  readonly roomName?: string;
  readonly activityName?: string;
  readonly referrerPage?: string;
  readonly activeSupervisionName?: string;
  readonly ogsGroupName?: string;
}

export function Header({
  userName = "Benutzer",
  userEmail = "",
  userRole = "",
  customPageTitle,
  studentName,
  roomName,
  activityName,
  referrerPage,
  activeSupervisionName,
  ogsGroupName,
}: HeaderProps) {
  const [isProfileMenuOpen, setIsProfileMenuOpen] = useState(false);
  const [isLogoutModalOpen, setIsLogoutModalOpen] = useState(false);
  const [isScrolled, setIsScrolled] = useState(false);
  const pathname = usePathname();
  const helpContent = getHelpContent(pathname);
  const pageTitle = customPageTitle ?? getPageTitle(pathname);
  const { data: session } = useSession();
  const { profile } = useProfile();

  // Scroll effect for header shrinking
  useEffect(() => {
    const handleScroll = () => {
      setIsScrolled(window.scrollY > 20);
    };
    window.addEventListener("scroll", handleScroll, { passive: true });
    return () => window.removeEventListener("scroll", handleScroll);
  }, []);

  // Get page type information
  const pageTypeInfo = getPageTypeInfo(pathname);
  const referrer = referrerPage ?? "/students/search";
  const breadcrumbLabel = getBreadcrumbLabel(referrer);
  const historyType = getHistoryType(pathname);
  const subPageLabel = getSubPageLabel(pathname);

  // Profile data from context or props
  const displayName = profile
    ? `${profile.firstName ?? ""} ${profile.lastName ?? ""}`.trim() || userName
    : userName;
  const displayAvatar = profile?.avatar;

  const isSessionExpired = session?.error === "RefreshTokenExpired";

  return (
    <header
      className={`sticky top-0 z-50 w-full bg-white transition-all duration-300 ${
        isScrolled ? "shadow-sm" : ""
      }`}
    >
      <div className="w-full px-4 sm:px-6 lg:px-8">
        <div
          className={`flex w-full items-center transition-all duration-300 ${
            isScrolled ? "h-12 lg:h-16" : "h-14 lg:h-16"
          }`}
        >
          {/* Left section: Logo + Brand + Context */}
          <div className="flex flex-shrink-0 items-center space-x-4">
            <BrandLink isScrolled={isScrolled} />
            <BreadcrumbDivider />
            <HeaderBreadcrumb
              pathname={pathname}
              pageTitle={pageTitle}
              pageTypeInfo={pageTypeInfo}
              subPageLabel={subPageLabel}
              isScrolled={isScrolled}
              studentName={studentName}
              roomName={roomName}
              activityName={activityName}
              referrer={referrer}
              breadcrumbLabel={breadcrumbLabel}
              historyType={historyType}
              ogsGroupName={ogsGroupName}
              activeSupervisionName={activeSupervisionName}
            />
          </div>

          {/* Right section: Actions + Profile */}
          <div className="ml-auto flex items-center space-x-3">
            {/* Desktop actions */}
            <div className="hidden items-center space-x-2 lg:flex">
              <SessionWarning isExpired={isSessionExpired} variant="desktop" />
              <HelpButton
                title={helpContent.title}
                content={helpContent.content}
                buttonClassName="!w-[40px] !h-[40px] !min-w-[40px] !min-h-[40px] p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 !bg-transparent rounded-lg transition-colors duration-200"
              />
            </div>

            {/* Mobile actions */}
            <div className="flex items-center space-x-2 lg:hidden">
              <SessionWarning isExpired={isSessionExpired} variant="mobile" />
            </div>

            {/* User menu */}
            <div className="relative">
              <ProfileTrigger
                displayName={displayName}
                displayAvatar={displayAvatar}
                userRole={userRole}
                isOpen={isProfileMenuOpen}
                onClick={() => setIsProfileMenuOpen(!isProfileMenuOpen)}
              />
              <ProfileDropdownMenu
                isOpen={isProfileMenuOpen}
                displayName={displayName}
                displayAvatar={displayAvatar}
                userEmail={userEmail}
                onClose={() => setIsProfileMenuOpen(false)}
                onLogout={() => setIsLogoutModalOpen(true)}
              />
            </div>
          </div>
        </div>
      </div>

      <LogoutModal
        isOpen={isLogoutModalOpen}
        onClose={() => setIsLogoutModalOpen(false)}
      />
    </header>
  );
}

// Re-export UserAvatar for external use
export { UserAvatar } from "./header/profile-dropdown";

/**
 * Breadcrumb section component - handles routing logic for different page types
 */
interface HeaderBreadcrumbProps {
  readonly pathname: string;
  readonly pageTitle: string;
  readonly pageTypeInfo: ReturnType<typeof getPageTypeInfo>;
  readonly subPageLabel: string;
  readonly isScrolled: boolean;
  readonly studentName?: string;
  readonly roomName?: string;
  readonly activityName?: string;
  readonly referrer: string;
  readonly breadcrumbLabel: string;
  readonly historyType: string;
  readonly ogsGroupName?: string;
  readonly activeSupervisionName?: string;
}

function HeaderBreadcrumb({
  pathname,
  pageTitle,
  pageTypeInfo,
  subPageLabel,
  isScrolled,
  studentName,
  roomName,
  activityName,
  referrer,
  breadcrumbLabel,
  historyType,
  ogsGroupName,
  activeSupervisionName,
}: HeaderBreadcrumbProps) {
  // Database sub-pages
  if (pageTypeInfo.isDatabaseSubPage) {
    return (
      <DatabaseBreadcrumb
        pathname={pathname}
        pageTitle={pageTitle}
        subPageLabel={subPageLabel}
        isDeepPage={pageTypeInfo.isDatabaseDeepPage}
      />
    );
  }

  // OGS Groups page
  if (pathname === "/ogs-groups") {
    return <OgsGroupsBreadcrumb groupName={ogsGroupName} />;
  }

  // Active Supervisions page
  if (pathname === "/active-supervisions") {
    return (
      <ActiveSupervisionsBreadcrumb supervisionName={activeSupervisionName} />
    );
  }

  // Invitations page
  if (pathname === "/invitations") {
    return <InvitationsBreadcrumb />;
  }

  // Activity detail page
  if (pageTypeInfo.isActivityDetailPage && activityName) {
    return <ActivityBreadcrumb activityName={activityName} />;
  }

  // Room detail page
  if (pageTypeInfo.isRoomDetailPage && roomName) {
    return <RoomBreadcrumb roomName={roomName} />;
  }

  // Student history sub-page (3 levels)
  if (pageTypeInfo.isStudentHistoryPage && studentName) {
    return (
      <StudentHistoryBreadcrumb
        referrer={referrer}
        breadcrumbLabel={breadcrumbLabel}
        pathname={pathname}
        studentName={studentName}
        historyType={historyType}
        isScrolled={isScrolled}
      />
    );
  }

  // Student detail page (2 levels)
  if (pageTypeInfo.isStudentDetailPage && studentName) {
    return (
      <StudentDetailBreadcrumb
        referrer={referrer}
        breadcrumbLabel={breadcrumbLabel}
        studentName={studentName}
        isScrolled={isScrolled}
      />
    );
  }

  // Simple page routes
  const simpleTitleRoutes = [
    "/rooms",
    "/activities",
    "/staff",
    "/substitutions",
    "/statistics",
  ];

  if (simpleTitleRoutes.includes(pathname)) {
    return <PageTitleDisplay title={pageTitle} isScrolled={isScrolled} />;
  }

  // Default: show page title
  return <PageTitleDisplay title={pageTitle} isScrolled={isScrolled} />;
}

// components/dashboard/header.tsx
// Refactored with extracted sub-components to reduce cognitive complexity
"use client";

import { useState, useEffect, useRef } from "react";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { LogoutModal } from "~/components/ui/logout-modal";
import { BrandTenantSwitcher } from "~/components/tenant/tenant-switcher";
import { useShellAuth } from "~/lib/shell-auth-context";
import { useBreadcrumb } from "~/lib/breadcrumb-context";
import { LanguageSwitcher } from "~/components/parent/language-switcher";
import {
  useTenantRoutingModeSafe,
  useTenantSafe,
  useTenantSlugSafe,
} from "~/lib/tenant-context";
import { normalizeTenantPathname } from "~/lib/tenant-path";
import { matchesPathPrefix } from "~/lib/section-navigation";

// Import extracted components
import { BrandLink, BreadcrumbDivider } from "./header/brand-link";
import { RefreshButton } from "./header/refresh-button";
import { RemindersBell } from "./header/reminders-bell";
import { SessionWarning } from "./header/session-warning";
import { ProfileTrigger, ProfileDropdownMenu } from "./header/profile-dropdown";
import {
  SectionBreadcrumb,
  OgsGroupsBreadcrumb,
  ActiveSupervisionsBreadcrumb,
  EnrollmentBreadcrumb,
  StudentHistoryBreadcrumb,
  StudentDetailBreadcrumb,
  StaffDetailBreadcrumb,
  ParentChildBreadcrumb,
  PageTitleDisplay,
} from "./header/breadcrumb-components";
import {
  getPageTitle,
  getBreadcrumbLabel,
  getHistoryType,
  getPageTypeInfo,
  getSectionBreadcrumb,
} from "./header/breadcrumb-utils";

export function Header() {
  const { breadcrumb } = useBreadcrumb();
  const {
    studentName,
    staffName,
    referrerPage,
    activeSupervisionName,
    ogsGroupName,
    pageTitle: customPageTitle,
  } = breadcrumb;
  const [isProfileMenuOpen, setIsProfileMenuOpen] = useState(false);
  const [isLogoutModalOpen, setIsLogoutModalOpen] = useState(false);
  const [isScrolled, setIsScrolled] = useState(false);
  const profileMenuRef = useRef<HTMLDivElement | null>(null);
  const rawPathname = usePathname();
  const tenantSlug = useTenantSlugSafe();
  const routingMode = useTenantRoutingModeSafe();
  const pathname = normalizeTenantPathname(
    rawPathname,
    tenantSlug,
    routingMode,
  );
  const tenantContext = useTenantSafe();
  // parentNav is available in every shell; only parent-mode branches read it, so
  // the German staff/operator labels are untouched (they render the de mirror).
  const tParentNav = useTranslations("parentNav");
  const pageTitle = customPageTitle ?? getPageTitle(pathname);
  const {
    user,
    profile,
    isSessionExpired: sessionExpired,
    mode,
    homeUrl,
    profileUrl,
  } = useShellAuth();
  // getPageTitle() is a pure helper and can't read the locale, so the
  // parent-portal page titles it returns ("Meine Kinder", "Kinderprofil", …)
  // would render German even on a localized portal. Override them here, where
  // the parentNav catalog is in scope. Staff/operator modes keep getPageTitle.
  const parentPageTitle = (() => {
    if (mode !== "parent") return null;
    if (pathname === "/" || pathname === "/parents") return tParentNav("start");
    if (pathname === "/parents/children") return tParentNav("children");
    if (pathname.startsWith("/parents/children/"))
      return tParentNav("childProfile");
    if (matchesPathPrefix(pathname, "/parents/messages"))
      return tParentNav("messages");
    if (matchesPathPrefix(pathname, "/messages")) return tParentNav("messages");
    if (pathname === "/parents/news" || pathname === "/news")
      return tParentNav("news");
    if (pathname === "/parents/settings" || pathname === "/settings")
      return tParentNav("settings");
    if (pathname === "/parents/meal-plan" || pathname === "/meal-plan")
      return tParentNav("mealPlan");
    return null;
  })();
  const displayedPageTitle = parentPageTitle ?? pageTitle;

  // Derive user info from ShellAuth context
  const userName = user?.name ?? "Benutzer";
  const userEmail = user?.email ?? "";
  const userRoles = user?.roles ?? [];
  const userRole =
    mode === "operator"
      ? "Operator"
      : mode === "parent"
        ? tParentNav("role")
        : userRoles.includes("admin")
          ? "Admin"
          : "Betreuer";

  // Scroll effect for header shrinking (hysteresis to prevent flicker)
  useEffect(() => {
    const handleScroll = () => {
      const y = globalThis.window.scrollY;
      setIsScrolled((prev) => (prev ? y > 10 : y > 30));
    };
    globalThis.window.addEventListener("scroll", handleScroll, {
      passive: true,
    });
    return () => globalThis.window.removeEventListener("scroll", handleScroll);
  }, []);

  useEffect(() => {
    if (!isProfileMenuOpen) return;

    const handlePointerDown = (event: MouseEvent) => {
      const target = event.target;
      if (!(target instanceof Node)) return;
      if (profileMenuRef.current?.contains(target)) return;
      setIsProfileMenuOpen(false);
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setIsProfileMenuOpen(false);
      }
    };

    document.addEventListener("mousedown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("mousedown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [isProfileMenuOpen]);

  // Get page type information
  const pageTypeInfo = getPageTypeInfo(pathname);
  const referrer = referrerPage ?? "/students/search";
  const breadcrumbLabel = getBreadcrumbLabel(referrer);
  const historyType = getHistoryType(pathname);
  // Nur im Mitarbeiter-Portal: die Kataloge beschreiben dessen Seitenleiste.
  // Das Elternportal teilt sich Pfade wie /messages und bekäme sonst eine
  // "Eltern › Nachrichten"-Breadcrumb aus der Mitarbeitersicht.
  const sectionBreadcrumb =
    mode === "teacher" ? getSectionBreadcrumb(pathname) : null;

  // Use JWT name as single source of truth (avoids flicker from async profile fetch)
  const displayName = userName;
  const displayAvatar = profile?.avatar;
  const brandLabel = mode === "teacher" ? tenantContext?.tenant?.name : null;

  const isSessionExpired = sessionExpired;

  return (
    <header
      // Stable hook so portaled overlays (e.g. the filter popover) can measure
      // this sticky topbar's height and pin themselves just below it instead of
      // scrolling underneath it.
      data-app-header
      className={`sticky top-0 z-50 w-full border-b border-gray-200/70 bg-white/95 backdrop-blur-md transition-[background-color,border-color,box-shadow] duration-300 ${
        isScrolled ? "shadow-sm" : ""
      }`}
    >
      <div className="w-full px-4 sm:px-6 lg:px-8">
        <div
          className={`flex w-full items-center transition-[height] duration-300 ${
            isScrolled ? "h-12 lg:h-16" : "h-14 lg:h-16"
          }`}
        >
          {/* Left section: Logo + Brand + Context — must be allowed to
              shrink (min-w-0) so long tenant names / breadcrumbs truncate
              instead of pushing the header past the viewport (#2011) */}
          <div className="flex min-w-0 flex-1 items-center space-x-4">
            {mode === "teacher" ? (
              // Brand doubles as tenant switcher when the account has
              // multiple tenants; renders a plain BrandLink otherwise.
              <BrandTenantSwitcher
                isScrolled={isScrolled}
                href={homeUrl}
                label={brandLabel}
              />
            ) : (
              <BrandLink
                isScrolled={isScrolled}
                href={homeUrl}
                label={brandLabel}
              />
            )}
            <BreadcrumbDivider />
            <HeaderBreadcrumb
              pathname={pathname}
              pageTitle={displayedPageTitle}
              pageTypeInfo={pageTypeInfo}
              sectionBreadcrumb={sectionBreadcrumb}
              isScrolled={isScrolled}
              studentName={studentName}
              staffName={staffName}
              referrer={referrer}
              breadcrumbLabel={breadcrumbLabel}
              historyType={historyType}
              ogsGroupName={ogsGroupName}
              activeSupervisionName={activeSupervisionName}
            />
          </div>

          {/* Right section: Actions + Profile */}
          <div className="ml-auto flex flex-shrink-0 items-center space-x-3">
            {/* Desktop actions */}
            <div className="hidden items-center space-x-2 lg:flex">
              <SessionWarning isExpired={isSessionExpired} variant="desktop" />
              <RefreshButton />
            </div>

            {/* Mobile actions */}
            <div className="flex items-center space-x-2 lg:hidden">
              <SessionWarning isExpired={isSessionExpired} variant="mobile" />
              <RefreshButton />
            </div>

            {mode === "teacher" ? <RemindersBell /> : null}
            {mode === "parent" ? <LanguageSwitcher compact /> : null}

            {/* User menu */}
            <div ref={profileMenuRef} className="relative">
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
                profileUrl={profileUrl}
                profileLabel={
                  mode === "operator"
                    ? "Profileinstellungen"
                    : mode === "parent"
                      ? tParentNav("settings")
                      : undefined
                }
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

/**
 * Enrich a bare referrer URL (e.g. "/ogs-groups") with the last-selected
 * sub-item param from localStorage so the sidebar highlights the correct item
 * when navigating back via breadcrumb link.
 */
function enrichReferrerWithParam(referrer: string): string {
  if (globalThis.window === undefined) return referrer;
  if (referrer === "/ogs-groups") {
    const groupId = localStorage.getItem("sidebar-last-group");
    if (groupId) return `/ogs-groups?group=${groupId}`;
  }
  if (referrer === "/active-supervisions") {
    const roomId = localStorage.getItem("sidebar-last-room");
    if (roomId) return `/active-supervisions?room=${roomId}`;
  }
  return referrer;
}

/**
 * Breadcrumb section component - handles routing logic for different page types
 */
interface HeaderBreadcrumbProps {
  readonly pathname: string;
  readonly pageTitle: string;
  readonly pageTypeInfo: ReturnType<typeof getPageTypeInfo>;
  readonly sectionBreadcrumb: ReturnType<typeof getSectionBreadcrumb>;
  readonly isScrolled: boolean;
  readonly studentName?: string;
  readonly staffName?: string;
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
  sectionBreadcrumb,
  isScrolled,
  studentName,
  staffName,
  referrer,
  breadcrumbLabel,
  historyType,
  ogsGroupName,
  activeSupervisionName,
}: HeaderBreadcrumbProps) {
  // Gruppierte Navigationsbereiche: Datenverwaltung, Planung, Eltern
  if (sectionBreadcrumb) {
    return <SectionBreadcrumb {...sectionBreadcrumb} isScrolled={isScrolled} />;
  }

  // OGS Groups page
  if (pathname === "/ogs-groups") {
    return (
      <OgsGroupsBreadcrumb groupName={ogsGroupName} isScrolled={isScrolled} />
    );
  }

  // Active Supervisions page
  if (pathname === "/active-supervisions") {
    return (
      <ActiveSupervisionsBreadcrumb
        supervisionName={activeSupervisionName}
        isScrolled={isScrolled}
      />
    );
  }

  if (pageTypeInfo.isEnrollmentPage) {
    return (
      <EnrollmentBreadcrumb
        current={pageTitle}
        pathname={pathname}
        isScrolled={isScrolled}
      />
    );
  }

  if (pathname.startsWith("/parents/children/")) {
    return (
      <ParentChildBreadcrumb childName={pageTitle} isScrolled={isScrolled} />
    );
  }

  // enrichReferrerWithParam liest localStorage; der Aufruf steht deshalb in
  // den beiden Zweigen, die das Ergebnis auch verwenden, statt auf jedem
  // Render-Pfad zu laufen.

  // Student history sub-page (3 or 4 levels depending on context)
  if (pageTypeInfo.isStudentHistoryPage) {
    const subSectionName = ogsGroupName ?? activeSupervisionName;
    return (
      <StudentHistoryBreadcrumb
        referrer={enrichReferrerWithParam(referrer)}
        breadcrumbLabel={breadcrumbLabel}
        pathname={pathname}
        studentName={studentName ?? "…"}
        historyType={historyType}
        isScrolled={isScrolled}
        subSectionName={subSectionName}
      />
    );
  }

  // Staff detail page (2 levels: Mitarbeiter / Name)
  if (pageTypeInfo.isStaffDetailPage) {
    return (
      <StaffDetailBreadcrumb
        staffName={staffName ?? "…"}
        isScrolled={isScrolled}
      />
    );
  }

  // Student detail page (2 or 3 levels depending on context)
  if (pageTypeInfo.isStudentDetailPage) {
    // When navigating from an accordion section, show the sub-section name
    // e.g. "Meine Gruppe > 1a > Mia Fischer" instead of "Meine Gruppe > Mia Fischer"
    const subSectionName = ogsGroupName ?? activeSupervisionName;
    return (
      <StudentDetailBreadcrumb
        referrer={enrichReferrerWithParam(referrer)}
        breadcrumbLabel={breadcrumbLabel}
        studentName={studentName ?? "…"}
        isScrolled={isScrolled}
        subSectionName={subSectionName}
      />
    );
  }

  // Default: show page title
  return <PageTitleDisplay title={pageTitle} isScrolled={isScrolled} />;
}

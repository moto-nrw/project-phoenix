// components/dashboard/header.tsx
// Refactored with extracted sub-components to reduce cognitive complexity
"use client";

import { useState, useEffect } from "react";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { PanelLeftClose, PanelLeftOpen } from "lucide-react";
import { Button } from "~/components/ui/button";
import { LogoutModal } from "~/components/ui/logout-modal";
import { AnchoredPopover } from "~/components/ui/anchored-popover";
import { useSidebarCollapsed } from "~/lib/hooks/use-sidebar-collapsed";
import { BrandTenantSwitcher } from "~/components/tenant/tenant-switcher";
import { StaffPreviewModal } from "~/components/staff-preview/staff-preview-modal";
import { useShellAuth } from "~/lib/shell-auth-context";
import { useBreadcrumb } from "~/lib/breadcrumb-context";
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

// Nur oberhalb von lg existiert die Desktop-Seitenleiste; darunter zeigt
// die App die mobile Bottom-Nav und der Shortcut hätte nichts zu schalten.
const SIDEBAR_VISIBLE_QUERY = "(min-width: 1024px)";

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return (
    target.isContentEditable ||
    target.tagName === "INPUT" ||
    target.tagName === "TEXTAREA" ||
    target.tagName === "SELECT"
  );
}

// Seitentitel des Schul-Portals. Beide Schreibweisen eines Pfades führen zum
// selben Titel: auf dem Schul-Host zeigt die Adresszeile "/" und
// "/aufsichten", intern laufen die Seiten unter /school und
// /school/aufsichten.
function schoolTitleForPath(pathname: string): string | null {
  if (pathname === "/" || pathname === "/school") return "Klassenansicht";
  // Die Klassenseite (#2294) ist eine Ebene der Klassenansicht, kein eigener
  // Bereich: der Kopf behält den Namen des Bereichs, den Klassennamen trägt
  // die Seite selbst.
  if (pathname === "/klasse" || pathname === "/school/klasse") {
    return "Klassenansicht";
  }
  if (pathname === "/aufsichten" || pathname === "/school/aufsichten") {
    return "Meine Aufsichten";
  }
  // Der Verlauf einer Unterhaltung (/nachrichten/{id}) bleibt im Bereich
  // "Nachrichten"; den Namen der Person trägt die Seite selbst.
  if (
    pathname === "/nachrichten" ||
    pathname === "/school/nachrichten" ||
    pathname.startsWith("/nachrichten/") ||
    pathname.startsWith("/school/nachrichten/")
  ) {
    return "Nachrichten";
  }
  if (pathname === "/einstellungen" || pathname === "/school/einstellungen") {
    return "Einstellungen";
  }
  return null;
}

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
  const [isPreviewModalOpen, setIsPreviewModalOpen] = useState(false);
  const [isScrolled, setIsScrolled] = useState(false);
  const rawPathname = usePathname();
  const tenantSlug = useTenantSlugSafe();
  const routingMode = useTenantRoutingModeSafe();
  const pathname = normalizeTenantPathname(
    rawPathname,
    tenantSlug,
    routingMode,
  );
  const tenantContext = useTenantSafe();
  // parentNav is available in every shell. Staff/operator shells use the de
  // mirror; the parents portal supplies its active locale.
  const tParentNav = useTranslations("parentNav");
  const pageTitle = customPageTitle ?? getPageTitle(pathname);
  const {
    user,
    profile,
    isSessionExpired: sessionExpired,
    mode,
    homeUrl,
    profileUrl,
    canStartStaffPreview,
  } = useShellAuth();
  // Ein-/Ausklappen der Desktop-Seitenleiste (#2825) — nur in den Portalen
  // mit einklappbarer Leiste; das Eltern- und das Schul-Portal haben eigene
  // Leisten ohne Klappzustand. Der Zustand syncht über den geteilten
  // useSidebarCollapsed-Store mit der Seitenleiste selbst.
  const hasCollapsibleSidebar = mode === "teacher" || mode === "operator";
  const { collapsed: sidebarCollapsed, toggleCollapsed: toggleSidebar } =
    useSidebarCollapsed();

  // Cmd+B (Mac) bzw. Strg+B (Windows/Linux) schaltet zusätzlich um — der
  // etablierte Standard (VS Code, shadcn). In Eingabefeldern und Editoren
  // bleibt die Tastenkombination unangetastet (dort heißt sie „fett").
  useEffect(() => {
    if (!hasCollapsibleSidebar) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() !== "b") return;
      if (!(event.metaKey || event.ctrlKey)) return;
      if (event.altKey || event.shiftKey) return;
      if (isEditableTarget(event.target)) return;
      if (!globalThis.matchMedia(SIDEBAR_VISIBLE_QUERY).matches) return;
      event.preventDefault();
      toggleSidebar();
    };
    globalThis.addEventListener("keydown", handleKeyDown);
    return () => globalThis.removeEventListener("keydown", handleKeyDown);
  }, [hasCollapsibleSidebar, toggleSidebar]);

  const sidebarToggleLabel = sidebarCollapsed
    ? "Seitenleiste ausklappen"
    : "Seitenleiste einklappen";
  // getPageTitle() is a pure helper and can't read the locale, so the
  // parent-portal page titles it returns ("Meine Kinder", "Kinderprofil", …)
  // would render German even on a localized portal. Override them here, where
  // the parentNav catalog is in scope. Staff/operator modes keep getPageTitle.
  const parentPageTitle = (() => {
    if (mode !== "parent") return null;
    if (pathname === "/" || pathname === "/parents") return tParentNav("start");
    if (pathname === "/parents/children" || pathname === "/children")
      return tParentNav("children");
    if (
      pathname.startsWith("/parents/children/") ||
      pathname.startsWith("/children/")
    )
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
    if (pathname === "/parents/calendar" || pathname === "/calendar")
      return tParentNav("calendar");
    if (
      matchesPathPrefix(pathname, "/parents/enroll") ||
      matchesPathPrefix(pathname, "/enroll")
    )
      return tParentNav("enroll");
    return null;
  })();
  // Schul-Portal (#2207): die Seiten des Schul-Hosts laufen ohne
  // /school-Präfix — getPageTitle kennt nur die Tenant-Pfade und würde den
  // Dashboard-Fallback anzeigen.
  const schoolPageTitle =
    mode === "school" ? schoolTitleForPath(pathname) : null;
  const displayedPageTitle = parentPageTitle ?? schoolPageTitle ?? pageTitle;

  // Derive user info from ShellAuth context
  const userName = user?.name ?? "Benutzer";
  const userEmail = user?.email ?? "";
  const userRoles = user?.roles ?? [];
  const userRole =
    mode === "operator"
      ? "Operator"
      : mode === "parent"
        ? tParentNav("role")
        : mode === "school"
          ? "Lehrkraft"
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
  const profileMenuLabel = tParentNav("profileMenu", { name: displayName });
  const brandLabel = mode === "teacher" ? tenantContext?.tenant?.name : null;

  const isSessionExpired = sessionExpired;

  // Ortsangabe für Bildschirme unter md. Der Bereich der Seitenleiste steht
  // voran, weil eine Seite wie "Vertretung" ohne ihn nicht verortet ist.
  const mobileLocation =
    mode === "parent"
      ? displayedPageTitle
      : sectionBreadcrumb
        ? `${sectionBreadcrumb.sectionLabel} › ${sectionBreadcrumb.deepLabel ?? sectionBreadcrumb.pageLabel}`
        : displayedPageTitle;

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
            {/* Seitenleisten-Toggle (#2825): erstes Element der Kopfzeile,
                links vom Logo — die Standardposition (Gmail, GitHub) für
                Layouts mit vollbreiter Topbar. */}
            {hasCollapsibleSidebar && (
              <Button
                type="button"
                variant="ghost"
                size="icon"
                onClick={toggleSidebar}
                title={sidebarToggleLabel}
                aria-label={sidebarToggleLabel}
                aria-expanded={!sidebarCollapsed}
                aria-keyshortcuts="Control+B Meta+B"
                // -ml-4 zieht den Button aus dem Header-Padding (lg:px-8)
                // nach links, sodass die Icon-Mitte (48px Button-Mitte -
                // 16px = 32px) exakt über der Icon-Spalte der eingeklappten
                // Seitenleiste (w-16, Mitte 32px) sitzt — der Toggle liest
                // sich als Kopf der Leiste, nicht als freischwebendes
                // Header-Element.
                className="-ml-4 hidden shrink-0 lg:inline-flex"
              >
                {sidebarCollapsed ? (
                  <PanelLeftOpen size={18} aria-hidden="true" />
                ) : (
                  <PanelLeftClose size={18} aria-hidden="true" />
                )}
              </Button>
            )}
            {mode === "teacher" ? (
              // Brand doubles as tenant switcher when the account has
              // multiple tenants; renders a plain BrandLink otherwise.
              <BrandTenantSwitcher
                isScrolled={isScrolled}
                href={homeUrl}
                label={brandLabel}
                hideLabelBelow="md"
              />
            ) : (
              <BrandLink
                isScrolled={isScrolled}
                href={homeUrl}
                label={brandLabel}
                hideLabelBelow={mode === "parent" ? "lg" : undefined}
              />
            )}
            <BreadcrumbDivider />
            {/* Ortsangabe auf dem Telefon: die Brotkrumen sind hidden md:flex,
                und seit die Seiten keine Bereichs-Überschrift mehr tragen, wäre
                unterhalb md sonst nirgends zu sehen, wo man ist. Im
                Mitarbeiterportal zeigt sie den Bereich der Seitenleiste mit,
                weil er dort die Brotkrume ersetzt. */}
            {mobileLocation && (
              <span className="min-w-0 truncate text-sm font-semibold text-gray-900 md:hidden">
                {mobileLocation}
              </span>
            )}
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
              {mode !== "parent" && <RefreshButton />}
            </div>

            {mode === "teacher" ? <RemindersBell /> : null}
            {/* User menu */}
            <AnchoredPopover
              open={isProfileMenuOpen}
              onOpenChange={setIsProfileMenuOpen}
              ariaLabel={profileMenuLabel}
              preferredWidth={288}
              align="end"
              className="border-0 bg-transparent p-0 shadow-none"
              renderTrigger={({ ref, toggle, panelId }) => (
                <ProfileTrigger
                  ref={ref}
                  menuId={panelId}
                  ariaLabel={profileMenuLabel}
                  displayName={displayName}
                  displayAvatar={displayAvatar}
                  userRole={userRole}
                  isOpen={isProfileMenuOpen}
                  compactOnTablet={mode === "parent"}
                  onClick={toggle}
                />
              )}
            >
              {({ close }) => (
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
                        : mode === "school"
                          ? "Einstellungen"
                          : undefined
                  }
                  onClose={close}
                  onLogout={() => setIsLogoutModalOpen(true)}
                  onStartPreview={
                    mode === "teacher" && canStartStaffPreview
                      ? () => setIsPreviewModalOpen(true)
                      : undefined
                  }
                />
              )}
            </AnchoredPopover>
          </div>
        </div>
      </div>

      <LogoutModal
        isOpen={isLogoutModalOpen}
        onClose={() => setIsLogoutModalOpen(false)}
      />
      {mode === "teacher" && canStartStaffPreview && (
        <StaffPreviewModal
          isOpen={isPreviewModalOpen}
          onClose={() => setIsPreviewModalOpen(false)}
        />
      )}
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
    // Prefer the precise session key (#2265); the room key is the legacy
    // fallback for state written before session tracking existed.
    const sessionId = localStorage.getItem("supervision-last-session");
    if (sessionId) return `/active-supervisions?session=${sessionId}`;
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

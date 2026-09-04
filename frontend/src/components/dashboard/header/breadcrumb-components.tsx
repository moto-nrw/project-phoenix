// Breadcrumb UI components for header navigation
// Extracted to reduce cognitive complexity in header.tsx

"use client";

import { NavLink } from "~/components/ui/nav-link";
import { useTranslations } from "next-intl";

import {
  ENROLLMENT_SECTION,
  ENROLLMENT_SUB_PAGES,
} from "~/lib/section-navigation";
import { useTenantAwarePath } from "~/lib/tenant-path";

// Die Hub-Seite der Anmeldungen ("Überblick") — erster Katalogeintrag, nicht
// der Sektionsname. Beide stehen in der Breadcrumb übereinander.
const ENROLLMENT_HUB_PAGE = ENROLLMENT_SUB_PAGES[0]!;

/**
 * Chevron separator icon for breadcrumbs
 */
function BreadcrumbSeparator() {
  return (
    <svg
      className="h-4 w-4 text-gray-400"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M9 5l7 7-7 7"
      />
    </svg>
  );
}

/**
 * Breadcrumb link component
 */
interface BreadcrumbLinkProps {
  readonly href: string;
  readonly children: React.ReactNode;
  readonly onClick?: () => void;
}

function BreadcrumbLink({ href, children, onClick }: BreadcrumbLinkProps) {
  const tenantPath = useTenantAwarePath();

  return (
    <NavLink
      href={tenantPath(href)}
      onClick={onClick}
      className="font-medium text-gray-500 transition-colors hover:text-gray-900"
    >
      {children}
    </NavLink>
  );
}

/**
 * Current breadcrumb item (not a link)
 */
interface BreadcrumbCurrentProps {
  readonly children: React.ReactNode;
}

function BreadcrumbCurrent({ children }: BreadcrumbCurrentProps) {
  return <span className="font-medium text-gray-900">{children}</span>;
}

/**
 * Breadcrumb container with responsive text sizing
 */
interface BreadcrumbNavProps {
  readonly children: React.ReactNode;
  readonly isScrolled?: boolean;
}

function BreadcrumbNav({ children, isScrolled = false }: BreadcrumbNavProps) {
  return (
    <nav
      className={`hidden items-center space-x-2 transition-all duration-300 md:flex ${
        isScrolled ? "text-sm" : "text-base"
      }`}
    >
      {children}
    </nav>
  );
}

/**
 * Simple page title display (for pages without breadcrumb trail)
 */
interface PageTitleDisplayProps {
  readonly title: string;
  readonly isScrolled?: boolean;
}

export function PageTitleDisplay({
  title,
  isScrolled = false,
}: PageTitleDisplayProps) {
  return (
    <span
      className={`hidden font-medium text-gray-600 transition-all duration-300 md:inline ${
        isScrolled ? "text-sm" : "text-base"
      }`}
    >
      {title}
    </span>
  );
}

/**
 * Breadcrumb einer gruppierten Navigationssektion: "Sektion › Seite", optional
 * mit dritter Stufe. Eine Komponente für Datenverwaltung, Planung und Eltern,
 * damit die drei Bereiche nicht auseinanderdriften.
 *
 * Ohne `sectionHref` steht der Sektionsname als reiner Text — die Planung hat
 * keine Hub-Seite, auf die man verlinken könnte.
 */
interface SectionBreadcrumbProps {
  readonly sectionLabel: string;
  readonly sectionHref?: string;
  readonly pageLabel: string;
  readonly pageHref?: string;
  readonly deepLabel?: string;
  readonly isScrolled?: boolean;
}

export function SectionBreadcrumb({
  sectionLabel,
  sectionHref,
  pageLabel,
  pageHref,
  deepLabel,
  isScrolled = false,
}: SectionBreadcrumbProps) {
  return (
    <BreadcrumbNav isScrolled={isScrolled}>
      {sectionHref ? (
        <BreadcrumbLink href={sectionHref}>{sectionLabel}</BreadcrumbLink>
      ) : (
        <span className="font-medium text-gray-500">{sectionLabel}</span>
      )}
      <BreadcrumbSeparator />
      {deepLabel ? (
        <>
          {pageHref ? (
            <BreadcrumbLink href={pageHref}>{pageLabel}</BreadcrumbLink>
          ) : (
            <span className="font-medium text-gray-500">{pageLabel}</span>
          )}
          <BreadcrumbSeparator />
          <BreadcrumbCurrent>{deepLabel}</BreadcrumbCurrent>
        </>
      ) : (
        <BreadcrumbCurrent>{pageLabel}</BreadcrumbCurrent>
      )}
    </BreadcrumbNav>
  );
}

/**
 * Breadcrumb eines Akkordeon-Bereichs ohne verlinkbare Hub-Seite: der
 * Bereichsname als Text, dahinter der gewählte Eintrag. Ohne Eintrag bleibt
 * nur der Bereichsname als Seitentitel.
 *
 * Die beiden Bereiche hatten diese Struktur vorher je einmal von Hand
 * nachgebaut — und dabei `isScrolled` nicht durchgereicht, weshalb ihre
 * Kopfzeile als einzige beim Scrollen nicht mitschrumpfte.
 */
interface AccordionSectionBreadcrumbProps {
  readonly sectionLabel: string;
  readonly itemName?: string;
  readonly isScrolled?: boolean;
}

function AccordionSectionBreadcrumb({
  sectionLabel,
  itemName,
  isScrolled = false,
}: AccordionSectionBreadcrumbProps) {
  if (!itemName) {
    return <PageTitleDisplay title={sectionLabel} isScrolled={isScrolled} />;
  }
  return (
    <SectionBreadcrumb
      sectionLabel={sectionLabel}
      pageLabel={itemName}
      isScrolled={isScrolled}
    />
  );
}

/**
 * OGS Groups breadcrumb with optional group name
 */
interface OgsGroupsBreadcrumbProps {
  readonly groupName?: string;
  readonly isScrolled?: boolean;
}

export function OgsGroupsBreadcrumb({
  groupName,
  isScrolled,
}: OgsGroupsBreadcrumbProps) {
  return (
    <AccordionSectionBreadcrumb
      sectionLabel="Meine Gruppe"
      itemName={groupName}
      isScrolled={isScrolled}
    />
  );
}

/**
 * Active Supervisions breadcrumb with optional room name
 */
interface ActiveSupervisionsBreadcrumbProps {
  readonly supervisionName?: string;
  readonly isScrolled?: boolean;
}

export function ActiveSupervisionsBreadcrumb({
  supervisionName,
  isScrolled,
}: ActiveSupervisionsBreadcrumbProps) {
  return (
    <AccordionSectionBreadcrumb
      sectionLabel="Aktuelle Aufsicht"
      itemName={supervisionName}
      isScrolled={isScrolled}
    />
  );
}

interface EnrollmentBreadcrumbProps {
  readonly current: string;
  readonly pathname?: string;
  readonly isScrolled?: boolean;
}

export function EnrollmentBreadcrumb({
  current,
  pathname,
  isScrolled = false,
}: EnrollmentBreadcrumbProps) {
  const nestedCurrent =
    pathname?.startsWith("/admin/enrollments/phases/") ||
    pathname?.startsWith("/admin/enrollments/")
      ? current
      : null;

  return (
    <BreadcrumbNav isScrolled={isScrolled}>
      <BreadcrumbLink href={ENROLLMENT_SECTION.href}>
        {ENROLLMENT_SECTION.label}
      </BreadcrumbLink>
      <BreadcrumbSeparator />
      {nestedCurrent ? (
        <>
          <BreadcrumbLink href={ENROLLMENT_HUB_PAGE.href}>
            {ENROLLMENT_HUB_PAGE.label}
          </BreadcrumbLink>
          <BreadcrumbSeparator />
          <BreadcrumbCurrent>{nestedCurrent}</BreadcrumbCurrent>
        </>
      ) : (
        <BreadcrumbCurrent>{current}</BreadcrumbCurrent>
      )}
    </BreadcrumbNav>
  );
}

interface ParentChildBreadcrumbProps {
  readonly childName: string;
  readonly isScrolled?: boolean;
}

export function ParentChildBreadcrumb({
  childName,
  isScrolled = false,
}: ParentChildBreadcrumbProps) {
  const t = useTranslations("parentNav");
  return (
    <BreadcrumbNav isScrolled={isScrolled}>
      <BreadcrumbLink href="/parents/children">{t("children")}</BreadcrumbLink>
      <BreadcrumbSeparator />
      <BreadcrumbCurrent>{childName}</BreadcrumbCurrent>
    </BreadcrumbNav>
  );
}

/**
 * Student history breadcrumb (3-level)
 */
interface StudentHistoryBreadcrumbProps {
  readonly referrer: string;
  readonly breadcrumbLabel: string;
  readonly pathname: string;
  readonly studentName: string;
  readonly historyType: string;
  readonly isScrolled?: boolean;
  readonly subSectionName?: string;
}

export function StudentHistoryBreadcrumb({
  referrer,
  breadcrumbLabel,
  pathname,
  studentName,
  historyType,
  isScrolled = false,
  subSectionName,
}: StudentHistoryBreadcrumbProps) {
  return (
    <BreadcrumbNav isScrolled={isScrolled}>
      <BreadcrumbLink href={referrer}>{breadcrumbLabel}</BreadcrumbLink>
      <BreadcrumbSeparator />
      {subSectionName ? (
        <>
          <BreadcrumbLink href={referrer}>{subSectionName}</BreadcrumbLink>
          <BreadcrumbSeparator />
        </>
      ) : null}
      <BreadcrumbLink href={pathname.split("/").slice(0, 3).join("/")}>
        {studentName}
      </BreadcrumbLink>
      <BreadcrumbSeparator />
      <BreadcrumbCurrent>{historyType}</BreadcrumbCurrent>
    </BreadcrumbNav>
  );
}

/**
 * Student detail breadcrumb (2-level, contextual)
 */
interface StudentDetailBreadcrumbProps {
  readonly referrer: string;
  readonly breadcrumbLabel: string;
  readonly studentName: string;
  readonly isScrolled?: boolean;
  readonly subSectionName?: string;
}

export function StudentDetailBreadcrumb({
  referrer,
  breadcrumbLabel,
  studentName,
  isScrolled = false,
  subSectionName,
}: StudentDetailBreadcrumbProps) {
  return (
    <BreadcrumbNav isScrolled={isScrolled}>
      <BreadcrumbLink href={referrer}>{breadcrumbLabel}</BreadcrumbLink>
      <BreadcrumbSeparator />
      {subSectionName ? (
        <>
          <BreadcrumbLink href={referrer}>{subSectionName}</BreadcrumbLink>
          <BreadcrumbSeparator />
        </>
      ) : null}
      <BreadcrumbCurrent>{studentName}</BreadcrumbCurrent>
    </BreadcrumbNav>
  );
}

/**
 * Staff detail breadcrumb (2-level: Mitarbeiter / Name)
 */
interface StaffDetailBreadcrumbProps {
  readonly staffName: string;
  readonly isScrolled?: boolean;
}

export function StaffDetailBreadcrumb({
  staffName,
  isScrolled = false,
}: StaffDetailBreadcrumbProps) {
  return (
    <BreadcrumbNav isScrolled={isScrolled}>
      <BreadcrumbLink href="/staff">Mitarbeiter</BreadcrumbLink>
      <BreadcrumbSeparator />
      <BreadcrumbCurrent>{staffName}</BreadcrumbCurrent>
    </BreadcrumbNav>
  );
}

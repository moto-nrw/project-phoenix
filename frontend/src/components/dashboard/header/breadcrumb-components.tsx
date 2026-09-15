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
 * Breadcrumb container. Eine feste Textgröße für die 48px hohe Kopfzeile
 * (#2827): das Schrumpfen beim Scrollen ist mit ihr entfallen.
 */
interface BreadcrumbNavProps {
  readonly children: React.ReactNode;
}

function BreadcrumbNav({ children }: BreadcrumbNavProps) {
  return (
    <nav className="hidden items-center space-x-2 text-sm md:flex">
      {children}
    </nav>
  );
}

/**
 * Simple page title display (for pages without breadcrumb trail)
 */
interface PageTitleDisplayProps {
  readonly title: string;
}

export function PageTitleDisplay({ title }: PageTitleDisplayProps) {
  return (
    <span className="hidden text-sm font-medium text-gray-600 md:inline">
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
}

export function SectionBreadcrumb({
  sectionLabel,
  sectionHref,
  pageLabel,
  pageHref,
  deepLabel,
}: SectionBreadcrumbProps) {
  return (
    <BreadcrumbNav>
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
 * nachgebaut; eine Komponente hält sie deckungsgleich.
 */
interface AccordionSectionBreadcrumbProps {
  readonly sectionLabel: string;
  readonly itemName?: string;
}

function AccordionSectionBreadcrumb({
  sectionLabel,
  itemName,
}: AccordionSectionBreadcrumbProps) {
  if (!itemName) {
    return <PageTitleDisplay title={sectionLabel} />;
  }
  return <SectionBreadcrumb sectionLabel={sectionLabel} pageLabel={itemName} />;
}

/**
 * OGS Groups breadcrumb with optional group name
 */
interface OgsGroupsBreadcrumbProps {
  readonly groupName?: string;
}

export function OgsGroupsBreadcrumb({ groupName }: OgsGroupsBreadcrumbProps) {
  return (
    <AccordionSectionBreadcrumb
      sectionLabel="Meine Gruppe"
      itemName={groupName}
    />
  );
}

/**
 * Active Supervisions breadcrumb with optional room name
 */
interface ActiveSupervisionsBreadcrumbProps {
  readonly supervisionName?: string;
}

export function ActiveSupervisionsBreadcrumb({
  supervisionName,
}: ActiveSupervisionsBreadcrumbProps) {
  return (
    <AccordionSectionBreadcrumb
      sectionLabel="Aktuelle Aufsicht"
      itemName={supervisionName}
    />
  );
}

interface EnrollmentBreadcrumbProps {
  readonly current: string;
  readonly pathname?: string;
}

export function EnrollmentBreadcrumb({
  current,
  pathname,
}: EnrollmentBreadcrumbProps) {
  const nestedCurrent =
    pathname?.startsWith("/admin/enrollments/phases/") ||
    pathname?.startsWith("/admin/enrollments/")
      ? current
      : null;

  return (
    <BreadcrumbNav>
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
}

export function ParentChildBreadcrumb({
  childName,
}: ParentChildBreadcrumbProps) {
  const t = useTranslations("parentNav");
  return (
    <BreadcrumbNav>
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
  readonly subSectionName?: string;
}

export function StudentHistoryBreadcrumb({
  referrer,
  breadcrumbLabel,
  pathname,
  studentName,
  historyType,
  subSectionName,
}: StudentHistoryBreadcrumbProps) {
  return (
    <BreadcrumbNav>
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
  readonly subSectionName?: string;
}

export function StudentDetailBreadcrumb({
  referrer,
  breadcrumbLabel,
  studentName,
  subSectionName,
}: StudentDetailBreadcrumbProps) {
  return (
    <BreadcrumbNav>
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
 * Staff detail breadcrumb: Mitarbeiter / Name, oder aus dem Register der
 * Datenverwaltung heraus Datenverwaltung / Personal / Name (#3115).
 */
interface StaffDetailBreadcrumbProps {
  readonly staffName: string;
  /** Die Sammlung, aus der die Personalakte geöffnet wurde (`?from=`). */
  readonly referrer?: string;
}

export function StaffDetailBreadcrumb({
  staffName,
  referrer = "/staff",
}: StaffDetailBreadcrumbProps) {
  const fromDatabase = referrer.startsWith("/database/personal");
  return (
    <BreadcrumbNav>
      {fromDatabase ? (
        <>
          <BreadcrumbLink href="/database">Datenverwaltung</BreadcrumbLink>
          <BreadcrumbSeparator />
          <BreadcrumbLink href={referrer}>Personal</BreadcrumbLink>
        </>
      ) : (
        <BreadcrumbLink href="/staff">Mitarbeiter</BreadcrumbLink>
      )}
      <BreadcrumbSeparator />
      <BreadcrumbCurrent>{staffName}</BreadcrumbCurrent>
    </BreadcrumbNav>
  );
}

interface AnnouncementDetailBreadcrumbProps {
  readonly title: string;
  /** Die Liste samt Reiter, aus der die Seite geöffnet wurde (`?from=`). */
  readonly referrer: string;
}

/** Die Mitteilungsseite /parent-announcements/[id] (#3115): Mitteilungen / Titel. */
export function AnnouncementDetailBreadcrumb({
  title,
  referrer,
}: AnnouncementDetailBreadcrumbProps) {
  return (
    <BreadcrumbNav>
      <BreadcrumbLink href={referrer}>Mitteilungen</BreadcrumbLink>
      <BreadcrumbSeparator />
      <BreadcrumbCurrent>{title}</BreadcrumbCurrent>
    </BreadcrumbNav>
  );
}

interface RoomDetailBreadcrumbProps {
  readonly roomName: string;
  /** Die Sammlung, aus der die Raumseite geöffnet wurde (`?from=`). */
  readonly referrer: string;
}

/**
 * Die Raumseite /rooms/[id] (#3115): zwei Stufen, die erste ist die
 * Sammlung, aus der man kam — die Übersicht „Räume" oder das Register der
 * Datenverwaltung.
 */
export function RoomDetailBreadcrumb({
  roomName,
  referrer,
}: RoomDetailBreadcrumbProps) {
  const fromDatabase = referrer.startsWith("/database/rooms");
  return (
    <BreadcrumbNav>
      {fromDatabase ? (
        <>
          <BreadcrumbLink href="/database">Datenverwaltung</BreadcrumbLink>
          <BreadcrumbSeparator />
          <BreadcrumbLink href={referrer}>Räume</BreadcrumbLink>
        </>
      ) : (
        <BreadcrumbLink href={referrer}>Räume</BreadcrumbLink>
      )}
      <BreadcrumbSeparator />
      <BreadcrumbCurrent>{roomName}</BreadcrumbCurrent>
    </BreadcrumbNav>
  );
}

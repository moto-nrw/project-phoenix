// Breadcrumb UI components for header navigation
// Extracted to reduce cognitive complexity in header.tsx

"use client";

import Link from "next/link";

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
  return (
    <Link
      href={href}
      onClick={onClick}
      className="font-medium text-gray-500 transition-colors hover:text-gray-900"
    >
      {children}
    </Link>
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
 * Database breadcrumb with optional deep page support
 */
interface DatabaseBreadcrumbProps {
  readonly pathname: string;
  readonly pageTitle: string;
  readonly subPageLabel: string;
  readonly isDeepPage: boolean;
}

export function DatabaseBreadcrumb({
  pathname,
  pageTitle,
  subPageLabel,
  isDeepPage,
}: DatabaseBreadcrumbProps) {
  return (
    <BreadcrumbNav>
      <BreadcrumbLink href="/database">Datenbank</BreadcrumbLink>
      <BreadcrumbSeparator />
      {isDeepPage ? (
        <>
          <BreadcrumbLink href={pathname.split("/").slice(0, 3).join("/")}>
            {pageTitle}
          </BreadcrumbLink>
          <BreadcrumbSeparator />
          <BreadcrumbCurrent>{subPageLabel}</BreadcrumbCurrent>
        </>
      ) : (
        <BreadcrumbCurrent>{pageTitle}</BreadcrumbCurrent>
      )}
    </BreadcrumbNav>
  );
}

/**
 * OGS Groups breadcrumb with optional group name
 */
interface OgsGroupsBreadcrumbProps {
  readonly groupName?: string;
}

export function OgsGroupsBreadcrumb({ groupName }: OgsGroupsBreadcrumbProps) {
  if (groupName) {
    return (
      <BreadcrumbNav>
        <span className="font-medium text-gray-500">Meine Gruppe</span>
        <BreadcrumbSeparator />
        <BreadcrumbCurrent>{groupName}</BreadcrumbCurrent>
      </BreadcrumbNav>
    );
  }
  return <PageTitleDisplay title="Meine Gruppe" />;
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
  if (supervisionName) {
    return (
      <BreadcrumbNav>
        <span className="font-medium text-gray-500">Aktuelle Aufsicht</span>
        <BreadcrumbSeparator />
        <BreadcrumbCurrent>{supervisionName}</BreadcrumbCurrent>
      </BreadcrumbNav>
    );
  }
  return <PageTitleDisplay title="Aktuelle Aufsicht" />;
}

/**
 * Invitations breadcrumb (3-level)
 */
export function InvitationsBreadcrumb() {
  return (
    <BreadcrumbNav>
      <BreadcrumbLink href="/database">Datenverwaltung</BreadcrumbLink>
      <BreadcrumbSeparator />
      <BreadcrumbLink href="/database/teachers">Betreuer</BreadcrumbLink>
      <BreadcrumbSeparator />
      <BreadcrumbCurrent>Einladungen</BreadcrumbCurrent>
    </BreadcrumbNav>
  );
}

/**
 * Activity detail breadcrumb
 */
interface ActivityBreadcrumbProps {
  readonly activityName: string;
}

export function ActivityBreadcrumb({ activityName }: ActivityBreadcrumbProps) {
  return (
    <BreadcrumbNav>
      <BreadcrumbLink href="/activities">Aktivitäten</BreadcrumbLink>
      <BreadcrumbSeparator />
      <BreadcrumbCurrent>{activityName}</BreadcrumbCurrent>
    </BreadcrumbNav>
  );
}

/**
 * Room detail breadcrumb
 */
interface RoomBreadcrumbProps {
  readonly roomName: string;
}

export function RoomBreadcrumb({ roomName }: RoomBreadcrumbProps) {
  return (
    <BreadcrumbNav>
      <BreadcrumbLink href="/rooms">Räume</BreadcrumbLink>
      <BreadcrumbSeparator />
      <BreadcrumbCurrent>{roomName}</BreadcrumbCurrent>
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
}

export function StudentHistoryBreadcrumb({
  referrer,
  breadcrumbLabel,
  pathname,
  studentName,
  historyType,
  isScrolled = false,
}: StudentHistoryBreadcrumbProps) {
  return (
    <BreadcrumbNav isScrolled={isScrolled}>
      <BreadcrumbLink href={referrer}>{breadcrumbLabel}</BreadcrumbLink>
      <BreadcrumbSeparator />
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
}

export function StudentDetailBreadcrumb({
  referrer,
  breadcrumbLabel,
  studentName,
  isScrolled = false,
}: StudentDetailBreadcrumbProps) {
  return (
    <BreadcrumbNav isScrolled={isScrolled}>
      <BreadcrumbLink href={referrer}>{breadcrumbLabel}</BreadcrumbLink>
      <BreadcrumbSeparator />
      <BreadcrumbCurrent>{studentName}</BreadcrumbCurrent>
    </BreadcrumbNav>
  );
}

"use client";

import type React from "react";
import { LocationBadge } from "@/components/ui/location-badge";
import type { ExtendedStudent } from "~/lib/hooks/use-student-data";
import type { SupervisorContact } from "~/lib/student-helpers";
import { InfoCard, InfoItem } from "~/components/ui/info-card";

// =============================================================================
// ICONS - Reusable SVG icons
// =============================================================================

function GroupIcon({
  className = "h-4 w-4",
}: Readonly<{ className?: string }>) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
      />
    </svg>
  );
}

export function PersonIcon({
  className = "h-5 w-5",
}: Readonly<{ className?: string }>) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
      />
    </svg>
  );
}

function ContactIcon({
  className = "h-5 w-5",
}: Readonly<{ className?: string }>) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M10 6H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V8a2 2 0 00-2-2h-5m-4 0V5a2 2 0 114 0v1m-4 0a2 2 0 104 0m-5 8a2 2 0 100-4 2 2 0 000 4zm0 0c1.306 0 2.417.835 2.83 2M9 14a3.001 3.001 0 00-2.83 2M15 11h3m-3 4h2"
      />
    </svg>
  );
}

function ViewOnlyIcon({
  className = "h-3 w-3 sm:h-3.5 sm:w-3.5",
}: Readonly<{ className?: string }>) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
      />
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
      />
    </svg>
  );
}

function EmailIcon({
  className = "h-4 w-4",
}: Readonly<{ className?: string }>) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
      />
    </svg>
  );
}

function ClockIcon({
  className = "h-5 w-5",
}: Readonly<{ className?: string }>) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
      />
    </svg>
  );
}

function ChevronRightIcon({
  className = "h-4 w-4",
}: Readonly<{ className?: string }>) {
  return (
    <svg
      className={className}
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

export function ChevronDownIcon({
  className = "h-4 w-4",
}: Readonly<{ className?: string }>) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M19 9l-7 7-7-7"
      />
    </svg>
  );
}

export function WarningIcon({
  className = "h-5 w-5",
}: Readonly<{ className?: string }>) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
      />
    </svg>
  );
}

function BuildingIcon({
  className = "h-4 w-4 text-white",
}: Readonly<{ className?: string }>) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
      />
    </svg>
  );
}

function ChatIcon({
  className = "h-4 w-4 text-white",
}: Readonly<{ className?: string }>) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M7 8h10M7 12h4m1 8l-4-4H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-3l-4 4z"
      />
    </svg>
  );
}

function ForkKnifeIcon({
  className = "h-4 w-4 text-white",
}: Readonly<{ className?: string }>) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M8.5 3v18M7 3v3.5M10 3v3.5M7 10h3M15.5 3v3c0 1-2 2-2 2v13"
      />
    </svg>
  );
}

// =============================================================================
// VIEW ONLY BADGE
// =============================================================================

function ViewOnlyBadge() {
  return (
    <span className="inline-flex flex-shrink-0 items-center gap-1 rounded-md bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 sm:px-2.5">
      <ViewOnlyIcon />
      <span className="hidden sm:inline">Nur Ansicht</span>
      <span className="sm:hidden">Ansicht</span>
    </span>
  );
}

// =============================================================================
// STUDENT HEADER
// =============================================================================

interface StudentHeaderProps {
  student: ExtendedStudent;
  myGroups: string[];
  myGroupRooms: string[];
  mySupervisedRooms: string[];
  todayPickupTime?: string;
  todayPickupNote?: string;
  isPickupException?: boolean;
}

export function StudentDetailHeader({
  student,
  myGroups,
  myGroupRooms,
  mySupervisedRooms,
  todayPickupTime,
  todayPickupNote,
  isPickupException,
}: Readonly<StudentHeaderProps>) {
  const badgeStudent = {
    current_location: student.current_location,
    location_since: student.location_since,
    group_id: student.group_id,
    group_name: student.group_name,
    sick: student.sick,
    sick_since: student.sick_since,
  };

  return (
    <div className="mb-6">
      <div className="flex items-end justify-between gap-4">
        <div className="ml-6 flex-1">
          <h1 className="text-2xl font-bold text-gray-900 md:text-3xl">
            {student.first_name} {student.second_name}
          </h1>
          {student.group_name && (
            <div className="mt-2 flex items-center gap-2 text-sm text-gray-600">
              <GroupIcon className="h-4 w-4 text-gray-400" />
              <span className="truncate">{student.group_name}</span>
            </div>
          )}
          {todayPickupTime && (
            <div className="mt-1.5 flex items-center gap-2 text-sm text-gray-600">
              <ClockIcon className="h-4 w-4 text-gray-400" />
              <span>
                Heutige Abholung:{" "}
                <span className="font-medium text-gray-900">
                  {todayPickupTime}
                </span>
                {todayPickupNote && (
                  <span className="ml-1 text-gray-500">
                    ({todayPickupNote})
                  </span>
                )}
                {isPickupException && (
                  <span
                    className="ml-1.5 inline-flex h-2 w-2 rounded-full bg-orange-400"
                    title="Ausnahme"
                  />
                )}
              </span>
            </div>
          )}
        </div>
        <div className="mr-4 flex-shrink-0 pb-3">
          <LocationBadge
            student={badgeStudent}
            displayMode="contextAware"
            userGroups={myGroups}
            groupRooms={myGroupRooms}
            supervisedRooms={mySupervisedRooms}
            variant="modern"
            size="md"
            showLocationSince={true}
          />
        </div>
      </div>
    </div>
  );
}

// =============================================================================
// SUPERVISORS CARD
// =============================================================================

interface SupervisorsCardProps {
  supervisors: SupervisorContact[];
  studentName: string;
}

export function SupervisorsCard({
  supervisors,
  studentName,
}: Readonly<SupervisorsCardProps>) {
  if (supervisors.length === 0) return null;

  return (
    <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6">
      <div className="mb-4 flex items-center justify-between gap-2">
        <div className="flex min-w-0 flex-1 items-center gap-2 sm:gap-3">
          <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-600 sm:h-10 sm:w-10">
            <ContactIcon />
          </div>
          <h2 className="truncate text-base font-semibold text-gray-900 sm:text-lg">
            Ansprechpartner
          </h2>
        </div>
        <ViewOnlyBadge />
      </div>

      <div className="space-y-4">
        {supervisors.map((supervisor, index) => (
          <SupervisorItem
            key={supervisor.id}
            supervisor={supervisor}
            studentName={studentName}
            showDivider={index > 0}
          />
        ))}
      </div>
    </div>
  );
}

interface SupervisorItemProps {
  supervisor: SupervisorContact;
  studentName: string;
  showDivider: boolean;
}

function SupervisorItem({
  supervisor,
  studentName,
  showDivider,
}: Readonly<SupervisorItemProps>) {
  const handleEmailClick = () => {
    globalThis.location.href = `mailto:${supervisor.email}?subject=Anfrage zu ${studentName}`;
  };

  return (
    <div>
      {showDivider && <div className="my-4 border-t border-gray-100" />}
      <div>
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-base font-semibold text-gray-900">
            {supervisor.first_name} {supervisor.last_name}
          </p>
          <span className="inline-flex items-center rounded-full bg-gray-100 px-2.5 py-0.5 text-xs font-medium text-gray-700">
            Gruppenleitung
          </span>
        </div>
        {supervisor.email && (
          <>
            <p className="mt-1 text-sm text-gray-500">{supervisor.email}</p>
            <button
              onClick={handleEmailClick}
              className="mt-3 inline-flex items-center gap-2 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:bg-gray-700 hover:shadow-lg active:scale-[0.98]"
            >
              <EmailIcon />
              Kontakt aufnehmen
            </button>
          </>
        )}
      </div>
    </div>
  );
}

// =============================================================================
// PERSONAL INFO READ ONLY
// =============================================================================

interface PersonalInfoReadOnlyProps {
  student: ExtendedStudent;
  showEditButton?: boolean;
  onEditClick?: () => void;
}

export function PersonalInfoReadOnly({
  student,
  showEditButton = false,
  onEditClick,
}: Readonly<PersonalInfoReadOnlyProps>) {
  const birthdayDisplay = student.birthday
    ? new Date(student.birthday).toLocaleDateString("de-DE", {
        day: "2-digit",
        month: "2-digit",
        year: "numeric",
      })
    : "Nicht angegeben";

  return (
    <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6">
      <div className="mb-4 flex items-center justify-between gap-2">
        <div className="flex min-w-0 flex-1 items-center gap-2 sm:gap-3">
          <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-[#83CD2D]/10 text-[#83CD2D] sm:h-10 sm:w-10">
            <PersonIcon />
          </div>
          <h2 className="truncate text-base font-semibold text-gray-900 sm:text-lg">
            Persönliche Informationen
          </h2>
        </div>
        {showEditButton && onEditClick ? (
          <button
            onClick={onEditClick}
            className="rounded-lg px-3 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100"
            title="Bearbeiten"
          >
            Bearbeiten
          </button>
        ) : (
          <ViewOnlyBadge />
        )}
      </div>
      <div className="space-y-3">
        <InfoItem label="Vollständiger Name" value={student.name} />
        <InfoItem label="Klasse" value={student.school_class} />
        <InfoItem
          label="Gruppe"
          value={student.group_name ?? "Nicht zugewiesen"}
        />
        <InfoItem label="Geburtsdatum" value={birthdayDisplay} />
        <InfoItem label="Buskind" value={student.buskind ? "Ja" : "Nein"} />
        <InfoItem
          label="Abholstatus"
          value={student.pickup_status ?? "Nicht gesetzt"}
        />
        <InfoItem
          label="Krankheitsstatus"
          value={<SicknessStatus student={student} />}
        />
        {student.health_info && (
          <InfoItem
            label="Gesundheitsinformationen"
            value={student.health_info}
          />
        )}
        {student.supervisor_notes && (
          <InfoItem label="Betreuernotizen" value={student.supervisor_notes} />
        )}
        {student.extra_info && (
          <InfoItem label="Elternnotizen" value={student.extra_info} />
        )}
      </div>
    </div>
  );
}

// =============================================================================
// FULL ACCESS PERSONAL INFO (with sickness status)
// =============================================================================

interface FullAccessPersonalInfoReadOnlyProps {
  student: ExtendedStudent;
  onEditClick: () => void;
}

export function FullAccessPersonalInfoReadOnly({
  student,
  onEditClick,
}: Readonly<FullAccessPersonalInfoReadOnlyProps>) {
  const birthdayDisplay = student.birthday
    ? new Date(student.birthday).toLocaleDateString("de-DE", {
        day: "2-digit",
        month: "2-digit",
        year: "numeric",
      })
    : "Nicht angegeben";

  return (
    <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6">
      <div className="mb-4 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-[#83CD2D]/10 text-[#83CD2D] sm:h-10 sm:w-10">
            <PersonIcon />
          </div>
          <h2 className="text-base font-semibold text-gray-900 sm:text-lg">
            Persönliche Informationen
          </h2>
        </div>
        <button
          onClick={onEditClick}
          className="rounded-lg px-3 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100"
          title="Bearbeiten"
        >
          Bearbeiten
        </button>
      </div>
      <div className="space-y-3">
        <InfoItem label="Vollständiger Name" value={student.name} />
        <InfoItem label="Klasse" value={student.school_class} />
        <InfoItem
          label="Gruppe"
          value={student.group_name ?? "Nicht zugewiesen"}
        />
        <InfoItem label="Geburtsdatum" value={birthdayDisplay} />
        <InfoItem label="Buskind" value={student.buskind ? "Ja" : "Nein"} />
        <InfoItem
          label="Abholstatus"
          value={student.pickup_status ?? "Nicht gesetzt"}
        />
        <InfoItem
          label="Krankheitsstatus"
          value={<SicknessStatus student={student} />}
        />
        {student.health_info && (
          <InfoItem
            label="Gesundheitsinformationen"
            value={student.health_info}
          />
        )}
        {student.supervisor_notes && (
          <InfoItem label="Betreuernotizen" value={student.supervisor_notes} />
        )}
        {student.extra_info && (
          <InfoItem label="Elternnotizen" value={student.extra_info} />
        )}
      </div>
    </div>
  );
}

function SicknessStatus({ student }: Readonly<{ student: ExtendedStudent }>) {
  if (!student.sick) {
    return (
      <span className="inline-flex items-center gap-1.5 rounded-full bg-green-100 px-2.5 py-1 text-xs font-medium text-green-800">
        <span className="h-2 w-2 rounded-full bg-green-500" />
        <span>Nicht krankgemeldet</span>
      </span>
    );
  }

  return (
    <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
      <span
        className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium text-white"
        style={{ backgroundColor: "#EAB308" }}
      >
        <span className="h-2 w-2 rounded-full bg-white/80" />
        <span>Krank</span>
      </span>
      {student.sick_since && (
        <span className="text-sm text-gray-500">
          seit{" "}
          {new Date(student.sick_since).toLocaleDateString("de-DE", {
            day: "2-digit",
            month: "2-digit",
            year: "numeric",
          })}
        </span>
      )}
    </div>
  );
}

// =============================================================================
// HISTORY SECTION
// =============================================================================

interface HistoryButtonProps {
  icon: React.ReactNode;
  title: string;
  description: string;
  bgColor: string;
}

function DisabledHistoryButton({
  icon,
  title,
  description,
  bgColor,
}: Readonly<HistoryButtonProps>) {
  return (
    <button
      type="button"
      disabled
      className="flex cursor-not-allowed items-center justify-between rounded-lg border border-gray-100 bg-gray-50 p-3 opacity-60"
    >
      <div className="flex items-center gap-3">
        <div
          className={`flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg sm:h-9 sm:w-9 ${bgColor}`}
        >
          {icon}
        </div>
        <div className="min-w-0 flex-1 text-left">
          <p className="text-sm font-medium text-gray-400 sm:text-base">
            {title}
          </p>
          <p className="text-xs text-gray-400">{description}</p>
        </div>
      </div>
      <ChevronRightIcon className="h-4 w-4 flex-shrink-0 text-gray-300" />
    </button>
  );
}

export function StudentHistorySection() {
  return (
    <InfoCard title="Historien" icon={<ClockIcon />}>
      <div className="grid grid-cols-1 gap-2">
        <DisabledHistoryButton
          icon={<BuildingIcon />}
          title="Raumverlauf"
          description="Verlauf der Raumbesuche"
          bgColor="bg-[#5080D8]"
        />
        <DisabledHistoryButton
          icon={<ChatIcon />}
          title="Feedbackhistorie"
          description="Feedback und Bewertungen"
          bgColor="bg-[#83CD2D]"
        />
        <DisabledHistoryButton
          icon={<ForkKnifeIcon />}
          title="Mensaverlauf"
          description="Mahlzeiten und Bestellungen"
          bgColor="bg-[#F78C10]"
        />
      </div>
    </InfoCard>
  );
}

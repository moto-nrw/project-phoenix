"use client";

import { useMemo } from "react";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import NavigationLink from "~/components/ui/navigation-link";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type { HolidayHomeSchoolBooking } from "~/lib/holiday-host-api";
import { holidayDayList } from "~/lib/holiday-days";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { useTenantAwarePath } from "~/lib/tenant-path";

/** „04.01. bis 08.01.2027“ */
function weekRange(booking: HolidayHomeSchoolBooking): string {
  return `${formatDate(booking.startsOn).slice(0, 6)} bis ${formatDate(booking.endsOn)}`;
}

/** Ob die Woche heute läuft oder noch kommt; vorbei ist sie hier nie. */
function statusBadge(booking: HolidayHomeSchoolBooking, today: string) {
  return booking.startsOn <= today ? (
    <StatusBadge tone="green" label="Läuft gerade" />
  ) : (
    <StatusBadge tone="blue" label="Bestätigt" />
  );
}

function columns(
  today: string,
  studentPath: (studentId: string) => string,
): DataTableColumn<HolidayHomeSchoolBooking>[] {
  return [
    {
      key: "name",
      header: "Kind",
      stacked: "title",
      render: (child) => (
        <span className="flex flex-col">
          <span className="font-medium text-gray-900">
            {child.lastName}, {child.firstName}
          </span>
          <span className="text-gray-500">
            geboren am {formatDate(child.birthDate)}
          </span>
        </span>
      ),
      stackedRender: (child) => `${child.lastName}, ${child.firstName}`,
      sortValue: (child) => `${child.lastName} ${child.firstName}`,
    },
    {
      key: "week",
      header: "Woche",
      render: (child) => (
        <span className="flex flex-col">
          <span className="whitespace-nowrap text-gray-900">
            {weekRange(child)}
          </span>
          <span className="text-gray-500">{child.periodName}</span>
        </span>
      ),
      sortValue: (child) => child.startsOn,
    },
    {
      key: "hostSchool",
      header: "Betreut von",
      render: (child) => child.hostSchoolName,
      sortValue: (child) => child.hostSchoolName,
    },
    {
      key: "timeModel",
      header: "Zeitmodell",
      render: (child) => (
        <span className="flex flex-col">
          <span className="text-gray-900">{child.timeModelName}</span>
          <span className="text-gray-500">
            {child.timeModelStartsAt} bis {child.timeModelEndsAt} Uhr
            {" · "}
            {child.days.length > 0 ? holidayDayList(child.days) : "ganze Woche"}
          </span>
        </span>
      ),
    },
    {
      key: "status",
      header: "Status",
      stacked: "meta",
      render: (child) => statusBadge(child, today),
    },
    {
      key: "studentFile",
      header: "Kinderakte",
      render: (child) =>
        child.studentId === null ? (
          <StatusBadge tone="warning" label="Nicht zugeordnet" />
        ) : (
          <NavigationLink
            className="whitespace-nowrap underline"
            href={studentPath(child.studentId)}
            aria-label={`Akte öffnen: ${child.firstName} ${child.lastName}`}
          >
            Akte öffnen
          </NavigationLink>
        ),
    },
  ];
}

/**
 * Kinder dieser OGS in Ferienwochen anderer OGS des Trägers (#6679): Die
 * Heimatschule sieht, welche ihrer Kinder in den Ferien woanders betreut
 * werden, mit Woche, betreuender OGS, Zeitmodell und Status. Nur bestätigte
 * Anmeldungen, bis zum letzten Tag der Woche. Ohne Gesundheitsangaben und
 * Kontakte: Die bleiben bei der OGS, die das Kind betreut. Nur zur
 * Information; Anmeldungen ändert der Träger.
 */
export function HolidayHomeSchoolBookings({
  bookings,
}: Readonly<{ bookings: readonly HolidayHomeSchoolBooking[] }>) {
  const today = useBerlinToday();
  const tenantPath = useTenantAwarePath();
  const tableColumns = useMemo(
    () =>
      columns(today, (studentId) =>
        tenantPath(`/students/${encodeURIComponent(studentId)}`),
      ),
    [today, tenantPath],
  );
  const unassigned = bookings.filter((child) => child.studentId === null);

  return (
    <SectionCard
      title="Ihre Kinder in Ferienwochen anderer OGS"
      description="Nur zur Information. Der Träger hat diese Anmeldungen bestätigt. Änderungen macht der Träger."
      action={
        unassigned.length > 0 ? (
          <StatusBadge
            tone="warning"
            label={`${unassigned.length} nicht zugeordnet`}
          />
        ) : undefined
      }
    >
      <div className="flex flex-col gap-4">
        <DataTable
          columns={tableColumns}
          rows={[...bookings]}
          getRowKey={(child) => `${child.weekId}:${child.childId}`}
          defaultSortKey="week"
          stackedOnMobile
          emptyState={
            <EmptyState
              title="Kein Kind Ihrer OGS ist bei einer anderen OGS angemeldet"
              description="Hier stehen Kinder Ihrer OGS, deren Anmeldung der Träger für eine andere OGS bestätigt hat. Offene oder abgesagte Anmeldungen und Plätze auf der Warteliste stehen hier nicht. Nach dem letzten Tag der Woche ist das Kind nicht mehr da."
            />
          }
        />
        {unassigned.length > 0 && (
          <p className="text-sm text-gray-600">
            „Nicht zugeordnet“: moto findet kein Kind oder mehrere Kinder mit
            diesem Namen und Geburtsdatum. Prüfen Sie die Kinderakte.
          </p>
        )}
      </div>
    </SectionCard>
  );
}

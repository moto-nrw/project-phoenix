"use client";

import { useState } from "react";
import { TenantPage } from "~/components/ui/tenant-page";
import { Alert } from "~/components/ui/alert";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { DataTable } from "~/components/ui/data-table";
import { Button } from "~/components/ui/button";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";
import {
  staffHistoryService,
  type Staff,
  type StaffHistorySession,
} from "~/lib/staff-api";
import { useSWRAuth } from "~/lib/swr";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { StammdatenTab } from "./stammdaten-tab";

// This content stays on /staff/[id]. Its tabs never mount a current-employment
// editor, including when an old bookmark requests an editing tab.
export function FormerEmploymentDetails({
  staff,
  referrer,
  backLabel,
  canViewSections,
  canViewFinancial,
  canViewTimeTracking,
  canViewDocuments,
}: Readonly<{
  staff: Staff;
  referrer: string;
  backLabel: string;
  canViewSections: boolean;
  canViewFinancial: boolean;
  canViewTimeTracking: boolean;
  canViewDocuments: boolean;
}>) {
  const [tab, setTab] = useState("beschaeftigung");
  return (
    <TenantPage
      title={`${staff.firstName} ${staff.lastName}`}
      stats={`${staff.startedOn ? formatDate(staff.startedOn) : "Beginn nicht hinterlegt"} bis ${formatDate(staff.endedOn!)}`}
      back
      backHref={referrer}
      backLabel={backLabel}
      actions={
        <div className="flex">
          <StatusBadge label="Ehemalig" tone="gray" />
        </div>
      }
      tabs={{
        value: tab,
        onChange: setTab,
        label: "Bereiche der früheren Beschäftigung",
        items: [
          { value: "beschaeftigung", label: "Beschäftigung" },
          ...(canViewSections || canViewFinancial
            ? [{ value: "stammdaten", label: "Stammdaten" }]
            : []),
          ...(canViewTimeTracking
            ? [{ value: "zeiterfassung", label: "Zeiterfassung" }]
            : []),
          ...(canViewDocuments
            ? [{ value: "dokumente", label: "Dokumente" }]
            : []),
        ],
      }}
    >
      <Alert
        type="info"
        announce="off"
        message="Diese Beschäftigung ist beendet. Sie können die Angaben nur lesen."
      />
      {tab === "beschaeftigung" && (
        <SectionCard
          title="Frühere Beschäftigung"
          description="Dieser Zeitraum zeigt, wann die Person in moto eingetragen war."
        >
          <DataGrid>
            <DataField label="Beginn">
              {staff.startedOn
                ? formatDate(staff.startedOn)
                : "Nicht hinterlegt"}
            </DataField>
            <DataField label="Austritt">{formatDate(staff.endedOn!)}</DataField>
            {staff.role && <DataField label="Position">{staff.role}</DataField>}
            {staff.qualifications && (
              <DataField label="Qualifikationen">
                {staff.qualifications}
              </DataField>
            )}
            {staff.staffNotes && (
              <DataField label="Notizen">{staff.staffNotes}</DataField>
            )}
          </DataGrid>
        </SectionCard>
      )}
      {tab === "stammdaten" && (
        <StammdatenTab
          staffId={staff.id}
          readOnly
          canViewSections={canViewSections}
          canViewFinancial={canViewFinancial}
          canManagePayroll={false}
          canManagePayrollSettings={false}
        />
      )}
      {tab === "zeiterfassung" && (
        <FormerEmploymentTimes key={staff.id} staff={staff} />
      )}
      {tab === "dokumente" && (
        <SectionCard
          title="Personaldokumente"
          description="Die Personaldokumente wurden beim Austritt endgültig gelöscht."
        />
      )}
    </TenantPage>
  );
}

function monthRange(anchor: string, staff: Staff) {
  const date = parseISODate(anchor);
  const first = toISODate(new Date(date.getFullYear(), date.getMonth(), 1));
  const last = toISODate(new Date(date.getFullYear(), date.getMonth() + 1, 0));
  return {
    from: staff.startedOn && staff.startedOn > first ? staff.startedOn : first,
    to: staff.endedOn && staff.endedOn < last ? staff.endedOn : last,
  };
}

function FormerEmploymentTimes({ staff }: Readonly<{ staff: Staff }>) {
  const [anchor, setAnchor] = useState(staff.endedOn!);
  const { from, to } = monthRange(anchor, staff);
  const { data, error, isLoading, mutate } = useSWRAuth<StaffHistorySession[]>(
    `staff-history-${staff.id}-${from}-${to}`,
    () => staffHistoryService.getHistory(staff.id, from, to),
  );
  const loadError = useSwrLoadError(error, "die Zeiterfassung", mutate);
  const moveMonth = (delta: number) => {
    const current = parseISODate(anchor);
    setAnchor(
      toISODate(new Date(current.getFullYear(), current.getMonth() + delta, 1)),
    );
  };
  const month = anchor.slice(0, 7);
  return (
    <SectionCard
      title="Zeiterfassung"
      description="Hier sehen Sie die erfassten Arbeitszeiten der früheren Beschäftigung."
    >
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <Button
          variant="secondary"
          size="sm"
          className="min-h-11 flex-1 sm:min-h-0 sm:flex-none"
          disabled={Boolean(
            staff.startedOn && month <= staff.startedOn.slice(0, 7),
          )}
          onClick={() => moveMonth(-1)}
        >
          Vorheriger Monat
        </Button>
        <span className="order-first w-full text-center sm:order-none sm:w-auto">
          {formatDate(from)} bis {formatDate(to)}
        </span>
        <Button
          variant="secondary"
          size="sm"
          className="min-h-11 flex-1 sm:min-h-0 sm:flex-none"
          disabled={month >= staff.endedOn!.slice(0, 7)}
          onClick={() => moveMonth(1)}
        >
          Nächster Monat
        </Button>
      </div>
      {error ? (
        <LoadErrorAlert error={loadError} />
      ) : (
        <DataTable
          caption="Erfasste Arbeitszeiten"
          rows={data ?? []}
          isLoading={isLoading}
          stackedOnMobile
          getRowKey={(row) => row.id ?? row.check_in_time}
          emptyState="In diesem Zeitraum wurden keine Arbeitszeiten erfasst."
          columns={[
            {
              key: "date",
              header: "Datum",
              render: (row) => formatDate(row.date),
            },
            {
              key: "in",
              header: "Beginn",
              render: (row) => formatInstant(row.check_in_time),
            },
            {
              key: "out",
              header: "Ende",
              render: (row) =>
                row.check_out_time
                  ? formatInstant(row.check_out_time)
                  : "Nicht erfasst",
            },
            {
              key: "minutes",
              header: "Arbeitszeit",
              render: (row) =>
                `${Math.floor(row.net_minutes / 60)} Std. ${row.net_minutes % 60} Min.`,
            },
            {
              key: "notes",
              header: "Notizen",
              render: (row) => row.notes ?? "–",
            },
          ]}
        />
      )}
    </SectionCard>
  );
}

function formatInstant(value: string) {
  return new Date(value).toLocaleString("de-DE", {
    timeZone: "Europe/Berlin",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

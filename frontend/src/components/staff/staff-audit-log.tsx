"use client";

// Cross-staff Änderungsprotokoll (#1417): who changed whose working time,
// when, and why — across session edits, absence decisions, Stundenkonto
// bookings, month close/reopen, and deletion tombstones. Rendered only with
// time_tracking:manage (the page never mounts it otherwise, so no request
// fires without the permission).

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { ISODatePicker } from "~/components/ui/date-picker";
import {
  FIELD_LABELS,
  formatEditValue,
} from "~/components/time-tracking/edit-history-accordion";
import { createLogger } from "~/lib/logger";
import {
  auditLogSourceLabels,
  staffAuditLogService,
  type AuditLogEvent,
  type AuditLogSource,
} from "~/lib/staff-audit-log-api";
import { formatDate } from "~/lib/date-helpers";
import { formatSignedDuration } from "~/components/staff/staff-time-views";
import {
  absenceTypeLabels,
  balanceAdjustmentTypeLabel,
  type AbsenceType,
  type BalanceAdjustmentType,
} from "~/lib/time-tracking-helpers";

const log = createLogger({ component: "StaffAuditLog" });

const PAGE_SIZE = 50;

interface AuditLogStaffOption {
  id: string;
  name: string;
}

interface StaffAuditLogProps {
  staffOptions: AuditLogStaffOption[];
}

interface SessionEditField {
  field: string;
  old: string | null;
  new: string | null;
}

const absenceStatusLabels: Record<string, string> = {
  reported: "gemeldet",
  requested: "beantragt",
  question: "Rückfrage",
  approved: "genehmigt",
  declined: "abgelehnt",
  canceled: "storniert",
};

// One German sentence per event; the typed detail payloads stay readable
// instead of being squeezed into a shared column schema.
function describeEvent(event: AuditLogEvent): string {
  const d = event.detail;
  switch (event.source) {
    case "session_edit": {
      const fields = (d.fields as SessionEditField[] | undefined) ?? [];
      const parts = fields.map(
        (f) =>
          `${FIELD_LABELS[f.field] ?? f.field}: ${formatEditValue(f.field, f.old)} → ${formatEditValue(f.field, f.new)}`,
      );
      const day = typeof d.session_date === "string" ? d.session_date : "";
      return `Session vom ${formatDate(day)}: ${parts.join(", ")}`;
    }
    case "absence": {
      const type =
        absenceTypeLabels[d.absence_type as AbsenceType] ??
        String(d.absence_type ?? "");
      const from =
        typeof d.date_start === "string" ? formatDate(d.date_start) : "";
      const to = typeof d.date_end === "string" ? formatDate(d.date_end) : "";
      const status =
        absenceStatusLabels[String(d.to_status)] ?? String(d.to_status ?? "");
      const range = from === to ? from : `${from}–${to}`;
      return `${type} ${range}: ${status}`;
    }
    case "adjustment": {
      const type = balanceAdjustmentTypeLabel(
        String(d.type ?? "") as BalanceAdjustmentType,
      );
      const minutes = typeof d.minutes_delta === "number" ? d.minutes_delta : 0;
      const day =
        typeof d.effective_date === "string"
          ? formatDate(d.effective_date)
          : "";
      return `${type} ${formatSignedDuration(minutes)} zum ${day}`;
    }
    case "month_close": {
      const count = typeof d.account_count === "number" ? d.account_count : 0;
      return `Monat ${String(d.month).padStart(2, "0")}/${String(d.year)} abgeschlossen (${count} ${count === 1 ? "Konto" : "Konten"})`;
    }
    case "month_reopen": {
      return `Monat ${String(d.month).padStart(2, "0")}/${String(d.year)} wieder geöffnet`;
    }
    case "deletion": {
      const payload = (d.payload ?? {}) as Record<string, unknown>;
      if (d.deleted_source === "balance_adjustment") {
        const type = balanceAdjustmentTypeLabel(
          String(payload.type ?? "") as BalanceAdjustmentType,
        );
        const minutes =
          typeof payload.minutes_delta === "number" ? payload.minutes_delta : 0;
        return `Buchung gelöscht: ${type} ${formatSignedDuration(minutes)}`;
      }
      const type =
        absenceTypeLabels[payload.absence_type as AbsenceType] ??
        String(payload.absence_type ?? "");
      const from =
        typeof payload.date_start === "string"
          ? formatDate(payload.date_start)
          : "";
      const to =
        typeof payload.date_end === "string"
          ? formatDate(payload.date_end)
          : "";
      return `Abwesenheit gelöscht: ${type} ${from === to ? from : `${from}–${to}`}`;
    }
    case "personnel_number": {
      const oldValue = typeof d.old_value === "string" ? d.old_value : "";
      const newValue = typeof d.new_value === "string" ? d.new_value : "";
      const from = oldValue === "" ? "nicht gesetzt" : oldValue;
      const to = newValue === "" ? "entfernt" : newValue;
      return `Personalnummer: ${from} → ${to}`;
    }
  }
}

function actorLabel(event: AuditLogEvent): string {
  if (event.actorIsSystem) return "System";
  if (!event.actorName) return "Unbekannt";
  return event.actorIsSelf ? `${event.actorName} (selbst)` : event.actorName;
}

function staffLabel(event: AuditLogEvent) {
  if (event.staffId === null) {
    return <span className="text-gray-400">Alle Mitarbeitenden</span>;
  }
  if (event.staffName) return event.staffName;
  return <span className="text-gray-400">Unbekannte/ehemalige Person</span>;
}

const sourceOptions = [
  { value: "all", label: "Alle Bereiche" },
  ...Object.entries(auditLogSourceLabels).map(([value, label]) => ({
    value,
    label,
  })),
];

export function StaffAuditLog({ staffOptions }: StaffAuditLogProps) {
  const [staffId, setStaffId] = useState("all");
  const [actorId, setActorId] = useState("all");
  const [source, setSource] = useState("all");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");

  const [events, setEvents] = useState<AuditLogEvent[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [retentionCutoff, setRetentionCutoff] = useState("");
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const requestVersionRef = useRef(0);

  const query = useMemo(
    () => ({
      from: from || undefined,
      to: to || undefined,
      staffId: staffId === "all" ? undefined : staffId,
      actorId: actorId === "all" ? undefined : actorId,
      sources: source === "all" ? undefined : [source as AuditLogSource],
      limit: PAGE_SIZE,
    }),
    [from, to, staffId, actorId, source],
  );

  useEffect(() => {
    const requestVersion = requestVersionRef.current + 1;
    requestVersionRef.current = requestVersion;
    setEvents([]);
    setNextCursor(null);
    setIsLoading(true);
    setError(null);
    staffAuditLogService
      .getAuditLog(query)
      .then((page) => {
        if (requestVersionRef.current !== requestVersion) return;
        setEvents(page.events);
        setNextCursor(page.nextCursor);
        setRetentionCutoff(page.retentionCutoff);
      })
      .catch((err: unknown) => {
        if (requestVersionRef.current !== requestVersion) return;
        log.error("failed to load audit log", { error: err });
        setError("Änderungsprotokoll konnte nicht geladen werden.");
      })
      .finally(() => {
        if (requestVersionRef.current === requestVersion) setIsLoading(false);
      });
    return () => {
      if (requestVersionRef.current === requestVersion) {
        requestVersionRef.current++;
      }
    };
  }, [query]);

  const loadMore = useCallback(() => {
    if (!nextCursor) return;
    const requestVersion = requestVersionRef.current;
    setIsLoading(true);
    staffAuditLogService
      .getAuditLog({ ...query, cursor: nextCursor })
      .then((page) => {
        if (requestVersionRef.current !== requestVersion) return;
        setEvents((prev) => [...prev, ...page.events]);
        setNextCursor(page.nextCursor);
      })
      .catch((err: unknown) => {
        if (requestVersionRef.current !== requestVersion) return;
        log.error("failed to load more audit log entries", { error: err });
        setError("Weitere Einträge konnten nicht geladen werden.");
      })
      .finally(() => {
        if (requestVersionRef.current === requestVersion) setIsLoading(false);
      });
  }, [nextCursor, query]);

  const columns: DataTableColumn<AuditLogEvent>[] = useMemo(
    () => [
      {
        key: "occurredAt",
        header: "Zeitpunkt",
        render: (row) =>
          new Date(row.occurredAt).toLocaleString("de-DE", {
            day: "2-digit",
            month: "2-digit",
            year: "numeric",
            hour: "2-digit",
            minute: "2-digit",
          }),
        className: "whitespace-nowrap text-gray-600",
      },
      {
        key: "staff",
        header: "Mitarbeiter:in",
        render: staffLabel,
      },
      {
        key: "source",
        header: "Bereich",
        render: (row) => (
          <span className="inline-flex rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700">
            {auditLogSourceLabels[row.source]}
          </span>
        ),
      },
      {
        key: "detail",
        header: "Änderung",
        render: (row) => describeEvent(row),
      },
      {
        key: "actor",
        header: "Bearbeitet von",
        render: (row) => actorLabel(row),
        className: "whitespace-nowrap",
      },
      {
        key: "reason",
        header: "Begründung",
        render: (row) =>
          row.reason ? (
            <span className="text-gray-600 italic">{row.reason}</span>
          ) : (
            <span className="text-gray-300">–</span>
          ),
      },
    ],
    [],
  );

  const staffSelectOptions = useMemo(
    () => [
      { value: "all", label: "Alle Mitarbeitenden" },
      ...staffOptions.map((s) => ({ value: s.id, label: s.name })),
    ],
    [staffOptions],
  );

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-end gap-3">
        <div className="w-56">
          <label
            htmlFor="audit-staff"
            className="mb-1 block text-xs font-medium text-gray-600"
          >
            Mitarbeiter:in
          </label>
          <CustomSelect
            id="audit-staff"
            value={staffId}
            options={staffSelectOptions}
            onChange={setStaffId}
            ariaLabel="Nach Mitarbeiter:in filtern"
          />
        </div>
        <div className="w-56">
          <label
            htmlFor="audit-actor"
            className="mb-1 block text-xs font-medium text-gray-600"
          >
            Bearbeitet von
          </label>
          <CustomSelect
            id="audit-actor"
            value={actorId}
            options={staffSelectOptions}
            onChange={setActorId}
            ariaLabel="Nach bearbeitender Person filtern"
          />
        </div>
        <div className="w-48">
          <label
            htmlFor="audit-source"
            className="mb-1 block text-xs font-medium text-gray-600"
          >
            Bereich
          </label>
          <CustomSelect
            id="audit-source"
            value={source}
            options={sourceOptions}
            onChange={setSource}
            ariaLabel="Nach Bereich filtern"
          />
        </div>
        <div>
          <label
            htmlFor="audit-from"
            className="mb-1 block text-xs font-medium text-gray-600"
          >
            Von
          </label>
          <ISODatePicker
            id="audit-from"
            value={from}
            onChange={setFrom}
            max={to || undefined}
            calendarLayout="popover"
            controlSize="md"
            className="w-44"
          />
        </div>
        <div>
          <label
            htmlFor="audit-to"
            className="mb-1 block text-xs font-medium text-gray-600"
          >
            Bis
          </label>
          <ISODatePicker
            id="audit-to"
            value={to}
            onChange={setTo}
            min={from || undefined}
            calendarLayout="popover"
            controlSize="md"
            className="w-44"
          />
        </div>
      </div>

      {retentionCutoff && from && from < retentionCutoff && (
        <Alert
          type="info"
          message={`Einträge zur Zeiterfassung vor dem ${formatDate(retentionCutoff)} wurden gemäß Aufbewahrungsfrist gelöscht. Buchungen und Monatsabschlüsse bleiben vollständig.`}
        />
      )}

      {error && <Alert type="error" message={error} />}

      <DataTable
        columns={columns}
        rows={events}
        getRowKey={(row) => `${row.source}:${row.entryId}`}
        isLoading={isLoading && events.length === 0}
        emptyState={
          <div className="py-10 text-center">
            <p className="font-medium text-gray-900">Keine Einträge gefunden</p>
            <p className="text-sm text-gray-600">
              Zeitraum erweitern oder Filter zurücksetzen.
            </p>
          </div>
        }
      />

      {nextCursor && (
        <div className="flex justify-center">
          <Button
            type="button"
            size="md"
            variant="outline"
            onClick={loadMore}
            disabled={isLoading}
          >
            Weitere Einträge laden
          </Button>
        </div>
      )}
    </div>
  );
}

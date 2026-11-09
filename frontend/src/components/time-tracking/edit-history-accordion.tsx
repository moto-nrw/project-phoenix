"use client";

import { SquarePen } from "lucide-react";

import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import type { WorkSessionEdit } from "~/lib/time-tracking-helpers";
import { formatTime } from "~/lib/time-tracking-helpers";
import { SpinnerIcon } from "~/components/ui/icons";
import { InsetPanel } from "~/components/ui/inset-panel";

// Renders the audit trail for a single work session. Used by both the
// MA self-service /time-tracking page and the admin staff detail view —
// the underlying audit data (audit.work_session_edits) is identical, so
// the UI is too.
//
// Lifting this out of /time-tracking/page.tsx serves two goals:
//   1. DSGVO Art. 15 / "Mitarbeiter sieht eigene Daten" was already
//      covered there. The admin side needs the same view to answer
//      auskunft requests and audit lohn corrections.
//   2. Keeping a single source of truth for the field labels and value
//      formatting — diverging copies between MA and admin would be a
//      future incident waiting to happen.

// Exported for the cross-staff audit log (#1417) — one source of truth for
// field labels and value formatting, per the note above.
export const FIELD_LABELS: Record<string, string> = {
  check_in_time: "Start",
  check_out_time: "Ende",
  break_minutes: "Pause",
  break_duration: "Pausendauer",
  status: "Ort",
  notes: "Notiz",
  // Deviation from the planned shift (#1844): old = geplante, new = tatsächliche
  // Zeit (both "HH:MM"), the reason lands in the Grund column. So the row reads
  // "Abweichung | 08:00 → 07:30 | Grund: …" — plan, actual and reason together.
  deviation_reason: "Abweichung",
};

export function formatEditValue(
  fieldName: string,
  value: string | null,
): string {
  if (value === null || value === "") return "–";
  if (fieldName === "check_in_time" || fieldName === "check_out_time") {
    return formatTime(value);
  }
  if (fieldName === "break_minutes" || fieldName === "break_duration") {
    return `${value} min`;
  }
  if (fieldName === "status") {
    return value === "home_office" ? "Unterwegs" : "In der OGS";
  }
  return value;
}

interface EditRow {
  readonly edit: WorkSessionEdit;
  // Datum, Person und Grund stehen je Speichervorgang nur in der ersten Zeile.
  readonly firstOfSave: boolean;
  readonly dateStr: string;
  readonly editorLabel: string;
  readonly notes: string | null | undefined;
}

const EDIT_COLUMNS: DataTableColumn<EditRow>[] = [
  {
    key: "date",
    header: "Datum",
    className: "whitespace-nowrap",
    render: (row) =>
      row.firstOfSave ? (
        <span className="text-gray-500">{row.dateStr}</span>
      ) : null,
  },
  {
    key: "editor",
    header: "Von",
    className: "whitespace-nowrap",
    render: (row) =>
      row.firstOfSave ? (
        <span className="text-gray-600">{row.editorLabel}</span>
      ) : null,
  },
  {
    key: "field",
    header: "Feld",
    className: "whitespace-nowrap",
    render: (row) => (
      <span className="text-gray-600">
        {FIELD_LABELS[row.edit.fieldName] ?? row.edit.fieldName}
      </span>
    ),
  },
  {
    key: "old",
    header: "Vorher",
    className: "whitespace-nowrap",
    render: (row) => (
      <span className="text-moto-red line-through">
        {formatEditValue(row.edit.fieldName, row.edit.oldValue)}
      </span>
    ),
  },
  {
    key: "new",
    header: "Nachher",
    className: "whitespace-nowrap",
    render: (row) => (
      <span className="text-moto-green-strong font-medium">
        {formatEditValue(row.edit.fieldName, row.edit.newValue)}
      </span>
    ),
  },
  {
    key: "reason",
    header: "Grund",
    render: (row) => {
      if (!row.firstOfSave) return null;
      return row.notes ? (
        <span className="text-gray-400 italic">&ldquo;{row.notes}&rdquo;</span>
      ) : (
        "–"
      );
    },
  },
];

export function EditHistoryAccordion({
  edits,
  isLoading,
  onEdit,
}: {
  readonly edits: WorkSessionEdit[];
  readonly isLoading: boolean;
  readonly onEdit?: () => void;
}) {
  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-3">
        <SpinnerIcon className="h-4 w-4 text-gray-400" />
        <span className="ml-2 text-xs text-gray-400">Laden...</span>
      </div>
    );
  }

  if (edits.length === 0) {
    return <EmptyState variant="compact" title="Keine Änderungen vorhanden" />;
  }

  // Group edits by createdAt timestamp — edits from the same save action
  // share the same timestamp, so they belong in one visual row.
  const grouped = new Map<string, WorkSessionEdit[]>();
  for (const edit of edits) {
    const key = edit.createdAt;
    const existing = grouped.get(key);
    if (existing) {
      existing.push(edit);
    } else {
      grouped.set(key, [edit]);
    }
  }

  const rows = Array.from(grouped.entries()).map(([timestamp, group]) => {
    const date = new Date(timestamp);
    const dateStr = `${date.getDate().toString().padStart(2, "0")}.${(date.getMonth() + 1).toString().padStart(2, "0")}.${date.getFullYear()}, ${date.getHours().toString().padStart(2, "0")}:${date.getMinutes().toString().padStart(2, "0")}`;
    const fieldEdits = group.filter((e) => e.fieldName !== "notes");
    const notes = group[0]?.notes;
    // All edits in a group share the same author — they came from one save
    // action — so we read editor info from the first.
    const isSelfEdit = group[0]?.isSelfEdit ?? true;
    const editorName = group[0]?.editorName ?? "";
    const editorLabel = isSelfEdit
      ? editorName
        ? `${editorName} (selbst)`
        : "Selbst"
      : editorName
        ? `${editorName} (Admin)`
        : "Admin";
    return { timestamp, dateStr, fieldEdits, notes, editorLabel };
  });

  return (
    <div>
      {/* Mobile: Compact card layout */}
      <div className="space-y-2 md:hidden">
        {rows.map(({ timestamp, dateStr, fieldEdits, notes, editorLabel }) => (
          <InsetPanel key={timestamp} tone="white">
            <div className="mb-1.5 flex items-center justify-between text-xs text-gray-400">
              <span>{dateStr}</span>
              <span className="font-medium">{editorLabel}</span>
            </div>
            <div className="space-y-1">
              {fieldEdits.map((edit) => (
                <div key={edit.id} className="flex items-center gap-2 text-xs">
                  <span className="w-12 shrink-0 text-gray-500">
                    {FIELD_LABELS[edit.fieldName] ?? edit.fieldName}
                  </span>
                  <span className="text-moto-red line-through">
                    {formatEditValue(edit.fieldName, edit.oldValue)}
                  </span>
                  <span className="text-gray-300">→</span>
                  <span className="text-moto-green-strong font-medium">
                    {formatEditValue(edit.fieldName, edit.newValue)}
                  </span>
                </div>
              ))}
            </div>
            {notes && (
              <p className="mt-1.5 text-xs text-gray-400 italic">
                &ldquo;{notes}&rdquo;
              </p>
            )}
          </InsetPanel>
        ))}
      </div>

      {/* Desktop: Table layout */}
      <div className="hidden md:block">
        <DataTable
          columns={EDIT_COLUMNS}
          rows={rows.flatMap(({ dateStr, fieldEdits, notes, editorLabel }) =>
            fieldEdits.map((edit, index) => ({
              edit,
              firstOfSave: index === 0,
              dateStr,
              editorLabel,
              notes,
            })),
          )}
          getRowKey={(row) => row.edit.id}
          dense
        />
      </div>
      {onEdit && (
        <Button
          type="button"
          variant="dashed"
          size="compact"
          onClick={onEdit}
          className="mt-2 w-full"
        >
          <SquarePen className="h-3.5 w-3.5" />
          Weitere Änderung vornehmen
        </Button>
      )}
    </div>
  );
}

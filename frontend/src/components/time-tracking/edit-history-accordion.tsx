"use client";

import { SquarePen } from "lucide-react";

import type { WorkSessionEdit } from "~/lib/time-tracking-helpers";
import { formatTime } from "~/lib/time-tracking-helpers";

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

const FIELD_LABELS: Record<string, string> = {
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

function formatEditValue(fieldName: string, value: string | null): string {
  if (value === null || value === "") return "–";
  if (fieldName === "check_in_time" || fieldName === "check_out_time") {
    return formatTime(value);
  }
  if (fieldName === "break_minutes" || fieldName === "break_duration") {
    return `${value} min`;
  }
  if (fieldName === "status") {
    return value === "home_office" ? "Homeoffice" : "In der OGS";
  }
  return value;
}

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
        <div className="h-4 w-4 animate-spin rounded-full border-2 border-gray-300 border-t-transparent" />
        <span className="ml-2 text-xs text-gray-400">Laden...</span>
      </div>
    );
  }

  if (edits.length === 0) {
    return (
      <p className="py-2 text-xs text-gray-400">Keine Änderungen vorhanden.</p>
    );
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
          <div
            key={timestamp}
            className="rounded-lg border border-gray-100 bg-white p-2.5"
          >
            <div className="mb-1.5 flex items-center justify-between text-[10px] text-gray-400">
              <span>{dateStr}</span>
              <span className="font-medium">{editorLabel}</span>
            </div>
            <div className="space-y-1">
              {fieldEdits.map((edit) => (
                <div key={edit.id} className="flex items-center gap-2 text-xs">
                  <span className="w-12 shrink-0 text-gray-500">
                    {FIELD_LABELS[edit.fieldName] ?? edit.fieldName}
                  </span>
                  <span className="text-red-400 line-through">
                    {formatEditValue(edit.fieldName, edit.oldValue)}
                  </span>
                  <span className="text-gray-300">→</span>
                  <span className="font-medium text-[#83cd2d]">
                    {formatEditValue(edit.fieldName, edit.newValue)}
                  </span>
                </div>
              ))}
            </div>
            {notes && (
              <p className="mt-1.5 text-[10px] text-gray-400 italic">
                &ldquo;{notes}&rdquo;
              </p>
            )}
          </div>
        ))}
      </div>

      {/* Desktop: Table layout */}
      <table className="hidden w-full text-xs md:table">
        <thead>
          <tr className="border-b border-gray-100 text-left text-[10px] font-medium tracking-wide text-gray-400 uppercase">
            <th className="pr-4 pb-2">Datum</th>
            <th className="pr-4 pb-2">Von</th>
            <th className="pr-4 pb-2">Feld</th>
            <th className="pr-4 pb-2">Vorher</th>
            <th className="pr-4 pb-2" />
            <th className="pr-4 pb-2">Nachher</th>
            <th className="pb-2">Grund</th>
          </tr>
        </thead>
        <tbody>
          {rows.map(({ dateStr, fieldEdits, notes, editorLabel }) =>
            fieldEdits.map((edit, idx) => (
              <tr
                key={edit.id}
                className="border-b border-gray-50 last:border-b-0"
              >
                <td className="py-1.5 pr-4 whitespace-nowrap text-gray-500">
                  {idx === 0 ? dateStr : ""}
                </td>
                <td className="py-1.5 pr-4 whitespace-nowrap text-gray-600">
                  {idx === 0 ? editorLabel : ""}
                </td>
                <td className="py-1.5 pr-4 whitespace-nowrap text-gray-600">
                  {FIELD_LABELS[edit.fieldName] ?? edit.fieldName}
                </td>
                <td className="py-1.5 pr-4 whitespace-nowrap text-red-400 line-through">
                  {formatEditValue(edit.fieldName, edit.oldValue)}
                </td>
                <td className="py-1.5 pr-4 text-gray-300">&rarr;</td>
                <td className="py-1.5 pr-4 font-medium whitespace-nowrap text-[#83cd2d]">
                  {formatEditValue(edit.fieldName, edit.newValue)}
                </td>
                <td className="py-1.5 text-gray-400 italic">
                  {idx === 0 && notes ? (
                    <span>&ldquo;{notes}&rdquo;</span>
                  ) : null}
                </td>
              </tr>
            )),
          )}
        </tbody>
      </table>
      {onEdit && (
        <button
          type="button"
          onClick={onEdit}
          className="mt-2 flex w-full items-center justify-center gap-2 rounded-lg border border-dashed border-gray-300 py-1.5 text-xs text-gray-400 transition-colors hover:border-gray-400 hover:bg-gray-50 hover:text-gray-600"
        >
          <SquarePen className="h-3.5 w-3.5" />
          Weitere Änderung vornehmen
        </button>
      )}
    </div>
  );
}

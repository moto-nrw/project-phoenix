import type { ReactNode } from "react";
import { Pencil } from "lucide-react";

import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatDayCount } from "~/lib/absence-helpers";
import type { AbsenceTypeAllowanceSummary } from "~/lib/absence-type-api";
import { formatDate } from "~/lib/date-helpers";

// Wann der Rest verfällt (#3257): ein verfallener Rest bleibt sichtbar, und
// im neuen Jahr steht dabei, was aus dem Vorjahr noch nutzbar ist.
export function AllowanceExpiryNote({
  summary,
}: {
  readonly summary: AbsenceTypeAllowanceSummary;
}) {
  const lines: { text: string; expired: boolean }[] = [];
  const carried = summary.carriedIn;
  if (carried && carried.remainingDays > 0) {
    lines.push({
      text: `Dazu noch ${formatDayCount(carried.remainingDays)} aus ${carried.year}, nutzbar bis ${formatDate(carried.expiresOn)}. Diese gehen zuerst ab.`,
      expired: false,
    });
  }
  if (carried && carried.expiredDays > 0) {
    lines.push({
      text: `${formatDayCount(carried.expiredDays)} aus ${carried.year} am ${formatDate(carried.expiresOn)} verfallen.`,
      expired: true,
    });
  }
  if (summary.expiredDays > 0) {
    lines.push({
      text: `${formatDayCount(summary.expiredDays)} am ${formatDate(summary.expiresOn)} verfallen.`,
      expired: true,
    });
  } else if (summary.remainingDays > 0) {
    lines.push({
      text: `Rest verfällt am ${formatDate(summary.expiresOn)}.`,
      expired: false,
    });
  }
  if (lines.length === 0) return null;
  return (
    <ul className="space-y-1 text-xs">
      {lines.map((line) => (
        <li
          key={line.text}
          className={line.expired ? "text-moto-red-strong" : "text-gray-600"}
        >
          {line.text}
        </li>
      ))}
    </ul>
  );
}

// Eine Kontingent-Karte (#3256): Urlaub und jede eigene Art mit Kontingent
// sehen gleich aus, damit „wie viel ist noch übrig" an einer Stelle steht.
// Im Bearbeiten-Zustand weichen die Zahlen dem Formular.
export function AllowanceCard({
  title,
  year,
  retired = false,
  entitled,
  entitledHint,
  taken,
  takenHint,
  reserved,
  remaining,
  onEdit,
  extraAction,
  editForm,
  footer,
  headingLevel = 4,
}: {
  readonly title: string;
  readonly year: number;
  readonly retired?: boolean;
  readonly entitled: number;
  readonly entitledHint?: string;
  readonly taken: number;
  readonly takenHint?: string;
  readonly reserved: number;
  readonly remaining: number;
  readonly onEdit?: () => void;
  readonly extraAction?: ReactNode;
  readonly editForm?: ReactNode;
  readonly footer?: ReactNode;
  /** 4 under the Leitung's level-3 section, 3 under a level-2 card. */
  readonly headingLevel?: 3 | 4;
}) {
  const isEditing = Boolean(editForm);
  const Heading = `h${headingLevel}` as "h3" | "h4";
  return (
    <section
      aria-label={`${title} ${year}`}
      className={`moto-content-surface space-y-3 rounded-xl border p-4 shadow-sm ${
        isEditing ? "lg:col-span-2" : ""
      }`}
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <Heading className="truncate text-sm font-semibold text-gray-900">
            {title}
          </Heading>
          {retired ? <StatusBadge tone="gray" label="ausgeschaltet" /> : null}
        </div>
        {isEditing ? null : (
          <div className="flex flex-wrap items-center gap-1">
            {extraAction}
            {onEdit ? (
              <Button
                type="button"
                variant="ghost"
                size="compact"
                onClick={onEdit}
                aria-label={`Anspruch ändern: ${title}`}
              >
                <Pencil className="mr-1.5 h-3.5 w-3.5" aria-hidden />
                Anspruch ändern
              </Button>
            ) : null}
          </div>
        )}
      </div>
      {isEditing ? (
        editForm
      ) : (
        <DataGrid columns={4}>
          <DataField label="Anspruch">
            <span className="tabular-nums">{formatDayCount(entitled)}</span>
            {entitledHint ? (
              <span className="block text-xs text-gray-500">
                {entitledHint}
              </span>
            ) : null}
          </DataField>
          <DataField label="Genommen">
            <span className="tabular-nums">{formatDayCount(taken)}</span>
            {takenHint ? (
              <span className="block text-xs text-gray-500">{takenHint}</span>
            ) : null}
          </DataField>
          <DataField label="Vorgemerkt">
            <span className="tabular-nums">{formatDayCount(reserved)}</span>
            {reserved > 0 ? (
              <span className="block text-xs text-gray-500">
                offene Anträge
              </span>
            ) : null}
          </DataField>
          <DataField label="Übrig">
            <span
              className={`font-semibold tabular-nums ${
                remaining < 0 ? "text-moto-red-strong" : "text-gray-900"
              }`}
            >
              {formatDayCount(remaining)}
            </span>
          </DataField>
        </DataGrid>
      )}
      {isEditing ? null : footer}
    </section>
  );
}

import {
  type AllowedDepartureModes,
  CARE_DISPLAY_PILL,
  DEPARTURE_MODE_SHORT_LABELS,
  departureMatrixRows,
  hasAnyDepartureArrangement,
} from "~/lib/student-helpers";

/**
 * Read-only "Erlaubte Heimwege" display for the student Stammdaten. Mirrors the
 * editor's per-weekday layout: one row per weekday (Mo–Fr) with the allowed modes
 * as neutral badges, reusing the same labels and styling as the edit form
 * (DEPARTURE_MODE_* in student-helpers) so view and edit stay visually consistent.
 *
 * The badges wrap instead of forcing a single line, so a child with bus + pickup
 * on every weekday renders cleanly instead of overflowing the card. When the child
 * goes home alone every day the matrix collapses to a single "Geht immer alleine"
 * line rather than five identical "Zu Fuß" rows.
 */
export function AllowedDepartureModesDisplay({
  value,
}: Readonly<{ value?: AllowedDepartureModes | null }>) {
  if (!hasAnyDepartureArrangement(value)) {
    return <span className="text-gray-900">Geht immer alleine</span>;
  }

  return (
    <div className="space-y-1.5">
      {departureMatrixRows(value).map((row) => (
        <div key={row.key} className="flex items-center gap-3">
          <span className="w-7 shrink-0 text-xs font-semibold text-gray-500">
            {row.short}
          </span>
          <div className="flex flex-wrap gap-1">
            {row.modes.map((mode) => (
              <span key={mode} className={CARE_DISPLAY_PILL}>
                {DEPARTURE_MODE_SHORT_LABELS[mode]}
              </span>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

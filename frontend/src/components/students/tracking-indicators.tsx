// components/students/tracking-indicators.tsx
// Displays tracking indicator checkmarks/circles in student cards

import { Circle, CircleCheck } from "lucide-react";

interface TrackingIndicatorsProps {
  /** Configured indicator labels (e.g., ["Hausaufgaben", "Mensa"]) */
  readonly labels: string[];
  /** Match results aligned with labels (true = visited today) */
  readonly results: boolean[];
  /**
   * "stack" (default) is the right-aligned column of a Kinderkarte; "inline"
   * puts the indicators side by side in a table cell (#3834).
   */
  readonly layout?: "stack" | "inline";
}

/**
 * Renders right-aligned tracking indicators showing whether a student
 * has visited configured rooms/activities today.
 * Returns null if no labels are configured.
 */
export function TrackingIndicators({
  labels,
  results,
  layout = "stack",
}: TrackingIndicatorsProps) {
  if (labels.length === 0) return null;

  return (
    <div
      className={
        layout === "stack"
          ? "mt-1.5 flex flex-col items-end gap-0.5"
          : "flex flex-wrap items-center gap-x-3 gap-y-1"
      }
    >
      {labels.map((label, i) => {
        const matched = results[i] ?? false;
        return (
          <div key={label} className="flex items-center gap-1.5">
            <span className="text-xs font-medium text-gray-500">{label}</span>
            {matched ? (
              <CircleCheck
                className="text-moto-green h-4 w-4"
                aria-label={`${label}: erledigt`}
              />
            ) : (
              <Circle
                className="h-4 w-4 text-gray-300"
                aria-label={`${label}: ausstehend`}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}

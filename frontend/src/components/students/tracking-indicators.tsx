// components/students/tracking-indicators.tsx
// Displays tracking indicator checkmarks/circles in student cards

import { Circle, CircleCheck } from "lucide-react";

interface TrackingIndicatorsProps {
  /** Configured indicator labels (e.g., ["Hausaufgaben", "Mensa"]) */
  readonly labels: string[];
  /** Match results aligned with labels (true = visited today) */
  readonly results: boolean[];
}

/**
 * Renders right-aligned tracking indicators showing whether a student
 * has visited configured rooms/activities today.
 * Returns null if no labels are configured.
 */
export function TrackingIndicators({
  labels,
  results,
}: TrackingIndicatorsProps) {
  if (labels.length === 0) return null;

  return (
    <div className="mt-1.5 flex flex-col items-end gap-0.5">
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

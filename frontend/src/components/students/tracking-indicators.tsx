// components/students/tracking-indicators.tsx
// Displays tracking indicator checkmarks/circles in student cards

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
              <svg
                className="text-moto-green h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                aria-label={`${label}: erledigt`}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2.5}
                  d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
            ) : (
              <svg
                className="h-4 w-4 text-gray-300"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                aria-label={`${label}: ausstehend`}
              >
                <circle cx="12" cy="12" r="9" strokeWidth={2} />
              </svg>
            )}
          </div>
        );
      })}
    </div>
  );
}

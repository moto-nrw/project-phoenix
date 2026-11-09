import type { ReactNode } from "react";
import { Alert } from "~/components/ui/alert";

/** „1 Familie“ / „3 Familien“. */
export function familiesLabel(count: number): string {
  return count === 1 ? "1 Familie" : `${count} Familien`;
}

/**
 * What a run preview still asks to check, repeated in the release question.
 * The release numbers the invoices for good, so the hints from further down
 * the page stand where the decision is made.
 */
export function ReleaseOpenPoints({
  points,
  action,
}: Readonly<{
  points: readonly string[];
  /** The way to fix a point before releasing, e.g. „Fälligkeiten ändern“. */
  action?: ReactNode;
}>) {
  if (points.length === 0) return null;
  return (
    <Alert
      type="warning"
      announce="off"
      className="mt-3"
      title="Noch offen"
      action={action}
      message={
        <ul className="list-disc space-y-1 pl-4">
          {points.map((point) => (
            <li key={point}>{point}</li>
          ))}
        </ul>
      }
    />
  );
}

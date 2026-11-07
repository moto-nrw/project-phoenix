import type { ReactNode } from "react";
import { cn } from "~/lib/utils";

/** Ab dieser Höhe scrollt die Liste in ihrem Rahmen. */
const MAX_HEIGHT_CLASSES = {
  sm: "max-h-40",
  md: "max-h-72",
  lg: "max-h-96",
} as const;

interface FramedListProps {
  /** `li`-Elemente. */
  readonly children: ReactNode;
  readonly maxHeight?: keyof typeof MAX_HEIGHT_CLASSES;
  /** Grauer Kopfstreifen, etwa „Alle auswählen“. Scrollt nicht mit. */
  readonly header?: ReactNode;
  readonly className?: string;
  readonly "aria-label"?: string;
}

/** Umrandete Liste mit Trennlinien, innerhalb einer Karte oder eines Panels. */
export function FramedList({
  children,
  maxHeight,
  header,
  className,
  "aria-label": ariaLabel,
}: FramedListProps) {
  return (
    <div
      className={cn(
        "moto-content-surface overflow-hidden rounded-xl border",
        className,
      )}
    >
      {header ? (
        <div className="border-b border-gray-200 bg-gray-50 px-4 py-2.5">
          {header}
        </div>
      ) : null}
      <ul
        aria-label={ariaLabel}
        className={cn(
          "divide-y divide-gray-100",
          maxHeight && `${MAX_HEIGHT_CLASSES[maxHeight]} overflow-y-auto`,
        )}
      >
        {children}
      </ul>
    </div>
  );
}

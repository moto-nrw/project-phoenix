import type { ReactNode } from "react";

import { cn } from "~/lib/utils";
import { STICKY_ABOVE_BOTTOM_NAV, Z_LAYER } from "./overlay-styles";

/**
 * Die Leiste mit den Aktionen einer langen Seite (Import, Speiseplan). Sie
 * klebt am unteren Rand, solange die Seite scrollt. Seiten, die schon vor der
 * Umstellung Platz dafür reservierten, setzen `reserveSpace`.
 *
 * `summary` steht links und sagt, worauf sich die Aktionen beziehen; die
 * Aktionen stehen dann rechts in einer Zeile. Ohne `summary` füllen die
 * Aktionen die Leiste, am Telefon untereinander.
 */
export function StickyActionBar({
  summary,
  reserveSpace = false,
  children,
}: Readonly<{
  summary?: ReactNode;
  /** Platz für die Leiste nur erhalten, wenn die Seite ihn vorher hatte. */
  reserveSpace?: boolean;
  children: ReactNode;
}>) {
  return (
    <>
      {reserveSpace && <div aria-hidden="true" className="h-20" />}
      <div
        className={cn(
          STICKY_ABOVE_BOTTOM_NAV,
          `${Z_LAYER.sticky} rounded-2xl border border-gray-200 bg-white/95 px-4 py-3 shadow-lg backdrop-blur-sm`,
          summary
            ? "flex items-center justify-between gap-3"
            : "flex flex-col gap-2 sm:flex-row sm:gap-3",
        )}
      >
        {summary ? (
          <>
            <span className="min-w-0 text-sm font-medium text-gray-700">
              {summary}
            </span>
            <div className="flex shrink-0 gap-2">{children}</div>
          </>
        ) : (
          children
        )}
      </div>
    </>
  );
}

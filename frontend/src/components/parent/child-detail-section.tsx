import type React from "react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type { MotoConceptKey } from "~/lib/moto-concepts";

/**
 * Shared section wrapper for the parent child-detail views (master data +
 * care schedule). Lives in its own module so both views can import it without
 * creating a circular dependency between `child-master-data` and
 * `child-care-schedule`.
 *
 * The optional `concept` prop lifts the section onto the app-wide gray-tile
 * header pattern (see SectionHeader): a 36-40px rounded-xl bg-gray-100 icon
 * tile next to the title. Sections without a fitting concept keep the plain
 * text-only header.
 */
export function Section({
  title,
  hint,
  concept,
  children,
}: Readonly<{
  title: string;
  hint: string;
  concept?: MotoConceptKey;
  children: React.ReactNode;
}>) {
  return (
    <section className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
      <header className={concept ? "mb-4 flex items-start gap-3" : "mb-4"}>
        {concept ? (
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-gray-100 sm:h-10 sm:w-10">
            <MotoConceptIcon concept={concept} size={20} />
          </span>
        ) : null}
        <div className="min-w-0">
          <h2 className="text-lg font-semibold text-gray-900">{title}</h2>
          <p className="mt-0.5 text-xs text-gray-500">{hint}</p>
        </div>
      </header>
      <div className="space-y-4">{children}</div>
    </section>
  );
}

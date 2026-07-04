import type React from "react";

/**
 * Shared section wrapper for the parent child-detail views (master data +
 * care schedule). Lives in its own module so both views can import it without
 * creating a circular dependency between `child-master-data` and
 * `child-care-schedule`.
 */
export function Section({
  title,
  hint,
  children,
}: Readonly<{ title: string; hint: string; children: React.ReactNode }>) {
  return (
    <section className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
      <header className="mb-4">
        <h2 className="text-lg font-semibold text-gray-900">{title}</h2>
        <p className="mt-0.5 text-xs text-gray-500">{hint}</p>
      </header>
      <div className="space-y-4">{children}</div>
    </section>
  );
}

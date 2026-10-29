import type { ReactNode } from "react";

/**
 * The rows with a dividing line between them. From `sm` on the actions sit
 * in one column of their own (the rows share it through `subgrid`), so every
 * row's buttons start at the same place, however long its text is. Below
 * `sm` each row stacks: text above, buttons below. Every decision is the
 * same kit button, neither way the lesser one. Rows use `sm:col-span-2`
 * and `sm:grid-cols-subgrid` to share the action column.
 */
export function ResponsiveActionList({
  id,
  children,
}: Readonly<{ id?: string; children: ReactNode }>) {
  return (
    <ul
      id={id}
      className="divide-y divide-gray-100 sm:grid sm:grid-cols-[minmax(0,1fr)_auto] sm:gap-x-4"
    >
      {children}
    </ul>
  );
}

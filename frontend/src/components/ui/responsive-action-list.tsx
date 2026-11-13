import type { ComponentPropsWithoutRef, ReactNode } from "react";

import { cn } from "~/lib/utils";

/**
 * The rows with a dividing line between them. From `sm` on the actions sit
 * in one column of their own (the rows share it through `subgrid`), so every
 * row's buttons start at the same place, however long its text is. Below
 * `sm` each row stacks: text above, buttons below. Every decision is the
 * same kit button, neither way the lesser one. Rows are `ResponsiveActionRow`.
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

const ROW_ALIGN = {
  center: "sm:items-center",
  start: "sm:items-start",
} as const;

type ResponsiveActionRowProps = ComponentPropsWithoutRef<"li"> & {
  /** Vertical alignment of text and actions from `sm` on. */
  readonly align?: keyof typeof ROW_ALIGN;
};

/**
 * One row of a `ResponsiveActionList`: text first, actions second. From `sm`
 * on it spans both columns of the list and passes them on to its two
 * children (`subgrid`), so the actions of every row line up.
 * `className` adds to the row and replaces conflicting defaults.
 */
export function ResponsiveActionRow({
  align = "center",
  className,
  ...props
}: ResponsiveActionRowProps) {
  return (
    <li
      {...props}
      className={cn(
        "flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid",
        ROW_ALIGN[align],
        className,
      )}
    />
  );
}

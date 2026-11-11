import type { ComponentPropsWithoutRef, ReactNode } from "react";

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

const ROW_LAYOUT = {
  stacked: "flex flex-col sm:col-span-2 sm:grid sm:grid-cols-subgrid",
  inline:
    "flex flex-row items-start justify-between sm:col-span-2 sm:grid sm:grid-cols-subgrid",
  block: "block sm:col-span-2 sm:grid sm:grid-cols-subgrid",
} as const;

const ROW_SPACING = {
  default: "py-3",
  dense: "py-2",
  roomy: "py-4",
} as const;

const ROW_EDGES = {
  default: "first:pt-0 last:pb-0",
  dense: "first:pt-2 last:pb-2",
  roomy: "first:pt-0 last:pb-0",
  inset: "first:pt-3 last:pb-3",
} as const;

const ROW_GAP = {
  default: "gap-2",
  wide: "gap-3",
  none: "gap-[normal]",
} as const;

type ResponsiveActionRowProps = ComponentPropsWithoutRef<"li"> & {
  /** Vertical alignment of text and actions from `sm` on. */
  readonly align?: keyof typeof ROW_ALIGN;
  /** Mobile layout; every variant keeps the shared desktop action column. */
  readonly layout?: keyof typeof ROW_LAYOUT;
  /** Vertical padding. */
  readonly spacing?: keyof typeof ROW_SPACING;
  /** Space between text and actions on a phone. */
  readonly gap?: keyof typeof ROW_GAP;
  /** Keep the default vertical padding at the outer edges of the list. */
  readonly insetEdges?: boolean;
};

/**
 * One row of a `ResponsiveActionList`: text first, actions second. From `sm`
 * on it spans both columns of the list and passes them on to its two
 * children (`subgrid`), so the actions of every row line up.
 * `className` adds non-structural styles to the row.
 */
export function ResponsiveActionRow({
  align = "center",
  className,
  layout = "stacked",
  spacing = "default",
  gap = "default",
  insetEdges = false,
  ...props
}: ResponsiveActionRowProps) {
  return (
    <li
      {...props}
      className={[
        ROW_LAYOUT[layout],
        ROW_SPACING[spacing],
        ROW_EDGES[insetEdges ? "inset" : spacing],
        ROW_GAP[gap],
        ROW_ALIGN[align],
        className,
      ]
        .filter(Boolean)
        .join(" ")}
    />
  );
}

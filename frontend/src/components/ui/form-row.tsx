import type { ReactNode } from "react";

import { cn } from "~/lib/utils";

/**
 * One field and its button in one line (#5589): the field takes the free width,
 * the button stands level with the field, not with its label. On a phone both
 * stack and the button takes the full width. Replaces the spacer label above a
 * button. Expects exactly two children, the field first. A hint goes under the
 * row: inside the field it would push the button below the control.
 */
export function FormRow({
  children,
  className,
}: Readonly<{
  children: ReactNode;
  /** Width of the row only, e.g. a cap for a short field. */
  className?: string;
}>) {
  return (
    <div
      className={cn(
        "grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end",
        className,
      )}
    >
      {children}
    </div>
  );
}

"use client";

import React from "react";
import { cn } from "~/lib/utils";
import { Button } from "./button";

// Its own module, apart from `Modal`: pages that stub `~/components/ui/modal`
// in tests still render the dialogs built on this button.

/**
 * What the footer of the open `Modal` needs to know: whether every dismissal
 * path is locked, and whether its actions stack on a phone sheet.
 */
export const ModalFrameContext = React.createContext({
  isDismissDisabled: false,
  stackedSheetFooter: false,
});

type ModalCancelButtonProps = Omit<
  React.ComponentProps<typeof Button>,
  "variant" | "size" | "type"
>;

/**
 * The Abbrechen of a `Modal` footer. It is a dismissal path, so the dialog's
 * `isDismissDisabled` locks it like the X, Escape and the backdrop. Below
 * `sm`, on a phone sheet whose actions stack (`mobileSheet`,
 * `sheetFooter="stacked"`), it is hidden: the X closes the sheet. Render it
 * in `footer`; only there does it read the dialog's state.
 */
export function ModalCancelButton({
  className,
  disabled = false,
  ...props
}: ModalCancelButtonProps) {
  const frame = React.useContext(ModalFrameContext);
  return (
    <Button
      type="button"
      variant="outline"
      size="md"
      disabled={disabled || frame.isDismissDisabled}
      className={cn(
        className,
        frame.stackedSheetFooter && "hidden sm:inline-flex",
      )}
      {...props}
    />
  );
}

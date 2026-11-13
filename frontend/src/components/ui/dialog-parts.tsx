"use client";

import type { FieldsetHTMLAttributes } from "react";
import { cn } from "~/lib/utils";
import type { FormErrorInput } from "./form-error";
import { FormErrorAlert } from "./form-error-alert";

/**
 * The one place for a dialog's validation or save error (BAUARTEN-SPEC,
 * Bauart 2 Regel 5): an `Alert` at the top of the content, scrolled into view
 * when it changes. `FormModal`, `ChoiceModal` and `SlideOverBody` render it.
 */
export function DialogErrorSlot({
  error,
}: {
  readonly error: FormErrorInput | undefined;
}) {
  // mb-4 mirrors the gap of a `space-y-4` body: Tailwind v4 spaces siblings
  // with margin-bottom, so the two do not add up.
  return <FormErrorAlert message={error} className="mb-4" />;
}

type DialogActionsProps = FieldsetHTMLAttributes<HTMLFieldSetElement> & {
  /** Locks every action in the row, links (`ButtonLink`) included. */
  readonly locked: boolean;
};

/**
 * The action row of a dialog: right-aligned, wrapping, 12 px apart. While
 * `locked`, a disabled fieldset locks every action in it (Abbrechen too)
 * without each caller wiring it again. A link (`ButtonLink`) ignores the
 * fieldset, so a locked row also swallows its clicks: leaving would look like
 * a cancel too (#6533, #6593). `FormModal` and `SlideOverFooter` render it.
 */
export function DialogActions({
  locked,
  className,
  onClickCapture,
  ...props
}: DialogActionsProps) {
  return (
    <fieldset
      className={cn("flex min-w-0 flex-wrap justify-end gap-3", className)}
      {...props}
      // After the spread: no caller can lift the lock.
      disabled={locked}
      onClickCapture={(event) => {
        onClickCapture?.(event);
        if (!locked) return;
        event.preventDefault();
        event.stopPropagation();
      }}
    />
  );
}

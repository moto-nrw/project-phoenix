/**
 * The hint under a field that failed a check, for a control that has no
 * `error` prop of its own (a group of checkboxes, a row of time inputs, a
 * custom picker). `Input`, `Textarea` and `DatePicker` render the same hint
 * themselves. Point the control's `aria-describedby` at `id` and set its
 * `aria-invalid` while `message` is set.
 */
export function FieldErrorHint({
  id,
  message,
  live = false,
  as: Tag = "p",
  className = "",
}: Readonly<{
  /** The hint's own id, referenced by the control's `aria-describedby`. */
  id: string;
  /** Nothing renders while it is empty. */
  message: string | null | undefined;
  /**
   * Read the hint out the moment it appears (`role="alert"`). For a check
   * that runs while the person edits; a hint after Speichern is announced
   * by the form's error alert instead.
   */
  live?: boolean;
  /** `span` inside phrasing content such as a `label`. */
  as?: "p" | "span";
  className?: string;
}>) {
  if (!message) return null;
  return (
    <Tag
      id={id}
      role={live ? "alert" : undefined}
      className={`text-moto-red-strong mt-1 block text-xs ${className}`.trim()}
    >
      {message}
    </Tag>
  );
}

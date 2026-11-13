/**
 * The hint under a field that failed a check, for a control that has no
 * `error` prop of its own (a group of checkboxes, a row of time inputs, a
 * custom picker). The kit fields render it through `FieldShell`. Point the control's `aria-describedby` at `id` and set its
 * `aria-invalid` while `message` is set.
 */
export function FieldErrorHint({
  id,
  message,
  live = false,
  as: Tag = "p",
  tone = "strong",
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
  /**
   * `base`: the lighter red the date fields still use until #6798
   * decides (`FieldShell`); everything else keeps the default.
   */
  tone?: "strong" | "base";
  className?: string;
}>) {
  if (!message) return null;
  const color = tone === "base" ? "text-moto-red" : "text-moto-red-strong";
  return (
    <Tag
      id={id}
      role={live ? "alert" : undefined}
      className={`${color} mt-1 block text-sm ${className}`.trim()}
    >
      {message}
    </Tag>
  );
}

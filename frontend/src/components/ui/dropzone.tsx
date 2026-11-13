"use client";

import { useKitDefault } from "~/components/ui/kit-locale";
import { useRef, useState, type DragEvent } from "react";
import { Upload } from "lucide-react";
import { Button } from "~/components/ui/button";
import { SpinnerIcon } from "~/components/ui/icons";
import { cn } from "~/lib/utils";

interface DropzoneBaseProps {
  /** Value of the file input's `accept`; it limits the file dialog only. */
  readonly accept: string;
  /** Names the area for screen readers. */
  readonly legend: string;
  /** Main line, for example „Datei hierher ziehen“. */
  readonly label: string;
  /** Replaces `label` while a file is held over the area. */
  readonly dragLabel: string;
  /** Text of the button that opens the file dialog. */
  readonly buttonLabel: string;
  /** What the button does (#6744); defaults to „Öffnet die Dateiauswahl Ihres Geräts.“ */
  readonly buttonTooltip?: string;
  /** Names the hidden file input. Defaults to `buttonLabel`. */
  readonly inputLabel?: string;
  /** Limits worth knowing up front: file types, size. */
  readonly hint?: string;
  readonly loading?: boolean;
  /** Replaces `label` while `loading` is set. */
  readonly loadingLabel?: string;
  readonly disabled?: boolean;
}

type DropzoneProps = DropzoneBaseProps &
  (
    | {
        readonly multiple?: false;
        readonly onFile: (file: File) => void;
        readonly onFiles?: never;
      }
    | {
        readonly multiple: true;
        readonly onFiles: (files: File[]) => void;
        readonly onFile?: never;
      }
  );

/**
 * Drag handling for a surface that takes dropped files next to its own
 * controls, such as the chat composer (#6685): spread `dropProps` onto the
 * frame and style it while `dragging`. The file dialog stays with the caller.
 * Dropping skips any `accept`; the caller checks type and size.
 */
export function useFileDrop({
  onFiles,
  disabled = false,
}: Readonly<{
  onFiles: (files: File[]) => void;
  disabled?: boolean;
}>) {
  const [dragging, setDragging] = useState(false);
  // Only a drag that carries files counts: dragged text stays with the field.
  const carriesFiles = (event: DragEvent) =>
    Array.from(event.dataTransfer.types).includes("Files");
  const over = (event: DragEvent) => {
    if (!carriesFiles(event)) return;
    // Also while disabled: otherwise the browser opens the dropped file.
    event.preventDefault();
    if (!disabled) setDragging(true);
  };
  const leave = (event: DragEvent) => {
    const next = event.relatedTarget;
    if (next instanceof Node && event.currentTarget.contains(next)) return;
    setDragging(false);
  };
  const drop = (event: DragEvent) => {
    if (!carriesFiles(event)) return;
    event.preventDefault();
    setDragging(false);
    const files = Array.from(event.dataTransfer.files);
    if (!disabled && files.length > 0) onFiles(files);
  };
  return {
    dragging,
    dropProps: {
      onDragEnter: over,
      onDragOver: over,
      onDragLeave: leave,
      onDrop: drop,
    },
  };
}

/**
 * The one way to hand a file to the app by dragging or choosing it (#5599).
 *
 * The visible button is the only control: a real `Button`, so Enter and Space
 * work natively. Its hit area stretches over the whole frame, so a click
 * anywhere in the frame opens the dialog as well.
 *
 * Dropping skips the input's `accept`. Checking type and size stays with the
 * caller, which knows the message to show; the backend validates either way.
 */
export function Dropzone({
  accept,
  legend,
  label,
  dragLabel,
  buttonLabel,
  buttonTooltip,
  inputLabel = buttonLabel,
  hint,
  loading = false,
  loadingLabel,
  disabled = false,
  multiple = false,
  onFile,
  onFiles,
}: DropzoneProps) {
  const kitDefault = useKitDefault();
  const inputRef = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);
  const inactive = disabled || loading;

  const deliver = (files: File[]) => {
    if (inactive || files.length === 0) return;
    if (onFiles) {
      onFiles(files);
      return;
    }
    const [first] = files;
    if (first) onFile?.(first);
  };

  // preventDefault also runs while inactive: otherwise the browser navigates
  // away to the dropped file.
  const handleDragOver = (event: DragEvent) => {
    event.preventDefault();
    if (!inactive) setDragging(true);
  };

  const handleDragLeave = (event: DragEvent) => {
    event.preventDefault();
    // Moving onto a child also fires dragleave; only leaving the frame counts.
    const next = event.relatedTarget;
    if (next instanceof Node && event.currentTarget.contains(next)) return;
    setDragging(false);
  };

  const handleDrop = (event: DragEvent) => {
    event.preventDefault();
    setDragging(false);
    deliver(Array.from(event.dataTransfer.files));
  };

  let line = label;
  if (loading && loadingLabel) line = loadingLabel;
  else if (dragging) line = dragLabel;

  return (
    <fieldset
      disabled={inactive}
      aria-busy={loading || undefined}
      onDragEnter={handleDragOver}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
      className={cn(
        "relative m-0 flex min-w-0 flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed px-4 py-6 text-center transition-colors",
        dragging
          ? "border-moto-green bg-moto-green-soft"
          : "border-gray-200 bg-gray-50/50",
        !dragging && !inactive && "hover:border-gray-300",
      )}
    >
      <legend className="sr-only">{legend}</legend>
      {loading ? (
        <SpinnerIcon className="h-5 w-5 text-gray-500 motion-reduce:animate-none" />
      ) : (
        <Upload
          className={cn(
            "h-5 w-5 transition-colors",
            dragging ? "text-moto-green-strong" : "text-gray-400",
          )}
          aria-hidden="true"
        />
      )}
      {/* A finger cannot drag a file here: on a touch screen the frame
          offers only the button, unless it reports progress. */}
      <p
        className={cn(
          "text-sm font-medium text-gray-900",
          line === label && "pointer-coarse:sr-only",
        )}
        aria-live="polite"
      >
        {line}
      </p>
      {loading ? null : (
        <p className="text-xs text-gray-500 pointer-coarse:hidden">oder</p>
      )}
      <Button
        type="button"
        variant="outline"
        size="md"
        disabled={inactive}
        tooltip={
          buttonTooltip ?? kitDefault("Öffnet die Dateiauswahl Ihres Geräts.")
        }
        onClick={() => inputRef.current?.click()}
        // A disabled button swallows drag events: let them through to the frame.
        className="after:absolute after:inset-0 disabled:pointer-events-none"
      >
        {buttonLabel}
      </Button>
      {hint ? <p className="text-sm text-gray-500">{hint}</p> : null}
      <input
        ref={inputRef}
        type="file"
        accept={accept}
        multiple={multiple}
        aria-label={inputLabel}
        hidden
        disabled={inactive}
        onChange={(event) => {
          const picked = Array.from(event.currentTarget.files ?? []);
          // Without the reset, choosing the same file again fires no change:
          // the common „Datei korrigieren und erneut hochladen“ case.
          event.currentTarget.value = "";
          deliver(picked);
        }}
      />
    </fieldset>
  );
}

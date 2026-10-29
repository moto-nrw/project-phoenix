"use client";

import { ChevronDown, FileSpreadsheet, FileText, Printer } from "lucide-react";

import { buttonClassName } from "~/components/ui/button";
import {
  EXPORT_FORMAT_LABELS,
  type ExportFileFormat,
} from "~/components/ui/export-dialog";
import {
  OverflowMenu,
  type OverflowMenuEntry,
} from "~/components/ui/page-header/OverflowMenu";

/** One entry of an export menu: a file format, or Drucken. */
export interface ExportMenuOption {
  /** Stable key; also what `busyKey` names while this entry runs. */
  readonly key: string;
  /** The file format; omit for the Drucken entry. */
  readonly format?: ExportFileFormat;
  /** Visible label; defaults to the format name („PDF“, „Word“, …). */
  readonly label?: string;
}

/** One entry per format, in this order. */
export function exportFormatOptions(
  formats: readonly ExportFileFormat[],
): ExportMenuOption[] {
  return formats.map((format) => ({ key: format, format }));
}

interface ExportMenuItemsArgs {
  readonly options: readonly ExportMenuOption[];
  readonly onSelect: (option: ExportMenuOption) => void;
  /** The entry whose file is being created; every entry waits meanwhile. */
  readonly busyKey?: string | null;
  /** The data behind the export is not there yet (loading, failed). */
  readonly unavailable?: boolean;
  /** Group heading above the entries; `null` for none. */
  readonly header?: string | null;
}

function optionIcon(option: ExportMenuOption) {
  if (!option.format) return <Printer className="h-4 w-4" aria-hidden />;
  return option.format === "xlsx" || option.format === "csv" ? (
    <FileSpreadsheet className="h-4 w-4" aria-hidden />
  ) : (
    <FileText className="h-4 w-4" aria-hidden />
  );
}

/**
 * Entries for an export without options (docs/reference/exporte.md,
 * Regel 2): one per format, with the format names of Regel 3. Pages put them
 * into their own `OverflowMenu`, next to their other actions.
 */
export function exportMenuItems({
  options,
  onSelect,
  busyKey = null,
  unavailable = false,
  header = "Herunterladen",
}: ExportMenuItemsArgs): OverflowMenuEntry[] {
  const entries: OverflowMenuEntry[] = options.map((option) => ({
    label:
      busyKey === option.key
        ? option.format
          ? "Wird erstellt…"
          : "Wird geöffnet…"
        : (option.label ??
          (option.format ? EXPORT_FORMAT_LABELS[option.format] : "Drucken")),
    icon: optionIcon(option),
    disabled: unavailable || busyKey !== null,
    onClick: () => onSelect(option),
  }));
  return header ? [{ kind: "header", label: header }, ...entries] : entries;
}

/**
 * An export without options on a card or panel that has no page menu of its
 * own: a „Herunterladen“ button that opens the same entries as
 * `exportMenuItems`.
 */
export function ExportMenu({
  label = "Herunterladen",
  ariaLabel,
  ...args
}: ExportMenuItemsArgs & {
  /** Visible trigger text. */
  readonly label?: string;
  /** Accessible name of the trigger when the text alone is not enough. */
  readonly ariaLabel?: string;
}) {
  return (
    <OverflowMenu
      ariaLabel={ariaLabel ?? label}
      disabled={args.unavailable}
      items={exportMenuItems({ ...args, header: args.header ?? null })}
      triggerContent={
        <span
          className={buttonClassName({
            variant: "outline",
            size: "md",
            className: "gap-2 whitespace-nowrap",
          })}
        >
          {label}
          <ChevronDown className="h-4 w-4" aria-hidden />
        </span>
      }
    />
  );
}

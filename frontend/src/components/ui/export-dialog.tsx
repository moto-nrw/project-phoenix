"use client";

import { useEffect, useState } from "react";
import type { ReactNode, RefObject } from "react";
import { Download, Printer } from "lucide-react";

import { Button } from "~/components/ui/button";
import {
  EXPORT_ACTION_LABELS,
  EXPORT_BUSY_LABELS,
  EXPORT_FORMAT_LABELS,
  type ExportFileFormat,
  type ExportRunKind,
} from "~/components/ui/export-formats";
import type { FormErrorInput } from "~/components/ui/form-error";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { SegmentedControl } from "~/components/ui/segmented-control";
import {
  SlideOver,
  SlideOverBody,
  SlideOverCancelButton,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverDescription,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { StatusBadge } from "~/components/ui/status-badge";

/**
 * State of a running export. `useExportRun` (`~/hooks/use-export-run`)
 * produces it: the kit may not reach the toast context the error path
 * lives in.
 */
export interface ExportDialogRun<F extends ExportFileFormat> {
  readonly busy: ExportRunKind | null;
  readonly error: FormErrorInput;
  readonly bodyRef: RefObject<HTMLDivElement | null>;
  readonly download: (format: F) => void;
  /** Remembers a newly selected format for a possible retry. */
  readonly updateFormat: (format: F) => void;
  /** Present when the export can print. */
  readonly print?: () => void;
}

interface ExportDialogProps<F extends ExportFileFormat> {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  /** Heading of the dialog, e.g. „Klassenliste exportieren“. */
  readonly title: string;
  /** One sentence: which records the file contains. */
  readonly description?: string;
  /**
   * Filters the page hands over, already as readable labels („Klasse 3a“).
   * Shown read-only, so nobody wonders why the file is shorter than the
   * school.
   */
  readonly appliedFilters?: readonly string[];
  /** Offered formats in display order. One format shows no choice. */
  readonly formats: readonly F[];
  /** Preselected format; defaults to the first one. */
  readonly defaultFormat?: F;
  readonly run: ExportDialogRun<F>;
  /**
   * The options cannot produce a file right now (an invalid period). The
   * options themselves say why; Drucken and Herunterladen wait.
   */
  readonly blocked?: boolean;
  /**
   * Locks every way to close the dialog without a running operation; the
   * one name for this lock in every kit dialog (#6575). A running export locks it too.
   */
  readonly isDismissDisabled?: boolean;
  /** Options of this export (template, period, columns). */
  readonly children?: ReactNode;
}

/**
 * The one dialog behind every export with options (docs/reference/exporte.md,
 * Regel 2): the options of the export, the filters it took over from the
 * page, the format as a value choice, and Drucken next to Herunterladen where
 * the export can print. Errors stay in the open dialog.
 *
 * Exports without options do not open a dialog; they are entries in the
 * page's `OverflowMenu`, one per format.
 */
export function ExportDialog<F extends ExportFileFormat>({
  isOpen,
  onClose,
  title,
  description,
  appliedFilters,
  formats,
  defaultFormat,
  run,
  blocked = false,
  isDismissDisabled = false,
  children,
}: ExportDialogProps<F>) {
  const initialFormat = defaultFormat ?? formats[0];
  const [format, setFormat] = useState<F | undefined>(initialFormat);

  // Ein neu geöffneter Dialog beginnt wieder beim voreingestellten Format.
  useEffect(() => {
    if (isOpen) setFormat(initialFormat);
  }, [isOpen, initialFormat]);

  if (!isOpen) return null;

  const busy = run.busy;
  const filters = appliedFilters?.filter((label) => label.trim() !== "") ?? [];

  return (
    <SlideOver
      open={isOpen}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      // Solange ein Export läuft, bleibt der Dialog stehen.
      isDismissDisabled={isDismissDisabled || busy !== null}
    >
      <SlideOverContent size="md">
        <SlideOverHeader
          close={<SlideOverCloseButton aria-label="Export schließen" />}
        >
          <SlideOverTitle>{title}</SlideOverTitle>
          {description ? (
            <SlideOverDescription>{description}</SlideOverDescription>
          ) : null}
        </SlideOverHeader>
        <SlideOverBody ref={run.bodyRef} className="space-y-5">
          {/* Not the body's `error` slot: that keeps 16 px to the content,
              this dialog 20 px, the gap of its sections (#6716). */}
          <FormErrorAlert message={run.error} />
          {filters.length > 0 ? (
            <section>
              <p className="text-sm font-medium text-gray-900">
                Filter der Seite
              </p>
              <p className="text-xs text-gray-500">
                Die Datei enthält nur, was diese Filter zeigen.
              </p>
              <ul className="mt-2 flex flex-wrap gap-1.5">
                {filters.map((label) => (
                  <li key={label}>
                    <StatusBadge tone="gray" label={label} />
                  </li>
                ))}
              </ul>
            </section>
          ) : null}

          {children}

          {formats.length > 1 && format ? (
            <section>
              <p className="text-sm font-medium text-gray-900">Format</p>
              <div className="mt-2">
                <SegmentedControl<F>
                  ariaLabel="Format der Datei"
                  value={format}
                  onChange={(nextFormat) => {
                    setFormat(nextFormat);
                    run.updateFormat(nextFormat);
                  }}
                  items={formats.map((item) => ({
                    value: item,
                    label: EXPORT_FORMAT_LABELS[item],
                  }))}
                />
              </div>
            </section>
          ) : null}
        </SlideOverBody>
        <SlideOverFooter>
          <SlideOverCancelButton>Abbrechen</SlideOverCancelButton>
          {run.print ? (
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={run.print}
              disabled={busy !== null || blocked}
            >
              <Printer className="h-4 w-4" aria-hidden />
              {busy === "print"
                ? EXPORT_BUSY_LABELS.print
                : EXPORT_ACTION_LABELS.print}
            </Button>
          ) : null}
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => {
              if (format) run.download(format);
            }}
            disabled={busy !== null || blocked || !format}
          >
            <Download className="h-4 w-4" aria-hidden />
            {busy === "download"
              ? EXPORT_BUSY_LABELS.download
              : EXPORT_ACTION_LABELS.download}
          </Button>
        </SlideOverFooter>
      </SlideOverContent>
    </SlideOver>
  );
}

"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import { Download, Printer } from "lucide-react";

import { Button } from "~/components/ui/button";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { SegmentedControl } from "~/components/ui/segmented-control";
import {
  SlideOver,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverDescription,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ExportDialog" });

/** Every file format a moto export can produce. */
export type ExportFileFormat = "pdf" | "docx" | "xlsx" | "csv";

/**
 * Format names in the interface (docs/reference/exporte.md, Regel 3): the
 * names people know from their office programs, not the file endings.
 */
export const EXPORT_FORMAT_LABELS: Readonly<Record<ExportFileFormat, string>> =
  {
    pdf: "PDF",
    docx: "Word",
    xlsx: "Excel",
    csv: "CSV",
  };

type RunKind = "download" | "print";

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
  /** Creates and saves the file. Throws on failure. */
  readonly onDownload: (format: F) => Promise<void>;
  /**
   * Prints the PDF. `target` is the tab the dialog opened inside the click,
   * before any request: after an `await` the browser blocks new tabs
   * (docs/reference/exporte.md, Regel 4). Omit when the export cannot print.
   */
  readonly onPrint?: (target: Window) => Promise<void>;
  /** Checks the options before a run; a returned text blocks it. */
  readonly validate?: () => string | null;
  /** Object of the error text, e.g. „die Klassenliste“. */
  readonly errorObject: string;
  /** Success toast after a download. */
  readonly successMessage?: string;
  /** Panel width on desktop. */
  readonly widthClass?: string;
  /** Options of this export (template, period, columns). */
  readonly children?: ReactNode;
}

/**
 * The one dialog behind every export with options (docs/reference/exporte.md,
 * Regel 2): options of the export in the body, the filters it took over from
 * the page, the format as a value choice, and Drucken next to Herunterladen
 * where the export can print. Errors stay in the open dialog, a toast would
 * sit behind it.
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
  onDownload,
  onPrint,
  validate,
  errorObject,
  successMessage = "Die Datei ist heruntergeladen.",
  widthClass = "sm:w-[560px]",
  children,
}: ExportDialogProps<F>) {
  const toast = useToast();
  const initialFormat = defaultFormat ?? formats[0];
  const [format, setFormat] = useState<F | undefined>(initialFormat);
  const [busy, setBusy] = useState<RunKind | null>(null);
  const bodyRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(bodyRef);
  const clearErrors = formErrors.clear;
  // „Wiederholen“ läuft mit den dann gewählten Optionen.
  const latestRunRef = useRef<(kind: RunKind) => void>(() => undefined);

  // Ein neu geöffneter Dialog beginnt ohne den Fehler und das Format des
  // letzten Laufs.
  useEffect(() => {
    if (!isOpen) return;
    setFormat(initialFormat);
    clearErrors();
  }, [isOpen, initialFormat, clearErrors]);

  const run = async (kind: RunKind) => {
    const problem = validate?.() ?? null;
    if (problem) {
      formErrors.invalid(problem);
      return;
    }
    let target: Window | null = null;
    if (kind === "print") {
      target = globalThis.open("", "_blank");
      if (!target) {
        // Der Browser hat das neue Fenster blockiert. Das ist kein Fehler
        // der Schule oder des Servers, deshalb ein eigener Hinweis.
        formErrors.invalid(
          "Das Druckfenster ging nicht auf. Bitte erlauben Sie neue Fenster für moto.",
        );
        return;
      }
    }
    setBusy(kind);
    formErrors.clear();
    try {
      if (target && onPrint) {
        await onPrint(target);
      } else if (format) {
        await onDownload(format);
        toast.success(successMessage);
      }
      onClose();
    } catch (error) {
      if (target && !target.closed) target.close();
      logger.error("export_dialog_failed", {
        export: title,
        mode: kind,
        format: kind === "print" ? "pdf" : (format ?? ""),
        error: error instanceof Error ? error.message : String(error),
      });
      await formErrors.show(error, {
        object: errorObject,
        // Der Klick auf „Wiederholen“ ist wieder eine Nutzeraktion: das
        // Druckfenster darf dort erneut aufgehen.
        retry: () => latestRunRef.current(kind),
      });
    } finally {
      setBusy(null);
    }
  };
  useLayoutEffect(() => {
    latestRunRef.current = (kind) => void run(kind);
  });

  if (!isOpen) return null;

  const filters = appliedFilters?.filter((label) => label.trim() !== "") ?? [];

  return (
    <SlideOver
      open={isOpen}
      onOpenChange={(open) => {
        // Solange ein Export läuft, bleibt der Dialog stehen.
        if (!open && busy === null) onClose();
      }}
    >
      <SlideOverContent widthClass={widthClass}>
        <SlideOverHeader className="flex-row items-start justify-between gap-3">
          <div className="min-w-0">
            <SlideOverTitle>{title}</SlideOverTitle>
            {description ? (
              <SlideOverDescription>{description}</SlideOverDescription>
            ) : null}
          </div>
          <SlideOverCloseButton
            aria-label="Export schließen"
            disabled={busy !== null}
          />
        </SlideOverHeader>
        <div
          ref={bodyRef}
          className="flex-1 space-y-5 overflow-y-auto px-5 py-4"
        >
          <FormErrorAlert message={formErrors.error} />
          {filters.length > 0 ? (
            <section aria-labelledby="export-dialog-filters">
              <p
                id="export-dialog-filters"
                className="text-sm font-medium text-gray-900"
              >
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
                  onChange={setFormat}
                  items={formats.map((item) => ({
                    value: item,
                    label: EXPORT_FORMAT_LABELS[item],
                  }))}
                />
              </div>
            </section>
          ) : null}
        </div>
        <SlideOverFooter className="flex-row justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            disabled={busy !== null}
          >
            Abbrechen
          </Button>
          {onPrint ? (
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => void run("print")}
              disabled={busy !== null}
            >
              <Printer className="h-4 w-4" aria-hidden />
              {busy === "print" ? "Wird geöffnet…" : "Drucken"}
            </Button>
          ) : null}
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => void run("download")}
            disabled={busy !== null}
          >
            <Download className="h-4 w-4" aria-hidden />
            {busy === "download" ? "Wird erstellt…" : "Herunterladen"}
          </Button>
        </SlideOverFooter>
      </SlideOverContent>
    </SlideOver>
  );
}

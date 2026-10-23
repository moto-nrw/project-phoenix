"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";

import type {
  ExportDialogRun,
  ExportFileFormat,
  ExportRunKind,
} from "~/components/ui/export-dialog";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ExportDialog" });

interface UseExportRunOptions<F extends ExportFileFormat> {
  /** Whether the dialog is open; a reopened dialog starts without an error. */
  readonly isOpen: boolean;
  /** Creates and saves the file. Throws on failure. */
  readonly onDownload: (format: F) => Promise<void>;
  /**
   * Prints the PDF into `target`, the tab opened inside the click before any
   * request: after an `await` the browser blocks new tabs
   * (docs/reference/exporte.md, Regel 4). Omit when the export cannot print.
   */
  readonly onPrint?: (target: Window) => Promise<void>;
  /** Checks the options before a run; a returned text blocks it. */
  readonly validate?: () => string | null;
  /** Object of the error text, e.g. „die Klassenliste“. */
  readonly errorObject: string | ((kind: ExportRunKind) => string);
  /** Success toast after a download. */
  readonly successMessage?: string;
  /** Closes the dialog after a successful run. */
  readonly onDone: () => void;
  /** Names the export in the log line. */
  readonly logName: string;
}

/**
 * Runs an export from the shared `ExportDialog`: validation, the print tab
 * opened inside the click, the success toast, and failures through the error
 * path (`useApiFormError`). The error stays in the open dialog because a toast
 * would sit behind it; „Wiederholen“ runs with the options chosen by then.
 *
 * The dialog itself lives in the UI kit, which may not reach the toast
 * context, so the run lives here and the dialog receives its state.
 */
export function useExportRun<F extends ExportFileFormat>({
  isOpen,
  onDownload,
  onPrint,
  validate,
  errorObject,
  successMessage = "Die Datei ist heruntergeladen.",
  onDone,
  logName,
}: UseExportRunOptions<F>): ExportDialogRun<F> {
  const toast = useToast();
  const bodyRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(bodyRef);
  const clearErrors = formErrors.clear;
  const [busy, setBusy] = useState<ExportRunKind | null>(null);
  const latestRunRef = useRef<(kind: ExportRunKind, format: F) => void>(
    () => undefined,
  );
  const latestDownloadFormatRef = useRef<F | null>(null);

  useEffect(() => {
    if (isOpen) clearErrors();
  }, [isOpen, clearErrors]);

  const run = async (kind: ExportRunKind, format: F) => {
    if (kind === "download") latestDownloadFormatRef.current = format;
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
      } else {
        await onDownload(format);
        toast.success(successMessage);
      }
      onDone();
    } catch (error) {
      if (target && !target.closed) target.close();
      logger.error("export_failed", {
        export: logName,
        mode: kind,
        format: kind === "print" ? "pdf" : format,
        error: error instanceof Error ? error.message : String(error),
      });
      await formErrors.show(error, {
        object:
          typeof errorObject === "string" ? errorObject : errorObject(kind),
        // Der Klick auf „Wiederholen“ ist wieder eine Nutzeraktion: das
        // Druckfenster darf dort erneut aufgehen.
        retry: () =>
          latestRunRef.current(
            kind,
            kind === "download"
              ? (latestDownloadFormatRef.current ?? format)
              : format,
          ),
      });
    } finally {
      setBusy(null);
    }
  };
  useLayoutEffect(() => {
    latestRunRef.current = (kind, format) => void run(kind, format);
  });

  return {
    busy,
    error: formErrors.error,
    bodyRef,
    download: (format) => latestRunRef.current("download", format),
    updateFormat: (format) => {
      latestDownloadFormatRef.current = format;
    },
    print: onPrint
      ? () => latestRunRef.current("print", "pdf" as F)
      : undefined,
  };
}

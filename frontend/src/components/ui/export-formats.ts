/**
 * The file formats of moto exports, shared by the export dialog
 * (`ExportDialog`) and the export menu entries (`exportMenuItems`).
 */

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

/** What a running export does: create a file or open the print view. */
export type ExportRunKind = "download" | "print";

/** Label of the action while its export runs. */
export const EXPORT_BUSY_LABELS: Readonly<Record<ExportRunKind, string>> = {
  download: "Wird erstellt…",
  print: "Wird geöffnet…",
};

import { DataTable } from "~/components/ui/data-table";
import type { DocumentTemplatePreview } from "~/lib/traeger-document-templates";
import { cn } from "~/lib/utils";

/**
 * Der Text eines Briefs aus einer Vorlage mit Beispieldaten (#4677): Betreff,
 * Anrede, Absätze mit Überschrift und Tabelle, Grußformel. So legt ihn auch
 * das PDF an, nur ohne Seitenumbruch.
 */
export function DocumentTemplatePreviewBody({
  preview,
}: Readonly<{ preview: DocumentTemplatePreview }>) {
  return (
    <div className="space-y-4 wrap-anywhere">
      <p className="font-semibold text-gray-900">{preview.subject}</p>
      {preview.salutation ? <p>{preview.salutation}</p> : null}
      {preview.blocks.map((block, index) => (
        // Die Absätze haben keine eigene Kennung; ihre Reihenfolge ist fest.
        // oxlint-disable-next-line react/no-array-index-key -- static order of the filled template
        <div key={index} className="space-y-2">
          {block.heading ? (
            <p className="font-semibold text-gray-900">{block.heading}</p>
          ) : null}
          {block.text ? (
            <p
              className={cn(
                "whitespace-pre-line",
                block.bold && "font-semibold text-gray-900",
              )}
            >
              {block.text}
            </p>
          ) : null}
          {block.rows.length > 0 ? (
            <DataTable
              columns={[
                {
                  key: "label",
                  header: "Bezeichnung",
                  render: (row) => row.label,
                },
                {
                  key: "value",
                  header: "Wert",
                  render: (row) => row.value,
                  align: "right",
                  className: "whitespace-nowrap tabular-nums",
                },
              ]}
              rows={block.rows.map((row, rowIndex) => ({ ...row, rowIndex }))}
              getRowKey={(row) => row.rowIndex}
              rowClassName={(row) =>
                row.bold ? "font-semibold text-gray-900" : ""
              }
            />
          ) : null}
        </div>
      ))}
      {preview.closing.length > 0 ? (
        <div>
          {preview.closing.map((line) => (
            <p key={line}>{line}</p>
          ))}
        </div>
      ) : null}
    </div>
  );
}

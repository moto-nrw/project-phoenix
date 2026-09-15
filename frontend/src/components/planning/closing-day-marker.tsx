"use client";

/**
 * Schließtag-Markierung und -Warnung für die Planungsraster (#2032).
 *
 * Schließtage werden sichtbar gemacht und beim Planen bestätigt, nicht
 * blockiert: Ferien- und Notbetreuung an Schließtagen kommt vor, und der
 * Dienstplan braucht an pädagogischen Tagen ohnehin Schichten.
 *
 * Farbgebung: neutrales Grau (LOCATION_COLORS.UNKNOWN) für die Markierung,
 * dieselbe Sprache wie das „Schließtag“-Badge der Zeiterfassung (#1418 3b),
 * und das Warn-Orange (LOCATION_COLORS.SCHOOLYARD) für den Bestätigen-Knopf,
 * wie bei den übrigen „Trotzdem fortfahren“-Dialogen des Betreuungsplans.
 *
 * Eigene Komponente statt ui/StatusBadge: die Raster brauchen Symbol, Tooltip
 * und eine 10px-Zeile für Spalten ab 52px Breite; StatusBadge kennt weder
 * Symbol noch Tooltip und ist doppelt so hoch. ui/OriginChip scheidet laut
 * eigener Doku aus (bewusst auf drei Stellen begrenzt).
 */

import { ConfirmationModal } from "~/components/ui/modal";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { formatDate } from "~/lib/date-helpers";

/** Was auf den Schließtag gelegt werden soll, steuert nur die Wortwahl. */
export type ClosingDaySubject = "termin" | "schicht";

/**
 * Kennzeichnung eines Schließtags. Beschriftung und Grund stehen immer im
 * Tooltip; `text` ersetzt den sichtbaren Teil für schmale Spalten.
 */
export function ClosingDayChip({
  reason,
  label = "Schließtag",
  text,
  wrap = false,
  className,
}: {
  /** Grund des Schließtags; in der Halbjahres-Spalte die Liste der Tage. */
  readonly reason: string;
  /** Überschrift. Default „Schließtag“; Wochenspalten setzen „3 Schließtage“. */
  readonly label?: string;
  /** Sichtbarer Kurztext statt „Label · Grund“, z. B. die Anzahl. */
  readonly text?: string;
  /** Text umbrechen statt kürzen, für schmale Spalten mit Platz nach unten. */
  readonly wrap?: boolean;
  readonly className?: string;
}) {
  const title = `${label}: ${reason}`;
  return (
    <span
      title={title}
      className={`inline-flex max-w-full items-center gap-1 rounded-full bg-gray-100 px-1.5 py-0.5 text-[10px] leading-tight font-medium text-gray-700 ${className ?? ""}`}
    >
      <MotoConceptIcon concept="closingDays" size={14} />
      {text === undefined ? (
        <span
          className={wrap ? "break-words" : "truncate"}
        >{`${label} · ${reason}`}</span>
      ) : (
        <>
          <span aria-hidden className="tabular-nums">
            {text}
          </span>
          <span className="sr-only">{title}</span>
        </>
      )}
    </span>
  );
}

/**
 * Bestätigbare Warnung, bevor ein Termin bzw. eine Schicht auf einen
 * Schließtag gelegt wird. Bestätigen führt die Aktion aus, blockiert wird
 * nichts. Der Aufrufer mountet den Dialog nur, solange er offen sein soll.
 */
export function ClosingDayConfirmModal({
  dateISO,
  reason,
  subject,
  onCancel,
  onConfirm,
}: {
  /** Betroffener Kalendertag als "YYYY-MM-DD". */
  readonly dateISO: string;
  readonly reason: string;
  readonly subject: ClosingDaySubject;
  readonly onCancel: () => void;
  readonly onConfirm: () => void;
}) {
  return (
    <ConfirmationModal
      isOpen
      onClose={onCancel}
      onConfirm={onConfirm}
      title="An einem Schließtag planen?"
      confirmText="Trotzdem planen"
      cancelText="Abbrechen"
      confirmVariant="warning"
    >
      <div className="flex flex-col gap-3">
        <p className="text-sm leading-relaxed text-gray-600">
          Für den{" "}
          <span className="font-semibold text-gray-900">
            {formatDate(dateISO)}
          </span>{" "}
          ist ein Schließtag hinterlegt:{" "}
          <span className="font-medium">{reason}</span>.
        </p>
        <p className="text-sm leading-relaxed text-gray-600">
          {subject === "termin"
            ? "Der Termin wird trotzdem angelegt, etwa für eine Ferien- oder Notbetreuung."
            : "Die Schicht wird trotzdem geplant, etwa für einen pädagogischen Tag oder eine Notbetreuung."}
        </p>
      </div>
    </ConfirmationModal>
  );
}

"use client";

/**
 * Schließtag-Markierung und -Warnung für die Planungsraster (#2032).
 *
 * Schließtage werden sichtbar gemacht und beim Planen bestätigt, nicht
 * blockiert: Ferien- und Notbetreuung an Schließtagen kommt vor, und der
 * Dienstplan braucht an pädagogischen Tagen ohnehin Schichten.
 *
 * Farbgebung: neutrales Grau (LOCATION_COLORS.UNKNOWN) für die Markierung —
 * dieselbe Sprache wie das „Schließtag“-Badge der Zeiterfassung (#1418 3b) —
 * und das Warn-Orange (LOCATION_COLORS.SCHOOLYARD) für den Bestätigen-Knopf,
 * wie bei den übrigen „Trotzdem fortfahren“-Dialogen des Betreuungsplans.
 */

import { CalendarOff } from "lucide-react";

import { ConfirmationModal } from "~/components/ui/modal";
import { formatDate } from "~/lib/date-helpers";
import { LOCATION_COLORS } from "~/lib/location-helper";

/** Was auf den Schließtag gelegt werden soll — steuert nur die Wortwahl. */
export type ClosingDaySubject = "termin" | "schicht";

const SUBJECT_LABEL: Record<ClosingDaySubject, string> = {
  termin: "Termin",
  schicht: "Schicht",
};

/**
 * Kompakte Kennzeichnung eines Schließtags mit Grund. `variant="compact"`
 * zeigt nur das Symbol (für schmale Spaltenköpfe), der Grund steht dann im
 * Tooltip und im Screenreader-Text.
 */
export function ClosingDayChip({
  reason,
  label: caption = "Schließtag",
  variant = "full",
  className,
}: {
  readonly reason: string;
  /** Überschrift des Chips. Default „Schließtag“; Wochenspalten setzen z. B.
   *  „3 Schließtage“ und übergeben die Tage als `reason`. */
  readonly label?: string;
  readonly variant?: "full" | "compact";
  readonly className?: string;
}) {
  const label = reason === "" ? caption : `${caption}: ${reason}`;
  return (
    <span
      title={label}
      className={`inline-flex max-w-full items-center gap-1 rounded-full bg-gray-100 px-1.5 py-0.5 text-[10px] leading-tight font-medium text-gray-700 ${className ?? ""}`}
    >
      <CalendarOff
        className="h-3 w-3 shrink-0"
        style={{ color: LOCATION_COLORS.UNKNOWN }}
        aria-hidden
      />
      {variant === "compact" ? (
        <span className="sr-only">{label}</span>
      ) : (
        <span className="truncate">
          {caption}
          {reason === "" ? "" : ` · ${reason}`}
        </span>
      )}
    </span>
  );
}

/**
 * Bestätigbare Warnung, bevor ein Termin bzw. eine Schicht auf einen
 * Schließtag gelegt wird. Bestätigen führt die Aktion aus — es wird nichts
 * hart blockiert.
 */
export function ClosingDayConfirmModal({
  isOpen,
  dateISO,
  reason,
  subject,
  onCancel,
  onConfirm,
  isConfirmLoading = false,
}: {
  readonly isOpen: boolean;
  /** Betroffener Kalendertag als "YYYY-MM-DD". */
  readonly dateISO: string;
  readonly reason: string;
  readonly subject: ClosingDaySubject;
  readonly onCancel: () => void;
  readonly onConfirm: () => void;
  readonly isConfirmLoading?: boolean;
}) {
  const subjectLabel = SUBJECT_LABEL[subject];
  return (
    <ConfirmationModal
      isOpen={isOpen}
      onClose={onCancel}
      onConfirm={onConfirm}
      title="An einem Schließtag planen?"
      confirmText="Trotzdem planen"
      cancelText="Abbrechen"
      isConfirmLoading={isConfirmLoading}
      confirmButtonClass="bg-[#F78C10] hover:bg-[#d97908]"
    >
      <div className="flex flex-col gap-3">
        <p className="text-sm leading-relaxed text-gray-600">
          Für den{" "}
          <span className="font-semibold text-gray-900">
            {formatDate(dateISO)}
          </span>{" "}
          ist ein Schließtag hinterlegt
          {reason === "" ? "" : ": "}
          {reason === "" ? "" : <span className="font-medium">{reason}</span>}.
        </p>
        <p className="text-sm leading-relaxed text-gray-600">
          {subjectLabel === "Termin"
            ? "Der Termin wird trotzdem angelegt — etwa für eine Ferien- oder Notbetreuung."
            : "Die Schicht wird trotzdem geplant — etwa für einen pädagogischen Tag oder eine Notbetreuung."}
        </p>
      </div>
    </ConfirmationModal>
  );
}

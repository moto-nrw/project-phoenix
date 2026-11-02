// CSV-Export der Nachweise (#4808).

import { formatDate } from "./date-helpers";
import {
  QUALIFICATION_STATUS_LABELS,
  type QualificationOverviewRow,
} from "./staff-qualification-api";

export function qualificationPersonName(row: QualificationOverviewRow): string {
  return `${row.firstName} ${row.lastName}`.trim();
}

function csvCell(raw: string): string {
  // Ein Wert, den Excel als Formel läse (= + - @, Tab, Wagenrücklauf),
  // bekommt ein Apostroph davor: ein freier Name wie „=HYPERLINK(…)“ bleibt
  // Text.
  const value = /^[=+\-@\t\r]/.test(raw) ? `'${raw}` : raw;
  return /[";\n\r]/.test(value) ? `"${value.replaceAll('"', '""')}"` : value;
}

/**
 * Eine CSV-Datei, die Excel ohne Import-Dialog richtig öffnet: Semikolon als
 * Trenner, UTF-8 mit BOM für die Umlaute, Datum als TT.MM.JJJJ.
 */
export function qualificationsCsv(
  rows: readonly QualificationOverviewRow[],
): string {
  const header = [
    "Name",
    "Nachweis",
    "Erworben",
    "Gültig bis",
    "Status",
    "Pflicht",
  ];
  const lines = rows.map((row) =>
    [
      qualificationPersonName(row),
      row.name,
      row.acquiredOn ? formatDate(row.acquiredOn) : "",
      row.expiresOn ? formatDate(row.expiresOn) : "",
      QUALIFICATION_STATUS_LABELS[row.status].label,
      row.mandatory ? "Ja" : "Nein",
    ]
      .map(csvCell)
      .join(";"),
  );
  return `﻿${[header.join(";"), ...lines].join("\r\n")}\r\n`;
}

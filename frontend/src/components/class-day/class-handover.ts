// Übergabe nach dem Unterricht in der Klassenansicht (#4794).
//
// Die Lehrkraft übergibt die Klasse heute an die OGS. Übergeben werden die
// Kinder, die in der Betreuung bleiben; vorbelegt sind sie mit „Da“, ein Kind,
// das am Vormittag als „fehlt“ eingetragen ist, mit „Nicht da“. Die Lehrkraft
// ändert einzelne Kinder und bestätigt die Übergabe für die ganze Klasse.

import type { ClassDayHandover, ClassDayRow } from "~/lib/class-day-api";
import { berlinClockFromISO } from "~/lib/date-helpers";
import {
  morningStatus,
  type MorningAttendanceDraft,
} from "./morning-attendance";

/** Die Kinder, die heute aus der Klasse in die OGS gehen. */
export function handoverRows(rows: readonly ClassDayRow[]): ClassDayRow[] {
  return rows.filter(
    (row) => row.stays_today && !row.list_entry && row.student_id > 0,
  );
}

/**
 * Die Vorbelegung „Nicht da“: bei einer bestehenden Übergabe ihr Stand,
 * sonst die Kinder, die am Vormittag als „fehlt“ eingetragen sind, auch
 * wenn diese Eintragung noch nicht gespeichert ist.
 */
export function initialAbsentIds(
  rows: readonly ClassDayRow[],
  handedOver: boolean,
  draft: MorningAttendanceDraft = {},
): ReadonlySet<number> {
  const absent = new Set<number>();
  for (const row of handoverRows(rows)) {
    const marked = handedOver
      ? row.handover_absent === true
      : morningStatus(row, draft) === "absent";
    if (marked) absent.add(row.student_id);
  }
  return absent;
}

/**
 * „Übergeben um 11:45 Uhr von Petra Lehrkraft · 1 Kind nicht da“. Ohne
 * Kinder, die in die OGS gehen, (`ogsChildren` 0) entfällt „alle da“.
 */
export function handoverLine(
  handover: ClassDayHandover,
  ogsChildren?: number,
): string {
  const clock = berlinClockFromISO(handover.handed_over_at);
  const by = handover.handed_over_by ? ` von ${handover.handed_over_by}` : "";
  const absent =
    handover.absent_count === 0
      ? ogsChildren === 0
        ? ""
        : " · alle da"
      : ` · ${handover.absent_count === 1 ? "1 Kind" : `${handover.absent_count} Kinder`} nicht da`;
  const at = clock ? ` um ${clock} Uhr` : "";
  return `Übergeben${at}${by}${absent}`;
}

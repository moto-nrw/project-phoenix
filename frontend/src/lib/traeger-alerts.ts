// Texte der Auffälligkeiten im Dashboard des Trägers (#4675). Die Regeln, wann
// eine Schule auffällt, prüft das Backend gegen die Grenzen des Trägers; hier
// stehen nur die Sätze dazu, aus den Zahlen der Schule.

import type {
  TraegerAlertKind,
  TraegerAlertThresholds,
  TraegerSchoolAlerts,
} from "~/lib/traeger-api";

const ratioFormat = new Intl.NumberFormat("de-DE", {
  maximumFractionDigits: 1,
});

const percentFormat = new Intl.NumberFormat("de-DE", {
  style: "percent",
  maximumFractionDigits: 0,
});

/** Name der Auffälligkeit, fett vor dem Satz. */
export const ALERT_LABELS: Readonly<Record<TraegerAlertKind, string>> = {
  staff_shortage: "Zu wenig Personal",
  sick_children: "Viele Krankmeldungen",
  open_enrollments: "Offene Anmeldungen",
};

function children(count: number): string {
  return count === 1 ? "1 Kind" : `${count} Kinder`;
}

/** Der Satz zu einer Auffälligkeit, nur mit den Zahlen dieser Schule. */
export function alertSentence(
  kind: TraegerAlertKind,
  school: Pick<TraegerSchoolAlerts, "figures" | "openEnrollments">,
): string {
  const { figures } = school;
  switch (kind) {
    case "staff_shortage":
      if (figures.staffPresent === 0 || figures.careRatio === null) {
        return `${children(figures.childrenPresent)} ${figures.childrenPresent === 1 ? "ist" : "sind"} da. Keine Betreuungskraft ist in der Zeiterfassung angemeldet.`;
      }
      return `${children(figures.childrenPresent)} auf ${figures.staffPresent} ${figures.staffPresent === 1 ? "Betreuungskraft" : "Betreuungskräfte"}, Schlüssel 1 : ${ratioFormat.format(figures.careRatio)}.`;
    case "sick_children": {
      const share =
        figures.childrenTotal > 0
          ? figures.childrenSick / figures.childrenTotal
          : 0;
      return `${figures.childrenSick} von ${figures.childrenTotal} Kindern sind heute krank (${percentFormat.format(share)}).`;
    }
    case "open_enrollments":
      // The school decides, in its own app; the sentence names who acts.
      return school.openEnrollments === 1
        ? "1 Kind wartet auf eine Entscheidung der Schule."
        : `${school.openEnrollments} Kinder warten auf eine Entscheidung der Schule.`;
  }
}

/**
 * Die Grenzen des Trägers in einer Zeile, einmal über der Liste. Eine Grenze
 * mit 0 ist aus und steht so da.
 */
export function thresholdsSentence(thresholds: TraegerAlertThresholds): string {
  const staff =
    thresholds.childrenPerStaff > 0
      ? `mehr als ${children(thresholds.childrenPerStaff)} je Betreuungskraft`
      : "Personal: aus";
  const sick =
    thresholds.sickPercent > 0
      ? `ab ${thresholds.sickPercent} % kranke Kinder`
      : "Krankmeldungen: aus";
  const enrollments =
    thresholds.openEnrollments > 0
      ? `ab ${thresholds.openEnrollments === 1 ? "1 offenen Anmeldung" : `${thresholds.openEnrollments} offenen Anmeldungen`}`
      : "Anmeldungen: aus";
  return `Ihre Grenzen: ${staff} · ${sick} · ${enrollments}`;
}

/** „1 Schule“, „3 Schulen“: die Zahl im Titel der Karte. */
export function schoolCountLabel(count: number): string {
  return count === 1 ? "1 Schule" : `${count} Schulen`;
}

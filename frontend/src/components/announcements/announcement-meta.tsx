import { StatusBadge } from "~/components/ui/status-badge";
import type { StatusBadgeTone } from "~/components/ui/status-badge";
import type { Group } from "~/lib/api";
import type { Activity } from "~/lib/activity-helpers";
import { isLetter, isPoll } from "~/lib/parent-announcements-api";
import type {
  Announcement,
  AnnouncementResponseType,
  AnnouncementStatus,
  AnnouncementTarget,
  AnnouncementTargetType,
} from "~/lib/parent-announcements-api";

/**
 * Was die Seite „Mitteilungen" verwaltet. Alle drei sind im Backend dieselbe
 * Entität (eine Umfrage ist eine Mitteilung mit Antwortmöglichkeiten, #1371;
 * ein Elternbrief eine mit Pflichtkanälen, #2384). Die Trennung ist eine der
 * Oberfläche, weil Informieren, Bestätigen lassen und Fragen verschiedene
 * Arbeiten mit verschiedener Nacharbeit sind.
 */
export type AnnouncementKind = "announcement" | "letter" | "poll";

/**
 * Welchem Reiter eine Mitteilung gehört. Die drei Arten schließen sich auch
 * im Backend aus (ein Brief kann nie Umfrage sein, das erzwingt eine
 * DB-Bedingung), diese Reihenfolge kann also keine Zeile verstecken.
 */
export function kindOf(announcement: Announcement): AnnouncementKind {
  if (isPoll(announcement)) return "poll";
  if (isLetter(announcement)) return "letter";
  return "announcement";
}

/**
 * Der Reiter in der Adresse (`?art=`), damit die Objektseite den Rückweg auf
 * den richtigen Reiter setzen kann und ein Link auf „Umfragen" teilbar ist.
 */
export const KIND_PARAM: Record<AnnouncementKind, string> = {
  announcement: "mitteilungen",
  letter: "elternbriefe",
  poll: "umfragen",
};

export function kindFromParam(value: string | null): AnnouncementKind {
  switch (value) {
    case "elternbriefe":
      return "letter";
    case "umfragen":
      return "poll";
    default:
      return "announcement";
  }
}

/** Die Sammlung, zu der eine Mitteilung gehört, samt Reiter. */
export function announcementCollectionPath(kind: AnnouncementKind): string {
  return `/parent-announcements?art=${KIND_PARAM[kind]}`;
}

export const STATUS_LABEL: Record<AnnouncementStatus, string> = {
  draft: "Entwurf",
  published: "Veröffentlicht",
  expired: "Abgelaufen",
};

const STATUS_TONE: Record<AnnouncementStatus, StatusBadgeTone> = {
  draft: "gray",
  published: "green",
  expired: "orange",
};

export function AnnouncementStatusBadge({
  status,
}: {
  readonly status: AnnouncementStatus;
}) {
  return (
    <StatusBadge label={STATUS_LABEL[status]} tone={STATUS_TONE[status]} />
  );
}

export const RESPONSE_TYPE_LABEL: Record<AnnouncementResponseType, string> = {
  none: "Keine Rückmeldung",
  single_choice: "Eine Antwort",
  multi_choice: "Mehrere Antworten",
};

const TARGET_TYPE_PLURAL: Record<AnnouncementTargetType, string> = {
  school_all: "Ganze Schule",
  pending_enrollment: "Offene Anmeldungen",
  class: "Klassen",
  group: "Gruppen",
  activity_group: "AGs",
  student: "Kinder",
};

/** Kurze, namenlose Zusammenfassung für die Liste (z. B. „2 Gruppen, 1 Klasse"). */
export function summarizeTargets(targets: AnnouncementTarget[]): string {
  if (targets.length === 0) return "–";
  if (targets.some((t) => t.target_type === "school_all")) {
    // Whole-school subsumes the class/group/student targets, but pending
    // enrollments are separate applicants the backend still notifies — so
    // surface that combination instead of collapsing it to "Ganze Schule".
    return targets.some((t) => t.target_type === "pending_enrollment")
      ? "Ganze Schule, Offene Anmeldungen"
      : "Ganze Schule";
  }

  const counts = new Map<AnnouncementTargetType, number>();
  for (const t of targets) {
    counts.set(t.target_type, (counts.get(t.target_type) ?? 0) + 1);
  }
  return [...counts.entries()]
    .map(([type, count]) => {
      if (type === "school_all" || type === "pending_enrollment") {
        return TARGET_TYPE_PLURAL[type];
      }
      return `${count} ${TARGET_TYPE_PLURAL[type]}`;
    })
    .join(", ");
}

/** Die Zielgruppen mit echten Namen, für die Objektseite. */
export function targetChips(
  targets: AnnouncementTarget[],
  groups: Group[],
  activities: Activity[],
): string[] {
  if (targets.some((t) => t.target_type === "school_all")) {
    // Whole-school subsumes class/group/student targets; pending enrollments
    // are additional recipients the backend still reaches, so keep that chip.
    const schoolChips = ["Ganze Schule"];
    if (targets.some((t) => t.target_type === "pending_enrollment"))
      schoolChips.push("Offene Anmeldungen");
    return schoolChips;
  }
  const groupNames = new Map(groups.map((g) => [g.id, g.name]));
  const activityNames = new Map(activities.map((a) => [a.id, a.name]));
  const chips: string[] = [];
  let studentCount = 0;
  for (const t of targets) {
    switch (t.target_type) {
      case "pending_enrollment":
        chips.push("Offene Anmeldungen");
        break;
      case "class":
        chips.push(`Klasse ${t.ref_text ?? "?"}`);
        break;
      case "group":
        chips.push(groupNames.get(t.ref_id ?? "") ?? "Gruppe");
        break;
      case "activity_group":
        chips.push(activityNames.get(t.ref_id ?? "") ?? "AG");
        break;
      case "student":
        studentCount++;
        break;
      case "school_all":
        break;
    }
  }
  if (studentCount > 0) {
    chips.push(
      studentCount === 1
        ? "1 einzelnes Kind"
        : `${studentCount} einzelne Kinder`,
    );
  }
  return chips;
}

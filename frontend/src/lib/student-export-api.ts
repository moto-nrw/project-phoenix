import { trackEvent } from "~/lib/analytics";
import { berlinTodayISO } from "~/lib/date-helpers";

export type StudentExportFormat = "pdf" | "docx" | "xlsx";

export type StudentExportPreset =
  | "ogs_weekly"
  | "ogs_compact"
  | "class_roster"
  | "daily_planning"
  | "attendance_snapshot"
  | "pickup_list"
  | "blank_checklist"
  | "birthday_list";

export type StudentExportColumn =
  | "name"
  | "school_class"
  | "group"
  | "enrollment_summary"
  | "care_days"
  | "weekly_monday"
  | "weekly_tuesday"
  | "weekly_wednesday"
  | "weekly_thursday"
  | "weekly_friday"
  | "planned_arrival"
  | "planned_pickup"
  | "daily_status"
  | "departure"
  | "daily_notes"
  | "current_location"
  | "birthday"
  | "age";

export interface StudentExportFilters {
  search?: string;
  group_id?: string;
  room_id?: string;
  year?: string;
  school_class?: string;
  status?: string;
  bus?: string;
  photo_consent?: string;
  pickup_status?: string;
  day_status?: string;
  pickup_time?: string;
  arrival_time?: string;
  sort?: string;
  group_by_class?: boolean;
  /**
   * Birth months ("01".."12") a birthday list is limited to. Empty means every
   * month. A birthday recurs every year, so this matches on the month alone and
   * never on the birth year.
   */
  months?: string[];
}

export interface BirthdayMonthOption {
  value: string;
  label: string;
}

/**
 * The twelve months as picker options, labelled via the same de-DE formatting
 * the rest of the date helpers use rather than a hand-kept name list.
 */
export const BIRTHDAY_MONTH_OPTIONS: BirthdayMonthOption[] = Array.from(
  { length: 12 },
  (_, index) => ({
    value: String(index + 1).padStart(2, "0"),
    label: new Date(Date.UTC(2000, index, 1)).toLocaleDateString("de-DE", {
      month: "long",
      timeZone: "UTC",
    }),
  }),
);

/**
 * The current month in the school's timezone, pre-selected when a birthday list
 * is opened. Derived from Berlin — not the browser calendar — so a staff member
 * abroad does not silently export the neighbouring month around a month
 * boundary.
 */
export function currentBirthdayMonth(): string {
  return berlinTodayISO().slice(5, 7);
}

export interface StudentExportRequest {
  format: StudentExportFormat;
  preset: StudentExportPreset;
  title: string;
  filters: StudentExportFilters;
  columns: StudentExportColumn[];
}

export interface StudentExportColumnOption {
  id: StudentExportColumn;
  label: string;
  group: "base" | "weekly" | "daily" | "snapshot";
  description: string;
}

const WEEKLY_COLUMNS: Array<[StudentExportColumn, string, string]> = [
  ["weekly_monday", "Montag", "Montags"],
  ["weekly_tuesday", "Dienstag", "Dienstags"],
  ["weekly_wednesday", "Mittwoch", "Mittwochs"],
  ["weekly_thursday", "Donnerstag", "Donnerstags"],
  ["weekly_friday", "Freitag", "Freitags"],
];

export const STUDENT_EXPORT_COLUMNS: StudentExportColumnOption[] = [
  {
    id: "name",
    label: "Name",
    group: "base",
    description: "Vorname und Nachname des Kindes aus den Stammdaten.",
  },
  {
    id: "school_class",
    label: "Klasse",
    group: "base",
    description: "Aktuelle Schulklasse aus den Kind-Stammdaten.",
  },
  {
    id: "group",
    label: "Gruppe",
    group: "base",
    description: "Zugeordnete OGS-Gruppe des Kindes.",
  },
  {
    id: "enrollment_summary",
    label: "Betreuungs-/Anmeldestatus",
    group: "base",
    description:
      "Aktive Randstunden- oder Ganztagszuordnung; Kinder ohne Anmeldung werden ausdrücklich markiert.",
  },
  {
    id: "care_days",
    label: "Betreuungstage",
    group: "weekly",
    description:
      "Mo bis Fr, an denen für das Kind ein Wochenplan für Ankunft oder Abholung hinterlegt ist.",
  },
  ...WEEKLY_COLUMNS.map(([id, label, weekday]) => ({
    id,
    label,
    group: "weekly" as const,
    description: `Regelmäßige ${weekday}-Ankunft und ${weekday}-Abholung aus dem Wochenplan.`,
  })),
  {
    id: "planned_arrival",
    label: "Geplante Ankunft",
    group: "daily",
    description:
      "Geplante Ankunft für den heutigen Tag, inklusive Tagesausnahmen.",
  },
  {
    id: "planned_pickup",
    label: "Geplante Abholung",
    group: "daily",
    description:
      "Geplante Abholung für den heutigen Tag, inklusive Tagesausnahmen.",
  },
  {
    id: "daily_status",
    label: "Tagesstatus",
    group: "daily",
    description:
      "Ob das Kind heute erwartet wird oder als Krank, Entschuldigt bzw. Klassenfahrt markiert ist.",
  },
  {
    id: "departure",
    label: "Geh-/Abholweise",
    group: "base",
    description:
      "Wie das Kind je Wochentag nach Hause geht: geht alleine, fährt Bus oder wird abgeholt.",
  },
  {
    id: "daily_notes",
    label: "Tageshinweise",
    group: "daily",
    description:
      "Hinweise zur heutigen Ankunft oder Abholung, zum Beispiel Ausnahme-Notizen.",
  },
  {
    id: "current_location",
    label: "Aktueller Aufenthaltsort",
    group: "snapshot",
    description:
      "Aktueller Aufenthaltsort aus der Live-Anwesenheit. Nur als Momentaufnahme geeignet.",
  },
  {
    id: "birthday",
    label: "Geburtstag",
    group: "base",
    description: "Geburtsdatum des Kindes aus den Stammdaten.",
  },
  {
    id: "age",
    label: "Alter",
    group: "base",
    description:
      "Alter in Jahren am heutigen Tag, berechnet aus dem Geburtsdatum.",
  },
];

export const STUDENT_EXPORT_PRESETS: Array<{
  id: StudentExportPreset;
  label: string;
  description: string;
  columns: StudentExportColumn[];
}> = [
  {
    id: "ogs_weekly",
    label: "OGS Wochenliste",
    description: "Wochenmatrix mit Ankunft und Abholung je Wochentag.",
    columns: [
      "name",
      "school_class",
      "group",
      "weekly_monday",
      "weekly_tuesday",
      "weekly_wednesday",
      "weekly_thursday",
      "weekly_friday",
    ],
  },
  {
    id: "ogs_compact",
    label: "OGS Kompaktliste",
    description: "Kompakte Übersicht der Betreuungstage und heutigen Abholung.",
    columns: [
      "name",
      "school_class",
      "group",
      "care_days",
      "departure",
      "planned_pickup",
    ],
  },
  {
    id: "class_roster",
    label: "Klassenliste",
    description:
      "Gesamter Klassenverband mit Anmeldestatus, Betreuungstagen und Geh-/Abholweise.",
    columns: [
      "name",
      "school_class",
      "group",
      "enrollment_summary",
      "care_days",
      "weekly_monday",
      "weekly_tuesday",
      "weekly_wednesday",
      "weekly_thursday",
      "weekly_friday",
      "departure",
    ],
  },
  {
    id: "daily_planning",
    label: "Tagesliste",
    description: "Planungsdaten für den heutigen Tag mit Hinweisen.",
    columns: [
      "name",
      "school_class",
      "group",
      "daily_status",
      "planned_arrival",
      "planned_pickup",
      "daily_notes",
    ],
  },
  {
    id: "attendance_snapshot",
    label: "Anwesenheitsliste",
    description: "Momentaufnahme mit aktuellem Aufenthaltsort.",
    columns: [
      "name",
      "school_class",
      "group",
      "current_location",
      "planned_pickup",
    ],
  },
  {
    id: "pickup_list",
    label: "Abholliste",
    description: "Geh-/Abholweise, Gehzeiten und Hinweise für den Tag.",
    columns: [
      "name",
      "school_class",
      "group",
      "departure",
      "planned_pickup",
      "daily_notes",
    ],
  },
  {
    id: "blank_checklist",
    label: "Checkliste",
    description: "Einfache Liste zum manuellen Abhaken.",
    columns: ["name", "school_class", "group"],
  },
  {
    id: "birthday_list",
    label: "Geburtstagsliste",
    description:
      "Geburtstage nach Kalender sortiert. Kinder ohne hinterlegtes Geburtsdatum fehlen in dieser Liste.",
    columns: ["name", "school_class", "group", "birthday", "age"],
  },
];

export async function exportStudents(
  request: StudentExportRequest,
): Promise<void> {
  const response = await fetch("/api/students/export", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request),
  });

  if (!response.ok) {
    throw new Error(await response.text());
  }

  trackEvent("data_exported", {
    export_type: "students",
    format: request.format,
  });

  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download =
    filenameFromDisposition(response) ?? fallbackFilename(request);
  document.body.append(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

function filenameFromDisposition(response: Response): string | null {
  const disposition = response.headers.get("content-disposition");
  const match = /filename="([^"]+)"/.exec(disposition ?? "");
  return match?.[1] ?? null;
}

function fallbackFilename(request: StudentExportRequest): string {
  const title = request.title.trim().toLowerCase().replace(/\s+/g, "-");
  return `${title || "kindersuche-export"}.${request.format}`;
}

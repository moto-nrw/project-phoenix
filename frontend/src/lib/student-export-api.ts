export type StudentExportFormat = "pdf" | "docx" | "xlsx";

export type StudentExportPreset =
  | "ogs_weekly"
  | "ogs_compact"
  | "daily_planning"
  | "attendance_snapshot"
  | "pickup_list"
  | "blank_checklist";

export type StudentExportColumn =
  | "name"
  | "school_class"
  | "group"
  | "care_days"
  | "weekly_monday"
  | "weekly_tuesday"
  | "weekly_wednesday"
  | "weekly_thursday"
  | "weekly_friday"
  | "planned_arrival"
  | "planned_pickup"
  | "daily_notes"
  | "current_location";

export interface StudentExportFilters {
  search?: string;
  group_id?: string;
  year?: string;
  status?: string;
  pickup_time?: string;
  arrival_time?: string;
  sort?: string;
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
    columns: ["name", "school_class", "group", "care_days", "planned_pickup"],
  },
  {
    id: "daily_planning",
    label: "Tagesliste",
    description: "Planungsdaten für den heutigen Tag mit Hinweisen.",
    columns: [
      "name",
      "school_class",
      "group",
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
    description: "Abholzeiten und Abholhinweise für den Tag.",
    columns: ["name", "school_class", "group", "planned_pickup", "daily_notes"],
  },
  {
    id: "blank_checklist",
    label: "Checkliste",
    description: "Einfache Liste zum manuellen Abhaken.",
    columns: ["name", "school_class", "group"],
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

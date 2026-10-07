// Client for the printable weekly plans (#2079): the Dienstplan (staff
// view) and the Betreuungsplan (child view). Both answer with a rendered
// PDF or XLSX; "Drucken" is the PDF opened in a print dialog, not a second
// rendering path.

import { apiErrorFromResponse, transportFetch } from "~/lib/api-error";
import {
  downloadBlob,
  filenameFromDisposition,
  openBlobForPrint,
} from "~/lib/file-download";

export type PlanExportPlan = "dienstplan" | "betreuungsplan";
export type PlanExportFormat = "pdf" | "xlsx";
export type PlanExportMode = "download" | "print";
export type PlanExportVariant = "aushang" | "intern";
export type PlanExportTemplate = "persons" | "areas" | "hours" | "offerings";

export interface PlanExportRequest {
  /** Any day in the first week; the backend widens it to that Monday. */
  from: string;
  /** Any day in the last week; the backend widens it to that Friday. */
  to: string;
  template: PlanExportTemplate;
  variant: PlanExportVariant;
}

const ROUTES: Record<PlanExportPlan, string> = {
  dienstplan: "/api/staff-shifts/export",
  betreuungsplan: "/api/timetable/betreuungsplan/export",
};

export interface PlanExportTemplateOption {
  id: PlanExportTemplate;
  label: string;
  description: string;
}

/** The row axes each plan offers, in the order the dialog lists them. */
export const PLAN_EXPORT_TEMPLATES: Record<
  PlanExportPlan,
  PlanExportTemplateOption[]
> = {
  dienstplan: [
    {
      id: "persons",
      label: "Nach Personen",
      description:
        "Eine Zeile je Mitarbeitenden, wie im Dienstplan auf dem Bildschirm.",
    },
    {
      id: "areas",
      label: "Nach Einsatzbereich",
      description:
        "Eine Zeile je Schichtart oder Angebot, die Namen stehen in den Feldern.",
    },
    {
      id: "hours",
      label: "Stundenübersicht",
      description:
        "Eine Zeile je Mitarbeitenden mit den Stunden je Schichtart und dem Soll.",
    },
  ],
  betreuungsplan: [
    {
      id: "offerings",
      label: "Nach Angebot",
      description:
        "Eine Zeile je Betreuungsblock mit Raum, Personal und Kinderzahl.",
    },
  ],
};

export interface PlanExportVariantOption {
  id: PlanExportVariant;
  label: string;
  description: string;
}

export const PLAN_EXPORT_VARIANTS: PlanExportVariantOption[] = [
  {
    id: "aushang",
    label: "Aushang",
    description:
      "Für den Aushang: Ausfälle und Vertretungen stehen drauf, Gründe nicht.",
  },
  {
    id: "intern",
    label: "Interne Fassung",
    description:
      "Zusätzlich Gründe, Abwesenheiten, offene Lücken und Hinweise.",
  },
];

/**
 * Renders a plan and hands the file to the user.
 *
 * `printTarget` is a tab the caller opened synchronously inside the click
 * handler: after the fetch below, popup blockers refuse to open one. Pass
 * `null` to have the caller's failure surface, `undefined` for downloads.
 */
export async function exportPlan(
  plan: PlanExportPlan,
  request: PlanExportRequest,
  format: PlanExportFormat,
  mode: PlanExportMode,
  printTarget?: Window | null,
): Promise<void> {
  if (mode === "print" && !printTarget) {
    throw new Error("Der Druckdialog konnte nicht geöffnet werden.");
  }

  try {
    const response = await transportFetch(ROUTES[plan], {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ...request, format }),
    });

    if (!response.ok) {
      throw await apiErrorFromResponse(response, `${plan} export failed`);
    }

    const blob = await response.blob();
    if (mode === "print") {
      openBlobForPrint(blob, printTarget);
      return;
    }
    downloadBlob(
      blob,
      filenameFromDisposition(response) ?? `${plan}-${request.from}.${format}`,
    );
  } catch (error) {
    printTarget?.close();
    throw error;
  }
}

/**
 * Client of the occupancy of the Träger portal (#4676): how many children
 * each school, OGS group and care form holds today. Counts and the names of
 * groups and offers only, never a child. Backend int64 IDs leave as strings.
 */

import { apiErrorFromBody, transportFetch } from "./api-error";
import { filenameFromDisposition } from "./file-download";
import type { TraegerSchool } from "./traeger-api";

interface TraegerOccupancyGroup {
  readonly id: string;
  readonly name: string;
  readonly children: number;
}

export interface TraegerOccupancyCareForm {
  readonly id: string;
  readonly name: string;
  readonly children: number;
  /** The places of the offer; `null` without a limit. */
  readonly capacity: number | null;
}

export interface TraegerSchoolOccupancy extends TraegerSchool {
  /** The children of the Stichtagszahl rule, counted today. */
  readonly children: number;
  readonly withoutGroup: number;
  readonly groups: readonly TraegerOccupancyGroup[];
  readonly withoutCareForm: number;
  readonly careForms: readonly TraegerOccupancyCareForm[];
}

export interface TraegerOccupancy {
  /** The day counted, `YYYY-MM-DD`. */
  readonly day: string;
  readonly asOf: string;
  readonly schools: readonly TraegerSchoolOccupancy[];
}

interface BackendOccupancy {
  day: string;
  as_of: string;
  schools?: {
    id: number;
    name: string;
    slug: string;
    subdomain: string;
    city?: string;
    children: number;
    without_group: number;
    groups?: { id: number; name: string; children: number }[];
    without_care_form: number;
    care_forms?: {
      id: number;
      name: string;
      children: number;
      capacity: number | null;
    }[];
  }[];
}

async function readJson(response: Response): Promise<unknown> {
  const text = await response.text();
  if (!text) return null;
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return null;
  }
}

/** The occupancy of every school of the Träger, today. */
export async function fetchTraegerOccupancy(): Promise<TraegerOccupancy> {
  const response = await transportFetch("/api/traeger/schools/occupancy", {
    credentials: "include",
    cache: "no-store",
  });
  const data = await readJson(response);
  if (!response.ok) {
    throw apiErrorFromBody(
      `Request failed (${response.status})`,
      response.status,
      data,
    );
  }
  const occupancy = (data as { data: BackendOccupancy }).data;
  return {
    day: occupancy.day,
    asOf: occupancy.as_of,
    schools: (occupancy.schools ?? []).map((school) => ({
      id: String(school.id),
      name: school.name,
      slug: school.slug,
      subdomain: school.subdomain,
      city: school.city ?? "",
      children: school.children,
      withoutGroup: school.without_group,
      groups: (school.groups ?? []).map((group) => ({
        id: String(group.id),
        name: group.name,
        children: group.children,
      })),
      withoutCareForm: school.without_care_form,
      careForms: (school.care_forms ?? []).map((form) => ({
        id: String(form.id),
        name: form.name,
        children: form.children,
        capacity: form.capacity ?? null,
      })),
    })),
  };
}

/** The CSV export of the occupancy, with the file name the backend chose. */
export async function downloadTraegerOccupancy(): Promise<{
  readonly blob: Blob;
  readonly filename: string | null;
}> {
  const response = await transportFetch(
    "/api/traeger/schools/occupancy/export",
    { credentials: "include", cache: "no-store" },
  );
  if (!response.ok) {
    throw apiErrorFromBody(
      `Request failed (${response.status})`,
      response.status,
      await readJson(response),
    );
  }
  return {
    blob: await response.blob(),
    filename: filenameFromDisposition(response),
  };
}

// Client-side API für die Klassenansicht im Schul-Portal (#2207). Gleiche
// Report-Typen wie das Tenant-Portal (lib/class-day-api), aber die Next.js
// Routen unter /api/school/* laufen über die school-Session und rufen das
// Backend mit dem school-Token unter /school/class-day auf.

import { authFetch } from "./api-helpers";
import type { ClassDayReport } from "./class-day-api";

interface ProxyResponse<T> {
  success: boolean;
  data: T;
}

export async function fetchMyClassesSchool(): Promise<string[]> {
  const response = await authFetch<ProxyResponse<{ classes: string[] }>>(
    "/api/school/class-day/classes",
  );
  return response.data.classes ?? [];
}

export async function fetchClassDaySchool(
  schoolClass: string,
  date: string,
): Promise<ClassDayReport> {
  const params = new URLSearchParams({ class: schoolClass, date });
  const response = await authFetch<ProxyResponse<ClassDayReport>>(
    `/api/school/class-day?${params.toString()}`,
  );
  return response.data;
}

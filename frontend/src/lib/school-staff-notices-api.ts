// Tagesinformationen in moto schule (#2208): dieselben Hinweise der
// OGS-Leitung wie im OGS-Portal, gelesen über die school-Session und die
// Proxy-Routen unter /api/school/staff-notices (Backend:
// /school/staff-notices). Das Backend liefert nur, was "alle" oder "nur
// Lehrkräfte" erreicht.

import { createStaffNoticesReadApi } from "./staff-notices-api";

export const schoolStaffNoticesApi = createStaffNoticesReadApi(
  "/api/school/staff-notices",
);

/**
 * SWR-Schlüssel der heutigen Hinweise in moto schule. Die Seite und die
 * Karte auf der Klassenansicht teilen ihn, damit eine Kenntnisnahme beide
 * sofort nachzieht; useSWRAuth hängt die Schul-Sitzung an.
 */
export const SCHOOL_NOTICES_TODAY_KEY = "school-staff-notices-today";

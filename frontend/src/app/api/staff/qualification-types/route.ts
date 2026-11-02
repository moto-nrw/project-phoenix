import { proxyGet, proxyPost } from "~/lib/route-proxy.server";

interface QualificationTypeBody {
  name: string;
  validity_months: number | null;
  reminder_lead_weeks: number;
  mandatory: boolean;
  archived: boolean;
}

/**
 * GET /api/staff/qualification-types
 * Die Nachweisarten der Schule (#4808), mit der Standardliste.
 */
export const GET = proxyGet("/api/staff/qualification-types");

/**
 * POST /api/staff/qualification-types
 * Eine eigene Nachweisart anlegen.
 */
export const POST = proxyPost<unknown, QualificationTypeBody>(
  "/api/staff/qualification-types",
);

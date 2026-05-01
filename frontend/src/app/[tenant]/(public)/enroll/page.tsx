"use client";

import { useEffect, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- public page; tenant-router not needed
import { useRouter } from "next/navigation";
import { EnrollmentForm } from "~/components/enrollment/enrollment-form";
import {
  listCalendarPeriods,
  type CalendarPeriodSummary,
} from "~/lib/care-offering-api";
import { useTenant } from "~/components/tenant/tenant-provider";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollPage" });

/**
 * Public enrollment page. Shows a school-year picker, then renders
 * the dynamic form for the selected period.
 *
 * The tenant layout already validates the slug + sets the tenant
 * provider context, so the form can call useTenant() directly.
 *
 * Calendar periods come from the existing /api/timetable/periods
 * endpoint — public via the tenant layout's auth context. PR 7 ships
 * the form skeleton; PR 7's parent-facing care-offerings endpoint
 * filters by open window so the form only shows currently-applicable
 * choices. The grade-level cap comes from
 * enrollment.grade_level_max via a server-rendered config payload —
 * for PR 7 we hardcode the OGS default of 4 client-side and let
 * future revisions read it from /api/me or a dedicated config
 * endpoint.
 */
export default function EnrollPage() {
  const { tenantSlug, tenant } = useTenant();
  const router = useRouter();
  const [periods, setPeriods] = useState<CalendarPeriodSummary[]>([]);
  const [selectedPeriod, setSelectedPeriod] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      setError(null);
      try {
        const list = await listCalendarPeriods();
        if (cancelled) return;
        const schoolYears = list.filter((p) => p.period_type === "school_year");
        setPeriods(schoolYears);
        if (schoolYears.length > 0 && !selectedPeriod) {
          setSelectedPeriod(schoolYears[0]?.id ?? "");
        }
      } catch (err) {
        if (cancelled) return;
        const message =
          err instanceof Error ? err.message : "Unbekannter Fehler";
        logger.error("enroll_page_load_failed", { error: message });
        setError(message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tenantSlug]);

  const handleSubmitted = (statusURL: string) => {
    // statusURL is the absolute frontend URL the backend built. We
    // navigate via the path so Next.js' client routing kicks in
    // (avoids a full reload).
    try {
      const u = new URL(statusURL);
      router.push(`${u.pathname}?submitted=1`);
    } catch {
      // Fall back to absolute navigation if the URL doesn't parse.
      globalThis.window.location.href = statusURL;
    }
  };

  if (loading) {
    return (
      <main className="mx-auto max-w-3xl p-6 text-sm text-gray-500">
        Wird geladen...
      </main>
    );
  }

  return (
    <main className="mx-auto max-w-3xl space-y-6 p-6">
      <header className="space-y-2">
        <h1 className="text-2xl font-semibold text-gray-900">
          Anmeldung an der {tenant?.name ?? "Schule"}
        </h1>
        <p className="text-sm text-gray-600">
          Bitte fülle das Formular vollständig aus. Du erhältst nach dem
          Absenden eine Bestätigungs-E-Mail mit einem Link, über den du den
          Status deiner Anmeldung jederzeit einsehen kannst.
        </p>
      </header>

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800">
          {error}
        </div>
      )}

      {periods.length === 0 ? (
        <div className="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900">
          Aktuell ist kein Schuljahr für die Anmeldung geöffnet. Bitte komm
          später wieder oder wende dich an die Schulleitung.
        </div>
      ) : (
        <>
          <label className="block">
            <span className="block text-xs font-medium text-gray-600">
              Schuljahr
            </span>
            <select
              value={selectedPeriod}
              onChange={(e) => setSelectedPeriod(e.target.value)}
              className="mt-1 w-full rounded-md border-gray-300 px-3 py-2 text-sm shadow-sm"
            >
              {periods.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </select>
          </label>

          {selectedPeriod && (
            <EnrollmentForm
              calendarPeriodID={selectedPeriod}
              gradeLevelMax={4}
              onSubmitted={handleSubmitted}
            />
          )}
        </>
      )}
    </main>
  );
}

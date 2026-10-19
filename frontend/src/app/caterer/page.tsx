"use client";

// Overview of the caterer portal (ADR 0074): the orders of every school the
// caterer supplies, one week at a time. Each school is read on its own, in
// its own transaction; a school that ends the supply drops out at once.

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { signOut } from "next-auth/react";
import { MotoBrand } from "~/components/auth/auth-shell";
import { CatererSchoolOrders } from "~/components/caterer/caterer-school-orders";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Loading } from "~/components/ui/loading";
import { PlanningContextBar } from "~/components/ui/planning-context-bar";
import { useApiLoadError } from "~/contexts/ToastContext";
import { fetchCatererSchools, type CatererSchools } from "~/lib/caterer-api";
import { catererAbsoluteUrl } from "~/lib/caterer-url";
import { parseISODate, todayISO, toISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { mondayOf } from "~/lib/meal-plan-api";
import { DELIBERATE_LOGOUT_KEY, clearSessionCache } from "~/lib/session-cache";

const logger = createLogger({ component: "CatererOverview" });

function addDays(iso: string, days: number): string {
  const date = parseISODate(iso);
  date.setDate(date.getDate() + days);
  return toISODate(date);
}

function shortDate(iso: string): string {
  return parseISODate(iso).toLocaleDateString("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

async function logout() {
  try {
    sessionStorage.setItem(DELIBERATE_LOGOUT_KEY, "1");
  } catch {
    // sessionStorage unavailable: the login page shows its notice.
  }
  try {
    await fetch("/api/caterer/auth/logout", {
      method: "POST",
      signal: AbortSignal.timeout(5000),
    });
  } catch (cause) {
    // Best effort: signing out of this device must not wait for it.
    logger.warn("caterer_backend_logout_failed", {
      error: cause instanceof Error ? cause.message : String(cause),
    });
  }
  clearSessionCache();
  await signOut({ callbackUrl: catererAbsoluteUrl("/caterer/login") });
}

export default function CatererOverviewPage() {
  const currentMonday = useMemo(() => mondayOf(parseISODate(todayISO())), []);
  const [mondayISO, setMondayISO] = useState(currentMonday);
  const fridayISO = addDays(mondayISO, 4);
  const [schools, setSchools] = useState<CatererSchools | null>(null);
  const [loading, setLoading] = useState(true);
  const { error, show, clear } = useApiLoadError();
  const reloadRef = useRef<() => void>(() => undefined);

  const load = useCallback(async () => {
    setLoading(true);
    clear();
    try {
      setSchools(await fetchCatererSchools());
    } catch (cause) {
      logger.error("caterer_schools_load_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      void show(cause, {
        object: "Ihre Schulen",
        retry: () => reloadRef.current(),
      });
    } finally {
      setLoading(false);
    }
  }, [clear, show]);
  useEffect(() => {
    reloadRef.current = () => void load();
  });
  useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="border-b border-gray-200 bg-white">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-4 py-3">
          <div className="flex items-center gap-3">
            <MotoBrand />
            {schools?.catererName ? (
              <span className="text-sm font-medium text-gray-700">
                {schools.catererName}
              </span>
            ) : null}
          </div>
          <Button
            type="button"
            variant="ghost"
            size="compact"
            onClick={() => void logout()}
          >
            Abmelden
          </Button>
        </div>
      </header>

      <main className="mx-auto max-w-6xl space-y-6 px-4 py-6">
        <PlanningContextBar
          dateLabel={`Woche vom ${shortDate(mondayISO)} bis ${shortDate(fridayISO)}`}
          onPrevious={() => setMondayISO(addDays(mondayISO, -7))}
          onNext={() => setMondayISO(addDays(mondayISO, 7))}
          previousLabel="Vorherige Woche"
          nextLabel="Nächste Woche"
          todayLabel="Diese Woche"
          onToday={
            mondayISO === currentMonday
              ? undefined
              : () => setMondayISO(currentMonday)
          }
          withoutContextRow
        />

        {loading ? (
          <Loading message="Wird geladen …" />
        ) : error ? (
          <LoadErrorAlert error={error} />
        ) : schools && schools.schools.length > 0 ? (
          schools.schools.map((school) => (
            <CatererSchoolOrders
              key={school.id}
              school={school}
              mondayISO={mondayISO}
              fridayISO={fridayISO}
            />
          ))
        ) : (
          <EmptyState
            title="Sie beliefern zurzeit keine Schule"
            description="Sobald eine Schule Sie einlädt, sehen Sie hier ihre Bestellungen."
          />
        )}
      </main>
    </div>
  );
}

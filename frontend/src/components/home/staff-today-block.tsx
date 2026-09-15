"use client";

import { Alert } from "~/components/ui/alert";
import { SectionCard } from "~/components/ui/section-card";
import { StatCard } from "~/components/ui/stat-card";
import { HOME_CARD_BODY, HomeCardIcon } from "~/components/home/home-card";
import { HomeCardLink } from "~/components/home/home-card-rows";
import type { DashboardAnalytics } from "~/lib/dashboard-helpers";
import { createLogger } from "~/lib/logger";
import {
  staffOverviewService,
  type DashboardSummary,
} from "~/lib/staff-overview-api";
import { useSWRAuth } from "~/lib/swr";
import { useTenantAwarePath } from "~/lib/tenant-path";

const logger = createLogger({ component: "StaffTodayBlock" });

/**
 * Baustein „Personal heute" (#2180): der Teamstand aus Leitungssicht. Wer in
 * Aufsicht ist, wer heute ausfällt, ob die Stempeluhr läuft.
 *
 * Die offenen Anträge stehen bewusst NICHT hier: sie zählen im Baustein
 * „Offene Anfragen", und zwei Karten mit derselben Zahl sagen nicht mehr als
 * eine.
 */
export function StaffTodayBlock({
  analytics,
}: {
  /** Betriebszahlen der Seite; ohne groups:read fehlt die Aufsicht schlicht. */
  readonly analytics: DashboardAnalytics | undefined;
}) {
  const tenantPath = useTenantAwarePath();
  const {
    data: summary,
    error,
    isLoading,
  } = useSWRAuth<DashboardSummary>(
    "home-staff-summary",
    () => staffOverviewService.getDashboardSummary("week"),
    { refreshInterval: 5 * 60 * 1000 },
  );

  if (error) {
    logger.error("home_staff_summary_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
  }

  const tiles = staffTiles(summary, analytics);

  return (
    <SectionCard
      title="Personal heute"
      leading={<HomeCardIcon concept="staff" />}
      className="flex h-full flex-col"
      bodyClassName={HOME_CARD_BODY}
      inlineActions
      actions={
        <HomeCardLink
          href={tenantPath("/staff")}
          label="Personal heute: zum Team"
        >
          Zum Team
        </HomeCardLink>
      }
    >
      {error ? (
        <Alert
          type="error"
          message="Der Teamstand konnte nicht geladen werden. Bitte die Seite neu laden."
        />
      ) : (
        <div className="grid grid-cols-2 gap-2">
          {tiles.map((tile) => (
            <StatCard
              key={tile.label}
              variant="tile"
              label={tile.label}
              value={isLoading && summary === undefined ? "–" : tile.value}
              tone={tile.tone}
            />
          ))}
        </div>
      )}
    </SectionCard>
  );
}

interface StaffTile {
  readonly label: string;
  readonly value: string | number;
  readonly tone?: "red" | "blue";
}

/**
 * Vier Zahlen, die in eine Karte dieser Höhe passen. Die Stempeluhr steht nur
 * da, wo sie benutzt wird: „0 von 0" neben „5 in Aufsicht" läse sich wie ein
 * Widerspruch.
 */
export function staffTiles(
  summary: DashboardSummary | undefined,
  analytics: DashboardAnalytics | undefined,
): readonly StaffTile[] {
  const tiles: StaffTile[] = [];
  if (analytics) {
    tiles.push({ label: "In Aufsicht", value: analytics.supervisorsToday });
  }
  const sick = summary?.sickToday ?? 0;
  const vacation = summary?.vacationToday ?? 0;
  tiles.push({
    label: "Krank",
    value: sick,
    tone: sick > 0 ? "red" : undefined,
  });
  tiles.push({
    label: "Im Urlaub",
    value: vacation,
    tone: vacation > 0 ? "blue" : undefined,
  });
  const expected = summary?.expectedClockedIn ?? 0;
  const clockedIn = summary?.currentlyClockedIn ?? 0;
  if (expected > 0 || clockedIn > 0) {
    tiles.push({
      label: "Eingestempelt",
      value: expected > 0 ? `${clockedIn} von ${expected}` : clockedIn,
    });
  }
  if (tiles.length < 4) {
    tiles.push({ label: "Team gesamt", value: summary?.activeStaffCount ?? 0 });
  }
  return tiles.slice(0, 4);
}

"use client";

import { useMemo, useState } from "react";
import type { DateRange } from "react-day-picker";

import {
  buildDefaultPresets,
  DateRangePicker,
} from "~/components/ui/date-range-picker";
import {
  Area,
  AreaChart,
  CartesianGrid,
  Cell,
  Label,
  Line,
  LineChart,
  Pie,
  PieChart,
  ReferenceLine,
  XAxis,
  YAxis,
} from "recharts";

import { Alert } from "~/components/ui/alert";
import { SectionCard } from "~/components/ui/section-card";
import {
  type ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from "~/components/ui/chart";
import { UebersichtTabSkeleton } from "~/components/staff/uebersicht-tab-skeleton";
import {
  staffAbsenceService,
  staffBalanceAdjustmentService,
  staffHistoryService,
  staffMonthSummaryService,
} from "~/lib/staff-api";
import type { StaffAbsenceRow, StaffHistorySession } from "~/lib/staff-api";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import {
  endOfWeek,
  getDeltaStatus,
  indexAbsenceCreditByDay,
  isHalfAbsenceBoundary,
  resolveAccountStartDate,
  startOfWeek,
  toDateKey,
  toIsoDayOfWeek,
} from "~/lib/staff-metrics-helpers";
import {
  berlinTodayISO,
  endOfBerlinDayISO,
  parseISODate,
} from "~/lib/date-helpers";
import { useAccountBalance } from "~/lib/hooks/use-account-balance";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { useSWRAuth, useTenantMutateMatching } from "~/lib/swr";
import { timeTrackingService } from "~/lib/time-tracking-api";
import type { BalanceAdjustment } from "~/lib/time-tracking-helpers";

import { formatSignedDuration, KpiCard } from "./staff-time-views";
import { StundenkontoPanel } from "./stundenkonto-panel";

/**
 * Soll-Minuten je Kalendertag, aufgelöst gegen den Plan, der AN diesem Tag
 * galt (#1842). Ein Tag außerhalb des geladenen Fensters ist 0 wert.
 */
type TargetsByDay = ReadonlyMap<string, number>;

const EMPTY_TARGETS: TargetsByDay = new Map<string, number>();

type DistributionCenterLabelProps = {
  readonly total: number;
  readonly viewBox?: unknown;
};

type ConcreteDateRange = {
  readonly from: Date;
  readonly to: Date;
};

function DistributionCenterLabel({
  total,
  viewBox,
}: DistributionCenterLabelProps) {
  const center =
    viewBox && typeof viewBox === "object"
      ? (viewBox as { cx?: unknown; cy?: unknown })
      : null;

  if (
    center &&
    typeof center.cx === "number" &&
    typeof center.cy === "number"
  ) {
    return (
      <text
        x={center.cx}
        y={center.cy}
        textAnchor="middle"
        dominantBaseline="middle"
      >
        <tspan
          x={center.cx}
          y={center.cy}
          className="fill-gray-900 text-3xl font-bold"
        >
          {total}
        </tspan>
        <tspan
          x={center.cx}
          y={center.cy + 22}
          className="fill-gray-500 text-sm"
        >
          Tage
        </tspan>
      </text>
    );
  }
  return null;
}

// Long-term analytical view for a staff member. Owns three sections:
//   A. Jahres-Header — 3 KpiCards (Saldo, Urlaub, Krank seit Jahresbeginn)
//   B. Stundenkonto-Verlauf — shadcn AreaChart with gradient, last 12 weeks
//   C. Zeitverteilung — shadcn Donut with centered total, OGS/HO/Urlaub/...
//
// Period-bound KPIs (this week, this month) live on the Zeiterfassung tab.
export function UebersichtTab({ staffId }: { readonly staffId: string }) {
  // The Berlin day, not the browser's, and re-rendered on the rollover: the
  // overview's target range and account balance both derive from "today", and
  // the server-computed Stundenkonto uses the Berlin month. A non-Berlin
  // browser (or one crossing midnight) would otherwise range its charts against
  // a different day than the headline balance and drift apart (#1842).
  const todayISO = useBerlinToday();
  const today = useMemo(() => parseISODate(todayISO), [todayISO]);

  const { data: timeTrackingConfig } = useSWRAuth("time-tracking-config", () =>
    timeTrackingService.getConfig(),
  );

  const accountAnchor = useMemo(() => {
    return resolveAccountStartDate(today, timeTrackingConfig?.accountStartDate);
  }, [timeTrackingConfig?.accountStartDate, today]);

  const accountStartKey = toDateKey(accountAnchor);
  const yearEndKey = toDateKey(today);
  const adjustmentHistoryEndKey = "9999-12-31";
  const { data: accountSessions, isLoading: sessionsLoading } = useSWRAuth<
    StaffHistorySession[]
  >(`staff-history-account-${staffId}-${accountStartKey}-${yearEndKey}`, () =>
    staffHistoryService.getHistory(staffId, accountStartKey, yearEndKey),
  );
  const { data: accountAbsences, isLoading: absencesLoading } = useSWRAuth<
    StaffAbsenceRow[]
  >(`staff-absences-account-${staffId}-${accountStartKey}-${yearEndKey}`, () =>
    staffAbsenceService.getAbsences(staffId, accountStartKey, yearEndKey),
  );
  // Stundenkonto-Buchungen (#1420) — Teil der Saldo-Wahrheit: sie fliessen in
  // den kumulativen Saldo-Verlauf ein, sonst widerspricht die Kurve der
  // "Stundenkonto"-Kachel direkt darueber.
  const {
    data: accountAdjustments,
    isLoading: adjustmentsLoading,
    error: adjustmentsError,
  } = useSWRAuth<BalanceAdjustment[]>(
    `staff-balance-adjustments-${staffId}-${accountStartKey}-${adjustmentHistoryEndKey}`,
    () =>
      staffBalanceAdjustmentService.list(
        staffId,
        accountStartKey,
        adjustmentHistoryEndKey,
      ),
  );
  const refreshStundenkonto = useTenantMutateMatching([
    `staff-month-summary-${staffId}-`,
    `staff-balance-adjustments-${staffId}-`,
    `staff-history-account-${staffId}-`,
    `staff-absences-account-${staffId}-`,
  ]);

  // Date-valid Soll for the whole account range — the same map the Monatskarte
  // and the daily rows are priced against (#1842). The charts used to resolve
  // every historical day against the CURRENT StaffSchedule, so after a
  // contract change (8h -> 4h) the Saldo line re-priced months of history at
  // today's hours and drifted away from the "Stundenkonto" headline right
  // above it. Chunked: "Gesamt" can outrun the endpoint's 366-day window.
  const {
    data: accountTargets,
    isLoading: targetsLoading,
    error: targetsError,
  } = useSWRAuth<TargetsByDay>(
    `staff-schedule-targets-account-${staffId}-${accountStartKey}-${yearEndKey}`,
    () =>
      staffMonthSummaryService.getScheduleTargetsRange(
        staffId,
        accountStartKey,
        yearEndKey,
      ),
  );

  // Date-valid Stundenkonto from the server-computed month model (#1842).
  const { balanceMinutes: accountBalanceMinutes } = useAccountBalance(staffId);

  // Three independent date-range states — each chart has its own picker so the
  // user can compare timeframes (e.g. "Mai" in the donut vs "letzte 12 Wochen"
  // in the saldo) on the same screen.
  // Unified default: last 60 days. Compromise between readable Tagesvergleich
  // (~42 working days, dots get tight but not unreadable) and meaningful
  // Saldo trend (~8.5 weekly buckets). Donut shows a 2-month distribution.
  // Users can shrink to 7/30 or expand to 90/year/total per chart via picker.
  const [donutRange, setDonutRange] = useState<DateRange | undefined>(() => ({
    from: addDaysSafe(today, -59),
    to: today,
  }));
  const [saldoRange, setSaldoRange] = useState<DateRange | undefined>(() => ({
    from: addDaysSafe(today, -59),
    to: today,
  }));
  const [dailyRange, setDailyRange] = useState<DateRange | undefined>(() => ({
    from: addDaysSafe(today, -59),
    to: today,
  }));

  const clampedDonutRange = clampDateRange(donutRange, accountAnchor, today);
  const clampedSaldoRange = clampDateRange(saldoRange, accountAnchor, today);
  const clampedDailyRange = clampDateRange(dailyRange, accountAnchor, today);
  const donutFrom = clampedDonutRange.from;
  const donutTo = clampedDonutRange.to;
  const saldoFrom = clampedSaldoRange.from;
  const saldoTo = clampedSaldoRange.to;
  const dailyFrom = clampedDailyRange.from;
  const dailyTo = clampedDailyRange.to;

  const absenceDays = useMemo(
    () => countAbsenceDays(accountAbsences ?? [], donutFrom, donutTo),
    [accountAbsences, donutFrom, donutTo],
  );
  const sessionDays = useMemo(
    () => countSessionDaysInRange(accountSessions ?? [], donutFrom, donutTo),
    [accountSessions, donutFrom, donutTo],
  );

  const distributionData = useMemo(
    () => buildDistribution(sessionDays, absenceDays),
    [sessionDays, absenceDays],
  );
  const distributionTotal = useMemo(
    () => distributionData.reduce((acc, x) => acc + x.value, 0),
    [distributionData],
  );

  const weeklyTrendData = useMemo(
    () =>
      buildWeeklyBalanceSeriesRange(
        accountTargets ?? EMPTY_TARGETS,
        accountSessions ?? [],
        accountAbsences ?? [],
        accountAdjustments ?? [],
        saldoFrom,
        saldoTo,
      ),
    [
      accountTargets,
      accountSessions,
      accountAbsences,
      accountAdjustments,
      saldoFrom,
      saldoTo,
    ],
  );

  const dailyTrendData = useMemo(
    () =>
      buildDailyIstSollSeriesRange(
        accountTargets ?? EMPTY_TARGETS,
        accountSessions ?? [],
        accountAbsences ?? [],
        dailyFrom,
        dailyTo,
      ),
    [accountTargets, accountSessions, accountAbsences, dailyFrom, dailyTo],
  );

  const hasTimeTrendData =
    (accountSessions?.length ?? 0) + (accountAbsences?.length ?? 0) > 0;
  const hasBalanceTrendData =
    hasTimeTrendData || (accountAdjustments?.length ?? 0) > 0;

  // The Soll is a chart axis here, not a decoration: rendering before the
  // targets land would draw a 0-Soll baseline and a Saldo line that jumps once
  // the real map arrives.
  if (
    sessionsLoading ||
    absencesLoading ||
    targetsLoading ||
    adjustmentsLoading
  ) {
    return <UebersichtTabSkeleton />;
  }

  // A failed targets fetch must NOT fall through to EMPTY_TARGETS: that would
  // price every contractual day at 0 Soll and render a Saldo line that reads as
  // a huge surplus. The Soll is the axis the whole view hangs on, so surface the
  // error instead of drawing a confidently-wrong chart (#1842).
  if (targetsError) {
    return (
      <div className="space-y-5">
        <Alert
          type="error"
          message="Die Sollzeiten konnten nicht geladen werden. Die Auswertung wird nicht angezeigt, um keine falschen Soll- und Saldo-Werte darzustellen. Bitte lade die Seite neu."
        />
      </div>
    );
  }

  // Gleiche Logik für die Stundenkonto-Buchungen: ein Fetch-Fehler darf nicht
  // als leeres Buchungsprotokoll ("Noch keine Buchungen") durchgehen — dann
  // fehlen Auszahlungen und Resets sowohl im Protokoll als auch in der
  // Saldo-Kurve, die der "Stundenkonto"-Kachel direkt widersprechen würde.
  if (adjustmentsError) {
    return (
      <div className="space-y-5">
        <Alert
          type="error"
          message="Die Stundenkonto-Buchungen konnten nicht geladen werden. Die Auswertung wird nicht angezeigt, um keine falschen Saldo-Werte darzustellen. Bitte lade die Seite neu."
        />
      </div>
    );
  }

  const yearStartLabel = accountAnchor.toLocaleDateString("de-DE", {
    timeZone: "Europe/Berlin",
    day: "numeric",
    month: "long",
    year: "numeric",
  });

  const accountColor =
    accountBalanceMinutes === null
      ? "gray"
      : getDeltaStatus(accountBalanceMinutes);

  return (
    <div className="space-y-5">
      {/* A — Jahres-Header (KpiCards, gleiches Layout wie Zeiterfassung) */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <KpiCard
          label="Stundenkonto"
          primary={
            accountBalanceMinutes === null
              ? "–"
              : formatSignedDuration(accountBalanceMinutes)
          }
          secondary={
            accountBalanceMinutes === 0
              ? `Soll und Ist ausgeglichen seit ${yearStartLabel}`
              : `seit ${yearStartLabel}`
          }
          color={accountColor}
        />
        <KpiCard
          label="Urlaubstage"
          primary={`${absenceDays.vacation}`}
          secondary={`genommen seit ${yearStartLabel}`}
          color="gray"
        />
        <KpiCard
          label="Krankheitstage"
          primary={`${absenceDays.sick}`}
          secondary={`seit ${yearStartLabel}`}
          color="gray"
        />
      </div>

      {/* A2 — Stundenkonto-Verwaltung (#1420): Auszahlung / FZA / Reset */}
      <StundenkontoPanel
        staffId={staffId}
        accountStartKey={accountStartKey}
        todayKey={todayISO}
        balanceMinutes={accountBalanceMinutes}
        adjustments={accountAdjustments ?? []}
        onChanged={refreshStundenkonto}
      />

      {/* B — Zwei Charts side-by-side */}
      <div className="grid grid-cols-1 gap-5 md:grid-cols-2">
        {/* B1 — Tagesvergleich Ist/Soll */}
        <SectionCard
          title="Tagesvergleich Ist / Soll"
          headingLevel={3}
          action={
            <DateRangePicker
              value={clampedDailyRange}
              onChange={setDailyRange}
              presets={buildDefaultPresets(accountAnchor, today)}
              fromMin={accountAnchor}
              toMax={today}
            />
          }
        >
          {!hasTimeTrendData || dailyTrendData.length < 2 ? (
            <p className="py-10 text-center text-sm text-gray-400">
              {hasTimeTrendData
                ? "Noch zu wenige Werktage erfasst — sobald ein zweiter Tag dazukommt, erscheint der Vergleich."
                : "Noch keine Daten — sobald die erste Arbeitszeit erfasst ist, erscheint der Vergleich."}
            </p>
          ) : (
            <ChartContainer
              config={dailyConfig}
              className="!aspect-auto h-64 w-full"
            >
              <LineChart
                accessibilityLayer
                data={dailyTrendData}
                margin={{ top: 8, right: 12, left: 0, bottom: 0 }}
              >
                <CartesianGrid vertical={false} strokeDasharray="3 3" />
                <XAxis
                  dataKey="dayLabel"
                  tickLine={false}
                  axisLine={false}
                  tickMargin={8}
                  fontSize={11}
                  interval="preserveStartEnd"
                />
                <YAxis
                  tickFormatter={(v) => formatHoursCompact(Number(v))}
                  tickLine={false}
                  axisLine={false}
                  tickMargin={4}
                  fontSize={11}
                  width={56}
                />
                <ChartTooltip
                  cursor={false}
                  content={
                    <ChartTooltipContent
                      indicator="line"
                      labelFormatter={(_l, payload) => {
                        const p = payload?.[0]?.payload as
                          { fullLabel?: string } | undefined;
                        return p?.fullLabel ?? "";
                      }}
                      formatter={(value) => formatSignedDuration(Number(value))}
                    />
                  }
                />
                <Line
                  dataKey="soll"
                  name="Soll"
                  type="monotone"
                  stroke="var(--color-soll)"
                  strokeWidth={1.5}
                  strokeDasharray="4 4"
                  dot={false}
                  isAnimationActive={false}
                />
                <Line
                  dataKey="ist"
                  name="Ist"
                  type="monotone"
                  stroke="var(--color-ist)"
                  strokeWidth={2.5}
                  dot={{ fill: "var(--color-ist)", r: 3 }}
                  activeDot={{ r: 5 }}
                />
              </LineChart>
            </ChartContainer>
          )}
        </SectionCard>

        {/* B2 — Saldo-Verlauf kumulativ */}
        <SectionCard
          title="Saldo-Verlauf"
          headingLevel={3}
          action={
            <DateRangePicker
              value={clampedSaldoRange}
              onChange={setSaldoRange}
              presets={buildDefaultPresets(accountAnchor, today)}
              fromMin={accountAnchor}
              toMax={today}
            />
          }
        >
          {!hasBalanceTrendData || weeklyTrendData.length < 2 ? (
            <p className="py-10 text-center text-sm text-gray-400">
              {hasBalanceTrendData
                ? "Noch zu wenige Wochen für einen Verlauf — sobald eine zweite Woche dazukommt, erscheint der Saldo."
                : "Noch keine Daten — der Saldo erscheint, sobald die erste Woche erfasst ist."}
            </p>
          ) : (
            <ChartContainer
              config={trendConfig}
              className="!aspect-auto h-64 w-full"
            >
              <AreaChart
                accessibilityLayer
                data={weeklyTrendData}
                margin={{ top: 8, right: 12, left: 0, bottom: 0 }}
              >
                <defs>
                  <linearGradient id="fillBalance" x1="0" y1="0" x2="0" y2="1">
                    <stop
                      offset="5%"
                      stopColor="var(--color-balance)"
                      stopOpacity={0.18}
                    />
                    <stop
                      offset="95%"
                      stopColor="var(--color-balance)"
                      stopOpacity={0.02}
                    />
                  </linearGradient>
                </defs>
                <CartesianGrid vertical={false} strokeDasharray="3 3" />
                <XAxis
                  dataKey="weekLabel"
                  tickLine={false}
                  axisLine={false}
                  tickMargin={8}
                  fontSize={11}
                />
                <YAxis
                  tickFormatter={(v) => formatHoursCompact(Number(v))}
                  tickLine={false}
                  axisLine={false}
                  tickMargin={4}
                  fontSize={11}
                  width={56}
                />
                <ReferenceLine
                  y={0}
                  stroke={MOTO_COLOR_PALETTE.neutral.light}
                  strokeDasharray="3 3"
                  strokeWidth={1}
                />
                <ChartTooltip
                  cursor={false}
                  content={
                    <ChartTooltipContent
                      indicator="line"
                      formatter={(value) => formatSignedDuration(Number(value))}
                    />
                  }
                />
                <Area
                  dataKey="balance"
                  type="natural"
                  stroke="var(--color-balance)"
                  strokeWidth={2.5}
                  fill="url(#fillBalance)"
                />
              </AreaChart>
            </ChartContainer>
          )}
        </SectionCard>
      </div>

      {/* C — Zeitverteilung (shadcn Donut + vollständige Legende rechts) */}
      <SectionCard
        title="Zeitverteilung"
        headingLevel={3}
        action={
          <DateRangePicker
            value={clampedDonutRange}
            onChange={setDonutRange}
            presets={buildDefaultPresets(accountAnchor, today)}
            fromMin={accountAnchor}
            toMax={today}
          />
        }
      >
        {distributionTotal === 0 ? (
          <p className="py-10 text-center text-sm text-gray-400">
            Noch keine Tage erfasst.
          </p>
        ) : (
          <div className="grid grid-cols-1 items-center gap-6 md:grid-cols-2">
            <ChartContainer
              config={distributionConfig}
              className="mx-auto aspect-square h-64 w-full max-w-[18rem]"
            >
              <PieChart>
                <ChartTooltip
                  cursor={false}
                  content={
                    <ChartTooltipContent
                      hideLabel
                      formatter={(value, _name, item) => {
                        const days = Number(value);
                        const labelMaybe = item?.payload?.label;
                        const label =
                          typeof labelMaybe === "string" ? labelMaybe : "";
                        return `${label}: ${days} ${days === 1 ? "Tag" : "Tage"}`;
                      }}
                    />
                  }
                />
                <Pie
                  data={distributionData.filter((d) => d.value > 0)}
                  dataKey="value"
                  nameKey="key"
                  innerRadius={60}
                  outerRadius={95}
                  strokeWidth={3}
                >
                  {distributionData
                    .filter((d) => d.value > 0)
                    .map((d) => (
                      <Cell key={d.key} fill={d.color} />
                    ))}
                  <Label
                    content={
                      <DistributionCenterLabel total={distributionTotal} />
                    }
                  />
                </Pie>
              </PieChart>
            </ChartContainer>

            <div className="flex flex-col gap-2">
              {distributionData.map((d) => {
                const isZero = d.value === 0;
                const pct =
                  distributionTotal > 0
                    ? Math.round((d.value / distributionTotal) * 100)
                    : 0;
                return (
                  <div
                    key={d.key}
                    className={`flex items-center justify-between gap-3 text-sm ${
                      isZero ? "opacity-40" : ""
                    }`}
                  >
                    <span className="flex items-center gap-2 text-gray-700">
                      <span
                        className="h-2.5 w-2.5 rounded-full"
                        style={{ backgroundColor: d.color }}
                      />
                      {d.label}
                    </span>
                    <span className="text-gray-500 tabular-nums">
                      {d.value} {d.value === 1 ? "Tag" : "Tage"}
                      <span className="ml-2 text-gray-400">({pct}%)</span>
                    </span>
                  </div>
                );
              })}
            </div>
          </div>
        )}
      </SectionCard>
    </div>
  );
}

// ─── Chart configs ───────────────────────────────────────────────────────────

// Compact Y-axis label for the Stundenkonto-Verlauf chart. Shows whole hours
// only (e.g. "+12h", "−466h") so the value fits one line — the full precision
// stays in the tooltip via formatSignedDuration.
function formatHoursCompact(minutes: number): string {
  if (minutes === 0) return "0h";
  const sign = minutes > 0 ? "+" : "−";
  const hours = Math.round(Math.abs(minutes) / 60);
  return `${sign}${hours}h`;
}

const trendConfig = {
  balance: { label: "Saldo", color: MOTO_COLOR_PALETTE.green.base },
} satisfies ChartConfig;

const dailyConfig = {
  ist: { label: "Ist", color: MOTO_COLOR_PALETTE.green.base },
  soll: { label: "Soll", color: MOTO_COLOR_PALETTE.neutral.light },
} satisfies ChartConfig;

// Categories aligned to LOCATION_COLORS where the semantics match. Absence
// types pick distinct hues from the rest of the brand palette so the donut
// stays readable even when 5+ segments are visible. Krank folgt der
// verbindlichen Ton-Zuordnung (Krank = rot), nicht der Bernstein-Optik.
const distributionConfig = {
  ogs: { label: "OGS", color: MOTO_COLOR_PALETTE.green.base },
  // Muss dem RowStatusBadge in staff-session-table folgen: blue.base ist der
  // Ton, den getLocationBadgeTone fuer ABSENCE_TYPE_HEX.vacation liefert, und
  // Uebersicht, Zeiterfassung und Abwesenheiten sind Tabs derselben Seite.
  homeoffice: {
    label: "Homeoffice",
    color: MOTO_COLOR_PALETTE.timeTracking.base,
  },
  urlaub: { label: "Urlaub", color: MOTO_COLOR_PALETTE.orange.base },
  krank: { label: "Krank", color: MOTO_COLOR_PALETTE.red.base },
  fortbildung: { label: "Fortbildung", color: MOTO_COLOR_PALETTE.purple.base },
  freizeitausgleich: {
    label: "Freizeitausgleich",
    color: MOTO_COLOR_PALETTE.magenta.base,
  },
  sonstige: { label: "Sonstige", color: MOTO_COLOR_PALETTE.neutral.base },
} satisfies ChartConfig;

// ─── Helpers ─────────────────────────────────────────────────────────────────

interface AbsenceDayCounts {
  vacation: number;
  sick: number;
  training: number;
  compTime: number;
  other: number;
}

function countAbsenceDays(
  absences: readonly StaffAbsenceRow[],
  from: Date,
  to: Date,
): AbsenceDayCounts {
  const counts: AbsenceDayCounts = {
    vacation: 0,
    sick: 0,
    training: 0,
    compTime: 0,
    other: 0,
  };
  const fromKey = toDateKey(from);
  const toKey = toDateKey(to);
  for (const a of absences) {
    if (a.status !== "reported" && a.status !== "approved") continue;
    const startKey = a.date_start.slice(0, 10);
    const endKey = a.date_end.slice(0, 10);
    const clippedStart = startKey < fromKey ? fromKey : startKey;
    const clippedEnd = endKey > toKey ? toKey : endKey;
    if (clippedStart > clippedEnd) continue;
    const startDate = new Date(`${clippedStart}T00:00:00`);
    const endDate = new Date(`${clippedEnd}T00:00:00`);
    let days =
      Math.floor((endDate.getTime() - startDate.getTime()) / 86_400_000) + 1;
    if (a.absence_type === "comp_time") {
      if (isHalfAbsenceBoundary(a, clippedStart, startKey, endKey)) {
        days -= 0.5;
      }
      if (
        clippedEnd !== clippedStart &&
        isHalfAbsenceBoundary(a, clippedEnd, startKey, endKey)
      ) {
        days -= 0.5;
      }
    }
    const bucket =
      a.absence_type === "sick"
        ? "sick"
        : a.absence_type === "vacation"
          ? "vacation"
          : a.absence_type === "training"
            ? "training"
            : a.absence_type === "comp_time"
              ? "compTime"
              : "other";
    counts[bucket] += days;
  }
  return counts;
}

function countSessionDaysInRange(
  sessions: readonly StaffHistorySession[],
  from: Date,
  to: Date,
): { present: number; homeOffice: number } {
  const fromKey = toDateKey(from);
  const toKey = toDateKey(to);
  let present = 0;
  let homeOffice = 0;
  for (const s of sessions) {
    const key = s.date.slice(0, 10);
    if (key < fromKey || key > toKey) continue;
    if (s.status === "home_office") homeOffice += 1;
    else present += 1;
  }
  return { present, homeOffice };
}

function addDaysSafe(d: Date, days: number): Date {
  const result = new Date(d);
  result.setDate(result.getDate() + days);
  return result;
}

function clampDateRange(
  range: DateRange | undefined,
  accountAnchor: Date,
  today: Date,
): ConcreteDateRange {
  const minimum = accountAnchor > today ? today : accountAnchor;
  const fallbackFrom = addDaysSafe(today, -59);
  const fromCandidate = range?.from ?? fallbackFrom;
  const toCandidate = range?.to ?? today;
  const from = fromCandidate < minimum ? minimum : fromCandidate;
  const to = toCandidate < from ? from : toCandidate;
  return { from, to };
}

interface DistributionBucket {
  key: string;
  label: string;
  value: number;
  color: string;
}

function buildDistribution(
  sessionDays: { present: number; homeOffice: number },
  absenceDays: AbsenceDayCounts,
): DistributionBucket[] {
  // Colors come from distributionConfig rather than being repeated here. The
  // two lists mirror each other key for key, and the Pie renders THIS one
  // (<Cell fill={d.color}>) while the legend and tooltip read the config — so
  // a value changed in only one place silently splits the chart from its own
  // legend. That is exactly what happened when Homeoffice was moved off the
  // blue that also means "Urlaub" in the sibling tabs.
  const color = (key: keyof typeof distributionConfig) =>
    distributionConfig[key].color;

  return [
    {
      key: "ogs",
      label: "OGS",
      value: sessionDays.present,
      color: color("ogs"),
    },
    {
      key: "homeoffice",
      label: "Homeoffice",
      value: sessionDays.homeOffice,
      color: color("homeoffice"),
    },
    {
      key: "urlaub",
      label: "Urlaub",
      value: absenceDays.vacation,
      color: color("urlaub"),
    },
    {
      key: "krank",
      label: "Krank",
      value: absenceDays.sick,
      color: color("krank"),
    },
    {
      key: "fortbildung",
      label: "Fortbildung",
      value: absenceDays.training,
      color: color("fortbildung"),
    },
    {
      key: "freizeitausgleich",
      label: "Freizeitausgleich",
      value: absenceDays.compTime,
      color: color("freizeitausgleich"),
    },
    {
      key: "sonstige",
      label: "Sonstige",
      value: absenceDays.other,
      color: color("sonstige"),
    },
  ];
}

interface TrendPoint {
  weekLabel: string;
  balance: number;
}

// Splits each block at Berlin midnight before the chart series aggregate it.
// History rows retain their original check-in date by design, but the monthly
// balance books their net time on every Berlin calendar day the interval
// touches. Charts must use the same allocation or a night block shifts Ist and
// Saldo into the wrong day/week.
export function indexSessionNetMinutesByBerlinDate(
  sessions: readonly StaffHistorySession[],
): Map<string, number> {
  const byDate = new Map<string, number>();
  for (const session of sessions) {
    for (const [date, minutes] of splitSessionNetMinutesByBerlinDate(session)) {
      byDate.set(date, (byDate.get(date) ?? 0) + minutes);
    }
  }
  return byDate;
}

function splitSessionNetMinutesByBerlinDate(
  session: StaffHistorySession,
): Map<string, number> {
  const start = new Date(session.check_in_time);
  const end = session.check_out_time
    ? new Date(session.check_out_time)
    : new Date();
  if (
    Number.isNaN(start.getTime()) ||
    Number.isNaN(end.getTime()) ||
    !end.getTime() ||
    end <= start
  ) {
    return new Map([[session.date.slice(0, 10), session.net_minutes]]);
  }

  const grossByDate = new Map<string, number>();
  const breakByDate = new Map<string, number>();
  let cursor = start;
  while (cursor < end) {
    const date = berlinTodayISO(cursor);
    const nextMidnight =
      new Date(endOfBerlinDayISO(parseISODate(date))).getTime() + 1_000;
    const segmentEnd = new Date(Math.min(nextMidnight, end.getTime()));
    const grossMinutes = (segmentEnd.getTime() - cursor.getTime()) / 60_000;
    grossByDate.set(date, (grossByDate.get(date) ?? 0) + grossMinutes);

    for (const workBreak of session.breaks ?? []) {
      const breakStart = new Date(workBreak.started_at);
      const breakEnd = workBreak.ended_at ? new Date(workBreak.ended_at) : end;
      if (
        Number.isNaN(breakStart.getTime()) ||
        Number.isNaN(breakEnd.getTime())
      ) {
        continue;
      }
      const overlapStart = Math.max(cursor.getTime(), breakStart.getTime());
      const overlapEnd = Math.min(segmentEnd.getTime(), breakEnd.getTime());
      if (overlapEnd > overlapStart) {
        breakByDate.set(
          date,
          (breakByDate.get(date) ?? 0) + (overlapEnd - overlapStart) / 60_000,
        );
      }
    }
    cursor = segmentEnd;
  }

  const rawNetByDate = new Map<string, number>();
  let totalRawNet = 0;
  for (const [date, grossMinutes] of grossByDate) {
    const netMinutes = Math.max(0, grossMinutes - (breakByDate.get(date) ?? 0));
    rawNetByDate.set(date, netMinutes);
    totalRawNet += netMinutes;
  }
  return distributeNetMinutes(rawNetByDate, totalRawNet, session.net_minutes);
}

function distributeNetMinutes(
  rawNetByDate: ReadonlyMap<string, number>,
  totalRawNet: number,
  netMinutes: number,
): Map<string, number> {
  if (totalRawNet <= 0) {
    return new Map();
  }

  const shares = [...rawNetByDate.entries()].map(([date, rawMinutes]) => {
    const exact = (rawMinutes / totalRawNet) * netMinutes;
    return { date, minutes: Math.floor(exact), remainder: exact % 1 };
  });
  let remaining =
    netMinutes - shares.reduce((sum, share) => sum + share.minutes, 0);
  shares.sort(
    (left, right) =>
      right.remainder - left.remainder || left.date.localeCompare(right.date),
  );
  for (let index = 0; remaining > 0; index = (index + 1) % shares.length) {
    shares[index]!.minutes += 1;
    remaining -= 1;
  }
  return new Map(shares.map(({ date, minutes }) => [date, minutes]));
}

// Cumulative weekly balance series over [from, to]. Running total starts at 0
// at the first week of `from` — caller can shift the anchor by widening the
// range. Weeks beyond `to` are skipped; the final week is clipped at `to`.
function buildWeeklyBalanceSeriesRange(
  targets: TargetsByDay,
  sessions: readonly StaffHistorySession[],
  absences: readonly StaffAbsenceRow[],
  adjustments: readonly BalanceAdjustment[],
  from: Date,
  to: Date,
): TrendPoint[] {
  const points: TrendPoint[] = [];
  const istByDate = indexSessionNetMinutesByBerlinDate(sessions);
  const creditByDate = indexAbsenceCreditByDay(targets, absences);
  // Stundenkonto-Buchungen (#1420) als Stufen am Wirksamkeitstag — gespiegelt
  // zum Server (addAdjustments): der Saldo-Verlauf muss nach einer Auszahlung
  // oder einem Reset dieselbe Kurve zeigen wie die Kachel darueber.
  const adjustmentByDate = new Map<string, number>();
  for (const a of adjustments) {
    const key = a.effectiveDate.slice(0, 10);
    adjustmentByDate.set(
      key,
      (adjustmentByDate.get(key) ?? 0) + a.minutesDelta,
    );
  }

  const firstWeekStart = startOfWeek(from);
  const lastWeekStart = startOfWeek(to);
  const weekCount = Math.min(
    104,
    Math.floor(
      (lastWeekStart.getTime() - firstWeekStart.getTime()) / (7 * 86_400_000),
    ) + 1,
  );
  let running = 0;
  for (let i = 0; i < weekCount; i++) {
    const weekStart = new Date(firstWeekStart);
    weekStart.setDate(weekStart.getDate() + i * 7);
    const weekEnd = endOfWeek(weekStart);
    const clippedStart = weekStart < from ? from : weekStart;
    const clippedEnd = weekEnd > to ? to : weekEnd;
    const weekDelta = computeWeekDelta(
      targets,
      istByDate,
      creditByDate,
      adjustmentByDate,
      clippedStart,
      clippedEnd,
    );
    running += weekDelta;
    points.push({
      weekLabel: `KW ${isoWeekNumber(weekStart)}`,
      balance: running,
    });
  }
  return points;
}

function computeWeekDelta(
  targets: TargetsByDay,
  istByDate: ReadonlyMap<string, number>,
  creditByDate: ReadonlyMap<string, number>,
  adjustmentByDate: ReadonlyMap<string, number>,
  weekStart: Date,
  weekEnd: Date,
): number {
  let soll = 0;
  let ist = 0;
  let adjustment = 0;
  const dayCount =
    Math.floor((weekEnd.getTime() - weekStart.getTime()) / 86_400_000) + 1;
  for (let i = 0; i < dayCount; i++) {
    const day = new Date(weekStart);
    day.setDate(day.getDate() + i);
    const key = toDateKey(day);
    soll += targets.get(key) ?? 0;
    const dayIst = istByDate.get(key);
    if (dayIst !== undefined) {
      ist += dayIst;
    } else {
      ist += creditByDate.get(key) ?? 0;
    }
    adjustment += adjustmentByDate.get(key) ?? 0;
  }
  return ist + adjustment - soll;
}

function isoWeekNumber(d: Date): number {
  const target = new Date(d.getFullYear(), d.getMonth(), d.getDate());
  const dayNr = (target.getDay() + 6) % 7;
  target.setDate(target.getDate() - dayNr + 3);
  const firstThursday = new Date(target.getFullYear(), 0, 4);
  const diff = target.getTime() - firstThursday.getTime();
  return (
    1 +
    Math.round((diff / 86_400_000 - 3 + ((firstThursday.getDay() + 6) % 7)) / 7)
  );
}

interface DailyTrendPoint {
  dayLabel: string;
  fullLabel: string;
  ist: number;
  soll: number;
}

// Per-working-day Ist vs Soll across [from, to]. Same absence-credit semantics
// as the weekly series and the Monatskarte: reported/approved
// Krank/Urlaub/Fortbildung credit the day's Soll (half on half-day boundaries)
// so the Ist line stays meaningful during legitimate absences.
function buildDailyIstSollSeriesRange(
  targets: TargetsByDay,
  sessions: readonly StaffHistorySession[],
  absences: readonly StaffAbsenceRow[],
  from: Date,
  to: Date,
): DailyTrendPoint[] {
  const istByDate = indexSessionNetMinutesByBerlinDate(sessions);
  const creditByDate = indexAbsenceCreditByDay(targets, absences);

  const result: DailyTrendPoint[] = [];
  // Cap at 120 entries to keep the chart readable; wider ranges get truncated.
  const start = new Date(from.getFullYear(), from.getMonth(), from.getDate());
  const end = new Date(to.getFullYear(), to.getMonth(), to.getDate());
  const totalDays =
    Math.floor((end.getTime() - start.getTime()) / 86_400_000) + 1;
  for (let i = 0; i < totalDays && result.length < 120; i++) {
    const day = new Date(start);
    day.setDate(day.getDate() + i);
    const dow = toIsoDayOfWeek(day);
    if (dow < 5) {
      const key = toDateKey(day);
      const soll = targets.get(key) ?? 0;
      const dayIst = istByDate.get(key);
      let ist = 0;
      if (dayIst !== undefined) {
        ist = dayIst;
      } else {
        ist = creditByDate.get(key) ?? 0;
      }
      result.push({
        dayLabel: day.toLocaleDateString("de-DE", {
          timeZone: "Europe/Berlin",
          day: "2-digit",
          month: "2-digit",
        }),
        fullLabel: day.toLocaleDateString("de-DE", {
          timeZone: "Europe/Berlin",
          weekday: "short",
          day: "numeric",
          month: "long",
        }),
        ist,
        soll,
      });
    }
  }
  return result;
}

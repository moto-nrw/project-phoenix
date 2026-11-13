"use client";

// Stand einer Ferienanmeldung (#6676): die Seite hinter dem geheimen Link der
// Eingangsmail. Eltern ohne Konto sehen, ob die Anmeldung eingegangen,
// bestätigt oder storniert ist, und je Kind die Wochen mit Platz,
// Warteliste oder offenem Platz. Die Seite zeigt nur; der Träger entscheidet.

import { CalendarDays, Check, Clock, Link2Off, Lock, X } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { useCallback, useEffect, useState } from "react";
import { PublicEnrollmentContentSkeleton } from "~/components/enrollment/public-enrollment-shell";
import { Alert } from "~/components/ui/alert";
import { DataField, DataGrid } from "~/components/ui/data-field";
import { EmptyState } from "~/components/ui/empty-state";
import { FramedList } from "~/components/ui/framed-list";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { IconTile } from "~/components/ui/icon-tile";
import { SectionCard } from "~/components/ui/section-card";
import { SectionLabel } from "~/components/ui/section-label";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiLoadError } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { holidayWeeksConcept } from "~/lib/concepts/holidayWeeks";
import { berlinTodayISO, formatDate } from "~/lib/date-helpers";
import {
  fetchHolidayRegistrationStatus,
  type HolidayRegistrationStatus,
  type HolidayRegistrationStatusChild,
  type HolidayRegistrationStatusWeek,
} from "~/lib/holiday-registration-status-api";
import { createLogger } from "~/lib/logger";
import {
  HOLIDAY_PLACEMENT_BADGES,
  HOLIDAY_REGISTRATION_STATUS_TONES,
} from "./holiday-registration-badges";

const logger = createLogger({ component: "HolidayRegistrationStatusView" });

/** The code of a status link without a registration. */
const STATUS_NOT_FOUND = "enrollment.holiday_registration_status_not_found";

export function HolidayRegistrationStatusView({
  token,
}: Readonly<{ token: string }>) {
  const t = useTranslations("holidayRegistrationStatus");
  const [status, setStatus] = useState<HolidayRegistrationStatus | null>(null);
  const [notFound, setNotFound] = useState(false);
  const loadError = useApiLoadError();
  const { show, clear } = loadError;

  const load = useCallback(async () => {
    try {
      setStatus(await fetchHolidayRegistrationStatus(token));
      setNotFound(false);
      clear();
    } catch (err) {
      if (err instanceof ApiError && err.code === STATUS_NOT_FOUND) {
        // Kein Fehler der Seite: der Link führt zu keiner Anmeldung mehr.
        // Die Seite sagt das selbst, mit dem nächsten Schritt.
        setNotFound(true);
        return;
      }
      logger.error("holiday_registration_status_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await show(err, { object: t("errorObject"), retry: () => void load() });
    }
  }, [clear, show, t, token]);

  useEffect(() => {
    void load();
  }, [load]);

  if (notFound) return <StatusNotFound />;
  if (loadError.error) return <LoadErrorAlert error={loadError.error} />;
  if (!status) {
    return (
      <div role="status" aria-label={t("loading")}>
        <PublicEnrollmentContentSkeleton sections={1} />
      </div>
    );
  }
  return <StatusContent status={status} />;
}

function StatusContent({
  status,
}: Readonly<{ status: HolidayRegistrationStatus }>) {
  const t = useTranslations("holidayRegistrationStatus");
  const tr = useTranslations("holidayRegistration");
  const active = status.status !== "cancelled";
  const weeks = status.children.flatMap((child) => child.weeks);
  const waitlisted =
    active && weeks.some((week) => week.placement === "waitlisted");
  const pending = active && weeks.some((week) => week.placement === "pending");

  return (
    <div className="mx-auto w-full max-w-4xl space-y-5">
      <StatusHero status={status} />
      {status.children.length > 0 ? (
        <SectionCard
          concept={holidayWeeksConcept}
          title={t("weeksTitle")}
          description={t("weeksDescription")}
          namedRegion
        >
          <div className="space-y-6">
            <ul className="space-y-6">
              {status.children.map((child, index) => (
                <li key={`${child.firstName}-${index}`}>
                  <ChildWeeks child={child} active={active} />
                </li>
              ))}
            </ul>
            {pending ? <Alert type="info" message={tr("pendingNote")} /> : null}
            {waitlisted ? (
              <Alert type="info" message={tr("waitlistNote")} />
            ) : null}
            {active && status.binding ? (
              <p className="text-sm leading-6 text-gray-700">{tr("binding")}</p>
            ) : null}
          </div>
        </SectionCard>
      ) : null}
      <p className="flex items-start gap-2 px-1 text-sm leading-6 text-gray-600">
        <Lock className="mt-1 h-4 w-4 shrink-0" aria-hidden="true" />
        <span>{t("privacyNote")}</span>
      </p>
    </div>
  );
}

/** Kopf der Seite: der Stand in einem Satz und was als Nächstes passiert. */
function StatusHero({
  status,
}: Readonly<{ status: HolidayRegistrationStatus }>) {
  const t = useTranslations("holidayRegistrationStatus");
  const tp = useTranslations("parentHolidayRegistrations");
  const tr = useTranslations("holidayRegistration");
  const locale = useLocale();
  const submitted = formatDate(status.submittedAt, false, locale);
  const period =
    status.firstDay && status.lastDay
      ? tr("weekRange", {
          starts: formatDate(status.firstDay, false, locale),
          ends: formatDate(status.lastDay, false, locale),
        })
      : "";

  return (
    <SectionCard padding="none">
      <div className="grid lg:grid-cols-[minmax(0,1fr)_20rem]">
        <div className="p-5 sm:p-6">
          <p className="text-sm font-medium text-gray-600">
            {`${t("kicker")} · ${status.traegerName}`}
          </p>
          <div className="mt-5 flex items-center gap-3">
            <IconTile
              size="hero"
              tone={HERO_TONES[status.status]}
              icon={HERO_ICONS[status.status]}
            />
            <StatusBadge
              label={tp(`status.${status.status}`)}
              tone={HOLIDAY_REGISTRATION_STATUS_TONES[status.status]}
            />
          </div>
          <h1 className="mt-5 text-2xl font-bold tracking-tight text-balance text-gray-900 sm:text-3xl">
            {t(`title.${status.status}`)}
          </h1>
          <p className="mt-3 max-w-xl text-base leading-7 text-gray-600">
            {t(`lead.${status.status}`, { traeger: status.traegerName })}
          </p>
          <div className="mt-5">
            <DataGrid>
              <DataField
                label={t("periodLabel")}
                icon={<CalendarDays className="h-3.5 w-3.5" />}
              >
                <span className="block">{status.periodName}</span>
                {period ? (
                  <span className="block font-normal text-gray-700 tabular-nums">
                    {period}
                  </span>
                ) : null}
              </DataField>
              <DataField
                label={t("submittedLabel")}
                icon={<Clock className="h-3.5 w-3.5" />}
              >
                <span className="tabular-nums">{submitted}</span>
              </DataField>
            </DataGrid>
          </div>
        </div>
        <aside className="moto-dotted-background moto-dotted-background--split border-t border-gray-100 p-5 sm:p-6 lg:border-t-0 lg:border-l">
          <h2 className="text-base font-semibold text-gray-900">
            {t("stepsTitle")}
          </h2>
          <StatusSteps status={status} submitted={submitted} />
        </aside>
      </div>
    </SectionCard>
  );
}

const HERO_TONES = {
  received: "blue",
  confirmed: "green",
  cancelled: "neutral",
} as const;

const HERO_ICONS = {
  received: <Clock aria-hidden="true" />,
  confirmed: <Check aria-hidden="true" />,
  cancelled: <X aria-hidden="true" />,
} as const;

type StepState = "done" | "current" | "next";

/**
 * Die drei Schritte einer Anmeldung: eingegangen, Prüfung durch den Träger,
 * Zusage. Eine stornierte Anmeldung hat keinen nächsten Schritt.
 */
function StatusSteps({
  status,
  submitted,
}: Readonly<{ status: HolidayRegistrationStatus; submitted: string }>) {
  const t = useTranslations("holidayRegistrationStatus");
  const locale = useLocale();
  if (status.status === "cancelled") {
    return (
      <p className="mt-4 text-sm leading-6 text-gray-700">
        {t("stepsCancelled")}
      </p>
    );
  }
  const confirmed = status.status === "confirmed";
  const closes = formatDate(status.registrationClosesOn, false, locale);
  const windowOpen = berlinTodayISO() <= status.registrationClosesOn;
  let reviewDetail = t("stepReviewRunning");
  if (confirmed) reviewDetail = t("stepReviewDone");
  else if (windowOpen) reviewDetail = t("stepReviewAfter", { date: closes });
  const steps: readonly {
    readonly title: string;
    readonly detail: string;
    readonly state: StepState;
  }[] = [
    {
      title: t("stepReceived"),
      detail: t("stepReceivedDetail", { date: submitted }),
      state: "done",
    },
    {
      title: t("stepReview"),
      detail: reviewDetail,
      state: confirmed ? "done" : "current",
    },
    {
      title: t("stepDecision"),
      detail: confirmed ? t("stepDecisionDone") : t("stepDecisionNext"),
      state: confirmed ? "done" : "next",
    },
  ];
  return (
    <ol className="mt-4 space-y-4">
      {steps.map((step, index) => (
        <li
          key={step.title}
          className="flex gap-3"
          aria-current={step.state === "current" ? "step" : undefined}
        >
          <StepMarker state={step.state} number={index + 1} />
          <div className="min-w-0 pt-1">
            <p
              className={`text-sm font-semibold ${step.state === "next" ? "text-gray-600" : "text-gray-900"}`}
            >
              {step.title}
              <span className="sr-only">{`: ${t(`stepState.${step.state}`)}`}</span>
            </p>
            <p className="text-sm leading-6 text-gray-600">{step.detail}</p>
          </div>
        </li>
      ))}
    </ol>
  );
}

function StepMarker({
  state,
  number,
}: Readonly<{ state: StepState; number: number }>) {
  if (state === "done") {
    return (
      <IconTile
        size="xs"
        shape="round"
        tone="green"
        icon={<Check aria-hidden="true" />}
      />
    );
  }
  return (
    <IconTile
      size="xs"
      shape="round"
      tone={state === "current" ? "inverse" : "neutral"}
      icon={
        <span className="text-xs font-semibold tabular-nums" aria-hidden="true">
          {number}
        </span>
      }
    />
  );
}

function ChildWeeks({
  child,
  active,
}: Readonly<{ child: HolidayRegistrationStatusChild; active: boolean }>) {
  const t = useTranslations("holidayRegistrationStatus");
  return (
    <section className="space-y-2" aria-label={child.firstName}>
      <div className="flex items-baseline justify-between gap-3">
        <SectionLabel as="h3">{child.firstName}</SectionLabel>
        <span className="text-xs text-gray-600 tabular-nums">
          {t("weekCount", { count: child.weeks.length })}
        </span>
      </div>
      {child.weeks.length > 0 ? (
        <FramedList>
          {child.weeks.map((week) => (
            <WeekRow
              key={`${week.firstDay}-${week.lastDay}`}
              week={week}
              active={active}
            />
          ))}
        </FramedList>
      ) : (
        <p className="text-sm text-gray-600">{t("noWeeks")}</p>
      )}
    </section>
  );
}

function WeekRow({
  week,
  active,
}: Readonly<{ week: HolidayRegistrationStatusWeek; active: boolean }>) {
  const tr = useTranslations("holidayRegistration");
  const t = useTranslations("holidayRegistrationStatus");
  const locale = useLocale();
  const care =
    week.timeModelName && week.startsAt && week.endsAt
      ? tr("timeModel", {
          name: week.timeModelName,
          startsAt: week.startsAt,
          endsAt: week.endsAt,
        })
      : week.timeModelName;
  const details = [
    care,
    week.hostSchoolName ? t("hostSchool", { school: week.hostSchoolName }) : "",
  ].filter(Boolean);
  return (
    <li className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4 py-3">
      <div className="min-w-0">
        <p className="text-sm font-semibold text-gray-900 tabular-nums">
          {tr("weekRange", {
            starts: formatDate(week.firstDay, false, locale),
            ends: formatDate(week.lastDay, false, locale),
          })}
        </p>
        {details.length > 0 ? (
          <p className="mt-0.5 text-sm text-gray-600">{details.join(" · ")}</p>
        ) : null}
      </div>
      {active ? (
        <StatusBadge
          label={tr(HOLIDAY_PLACEMENT_BADGES[week.placement].key)}
          tone={HOLIDAY_PLACEMENT_BADGES[week.placement].tone}
        />
      ) : null}
    </li>
  );
}

/** Der Link führt zu keiner Anmeldung: vertippt, unvollständig oder gelöscht. */
function StatusNotFound() {
  const t = useTranslations("holidayRegistrationStatus");
  return (
    <div className="mx-auto w-full max-w-2xl">
      <SectionCard>
        <EmptyState
          icon={<Link2Off aria-hidden="true" />}
          title={t("notFoundTitle")}
          titleAs="h1"
          description={t("notFoundText")}
        />
      </SectionCard>
    </div>
  );
}

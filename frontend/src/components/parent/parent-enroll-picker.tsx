"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { ArrowLeft, ArrowRight, CalendarClock, School } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { type EnrollablePhase, listEnrollableSchools } from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";
import { formatLocalizedDate } from "~/lib/localized-date-format";

const logger = createLogger({ component: "ParentEnrollPicker" });

interface SchoolGroup {
  readonly schoolId: string;
  readonly schoolName: string;
  readonly schoolSlug: string;
  readonly alreadyLinked: boolean;
  readonly phases: EnrollablePhase[];
}

/**
 * Groups the pre-sorted phase list into one entry per school, preserving the
 * backend's order (already_linked DESC, school name, service start). Insertion
 * order of the Map keeps the first-seen school position.
 */
function groupBySchool(phases: readonly EnrollablePhase[]): SchoolGroup[] {
  const groups = new Map<string, SchoolGroup>();
  for (const phase of phases) {
    const existing = groups.get(phase.school_id);
    if (existing) {
      existing.phases.push(phase);
      continue;
    }
    groups.set(phase.school_id, {
      schoolId: phase.school_id,
      schoolName: phase.school_name,
      schoolSlug: phase.school_slug,
      alreadyLinked: phase.already_linked,
      phases: [phase],
    });
  }
  return [...groups.values()];
}

export function ParentEnrollPicker() {
  const t = useTranslations("parentEnroll");
  const locale = useLocale();
  const [phases, setPhases] = useState<EnrollablePhase[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setPhases(await listEnrollableSchools());
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unknown error";
      logger.warn("parent_enrollable_schools_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const { linked, other } = useMemo(() => {
    const groups = groupBySchool(phases);
    return {
      linked: groups.filter((group) => group.alreadyLinked),
      other: groups.filter((group) => !group.alreadyLinked),
    };
  }, [phases]);

  return (
    <div className="mx-auto w-full max-w-7xl space-y-6">
      <Link
        href="/parents"
        className="inline-flex items-center gap-2 text-sm font-semibold text-[#5080D8] hover:underline focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      >
        <ArrowLeft className="h-4 w-4" aria-hidden="true" />
        {t("back")}
      </Link>

      <section className="overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm">
        <div className="p-5 sm:p-6 lg:p-8">
          <h1 className="text-3xl font-semibold text-gray-900 sm:text-4xl">
            {t("title")}
          </h1>
          <p className="mt-3 max-w-3xl text-sm leading-6 text-gray-600 sm:text-base">
            {t("description")}
          </p>
        </div>
      </section>

      {loading ? (
        <ParentEnrollSkeleton />
      ) : error ? (
        <div className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-5 text-sm text-[#CC2626] shadow-sm">
          {t("loadError")}
        </div>
      ) : phases.length === 0 ? (
        <EmptyPhases />
      ) : (
        <div className="space-y-6">
          {linked.length > 0 && (
            <SchoolSection
              label={t("yourSchools")}
              groups={linked}
              locale={locale}
            />
          )}
          {other.length > 0 && (
            <SchoolSection
              label={t("otherSchools")}
              groups={other}
              locale={locale}
            />
          )}
        </div>
      )}
    </div>
  );
}

function SchoolSection({
  label,
  groups,
  locale,
}: Readonly<{ label: string; groups: SchoolGroup[]; locale: string }>) {
  return (
    <section className="space-y-3">
      <h2 className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
        {label}
      </h2>
      <div className="space-y-4">
        {groups.map((group) => (
          <SchoolCard key={group.schoolId} group={group} locale={locale} />
        ))}
      </div>
    </section>
  );
}

function SchoolCard({
  group,
  locale,
}: Readonly<{ group: SchoolGroup; locale: string }>) {
  return (
    <div className="overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm">
      <div className="flex items-center gap-3 border-b border-gray-100 p-4 sm:px-6">
        <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-[#83CD2D]/15 text-[#5A8E1F]">
          <School className="h-5 w-5" aria-hidden="true" />
        </span>
        <h3 className="min-w-0 text-base font-semibold break-words text-gray-900">
          {group.schoolName}
        </h3>
      </div>
      <ul className="divide-y divide-gray-100">
        {group.phases.map((phase) => (
          <li key={phase.phase_id}>
            <PhaseRow phase={phase} locale={locale} />
          </li>
        ))}
      </ul>
    </div>
  );
}

function PhaseRow({
  phase,
  locale,
}: Readonly<{ phase: EnrollablePhase; locale: string }>) {
  const t = useTranslations("parentEnroll");

  const kindLabel = {
    school_year: t("kindSchoolYear"),
    holiday: t("kindHoliday"),
    custom: t("kindCustom"),
  }[phase.phase_kind];

  const range = `${formatLocalizedDate(phase.service_start_date, locale)} ${t(
    "rangeConnector",
  )} ${formatLocalizedDate(phase.service_end_date, locale)}`;

  const audienceLabel =
    phase.audience === "new_students"
      ? t("audienceNewStudents")
      : phase.audience === "existing_students"
        ? t("audienceExistingStudents")
        : phase.audience === "linked_parents"
          ? t("audienceLinkedParents")
          : null;

  return (
    <Link
      href={`/parents/enroll/${phase.school_slug}/${phase.phase_id}`}
      aria-label={`${t("openPhase")}: ${phase.phase_name}`}
      className="group flex items-center gap-4 p-4 transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:px-6"
    >
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-sm font-semibold text-gray-900">
            {phase.phase_name}
          </span>
          <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-semibold text-gray-600">
            {kindLabel}
          </span>
          {audienceLabel && (
            <span className="rounded-full bg-[#5080D8]/10 px-2 py-0.5 text-[11px] font-semibold text-[#3558A8]">
              {audienceLabel}
            </span>
          )}
        </div>
        <p className="mt-1 text-sm text-gray-600">{range}</p>
        {phase.enrollment_close_at && (
          <p className="mt-1 inline-flex items-center gap-1.5 text-xs font-medium text-gray-500">
            <CalendarClock className="h-3.5 w-3.5" aria-hidden="true" />
            {t("closesAt", {
              date: formatLocalizedDate(phase.enrollment_close_at, locale),
            })}
          </p>
        )}
      </div>
      <ArrowRight
        className="h-4 w-4 shrink-0 text-gray-400 transition-colors group-hover:text-gray-700"
        aria-hidden="true"
      />
    </Link>
  );
}

function EmptyPhases() {
  const t = useTranslations("parentEnroll");
  return (
    <div className="rounded-2xl border border-dashed border-gray-300 bg-gray-50 p-8 text-center">
      <span className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-white text-gray-500 shadow-sm">
        <School className="h-5 w-5" aria-hidden="true" />
      </span>
      <p className="mt-3 text-sm leading-6 text-gray-600">{t("empty")}</p>
    </div>
  );
}

function ParentEnrollSkeleton() {
  return (
    <div className="space-y-4">
      {[0, 1].map((item) => (
        <div
          key={item}
          className="h-40 animate-pulse rounded-2xl border border-gray-200 bg-white shadow-sm"
        />
      ))}
    </div>
  );
}

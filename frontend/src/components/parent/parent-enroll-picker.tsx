"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { Skeleton } from "~/components/ui/skeleton";
import { ParentPage, ParentPageHeader } from "~/components/parent/parent-page";
import { useLocale, useTranslations } from "next-intl";
import { type EnrollablePhase, listEnrollableSchools } from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";
import {
  formatLocalizedDate,
  formatLocalizedDateTime,
} from "~/lib/localized-date-format";

const logger = createLogger({ component: "ParentEnrollPicker" });

interface SchoolGroup {
  readonly schoolId: string;
  readonly schoolName: string;
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
    <ParentPage>
      <ParentPageHeader
        kicker={t("kicker")}
        title={t("title")}
        description={t("description")}
      />

      {loading ? (
        <ParentEnrollSkeleton />
      ) : error ? (
        <div className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-2xl border p-5 text-sm shadow-sm">
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
    </ParentPage>
  );
}

function SchoolSection({
  label,
  groups,
  locale,
}: Readonly<{ label: string; groups: SchoolGroup[]; locale: string }>) {
  return (
    <section className="space-y-3">
      <h2 className="text-[15px] font-medium text-gray-700">{label}</h2>
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
        <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-gray-100">
          <MotoConceptIcon concept="schools" size={22} />
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

  // The link carries school_subdomain, never school_slug: the target page
  // resolves tenant metadata via resolveTenant() and loads the form through the
  // parent enrollment endpoints, and both look the school up by subdomain. A
  // school whose slug differs from its subdomain would get an unusable link
  // (#1663).
  return (
    <Link
      href={`/parents/enroll/${encodeURIComponent(
        phase.school_subdomain,
      )}/${encodeURIComponent(phase.phase_id)}`}
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
            <span className="bg-moto-blue/10 text-moto-blue-strong rounded-full px-2 py-0.5 text-[11px] font-semibold">
              {audienceLabel}
            </span>
          )}
        </div>
        <p className="mt-1 text-sm text-gray-600">{range}</p>
        {phase.enrollment_close_at && (
          <p className="mt-1 inline-flex items-center gap-1.5 text-xs font-medium text-gray-500">
            <MotoConceptIcon concept="calendar" size={16} />
            {t("closesAt", {
              date: formatLocalizedDateTime(phase.enrollment_close_at, locale),
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
      <span className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-gray-100">
        <MotoConceptIcon concept="schools" size={22} />
      </span>
      <p className="mt-3 text-sm leading-6 text-gray-600">{t("empty")}</p>
    </div>
  );
}

function ParentEnrollSkeleton() {
  return (
    <div
      data-testid="parent-enroll-skeleton"
      className="space-y-4"
      aria-hidden="true"
    >
      <Skeleton className="ml-1 h-5 w-32" />
      {[0, 1].map((item) => (
        <div
          key={item}
          className="overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm"
        >
          <div className="flex items-center gap-3 border-b border-gray-100 p-4 sm:px-6">
            <Skeleton className="size-11 shrink-0 rounded-xl" />
            <Skeleton className="h-5 w-48 max-w-2/3" />
          </div>
          <div className="flex items-center gap-4 p-4 sm:px-6">
            <div className="min-w-0 flex-1 space-y-2">
              <div className="flex gap-2">
                <Skeleton className="h-4 w-36" />
                <Skeleton className="h-5 w-20 rounded-full" />
              </div>
              <Skeleton className="h-4 w-56 max-w-full" />
              <Skeleton className="h-3 w-40" />
            </div>
            <Skeleton className="size-5 shrink-0 rounded" />
          </div>
        </div>
      ))}
    </div>
  );
}

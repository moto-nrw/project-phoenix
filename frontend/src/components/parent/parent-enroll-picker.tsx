"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { StatusBadge } from "~/components/ui/status-badge";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { ListSkeleton } from "~/components/ui/page-skeletons";
import { ParentPage, ParentPageHeader } from "~/components/parent/parent-page";
import { useLocale, useTranslations } from "next-intl";
import { type EnrollablePhase, listEnrollableSchools } from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";
import { useApiLoadError } from "~/contexts/ToastContext";
import {
  formatLocalizedDate,
  formatLocalizedDateTime,
} from "~/lib/localized-date-format";
import { calendarConcept } from "~/lib/concepts/calendar";
import { schoolsConcept } from "~/lib/concepts/schools";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import { SectionCard } from "~/components/ui/section-card";

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
  // A failed load replaces the list (#2515), so it never reads as "no
  // phase open".
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();

  const load = useCallback(async () => {
    setLoading(true);
    clearLoadError();
    try {
      setPhases(await listEnrollableSchools());
    } catch (err) {
      logger.warn("parent_enrollable_schools_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showLoadError(err, {
        object: t("errorObject"),
        retry: () => void loadRef.current(),
      });
    } finally {
      setLoading(false);
    }
  }, [clearLoadError, showLoadError, t]);
  // The retry runs the latest load, not the one of the failed attempt.
  const loadRef = useRef(load);
  useLayoutEffect(() => {
    loadRef.current = load;
  });

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
    <ParentPage
      header={
        <ParentPageHeader
          kicker={t("kicker")}
          title={t("title")}
          description={t("description")}
        />
      }
      loading={
        loading && (
          <div data-testid="parent-enroll-skeleton">
            <ListSkeleton rows={4} avatar={false} />
          </div>
        )
      }
      error={loadError}
      empty={
        phases.length === 0
          ? {
              title: t("empty"),
              icon: <MotoConceptIcon concept={schoolsConcept} />,
            }
          : null
      }
    >
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
    <SectionCard
      title={group.schoolName}
      headingLevel={3}
      titleClassName="break-words"
      leading={<ConceptIconTile concept={schoolsConcept} size="page" />}
      flush
    >
      <ul className="divide-y divide-gray-100 border-t border-gray-100">
        {group.phases.map((phase) => (
          <li key={phase.phase_id}>
            <PhaseRow phase={phase} locale={locale} />
          </li>
        ))}
      </ul>
    </SectionCard>
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
      href={`/parents/anmeldung/${encodeURIComponent(
        phase.school_subdomain,
      )}/${encodeURIComponent(phase.phase_id)}`}
      aria-label={`${t("openPhase")}: ${phaseDisplayName(phase, locale)}`}
      className="group flex items-center gap-4 p-4 transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:px-5"
    >
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-sm font-semibold text-gray-900">
            {phaseDisplayName(phase, locale)}
          </span>
          <StatusBadge tone="gray" compact label={kindLabel} />
          {audienceLabel && (
            <StatusBadge tone="blue" compact wrap label={audienceLabel} />
          )}
        </div>
        <p className="mt-1 text-sm text-gray-600">{range}</p>
        {phase.enrollment_close_at && (
          <p className="mt-1 inline-flex items-center gap-1.5 text-xs font-medium text-gray-500">
            <MotoConceptIcon concept={calendarConcept} size={16} />
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

/** Phase name in the parent's language where the school translated it (#3377). */
function phaseDisplayName(phase: EnrollablePhase, locale: string): string {
  return phase.phase_name_translations?.[locale] ?? phase.phase_name;
}

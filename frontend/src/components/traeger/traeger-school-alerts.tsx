"use client";

// Auffälligkeiten im Dashboard des Trägers (#4675): eine neutrale Karte mit
// einer Zeile je Schule, an der gerade etwas eine Grenze des Trägers
// erreicht. Farbe tragen nur das Symbol, die Zahl im Titel und die Symbole
// der Hinweise; die Zeilen bleiben weiß (frontend-ui-kit.md, „Several
// notices of the same kind“). Die Grenzen stehen einmal über der Liste.

import { useId, useMemo, useState } from "react";
import { ChevronDown, ChevronUp, TriangleAlert } from "lucide-react";
import { CheckCircleIcon } from "@phosphor-icons/react/ssr";
import { Button, ButtonLink } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import type { FormErrorInput } from "~/components/ui/form-error";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { enrollmentsConcept } from "~/lib/concepts/enrollments";
import { sickConcept } from "~/lib/concepts/sick";
import { staffConcept } from "~/lib/concepts/staff";
import type { MotoConcept } from "~/lib/moto-concepts";
import {
  ALERT_LABELS,
  alertSentence,
  schoolCountLabel,
  thresholdsSentence,
} from "~/lib/traeger-alerts";
import type {
  TraegerAlertKind,
  TraegerSchoolAlerts,
  TraegerSchoolsAlerts,
} from "~/lib/traeger-api";
import { traegerPath } from "~/lib/traeger-url";

/** Rows shown before „N weitere anzeigen“, as in the other hint lists. */
const INITIAL_ROW_COUNT = 5;

/** The value of the school filter that shows every school. */
const ALL_SCHOOLS = "";

/** The settings area with the thresholds (`?bereich=` on the settings page). */
const ALERT_SETTINGS_PATH = "/traeger/einstellungen?bereich=dashboard";

const ALERT_CONCEPTS: Readonly<Record<TraegerAlertKind, MotoConcept>> = {
  staff_shortage: staffConcept,
  sick_children: sickConcept,
  open_enrollments: enrollmentsConcept,
};

/**
 * The card above the figures of every school. Hidden while the Träger has no
 * school; the table below says so. A load error without confirmed hints shows
 * the error with a retry, never an empty list: no hint must not read as
 * "nothing to see".
 */
export function TraegerSchoolAlertsCard({
  alerts,
  error,
  canChangeThresholds,
}: Readonly<{
  alerts: TraegerSchoolsAlerts | null;
  error: FormErrorInput | null;
  /** With the right „Einstellungen“ the link changes the thresholds. */
  canChangeThresholds: boolean;
}>) {
  const [schoolId, setSchoolId] = useState(ALL_SCHOOLS);
  const [expanded, setExpanded] = useState(false);
  const listId = useId();
  const affected = useMemo(
    () => (alerts?.schools ?? []).filter((school) => school.alerts.length > 0),
    [alerts],
  );
  const options = useMemo(
    () => [
      { value: ALL_SCHOOLS, label: "Alle Schulen" },
      ...(alerts?.schools ?? []).map((school) => ({
        value: school.id,
        label:
          school.alerts.length > 0
            ? `${school.name} (${school.alerts.length})`
            : school.name,
      })),
    ],
    [alerts],
  );

  if (error) {
    return <LoadErrorAlert error={error} actionLayout="responsive" />;
  }
  if (alerts?.schools.length === 0) return null;

  const loading = alerts === null;
  const selected = alerts?.schools.find((school) => school.id === schoolId);
  const rows = selected
    ? affected.filter((school) => school.id === selected.id)
    : affected;
  const hiddenCount = Math.max(0, rows.length - INITIAL_ROW_COUNT);
  const visible = expanded ? rows : rows.slice(0, INITIAL_ROW_COUNT);
  const quiet = !loading && affected.length === 0;

  return (
    <SectionCard
      title="Auffälligkeiten"
      testId="traeger-school-alerts"
      loading={loading}
      titleBadge={
        affected.length > 0 ? (
          <StatusBadge
            tone="orange"
            label={schoolCountLabel(affected.length)}
          />
        ) : undefined
      }
      description="Hier stehen die Schulen, an denen gerade etwas Ihre Grenzen erreicht."
      leading={
        quiet ? (
          <CheckCircleIcon
            weight="duotone"
            className="text-moto-green-strong mt-0.5 size-5 shrink-0"
            aria-hidden="true"
          />
        ) : (
          <TriangleAlert
            className="text-moto-orange-strong mt-0.5 size-5 shrink-0"
            aria-hidden="true"
          />
        )
      }
      actions={
        alerts && alerts.schools.length > 1 ? (
          <CustomSelect
            value={schoolId}
            options={options}
            onChange={(value) => {
              setSchoolId(value);
              setExpanded(false);
            }}
            ariaLabel="Auffälligkeiten nach Schule filtern"
            controlSize="compact"
            searchable={alerts.schools.length > 8}
            searchPlaceholder="Schule suchen"
            className="w-full sm:w-64"
          />
        ) : undefined
      }
    >
      {alerts ? (
        <div className="mb-3 flex flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between sm:gap-4">
          <p className="text-sm text-gray-600">
            {thresholdsSentence(alerts.thresholds)}
          </p>
          <ButtonLink
            href={traegerPath(ALERT_SETTINGS_PATH)}
            variant="link"
            size="md"
            className="shrink-0 self-start sm:self-auto"
          >
            {canChangeThresholds ? "Grenzen ändern" : "Grenzen ansehen"}
          </ButtonLink>
        </div>
      ) : null}
      {quiet ? (
        <p className="text-sm text-gray-900">
          Gerade ist an keiner Schule etwas auffällig.
        </p>
      ) : null}
      {!quiet && selected && rows.length === 0 ? (
        <p className="text-sm text-gray-900">
          An dieser Schule ist gerade nichts auffällig.
        </p>
      ) : null}
      {visible.length > 0 ? (
        <ResponsiveActionList id={listId}>
          {visible.map((school) => (
            <SchoolAlertsRow key={school.id} school={school} />
          ))}
        </ResponsiveActionList>
      ) : null}
      {hiddenCount > 0 ? (
        <Button
          type="button"
          variant="ghost"
          size="md"
          className="mt-2 w-full"
          aria-expanded={expanded}
          aria-controls={listId}
          onClick={() => setExpanded((current) => !current)}
        >
          {expanded ? (
            <>
              Weniger anzeigen
              <ChevronUp className="size-4" aria-hidden="true" />
            </>
          ) : (
            <>
              {hiddenCount} weitere anzeigen
              <ChevronDown className="size-4" aria-hidden="true" />
            </>
          )}
        </Button>
      ) : null}
    </SectionCard>
  );
}

/** One school: its name, one line per hint and the way into the school. */
function SchoolAlertsRow({
  school,
}: Readonly<{ school: TraegerSchoolAlerts }>) {
  return (
    <li className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center">
      <div className="min-w-0">
        <p className="text-sm font-semibold text-gray-900">
          {school.name}
          {school.city ? (
            <span className="font-normal text-gray-500"> · {school.city}</span>
          ) : null}
        </p>
        <ul className="mt-1.5 flex flex-col gap-1.5">
          {school.alerts.map((kind) => (
            <li key={kind} className="flex items-start gap-2 text-sm">
              <MotoConceptIcon
                concept={ALERT_CONCEPTS[kind]}
                className="mt-0.5 size-4 shrink-0"
                aria-hidden="true"
              />
              <span className="min-w-0 text-gray-600">
                <span className="font-medium text-gray-900">
                  {ALERT_LABELS[kind]}:
                </span>{" "}
                <span className="tabular-nums">
                  {alertSentence(kind, school)}
                </span>
              </span>
            </li>
          ))}
        </ul>
      </div>
      <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
        <ButtonLink
          href={traegerPath(`/traeger/schulen/${school.id}`)}
          variant="surface"
          size="compact"
          aria-label={`Zur Schule: ${school.name}`}
        >
          Zur Schule
        </ButtonLink>
      </div>
    </li>
  );
}

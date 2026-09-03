"use client";

// OGS-Hülle des Klassen-Tagesausnahme-Bausteins (#2962): liest und schreibt
// über die Kinder-Routen des OGS-Portals, meldet per Toast und holt den
// Blockbeginn für „Unterricht fällt aus“ aus dem Betreuungsplan. Der
// Baustein selbst liegt portal-neutral unter components/class-arrival, weil
// moto schule ihn seit #2970 mit eigener Datenquelle nutzt.

import { useMemo } from "react";
import {
  type ClassArrivalExceptionApi,
  ClassArrivalExceptionPanel as SharedClassArrivalExceptionPanel,
} from "~/components/class-arrival/class-arrival-exception-panel";
import { useToast } from "~/contexts/ToastContext";
import {
  deleteClassArrivalException,
  fetchClassArrivalExceptions,
  upsertClassArrivalException,
} from "~/lib/student-arrival-api";
import { timetableService } from "~/lib/timetable-api";
import { normalizeSchoolClass } from "~/lib/timetable-helpers";

interface ClassArrivalExceptionPanelProps {
  readonly schoolClass: string;
  /** "Klasse 4a", used in the confirmation toast. */
  readonly classLabel: string;
  readonly onChanged?: () => void;
}

interface ClassTargetTemplate {
  readonly targetSchoolClass?: string;
  readonly targets?: ReadonlyArray<{ readonly schoolClass?: string }>;
  readonly sourceSchoolClasses?: readonly string[];
}

function appliesToSchoolClass(
  template: ClassTargetTemplate,
  schoolClass: string,
): boolean {
  const classes = [
    template.targetSchoolClass,
    ...(template.targets?.map((target) => target.schoolClass) ?? []),
    ...(template.sourceSchoolClasses ?? []),
  ].filter((value): value is string => value !== undefined);
  return classes.some(
    (schoolClassTarget) =>
      normalizeSchoolClass(schoolClassTarget) ===
      normalizeSchoolClass(schoolClass),
  );
}

/**
 * Earliest planned block start of a date ("HH:MM") or null when the day has
 * no block. Cancelled blocks do not count: the class would arrive into
 * nothing. The school portal answers the same question server-side
 * (GET /school/class-day/arrival-exceptions/block-start).
 */
async function earliestBlockStart(
  schoolClass: string,
  isoDate: string,
): Promise<string | null> {
  const [week, templates] = await Promise.all([
    timetableService.getWeek(isoDate, isoDate),
    timetableService.getTemplates(),
  ]);
  const templatesByID = new Map(
    templates.templates.map((template) => [template.id, template]),
  );
  const starts = week.instances
    .filter((instance) => {
      if (instance.date !== isoDate || instance.status === "cancelled") {
        return false;
      }
      const template = instance.activityGroupId
        ? templatesByID.get(instance.activityGroupId)
        : undefined;
      return (
        template !== undefined && appliesToSchoolClass(template, schoolClass)
      );
    })
    .map((instance) => instance.startTime)
    .sort();
  return starts[0] ?? null;
}

const ogsApi: ClassArrivalExceptionApi = {
  list: fetchClassArrivalExceptions,
  upsert: upsertClassArrivalException,
  remove: deleteClassArrivalException,
  earliestBlockStart,
};

/**
 * One date on which a whole class arrives at a different time (#2962).
 * Everybody sees the list; the form appears only for people the school lets
 * set one (operations.class_arrival_exception_editors).
 */
export function ClassArrivalExceptionPanel({
  schoolClass,
  classLabel,
  onChanged,
}: ClassArrivalExceptionPanelProps) {
  const { success, error } = useToast();
  const notify = useMemo(() => ({ success, error }), [success, error]);

  return (
    <SharedClassArrivalExceptionPanel
      schoolClass={schoolClass}
      classLabel={classLabel}
      api={ogsApi}
      notify={notify}
      onChanged={onChanged}
      readOnlyHint="Ändern kann das die Koordination."
      originLabel={(exception) =>
        exception.origin === "school" ? "Eingetragen von der Schule" : null
      }
    />
  );
}

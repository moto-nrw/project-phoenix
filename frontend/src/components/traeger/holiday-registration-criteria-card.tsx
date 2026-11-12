"use client";

// Angaben für die Vergabe einer Ferienanmeldung (#4700): was die Eltern zu
// jedem Kriterium gesagt haben und was das Büro geprüft hat. Die Prüfung
// zählt vor der Angabe der Eltern; „Nicht geprüft“ nimmt sie zurück. Eine
// Prüfung ändert den Vorschlag für offene Plätze und die Reihenfolge der
// Warteliste. Die Karte wechselt mit „Prüfen“ an Ort und Stelle in den
// Bearbeiten-Zustand.

import { BadgeCheck } from "lucide-react";
import { useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { EditActions } from "~/components/ui/edit-actions";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { SectionLabel } from "~/components/ui/section-label";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { formatBerlinDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  checkHolidayRegistrationCriteria,
  type HolidayCriterionCheckValue,
  type HolidayRegistrationCriterion,
  type HolidayRegistrationDetail,
} from "~/lib/traeger-holiday-registrations-api";

const logger = createLogger({ component: "HolidayRegistrationCriteriaCard" });

const STATED_LABELS = {
  yes: "Ja",
  no: "Nein",
  unknown: "Keine Angabe",
} as const;

type CheckChoice = "none" | "yes" | "no";

const CHECK_ITEMS: readonly { value: CheckChoice; label: string }[] = [
  { value: "none", label: "Nicht geprüft" },
  { value: "yes", label: "Trifft zu" },
  { value: "no", label: "Trifft nicht zu" },
];

function criterionKey(criterion: HolidayRegistrationCriterion): string {
  return `${criterion.criterionId}:${criterion.childId ?? "family"}`;
}

function choiceOf(value: HolidayCriterionCheckValue): CheckChoice {
  return value ?? "none";
}

/** What counts for the allocation: the office's check, else the parents. */
function effectiveMet(criterion: HolidayRegistrationCriterion): {
  readonly label: string;
  readonly tone: "green" | "gray" | "outline";
} {
  const value = criterion.checked ?? criterion.stated;
  if (value === "yes") return { label: "Zählt", tone: "green" };
  if (value === "no") return { label: "Zählt nicht", tone: "gray" };
  return { label: "Ungeklärt", tone: "outline" };
}

/** The criteria of one group: the family or one child. */
interface CriteriaGroup {
  readonly key: string;
  readonly title: string;
  readonly criteria: readonly HolidayRegistrationCriterion[];
}

function groups(registration: HolidayRegistrationDetail): CriteriaGroup[] {
  const criteria = registration.criteria ?? [];
  const family = criteria.filter((criterion) => criterion.childId === null);
  const result: CriteriaGroup[] = [];
  if (family.length > 0) {
    result.push({ key: "family", title: "Familie", criteria: family });
  }
  for (const child of registration.children) {
    const own = criteria.filter((criterion) => criterion.childId === child.id);
    if (own.length > 0) {
      result.push({
        key: child.id,
        title: `${child.firstName} ${child.lastName}`,
        criteria: own,
      });
    }
  }
  return result;
}

function CriterionReadout({
  criterion,
}: Readonly<{ criterion: HolidayRegistrationCriterion }>) {
  const met = effectiveMet(criterion);
  return (
    <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
      <div className="min-w-0">
        <p className="text-sm font-medium text-gray-900">{criterion.name}</p>
        <p className="text-sm text-gray-600">{criterion.question}</p>
        <p className="mt-1 text-sm text-gray-700">
          Eltern: {STATED_LABELS[criterion.stated]}
          {" · "}
          {criterion.checked ? (
            <span className="inline-flex items-center gap-1">
              <BadgeCheck className="h-4 w-4" aria-hidden="true" />
              Büro:{" "}
              {criterion.checked === "yes" ? "trifft zu" : "trifft nicht zu"}
              {criterion.checkedAt
                ? `, am ${formatBerlinDate(criterion.checkedAt)}`
                : ""}
            </span>
          ) : (
            "Büro: nicht geprüft"
          )}
        </p>
      </div>
      <StatusBadge tone={met.tone} size="xs" label={met.label} />
    </div>
  );
}

export function HolidayRegistrationCriteriaCard({
  periodId,
  registration,
  onChanged,
}: Readonly<{
  periodId: string;
  registration: HolidayRegistrationDetail;
  onChanged: (next: HolidayRegistrationDetail) => void;
}>) {
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const toast = useToast();
  const [checks, setChecks] = useState<Record<string, CheckChoice> | null>(
    null,
  );
  const [saving, setSaving] = useState(false);
  const criteria = registration.criteria ?? [];
  if (criteria.length === 0) return null;
  const cancelled = registration.status === "cancelled";
  const editing = checks !== null;
  const shown = groups(registration);

  const startEditing = () =>
    setChecks(
      Object.fromEntries(
        criteria.map((criterion) => [
          criterionKey(criterion),
          choiceOf(criterion.checked),
        ]),
      ),
    );

  const changedChecks = criteria.filter(
    (criterion) =>
      checks !== null &&
      checks[criterionKey(criterion)] !== choiceOf(criterion.checked),
  );

  const save = async () => {
    if (!checks) return;
    formErrors.clear();
    setSaving(true);
    try {
      const next = await checkHolidayRegistrationCriteria(
        periodId,
        registration.id,
        changedChecks.map((criterion) => {
          const choice = checks[criterionKey(criterion)] ?? "none";
          return {
            criterionId: criterion.criterionId,
            childId: criterion.childId,
            checked: choice === "none" ? null : choice,
          };
        }),
      );
      setChecks(null);
      toast.success("Die Prüfung ist gespeichert.");
      onChanged(next);
    } catch (err) {
      logger.error("holiday_registration_criteria_check_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Prüfung" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <SectionCard
      title="Angaben für die Vergabe"
      description={
        cancelled
          ? "Die Anmeldung ist storniert. Die Angaben lassen sich nicht mehr prüfen."
          : "Was die Eltern sagen und was Ihr Büro geprüft hat. Die Prüfung zählt vor der Angabe der Eltern."
      }
      action={
        cancelled || editing ? undefined : (
          <Button
            type="button"
            variant="outline"
            size="md"
            aria-label="Angaben für die Vergabe prüfen"
            tooltip="Öffnet die Angaben, damit Ihr Büro sie als geprüft festhält."
            onClick={startEditing}
          >
            Prüfen
          </Button>
        )
      }
    >
      <form
        ref={formRef}
        noValidate
        aria-label="Angaben für die Vergabe"
        className="space-y-5"
        onSubmit={(event) => {
          event.preventDefault();
          void save();
        }}
      >
        <FormErrorAlert message={formErrors.error} />
        {shown.map((group) => (
          <div key={group.key} className="space-y-2">
            <SectionLabel as="h3">{group.title}</SectionLabel>
            <ul className="divide-y divide-gray-100">
              {group.criteria.map((criterion) => {
                const key = criterionKey(criterion);
                return (
                  <li key={key} className="py-3 first:pt-0 last:pb-0">
                    {editing ? (
                      <div className="space-y-2">
                        <div>
                          <p className="text-sm font-medium text-gray-900">
                            {criterion.name}
                          </p>
                          <p className="text-sm text-gray-600">
                            {criterion.question} Eltern:{" "}
                            {STATED_LABELS[criterion.stated]}.
                          </p>
                        </div>
                        <SegmentedControl
                          items={
                            saving
                              ? CHECK_ITEMS.map((item) => ({
                                  ...item,
                                  disabled: true,
                                }))
                              : CHECK_ITEMS
                          }
                          value={checks[key] ?? "none"}
                          wrap
                          fullWidth
                          ariaLabel={`Prüfung: ${criterion.name}${criterion.childId ? `, ${group.title}` : ""}`}
                          onChange={(choice) =>
                            setChecks((current) =>
                              current ? { ...current, [key]: choice } : current,
                            )
                          }
                        />
                      </div>
                    ) : (
                      <CriterionReadout criterion={criterion} />
                    )}
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
        {editing ? (
          <>
            <p className="text-sm text-gray-600">
              Eine Prüfung ändert den Vorschlag für offene Plätze und die
              Reihenfolge der Warteliste. Eltern bekommen keine Nachricht.
            </p>
            <EditActions
              onCancel={() => {
                formErrors.clear();
                setChecks(null);
              }}
              cancelDisabled={saving}
              saving={saving}
              disabled={changedChecks.length === 0}
              saveType="submit"
              saveLabel="Prüfung speichern"
              stretchOnPhone
            />
          </>
        ) : null}
      </form>
    </SectionCard>
  );
}

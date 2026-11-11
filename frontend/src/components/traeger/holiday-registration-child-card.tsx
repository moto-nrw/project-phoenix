"use client";

// Ein Kind einer Ferienanmeldung (#4663): Schule, gebuchte Wochen,
// Gesundheitsangaben und der Hinweis auf eine mögliche Doppelanmeldung. Hier
// bucht das Träger-Büro das Kind um und ändert seine Angaben (#5629).

import { type ReactNode, useState } from "react";
import { CustomSelect } from "~/components/ui/custom-select";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import {
  DataField,
  DataGrid,
  InfoSection,
} from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { childrenConcept } from "~/lib/concepts/children";
import { useApiFormError } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import type { HolidayPeriod } from "~/lib/traeger-api";
import {
  type HolidayRegistrationChild,
  type HolidayRegistrationDetail,
  type HolidayRegistrationHealth,
  rebookHolidayChild,
} from "~/lib/traeger-holiday-registrations-api";
import { traegerPath } from "~/lib/traeger-url";
import {
  type HolidayRebookingDraft,
  rebookingBookings,
  rebookingChanged,
  rebookingDraft,
} from "./holiday-rebooking-model";
import {
  bookedWeeksOverlap,
  groupParallelWeeks,
  weekGroupBooking,
  withWeekGroupBooking,
} from "~/lib/holiday-week-groups";
import {
  answeredQuestions,
  HolidayRegistrationAnswers,
} from "./holiday-registration-answers";
import {
  childSchoolLabel,
  formatHolidayWeek,
} from "./holiday-registration-format";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";

const logger = createLogger({ component: "HolidayRegistrationChildCard" });

export function HolidayRegistrationChildCard({
  period,
  registration,
  child,
  health,
  healthHidden = false,
  names,
  onChanged,
  onEdit,
  editor,
}: Readonly<{
  period: HolidayPeriod;
  registration: HolidayRegistrationDetail;
  child: HolidayRegistrationChild;
  health: HolidayRegistrationHealth | undefined;
  /**
   * The parents gave health information, but the account lacks the right
   * „Gesundheitsangaben sehen“ (#4722): the card says so instead.
   */
  healthHidden?: boolean;
  names: ReadonlyMap<string, string>;
  onChanged: (registration: HolidayRegistrationDetail) => void;
  /** Opens the edit of the child's details; absent, the card offers none. */
  onEdit?: () => void;
  /** The edit state of the child's details, in place of the card's body. */
  editor?: ReactNode;
}>) {
  const [draft, setDraft] = useState<HolidayRebookingDraft | null>(null);
  const [saving, setSaving] = useState(false);
  const saveErrors = useApiFormError();
  const cancelled = registration.status === "cancelled";
  // The child's answers to the questions of the Träger (#5198); those to
  // health questions stand in the Gesundheit box.
  const answers = registration.childAnswers[child.id] ?? {};
  const answered = answeredQuestions(registration.form.fields, true, answers);
  const otherAnswers = answered.filter((field) => !field.health);
  const healthAnswers = answered.filter((field) => field.health);
  const healthFields = Boolean(
    health && (health.allergies || health.medication || health.healthNotes),
  );

  const save = async () => {
    if (!draft) return;
    setSaving(true);
    saveErrors.clear();
    try {
      onChanged(
        await rebookHolidayChild(
          period.id,
          registration.id,
          child.id,
          rebookingBookings(period, draft),
        ),
      );
      setDraft(null);
    } catch (err) {
      logger.error("holiday_child_rebook_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void saveErrors.show(err, { object: "die Wochen des Kindes" });
    } finally {
      setSaving(false);
    }
  };

  const bookings = draft ? rebookingBookings(period, draft) : [];
  const rebook =
    draft || cancelled || registration.frozen ? null : (
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={() => setDraft(rebookingDraft(period, child))}
      >
        Umbuchen
      </Button>
    );
  const edit =
    draft || !onEdit ? null : (
      <Button
        type="button"
        variant="outline"
        size="md"
        aria-label={`Angaben von ${child.firstName} ${child.lastName} ändern`}
        onClick={onEdit}
      >
        Ändern
      </Button>
    );

  if (editor) return editor;

  return (
    <SectionCard
      leading={
        <ConceptIconTile concept={childrenConcept} className="max-sm:hidden" />
      }
      title={`${child.firstName} ${child.lastName}`}
      description={`Geboren am ${formatDate(child.birthDate)}, ${childSchoolLabel(child, names)}`}
      footer={
        draft ? (
          <EditActions
            onCancel={() => {
              saveErrors.clear();
              setDraft(null);
            }}
            onSave={() => void save()}
            saving={saving}
            disabled={
              bookings.length === 0 ||
              !rebookingChanged(child, bookings) ||
              (draft !== null && weeksOverlap(period, draft))
            }
          />
        ) : undefined
      }
      action={
        edit || rebook ? (
          <span className="flex flex-wrap gap-2">
            {rebook}
            {edit}
          </span>
        ) : undefined
      }
    >
      <div className="space-y-4">
        {child.duplicateRegistrationIds.length > 0 ? (
          <Alert
            type="warning"
            message={
              child.duplicateRegistrationIds.length === 1
                ? "Vielleicht doppelt angemeldet. Ein Kind mit gleichem Namen und Geburtsdatum steht auch in einer anderen Anmeldung."
                : `Vielleicht doppelt angemeldet. Ein Kind mit gleichem Namen und Geburtsdatum steht auch in ${child.duplicateRegistrationIds.length} anderen Anmeldungen.`
            }
            actionLayout="stacked"
            action={
              // No row gap: by finger each link is 44 px high already.
              // -ml-2.5 takes back the links' padding: their labels start
              // with the notice's text.
              <span className="-ml-2.5 flex flex-wrap gap-x-3">
                {child.duplicateRegistrationIds.map((id) => (
                  // A kit link button: 44 px by finger, in the notice's colour.
                  <ButtonLink
                    key={id}
                    href={traegerPath(
                      `/traeger/ferien/${period.id}/anmeldungen/${id}`,
                    )}
                    variant="link_tone"
                    size="compact"
                  >
                    Anmeldung Nr. {id} öffnen
                  </ButtonLink>
                ))}
              </span>
            }
          />
        ) : null}

        {draft ? (
          <RebookingEditor
            period={period}
            draft={draft}
            names={names}
            onChange={setDraft}
            error={saveErrors.error}
          />
        ) : (
          <InfoSection title="Gebuchte Wochen">
            <ul className="space-y-1 text-sm text-gray-900">
              {child.bookings.map((booking) => (
                <li key={booking.weekId}>
                  {formatHolidayWeek(booking.weekStartsOn, booking.weekEndsOn)}:{" "}
                  {booking.timeModelName},{" "}
                  {names.get(booking.hostSchoolId) ??
                    "Schule nicht mehr vorhanden"}
                </li>
              ))}
            </ul>
            {registration.frozen && !cancelled ? (
              <p className="mt-2 text-sm text-gray-500">
                Zum Umbuchen öffnen Sie die Anmeldung zuerst wieder.
              </p>
            ) : null}
          </InfoSection>
        )}

        {otherAnswers.length > 0 ? (
          <InfoSection title="Weitere Angaben">
            <HolidayRegistrationAnswers
              questions={otherAnswers}
              answers={answers}
            />
          </InfoSection>
        ) : null}

        {healthHidden ? (
          <InfoSection title="Gesundheit">
            <p className="text-sm text-gray-700">
              Gesundheitsangaben sieht nur die Rolle Verwaltung.
            </p>
          </InfoSection>
        ) : null}

        {healthFields || healthAnswers.length > 0 ? (
          <InfoSection title="Gesundheit">
            <div className="space-y-4">
              {health && healthFields ? (
                <DataGrid>
                  {health.allergies ? (
                    <DataField label="Allergien" fullWidth>
                      <span className="whitespace-pre-wrap">
                        {health.allergies}
                      </span>
                    </DataField>
                  ) : null}
                  {health.medication ? (
                    <DataField label="Medikamente" fullWidth>
                      <span className="whitespace-pre-wrap">
                        {health.medication}
                      </span>
                    </DataField>
                  ) : null}
                  {health.healthNotes ? (
                    <DataField label="Weitere Hinweise" fullWidth>
                      <span className="whitespace-pre-wrap">
                        {health.healthNotes}
                      </span>
                    </DataField>
                  ) : null}
                </DataGrid>
              ) : null}
              {/* The answers to health questions stand with the health
                  fields: whoever reads this box before the holiday week
                  sees an intolerance asked by the form too. */}
              {healthAnswers.length > 0 ? (
                <HolidayRegistrationAnswers
                  questions={healthAnswers}
                  answers={answers}
                  markHealth={false}
                />
              ) : null}
            </div>
          </InfoSection>
        ) : null}
      </div>
    </SectionCard>
  );
}

function weeksOverlap(
  period: HolidayPeriod,
  draft: HolidayRebookingDraft,
): boolean {
  return bookedWeeksOverlap(
    period.weeks,
    rebookingBookings(period, draft).map((booking) => booking.weekId),
  );
}

function RebookingEditor({
  period,
  draft,
  names,
  onChange,
  error,
}: Readonly<{
  period: HolidayPeriod;
  draft: HolidayRebookingDraft;
  names: ReadonlyMap<string, string>;
  onChange: (draft: HolidayRebookingDraft) => void;
  error: Parameters<typeof FormErrorAlert>[0]["message"];
}>) {
  return (
    <div className="space-y-3">
      <FormErrorAlert message={error} />
      <p className="text-sm text-gray-700">
        Wählen Sie je Woche ein Zeitmodell. Gibt es eine Woche an mehreren
        Schulen, wählen Sie auch die Schule. Mindestens eine Woche bleibt
        gebucht.
      </p>
      <ul className="space-y-3">
        {groupParallelWeeks(period.weeks).map((group) => {
          const schoolOf = (week: (typeof group)[number]) =>
            names.get(week.hostSchoolId) ?? "Schule nicht mehr vorhanden";
          const first = group[0]!;
          const range = formatHolidayWeek(first.startsOn, first.endsOn);
          const parallel = group.length > 1;
          const label = parallel ? range : `${range}, ${schoolOf(first)}`;
          return (
            <li
              key={first.id ?? range}
              className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between"
            >
              <span className="text-sm text-gray-900">{label}</span>
              <CustomSelect
                value={weekGroupBooking(draft, group)}
                wrapLabels={parallel}
                options={[
                  { value: "", label: "Nicht angemeldet" },
                  ...group.flatMap((week) =>
                    // An archived time model takes no new booking (#5994);
                    // the one a week is booked on stays selectable.
                    week.timeModels
                      .filter(
                        (model) =>
                          !model.archived ||
                          (week.id !== undefined &&
                            draft[week.id] === model.id),
                      )
                      .map((model) => ({
                        value: model.id ?? "",
                        label: parallel
                          ? `${schoolOf(week)}: ${model.name}`
                          : model.name,
                      })),
                  ),
                ]}
                onChange={(value) =>
                  onChange(withWeekGroupBooking(draft, group, value))
                }
                ariaLabel={`Zeitmodell ${label}`}
                className="w-full sm:w-56"
              />
            </li>
          );
        })}
      </ul>
      <FieldErrorHint
        id="holiday-rebooking-overlap"
        message={
          weeksOverlap(period, draft)
            ? "Zwei dieser Wochen überschneiden sich. Bitte wählen Sie nur eine davon."
            : null
        }
      />
    </div>
  );
}

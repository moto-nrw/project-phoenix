"use client";

// Ein Kind einer Ferienanmeldung (#4663): Schule, gebuchte Wochen,
// Gesundheitsangaben, der Hinweis auf eine mögliche Doppelanmeldung und auf
// eine Schule des Trägers, die das Kind kennt (#6683). Hier
// bucht das Träger-Büro das Kind um und ändert seine Angaben (#5629).

import { School } from "lucide-react";
import { type ReactNode, useState } from "react";
import { CustomSelect } from "~/components/ui/custom-select";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import {
  DataField,
  DataGrid,
  InfoSection,
  InfoText,
} from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import NavigationLink from "~/components/ui/navigation-link";
import { SectionCard } from "~/components/ui/section-card";
import { useApiFormError } from "~/contexts/ToastContext";
import { WeekdayToggleGroup } from "~/components/ui/weekday-toggle-group";
import { formatDate } from "~/lib/date-helpers";
import {
  holidayCareDays,
  holidayDayLabel,
  holidayDayList,
  holidayWeekdayShort,
} from "~/lib/holiday-days";
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
  type HolidayRebookingDays,
  type HolidayRebookingDraft,
  rebookingBookings,
  rebookingChanged,
  rebookingDays,
  rebookingDraft,
  rebookingMissesDays,
} from "./holiday-rebooking-model";
import { StatusBadge } from "~/components/ui/status-badge";
import { holidayPlacementStatus } from "./holiday-placement";
import { HolidayWishSection } from "./holiday-wish";
import { knownChildView } from "./holiday-known-child";
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

const logger = createLogger({ component: "HolidayRegistrationChildCard" });

export function HolidayRegistrationChildCard({
  period,
  registration,
  child,
  health,
  healthHidden = false,
  names,
  knownSchoolId,
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
  /** The school of the Träger that knows the child (#6683), if one does. */
  knownSchoolId?: string;
  onChanged: (registration: HolidayRegistrationDetail) => void;
  /** Opens the edit of the child's details; absent, the card offers none. */
  onEdit?: () => void;
  /** The edit state of the child's details, in place of the card's body. */
  editor?: ReactNode;
}>) {
  const [draft, setDraft] = useState<HolidayRebookingDraft | null>(null);
  const [draftDays, setDraftDays] = useState<HolidayRebookingDays>({});
  const [saving, setSaving] = useState(false);
  const saveErrors = useApiFormError();
  const cancelled = registration.status === "cancelled";

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
          rebookingBookings(period, draft, draftDays),
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

  const bookings = draft ? rebookingBookings(period, draft, draftDays) : [];
  const rebook =
    draft || cancelled || registration.frozen ? null : (
      <Button
        type="button"
        variant="outline"
        size="md"
        tooltip="Öffnet die gebuchten Wochen des Kindes zum Umbuchen."
        onClick={() => {
          setDraft(rebookingDraft(period, child));
          setDraftDays(rebookingDays(child));
        }}
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
        tooltip="Öffnet die Angaben des Kindes zum Bearbeiten."
        onClick={onEdit}
      >
        Ändern
      </Button>
    );

  if (editor) return editor;
  // A cancelled registration needs no check of the school.
  const known = cancelled ? null : knownChildView(child, knownSchoolId, names);
  const namedSchool =
    child.homeSchoolId === null
      ? `Angemeldet ist ${child.firstName} als Feriengast.`
      : `In der Anmeldung steht die Schule „${names.get(child.homeSchoolId) ?? "nicht mehr vorhanden"}“.`;

  return (
    <SectionCard
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
              rebookingMissesDays(bookings) ||
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
        {known?.sameSchool ? (
          <StatusBadge
            tone={known.tone}
            label={known.label}
            icon={School}
            title={known.title}
            wrap
          />
        ) : known ? (
          <Alert
            type="info"
            icon={<School aria-hidden className="size-5" />}
            message={`Die Schule „${known.schoolName}“ führt ein Kind mit gleichem Namen und Geburtsdatum. ${namedSchool} Bitte prüfen Sie die Schule.`}
            actionLayout="responsive"
            action={
              !draft && onEdit ? (
                <Button
                  type="button"
                  variant="ghost_tone"
                  size="compact"
                  aria-label={`Schule von ${child.firstName} ${child.lastName} ändern`}
                  tooltip="Öffnet die Angaben des Kindes, dort wählen Sie die Schule."
                  onClick={onEdit}
                >
                  Schule ändern
                </Button>
              ) : undefined
            }
          />
        ) : null}
        {child.duplicateRegistrationIds.length > 0 ? (
          <Alert
            type="warning"
            message="Vielleicht doppelt angemeldet. Ein Kind mit gleichem Namen und Geburtsdatum steht auch in einer anderen Anmeldung."
            actionLayout="stacked"
            action={
              <span className="flex flex-wrap gap-3">
                {child.duplicateRegistrationIds.map((id) => (
                  <NavigationLink
                    key={id}
                    href={traegerPath(
                      `/traeger/ferien/${period.id}/anmeldungen/${id}`,
                    )}
                    className="font-medium underline"
                  >
                    Anmeldung Nr. {id} öffnen
                  </NavigationLink>
                ))}
              </span>
            }
          />
        ) : null}

        {draft ? (
          <RebookingEditor
            period={period}
            draft={draft}
            days={draftDays}
            savedBookings={child.bookings}
            names={names}
            onChange={setDraft}
            onDaysChange={setDraftDays}
            error={saveErrors.error}
          />
        ) : (
          <InfoSection title="Gebuchte Wochen">
            <ul className="space-y-1 text-sm text-gray-900">
              {child.bookings.map((booking) => (
                <li
                  key={booking.weekId}
                  className="flex flex-wrap items-center gap-x-2 gap-y-1"
                >
                  <span>
                    {formatHolidayWeek(
                      booking.weekStartsOn,
                      booking.weekEndsOn,
                    )}
                    : {booking.timeModelName},{" "}
                    {names.get(booking.hostSchoolId) ??
                      "Schule nicht mehr vorhanden"}
                    {booking.days && booking.days.length > 0
                      ? ` · ${holidayDayList(booking.days)}`
                      : ""}
                  </span>
                  {cancelled ? null : (
                    <StatusBadge
                      {...holidayPlacementStatus(booking)}
                      size="xs"
                    />
                  )}
                </li>
              ))}
            </ul>
            {registration.frozen && !cancelled ? (
              <p className="mt-2 text-xs text-gray-500">
                Zum Umbuchen öffnen Sie die Anmeldung zuerst wieder.
              </p>
            ) : null}
          </InfoSection>
        )}

        <HolidayWishSection
          periodId={period.id}
          registrationId={registration.id}
          child={child}
          wishRule={period.holidayWishRule}
          cancelled={cancelled}
          onChanged={onChanged}
        />

        <ChildAnswers registration={registration} childId={child.id} />

        {healthHidden ? (
          <InfoSection title="Gesundheit">
            <InfoText>
              Gesundheitsangaben sieht nur die Rolle Verwaltung.
            </InfoText>
          </InfoSection>
        ) : null}

        {health &&
        (health.allergies || health.medication || health.healthNotes) ? (
          <InfoSection title="Gesundheit">
            <DataGrid>
              {health.allergies ? (
                <DataField label="Allergien" fullWidth>
                  <InfoText>{health.allergies}</InfoText>
                </DataField>
              ) : null}
              {health.medication ? (
                <DataField label="Medikamente" fullWidth>
                  <InfoText>{health.medication}</InfoText>
                </DataField>
              ) : null}
              {health.healthNotes ? (
                <DataField label="Weitere Hinweise" fullWidth>
                  <InfoText>{health.healthNotes}</InfoText>
                </DataField>
              ) : null}
            </DataGrid>
          </InfoSection>
        ) : null}
      </div>
    </SectionCard>
  );
}

/** The child's answers to the questions of the Träger (#5198). */
function ChildAnswers({
  registration,
  childId,
}: Readonly<{ registration: HolidayRegistrationDetail; childId: string }>) {
  const answers = registration.childAnswers[childId] ?? {};
  const questions = answeredQuestions(registration.form.fields, true, answers);
  if (questions.length === 0) return null;
  return (
    <InfoSection title="Weitere Angaben">
      <HolidayRegistrationAnswers questions={questions} answers={answers} />
    </InfoSection>
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
  days,
  savedBookings,
  names,
  onChange,
  onDaysChange,
  error,
}: Readonly<{
  period: HolidayPeriod;
  draft: HolidayRebookingDraft;
  days: HolidayRebookingDays;
  savedBookings: HolidayRegistrationChild["bookings"];
  names: ReadonlyMap<string, string>;
  onChange: (draft: HolidayRebookingDraft) => void;
  onDaysChange: (days: HolidayRebookingDays) => void;
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
          const chosen = weekGroupBooking(draft, group);
          const bookedWeek = group.find((week) =>
            week.timeModels.some((model) => model.id === chosen),
          );
          const byDays =
            bookedWeek?.bookingUnit === "day" && bookedWeek.id !== undefined;
          const weekDays =
            byDays && bookedWeek.id ? (days[bookedWeek.id] ?? []) : [];
          return (
            <li key={first.id ?? range} className="space-y-2">
              <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                <span className="text-sm text-gray-900">
                  {label}
                  {group
                    .filter(
                      (week) =>
                        week.bookingUnit !== "day" && week.noCareDays?.length,
                    )
                    .map((week) => (
                      <span
                        key={week.id ?? week.startsOn}
                        className="block text-xs text-gray-600"
                      >
                        {parallel ? `${schoolOf(week)}: ` : ""}Keine neue
                        Buchung, weil am {formatDate(week.noCareDays![0]!.date)}{" "}
                        keine Betreuung ist ({week.noCareDays![0]!.name}).
                      </span>
                    ))}
                </span>
                <CustomSelect
                  value={weekGroupBooking(draft, group)}
                  wrapLabels={parallel}
                  options={[
                    { value: "", label: "Nicht angemeldet" },
                    ...group.flatMap((week) =>
                      // A week with a day without care at its host school
                      // takes no new booking (#4702); a booked one stays. A
                      // week by days only leaves that day out (#4701).
                      (week.bookingUnit !== "day" &&
                      week.noCareDays?.length &&
                      !savedBookings.some(
                        (booking) => booking.weekId === week.id,
                      )
                        ? []
                        : week.timeModels
                      )
                        .filter(
                          (model) =>
                            !model.archived ||
                            savedBookings.some(
                              (booking) =>
                                booking.weekId === week.id &&
                                booking.timeModelId === model.id,
                            ),
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
              </div>
              {byDays && bookedWeek.id ? (
                <WeekdayToggleGroup
                  ariaLabel={`Tage ${label}`}
                  days={holidayCareDays(bookedWeek.startsOn, bookedWeek.endsOn)
                    .filter(
                      (day) =>
                        savedBookings.some(
                          (booking) =>
                            booking.weekId === bookedWeek.id &&
                            booking.days?.includes(day),
                        ) ||
                        !bookedWeek.noCareDays?.some(
                          (none) => none.date === day,
                        ),
                    )
                    .map((day) => ({
                      value: day,
                      label: holidayWeekdayShort(day),
                      ariaLabel: holidayDayLabel(day),
                    }))}
                  value={weekDays}
                  onToggle={(day) => {
                    const weekId = bookedWeek.id ?? "";
                    onDaysChange({
                      ...days,
                      [weekId]: weekDays.includes(day)
                        ? weekDays.filter((current) => current !== day)
                        : [...weekDays, day].sort(),
                    });
                  }}
                  hint={
                    weekDays.length === 0
                      ? "Bitte wählen Sie mindestens einen Tag."
                      : `Gewählt: ${holidayDayList(weekDays)}`
                  }
                />
              ) : null}
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

"use client";

// Ein Kind einer Ferienanmeldung (#4663): Schule, gebuchte Wochen,
// Gesundheitsangaben, der Hinweis auf eine mögliche Doppelanmeldung und auf
// eine Schule des Trägers, die das Kind kennt (#6683). Hier
// bucht das Träger-Büro das Kind um und ändert seine Angaben (#5629). Zu
// einer Woche mit Mittagessen gehört, ob das Kind dort isst (#5631).

import { School } from "lucide-react";
import { type ReactNode, useState } from "react";
import { Checkbox } from "~/components/ui/checkbox";
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
import { formatEuro, lunchCountText } from "./contribution-format";
import {
  type HolidayRebookingDays,
  type HolidayRebookingDraft,
  type HolidayRebookingLunch,
  rebookingBookings,
  rebookingChanged,
  rebookingDays,
  rebookingDraft,
  rebookingLunch,
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
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";

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
  const [draftLunch, setDraftLunch] = useState<HolidayRebookingLunch>({});
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
          rebookingBookings(period, draft, draftDays, draftLunch),
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

  const bookings = draft
    ? rebookingBookings(period, draft, draftDays, draftLunch)
    : [];
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
          setDraftLunch(rebookingLunch(child));
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
      leading={<ConceptIconTile concept={childrenConcept} />}
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
                    tooltip="Öffnet die andere Anmeldung mit dem gleichen Kind."
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
            days={draftDays}
            lunch={draftLunch}
            savedBookings={child.bookings}
            confirmed={registration.status === "confirmed"}
            names={names}
            onChange={setDraft}
            onDaysChange={setDraftDays}
            onLunchChange={setDraftLunch}
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
                    {booking.lunch ? ` · ${bookedLunchText(booking)}` : ""}
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
              <p className="mt-2 text-sm text-gray-500">
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

/** "mit Mittagessen (5 Essen zu je 4,30 €)" of a booking with lunch. */
function bookedLunchText(
  booking: HolidayRegistrationChild["bookings"][number],
): string {
  const lunches = lunchCountText({
    amountCents: booking.lunchPriceCents ?? 0,
    bookedDays: booking.lunchDays ?? 0,
  });
  return lunches ? `mit Mittagessen (${lunches})` : "mit Mittagessen";
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
  lunch,
  savedBookings,
  confirmed,
  names,
  onChange,
  onDaysChange,
  onLunchChange,
  error,
}: Readonly<{
  period: HolidayPeriod;
  draft: HolidayRebookingDraft;
  days: HolidayRebookingDays;
  lunch: HolidayRebookingLunch;
  savedBookings: HolidayRegistrationChild["bookings"];
  /** The registration is confirmed: its contribution is frozen. */
  confirmed: boolean;
  names: ReadonlyMap<string, string>;
  onChange: (draft: HolidayRebookingDraft) => void;
  onDaysChange: (days: HolidayRebookingDays) => void;
  onLunchChange: (lunch: HolidayRebookingLunch) => void;
  error: Parameters<typeof FormErrorAlert>[0]["message"];
}>) {
  // A confirmed contribution stays as confirmed (ADR 0115): a lunch added
  // or taken off now reaches the invoice only through a correction.
  const lunchChanged = period.weeks.some(
    (week) =>
      week.id !== undefined &&
      week.lunchPriceCents != null &&
      Boolean(draft[week.id]) &&
      (lunch[week.id] ?? false) !==
        (savedBookings.find((booking) => booking.weekId === week.id)?.lunch ??
          false),
  );
  return (
    <div className="space-y-3">
      <FormErrorAlert message={error} />
      {confirmed && lunchChanged ? (
        <Alert
          type="info"
          size="compact"
          message="Die Anmeldung ist bestätigt. Der Beitrag bleibt, wie er bestätigt wurde. Das geänderte Mittagessen berechnen Sie als Korrektur in der Abrechnung."
        />
      ) : null}
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
              {bookedWeek?.id && bookedWeek.lunchPriceCents != null ? (
                <label
                  htmlFor={`holiday-rebooking-lunch-${bookedWeek.id}`}
                  className="flex items-center gap-3 text-sm text-gray-900 sm:justify-end"
                >
                  <Checkbox
                    id={`holiday-rebooking-lunch-${bookedWeek.id}`}
                    checked={lunch[bookedWeek.id] ?? false}
                    onChange={(event) =>
                      onLunchChange({
                        ...lunch,
                        [bookedWeek.id ?? ""]: event.target.checked,
                      })
                    }
                  />
                  <span>
                    Mit Mittagessen
                    <span className="text-gray-600">
                      {" "}
                      ({formatEuro(bookedWeek.lunchPriceCents)} je Essen)
                    </span>
                  </span>
                </label>
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

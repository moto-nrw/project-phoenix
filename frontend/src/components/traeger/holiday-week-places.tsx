"use client";

// Plätze und Warteliste einer Ferienwoche (#4699). Plätze gehen in der
// Reihenfolge, in der die Anmeldungen eingegangen sind. Ist die Woche voll,
// wartet ein Kind auf der Warteliste der Woche und rückt automatisch nach,
// sobald ein Platz frei wird. Seine Eltern bekommen dann eine E-Mail.
// Eine tageweise gebuchte Woche hat ihre Plätze an jedem Tag (#4701): ein
// Kind bekommt seinen Platz, wenn alle seine Tage frei sind.

import { Hourglass } from "lucide-react";
import { useRef, useState } from "react";
import {
  HOLIDAY_REGISTRATION_STATUS_LABELS,
  HOLIDAY_REGISTRATION_STATUS_TONES,
  formatHolidayWeek,
} from "~/components/traeger/holiday-registration-format";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { EditActions } from "~/components/ui/edit-actions";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { FramedList } from "~/components/ui/framed-list";
import { IconTile } from "~/components/ui/icon-tile";
import { ListRowLink } from "~/components/ui/list-row-link";
import { NumberStepper } from "~/components/ui/number-stepper";
import { ProgressBar } from "~/components/ui/progress-bar";
import { SectionCard } from "~/components/ui/section-card";
import { SectionLabel } from "~/components/ui/section-label";
import { StatCard } from "~/components/ui/stat-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { SwitchRow } from "~/components/ui/switch";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { formatBerlinDate } from "~/lib/date-helpers";
import {
  holidayDayDate,
  holidayDayList,
  holidayDayPromotions,
  holidayWeekdayLong,
  holidayWeekdayShort,
} from "~/lib/holiday-days";
import {
  type HolidayWeekPlaces,
  setHolidayWeekCapacity,
} from "~/lib/traeger-holiday-registrations-api";
import { traegerPath } from "~/lib/traeger-url";

/** Höchste Zahl an Plätzen einer Woche, wie im Backend. */
const HOLIDAY_WEEK_CAPACITY_MAX = 10000;

/** Wie voll eine Woche ist, in Worten und als Plakette. */
export function holidayWeekFill(week: HolidayWeekPlaces): {
  readonly label: string;
  readonly tone: "green" | "warning" | "gray";
} {
  if (week.capacity === null) {
    return { label: "Ohne Grenze", tone: "gray" };
  }
  if (week.bookingUnit === "day") {
    const capacity = week.capacity;
    const days = week.days ?? [];
    const full = days.filter((day) => day.placed >= capacity).length;
    if (days.length > 0 && full === days.length) {
      return { label: "Alle Tage voll", tone: "warning" };
    }
    if (full > 0) {
      return {
        label: full === 1 ? "1 Tag voll" : `${full} Tage voll`,
        tone: "warning",
      };
    }
    return { label: "Plätze frei", tone: "green" };
  }
  const free = week.capacity - week.placed;
  if (free <= 0) {
    return { label: "Voll", tone: "warning" };
  }
  return {
    label: free === 1 ? "1 Platz frei" : `${free} Plätze frei`,
    tone: "green",
  };
}

function waitingLabel(count: number): string {
  return count === 1 ? "1 Kind wartet" : `${count} Kinder warten`;
}

/**
 * Eine Woche mit Belegung und Warteliste. „Plätze ändern“ schaltet die Karte
 * selbst in den Bearbeiten-Zustand; die Plätze werden in ihr festgelegt.
 */
export function HolidayWeekPlacesCard({
  periodId,
  week,
  hostSchoolName,
  editing,
  onEdit,
  onCancel,
  onSaved,
}: Readonly<{
  periodId: string;
  week: HolidayWeekPlaces;
  hostSchoolName: string;
  editing: boolean;
  onEdit: () => void;
  onCancel: () => void;
  onSaved: (movedUp: number) => void;
}>) {
  const fill = holidayWeekFill(week);
  const title = formatHolidayWeek(week.startsOn, week.endsOn);
  return (
    <SectionCard
      title={title}
      titleBadge={<StatusBadge label={fill.label} tone={fill.tone} size="xs" />}
      description={hostSchoolName}
      action={
        editing ? undefined : (
          <Button
            type="button"
            variant="outline"
            size="compact"
            onClick={onEdit}
            aria-label={`Plätze für die Woche ${title} ändern`}
          >
            Plätze ändern
          </Button>
        )
      }
    >
      {editing ? (
        <HolidayCapacityEditor
          periodId={periodId}
          week={week}
          onCancel={onCancel}
          onSaved={onSaved}
        />
      ) : (
        <HolidayWeekPlacesReadout periodId={periodId} week={week} />
      )}
    </SectionCard>
  );
}

/**
 * Die Belegung jedes Tages einer tageweise gebuchten Woche: ein Feld je Tag
 * mit vergebenen Plätzen und Balken. Ein voller Tag ist orange.
 */
function HolidayDayPlacesGrid({ week }: Readonly<{ week: HolidayWeekPlaces }>) {
  const capacity = week.capacity;
  return (
    <div className="space-y-2">
      <SectionLabel as="h3">
        {capacity === null
          ? "Angemeldete Kinder je Tag"
          : `Plätze je Tag · ${capacity} ${capacity === 1 ? "Platz" : "Plätze"} an jedem Tag`}
      </SectionLabel>
      <ul className="grid grid-cols-2 gap-2 min-[420px]:grid-cols-3 sm:grid-cols-5">
        {(week.days ?? []).map((day) => {
          const full = capacity !== null && day.placed >= capacity;
          const label =
            capacity === null
              ? `${holidayWeekdayLong(day.date)}, ${holidayDayDate(day.date)}: ${day.placed} angemeldet`
              : `${holidayWeekdayLong(day.date)}, ${holidayDayDate(day.date)}: ${day.placed} von ${capacity} Plätzen vergeben${full ? ", voll" : ""}`;
          return (
            <li key={day.date} aria-label={label}>
              <StatCard
                label={`${holidayWeekdayShort(day.date)} ${holidayDayDate(day.date)}`}
                value={
                  capacity === null
                    ? day.placed
                    : `${day.placed} von ${capacity}`
                }
                hint={
                  capacity === null
                    ? "Kinder angemeldet"
                    : full
                      ? "Voll"
                      : "Plätze vergeben"
                }
                tone={full ? "orange" : "green"}
                progressPct={
                  capacity === null
                    ? undefined
                    : Math.min(100, (day.placed / Math.max(capacity, 1)) * 100)
                }
              />
            </li>
          );
        })}
      </ul>
    </div>
  );
}

function HolidayWeekPlacesReadout({
  periodId,
  week,
}: Readonly<{ periodId: string; week: HolidayWeekPlaces }>) {
  return (
    <div className="space-y-5">
      {week.bookingUnit === "day" ? (
        <HolidayDayPlacesGrid week={week} />
      ) : week.capacity === null ? (
        <p className="text-sm text-gray-700">
          <span className="font-semibold text-gray-900">{week.placed}</span>{" "}
          {week.placed === 1 ? "Kind ist" : "Kinder sind"} angemeldet. Es gibt
          keine Grenze.
        </p>
      ) : (
        <div className="space-y-2">
          <p className="text-sm text-gray-700">
            <span className="font-semibold text-gray-900">{week.placed}</span>{" "}
            von {week.capacity} Plätzen vergeben
          </p>
          <ProgressBar
            value={Math.min(week.placed, week.capacity)}
            max={Math.max(week.capacity, 1)}
            label={`${week.placed} von ${week.capacity} Plätzen vergeben`}
            tone={week.placed >= week.capacity ? "orange" : "green"}
            size="md"
          />
        </div>
      )}
      {week.waitlist.length === 0 ? (
        <p className="text-sm text-gray-500">
          Niemand steht auf der Warteliste.
        </p>
      ) : (
        <div className="space-y-2">
          <SectionLabel as="h3">
            Warteliste · {waitingLabel(week.waitlist.length)}
          </SectionLabel>
          <FramedList maxHeight="md">
            {week.waitlist.map((entry) => (
              <ListRowLink
                key={`${entry.registrationId}-${entry.childId}`}
                href={traegerPath(
                  `/traeger/ferien/${periodId}/anmeldungen/${entry.registrationId}`,
                )}
                icon={
                  <IconTile
                    icon={
                      <span className="text-sm font-semibold tabular-nums">
                        {entry.position}
                      </span>
                    }
                    size="sm"
                    shape="round"
                    tone="orange"
                  />
                }
                title={`${entry.childFirstName} ${entry.childLastName}`}
                subtitle={`Eingegangen am ${formatBerlinDate(entry.submittedAt)} · ${entry.timeModelName}${(entry.days ?? []).length > 0 ? ` · ${holidayDayList(entry.days ?? [])}` : ""}`}
                badge={
                  <StatusBadge
                    label={
                      HOLIDAY_REGISTRATION_STATUS_LABELS[
                        entry.registrationStatus
                      ]
                    }
                    tone={
                      HOLIDAY_REGISTRATION_STATUS_TONES[
                        entry.registrationStatus
                      ]
                    }
                    size="xs"
                  />
                }
                aria-label={`Platz ${entry.position} der Warteliste: ${entry.childFirstName} ${entry.childLastName}, Anmeldung öffnen`}
              />
            ))}
          </FramedList>
        </div>
      )}
    </div>
  );
}

/** Die Plätze einer Woche festlegen: eine Zahl oder ohne Grenze. */
function HolidayCapacityEditor({
  periodId,
  week,
  onCancel,
  onSaved,
}: Readonly<{
  periodId: string;
  week: HolidayWeekPlaces;
  onCancel: () => void;
  onSaved: (movedUp: number) => void;
}>) {
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const toast = useToast();
  const [limited, setLimited] = useState(week.capacity !== null);
  const [value, setValue] = useState(
    String(week.capacity ?? Math.max(week.placed, 1)),
  );
  const [saving, setSaving] = useState(false);
  const capacity = Number.parseInt(value, 10);
  const valid =
    !limited ||
    (Number.isInteger(capacity) &&
      capacity >= 0 &&
      capacity <= HOLIDAY_WEEK_CAPACITY_MAX);
  const waiting = week.waitlist.length;
  const placed = week.placed;

  async function submit() {
    if (!valid) {
      formErrors.invalid("Bitte geben Sie eine Zahl von 0 bis 10.000 ein.", {
        capacity: "Bitte eine Zahl von 0 bis 10.000.",
      });
      return;
    }
    setSaving(true);
    formErrors.clear();
    try {
      const change = await setHolidayWeekCapacity(
        periodId,
        week.weekId,
        limited ? capacity : null,
      );
      const moved = change.movedUp.length;
      toast.success(
        moved === 0
          ? "Die Plätze sind gespeichert."
          : `Die Plätze sind gespeichert. ${moved === 1 ? "1 Anmeldung ist" : `${moved} Anmeldungen sind`} nachgerückt.`,
      );
      onSaved(moved);
    } catch (err) {
      void formErrors.show(err, { object: "die Plätze" });
    } finally {
      setSaving(false);
    }
  }

  const byDays = week.bookingUnit === "day";
  const moving = byDays
    ? holidayDayPromotions(
        limited ? (valid ? capacity : null) : null,
        new Map((week.days ?? []).map((day) => [day.date, day.placed])),
        week.waitlist.map((entry) => entry.days ?? []),
      )
    : limited
      ? Math.min(waiting, Math.max(0, (valid ? capacity : placed) - placed))
      : waiting;
  const mostPlaced = byDays
    ? Math.max(0, ...(week.days ?? []).map((day) => day.placed))
    : placed;
  return (
    <form
      ref={formRef}
      noValidate
      aria-label="Plätze dieser Woche"
      className="space-y-5"
      onSubmit={(event) => {
        event.preventDefault();
        void submit();
      }}
    >
      <FormErrorAlert message={formErrors.error} />
      <SwitchRow
        framed
        label="Plätze begrenzen"
        description="Ohne Grenze bekommt jedes angemeldete Kind einen Platz."
        checked={limited}
        onCheckedChange={setLimited}
      />
      {limited ? (
        <div className="space-y-2">
          <SectionLabel as="p">
            {byDays ? "Plätze an jedem Tag" : "Plätze in dieser Woche"}
          </SectionLabel>
          <NumberStepper
            id="holiday-week-capacity"
            name="capacity"
            inputMode="numeric"
            value={value}
            onChange={(event) =>
              setValue(event.target.value.replace(/\D/g, ""))
            }
            onStep={(delta) =>
              setValue(
                String(
                  Math.min(
                    HOLIDAY_WEEK_CAPACITY_MAX,
                    Math.max(
                      0,
                      (Number.isNaN(capacity) ? 0 : capacity) + delta,
                    ),
                  ),
                ),
              )
            }
            decrementLabel="Einen Platz weniger"
            incrementLabel="Einen Platz mehr"
            canDecrement={capacity > 0}
            canIncrement={capacity < HOLIDAY_WEEK_CAPACITY_MAX}
            aria-label={
              byDays ? "Plätze an jedem Tag" : "Plätze in dieser Woche"
            }
            error={formErrors.fieldError("capacity")}
          />
          <p className="text-sm text-gray-600">
            {byDays
              ? "Die Zahl gilt für jeden Tag, für alle Zeitmodelle zusammen. Ein Kind bekommt seinen Platz, wenn alle seine Tage frei sind."
              : "Die Plätze gelten für alle Zeitmodelle dieser Woche zusammen."}
          </p>
        </div>
      ) : null}
      {moving > 0 ? (
        <Alert
          type="info"
          message={
            moving === 1
              ? "1 Kind rückt beim Speichern von der Warteliste nach. Seine Eltern bekommen eine E-Mail."
              : `${moving} Kinder rücken beim Speichern von der Warteliste nach. Ihre Eltern bekommen eine E-Mail.`
          }
        />
      ) : null}
      {limited && valid && capacity < mostPlaced ? (
        <Alert
          type="warning"
          message={
            byDays
              ? `An einem Tag haben schon ${mostPlaced} Kinder einen Platz. Sie behalten ihn. Neue Anmeldungen für diesen Tag kommen auf die Warteliste.`
              : `${placed} Kinder haben schon einen Platz. Sie behalten ihn. Neue Anmeldungen kommen auf die Warteliste.`
          }
        />
      ) : null}
      <EditActions
        onCancel={onCancel}
        saveType="submit"
        saving={saving}
        cancelDisabled={saving}
        disabled={!valid}
        saveLabel="Plätze speichern"
        stretchOnPhone
      />
    </form>
  );
}

/** Kurzer Hinweis für die Statuszeile, wenn Kinder warten. */
export function WaitlistFact({ count }: Readonly<{ count: number }>) {
  if (count === 0) return null;
  return (
    <span className="inline-flex items-center gap-1.5">
      <Hourglass className="h-4 w-4" aria-hidden="true" />
      {waitingLabel(count)} auf einen Platz
    </span>
  );
}

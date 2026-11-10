"use client";

// Plätze und Warteliste einer Ferienwoche (#4699). Plätze gehen in der
// Reihenfolge, in der die Anmeldungen eingegangen sind. Ist die Woche voll,
// wartet ein Kind auf der Warteliste der Woche und rückt automatisch nach,
// sobald ein Platz frei wird. Seine Eltern bekommen dann eine E-Mail.

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
import { StatusBadge } from "~/components/ui/status-badge";
import { SwitchRow } from "~/components/ui/switch";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { formatBerlinDate } from "~/lib/date-helpers";
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

function HolidayWeekPlacesReadout({
  periodId,
  week,
}: Readonly<{ periodId: string; week: HolidayWeekPlaces }>) {
  return (
    <div className="space-y-5">
      {week.capacity === null ? (
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
                subtitle={`Eingegangen am ${formatBerlinDate(entry.submittedAt)} · ${entry.timeModelName}`}
                badge={
                  <span className="inline-flex flex-wrap gap-1">
                    {entry.deferred ? (
                      <StatusBadge
                        label="Ohne Ferienwunsch"
                        tone="gray"
                        size="xs"
                      />
                    ) : null}
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
                  </span>
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

  const moving = limited
    ? Math.min(waiting, Math.max(0, (valid ? capacity : placed) - placed))
    : waiting;
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
          <SectionLabel as="p">Plätze in dieser Woche</SectionLabel>
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
            aria-label="Plätze in dieser Woche"
            error={formErrors.fieldError("capacity")}
          />
          <p className="text-sm text-gray-600">
            Die Plätze gelten für alle Zeitmodelle dieser Woche zusammen.
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
      {limited && valid && capacity < placed ? (
        <Alert
          type="warning"
          message={`${placed} Kinder haben schon einen Platz. Sie behalten ihn. Neue Anmeldungen kommen auf die Warteliste.`}
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

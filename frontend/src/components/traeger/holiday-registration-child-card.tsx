"use client";

// Ein Kind einer Ferienanmeldung (#4663): Schule, gebuchte Wochen,
// Gesundheitsangaben und der Hinweis auf eine mögliche Doppelanmeldung. Hier
// bucht das Träger-Büro das Kind um.

import { useState } from "react";
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
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import NavigationLink from "~/components/ui/navigation-link";
import { SectionCard } from "~/components/ui/section-card";
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
  childSchoolLabel,
  formatHolidayWeek,
} from "./holiday-registration-format";

const logger = createLogger({ component: "HolidayRegistrationChildCard" });

export function HolidayRegistrationChildCard({
  period,
  registration,
  child,
  health,
  names,
  onChanged,
}: Readonly<{
  period: HolidayPeriod;
  registration: HolidayRegistrationDetail;
  child: HolidayRegistrationChild;
  health: HolidayRegistrationHealth | undefined;
  names: ReadonlyMap<string, string>;
  onChanged: (registration: HolidayRegistrationDetail) => void;
}>) {
  const [draft, setDraft] = useState<HolidayRebookingDraft | null>(null);
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
  const action =
    draft || cancelled || registration.frozen ? undefined : (
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={() => setDraft(rebookingDraft(period, child))}
      >
        Umbuchen
      </Button>
    );

  return (
    <SectionCard
      title={`${child.firstName} ${child.lastName}`}
      description={`Geboren am ${formatDate(child.birthDate)} · ${childSchoolLabel(child, names)}`}
      action={action}
    >
      <div className="space-y-4">
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
            names={names}
            onChange={setDraft}
            error={saveErrors.error}
            footer={
              <EditActions
                onCancel={() => {
                  saveErrors.clear();
                  setDraft(null);
                }}
                onSave={() => void save()}
                saving={saving}
                disabled={
                  bookings.length === 0 || !rebookingChanged(child, bookings)
                }
              />
            }
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
              <p className="mt-2 text-xs text-gray-500">
                Zum Umbuchen öffnen Sie die Anmeldung zuerst wieder.
              </p>
            ) : null}
          </InfoSection>
        )}

        {health ? (
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

function RebookingEditor({
  period,
  draft,
  names,
  onChange,
  error,
  footer,
}: Readonly<{
  period: HolidayPeriod;
  draft: HolidayRebookingDraft;
  names: ReadonlyMap<string, string>;
  onChange: (draft: HolidayRebookingDraft) => void;
  error: Parameters<typeof FormErrorAlert>[0]["message"];
  footer: React.ReactNode;
}>) {
  return (
    <div className="space-y-3">
      <FormErrorAlert message={error} />
      <p className="text-sm text-gray-700">
        Wählen Sie je Woche ein Zeitmodell. Mindestens eine Woche bleibt
        gebucht.
      </p>
      <ul className="space-y-3">
        {period.weeks.map((week) => {
          const weekId = week.id ?? "";
          const label = `${formatHolidayWeek(week.startsOn, week.endsOn)}, ${names.get(week.hostSchoolId) ?? "Schule nicht mehr vorhanden"}`;
          return (
            <li
              key={weekId}
              className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between"
            >
              <span className="text-sm text-gray-900">{label}</span>
              <CustomSelect
                value={draft[weekId] ?? ""}
                options={[
                  { value: "", label: "Nicht angemeldet" },
                  ...week.timeModels.map((model) => ({
                    value: model.id ?? "",
                    label: model.name,
                  })),
                ]}
                onChange={(value) => onChange({ ...draft, [weekId]: value })}
                ariaLabel={`Zeitmodell ${label}`}
                className="w-full sm:w-56"
              />
            </li>
          );
        })}
      </ul>
      {footer}
    </div>
  );
}

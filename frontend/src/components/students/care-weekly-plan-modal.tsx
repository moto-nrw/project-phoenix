"use client";

import { useState } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "~/components/ui/button";
import { useFormError } from "~/components/ui/form-error";
import { FormModal } from "~/components/ui/form-modal";
import { useToast } from "~/contexts/ToastContext";
import type { CareDaysSource, SchoolPeriod } from "~/lib/student-arrival-api";
import type { ArrivalScheduleFormEntry } from "~/lib/arrival-schedule-helpers";
import type {
  BulkPickupScheduleFormData,
  PickupScheduleFormData,
} from "~/lib/pickup-schedule-helpers";
import {
  CareWeeklyPlanGrid,
  toWeeklySubmit,
  useWeeklyPlanDraft,
  validateWeeklyRows,
} from "./care-weekly-plan-editor";

interface CareWeeklyPlanModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly careDaysSource: CareDaysSource;
  /** Lessons the arrival can be picked by; none keeps the selection hidden. */
  readonly schoolPeriods?: readonly SchoolPeriod[];
  readonly initialArrivalSchedules: ArrivalScheduleFormEntry[];
  readonly initialPickupSchedules: PickupScheduleFormData[];
  readonly onSubmit: (data: {
    arrivalSchedules: ArrivalScheduleFormEntry[];
    pickupData: BulkPickupScheduleFormData;
  }) => Promise<void>;
  /** Success toast shown after `onSubmit` resolves. */
  readonly successMessage: string;
}

/**
 * Die `FormModal`-Hülle des Wochenplans für den Anlege-Assistenten: das Kind
 * gibt es noch nicht, der Plan wird nur zwischengespeichert und mit dem Kind
 * zusammen angelegt. Deshalb heißt der Dialog „Wochenplan festlegen“ und nicht
 * „bearbeiten“ (Bauart 2 Regel 3, #3119). Das Raster selbst ist dasselbe wie
 * im Bearbeiten-Zustand der Kinddetailseite (`care-weekly-plan-editor.tsx`).
 */
export function CareWeeklyPlanModal(props: CareWeeklyPlanModalProps) {
  // Der Entwurf lebt im inneren Formular, damit jedes Öffnen frisch aus den
  // zwischengespeicherten Zeiten startet.
  if (!props.isOpen) return null;
  return <CareWeeklyPlanModalForm {...props} />;
}

function CareWeeklyPlanModalForm({
  onClose,
  careDaysSource,
  schoolPeriods,
  initialArrivalSchedules,
  initialPickupSchedules,
  onSubmit,
  successMessage,
}: CareWeeklyPlanModalProps) {
  const toast = useToast();
  const draft = useWeeklyPlanDraft(
    initialArrivalSchedules,
    initialPickupSchedules,
  );
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useFormError();

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setError(null);

    const invalid = validateWeeklyRows(draft.rows);
    if (invalid) {
      setError(invalid);
      return;
    }

    const { arrivalSchedules, pickupSchedules } = toWeeklySubmit(
      draft.rows,
      careDaysSource === "bookings",
    );
    setIsSubmitting(true);
    try {
      await onSubmit({
        arrivalSchedules,
        pickupData: { schedules: pickupSchedules },
      });
      toast.success(successMessage);
      onClose();
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Wochenplan konnte nicht übernommen werden",
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  const footer = (
    <>
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={onClose}
        disabled={isSubmitting}
      >
        Abbrechen
      </Button>
      <Button
        type="submit"
        size="md"
        form="care-weekly-plan-form"
        className="gap-2"
        disabled={isSubmitting}
      >
        {isSubmitting ? (
          <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
        ) : null}
        Übernehmen
      </Button>
    </>
  );

  return (
    <FormModal
      isOpen
      onClose={onClose}
      title="Wochenplan festlegen"
      footer={footer}
      size="xl"
      mobilePosition="bottom"
      isBackdropDismissDisabled
      error={error}
    >
      <form
        id="care-weekly-plan-form"
        noValidate
        onSubmit={handleSubmit}
        className="space-y-4"
      >
        <p className="text-sm leading-6 text-gray-600">
          {careDaysSource === "bookings" ? (
            <>
              Die Betreuungstage kommen aus den Buchungen. Abholzeiten können
              Sie nur an gebuchten Tagen eintragen.
            </>
          ) : (
            <>
              Wählen Sie die Betreuungstage. Ohne eigene Zeit gilt die
              Klassenzeit.
            </>
          )}
        </p>
        <CareWeeklyPlanGrid
          draft={draft}
          careDaysSource={careDaysSource}
          removals={[]}
          disabled={isSubmitting}
          pickupNeedsCareDay
          schoolPeriods={schoolPeriods}
        />
      </form>
    </FormModal>
  );
}

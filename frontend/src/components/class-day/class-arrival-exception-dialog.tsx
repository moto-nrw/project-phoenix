"use client";

// Dialog der Klassenansicht in moto schule (#2970): die Lehrkraft trägt für
// eine ihrer Klassen die Tagesausnahme ein, die die OGS in der Kindersuche
// setzt (#2962). Der Baustein ist derselbe, die Datenquelle sind die
// /school-Routen, und Rückmeldungen stehen im Dialog statt als Toast: das
// Schul-Portal hat keine Toast-Leiste.

import { useCallback, useMemo, useState } from "react";
import {
  type ClassArrivalExceptionApi,
  ClassArrivalExceptionPanel,
} from "~/components/class-arrival/class-arrival-exception-panel";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { FormModal } from "~/components/ui/form-modal";
import {
  deleteClassArrivalExceptionSchool,
  fetchClassArrivalExceptionsSchool,
  fetchClassBlockStartSchool,
  upsertClassArrivalExceptionSchool,
} from "~/lib/school-class-day-api";
import { schoolClassLabel } from "~/lib/school-class-label";

const schoolApi: ClassArrivalExceptionApi = {
  list: fetchClassArrivalExceptionsSchool,
  upsert: upsertClassArrivalExceptionSchool,
  remove: deleteClassArrivalExceptionSchool,
  earliestBlockStart: fetchClassBlockStartSchool,
};

interface Feedback {
  readonly type: "success" | "error";
  readonly message: string;
}

export interface ClassArrivalExceptionDialogProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly schoolClass: string;
  /** Der angezeigte Tag als Vorbelegung des Datums. */
  readonly defaultDate: Date | null;
  /** Wird nach jedem Speichern oder Entfernen gerufen. */
  readonly onChanged?: () => void;
}

export function ClassArrivalExceptionDialog({
  isOpen,
  onClose,
  schoolClass,
  defaultDate,
  onChanged,
}: ClassArrivalExceptionDialogProps) {
  const [feedback, setFeedback] = useState<Feedback | null>(null);
  const notify = useMemo(
    () => ({
      success: (message: string) => setFeedback({ type: "success", message }),
      error: (message: string) => setFeedback({ type: "error", message }),
    }),
    [],
  );
  const close = useCallback(() => {
    setFeedback(null);
    onClose();
  }, [onClose]);

  return (
    <FormModal
      isOpen={isOpen}
      onClose={close}
      title={`Ankunftszeit an einem Tag für ${schoolClassLabel(schoolClass)}`}
      size="md"
      mobilePosition="bottom"
      footer={
        <div className="flex items-center justify-end gap-2 p-4">
          <Button type="button" variant="outline" size="md" onClick={close}>
            Schließen
          </Button>
        </div>
      }
    >
      <div className="space-y-4 p-4">
        <p className="text-sm text-gray-600">
          Die OGS sieht die neue Zeit sofort. Sie gilt nur für Kinder mit
          Betreuung an diesem Tag.
        </p>
        {feedback ? (
          <Alert type={feedback.type} message={feedback.message} />
        ) : null}
        {isOpen ? (
          <ClassArrivalExceptionPanel
            schoolClass={schoolClass}
            classLabel={schoolClassLabel(schoolClass)}
            api={schoolApi}
            notify={notify}
            onChanged={onChanged}
            defaultDate={defaultDate}
            originLabel={(exception) =>
              exception.origin === "school" ? null : "Eingetragen von der OGS"
            }
          />
        ) : null}
      </div>
    </FormModal>
  );
}

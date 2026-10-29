"use client";

import { useRef, useState } from "react";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { useApiFormError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { studentAccidentReportsService } from "~/lib/student-accident-reports-api";
import { useTenantRouter } from "~/lib/tenant-router";
import {
  AccidentReportFormFields,
  accidentReportDraftErrors,
  emptyAccidentReportDraft,
  fieldsFromDraft,
  type AccidentReportDraft,
} from "./accident-report-form";

const logger = createLogger({ component: "AccidentReportCreateModal" });

/**
 * „Unfall erfassen“ (#4771): records a new accident report of the child.
 * With `correctsReportId` the same dialog records the corrected version of
 * a signed report, prefilled with its content; the signed one stays.
 */
export function AccidentReportCreateModal({
  isOpen,
  onClose,
  studentId,
  correctsReportId,
  initialDraft,
  onCreated,
  openCreated = false,
}: Readonly<{
  isOpen: boolean;
  onClose: () => void;
  studentId: string;
  correctsReportId?: string;
  initialDraft?: AccidentReportDraft;
  onCreated: (reportId: string) => void;
  /** Opens the page of the new report after saving. */
  openCreated?: boolean;
}>) {
  const router = useTenantRouter();
  const [draft, setDraft] = useState<AccidentReportDraft>(
    () => initialDraft ?? emptyAccidentReportDraft(),
  );
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formError = useApiFormError(formRef);

  const close = () => {
    if (saving) return;
    formError.clear();
    setDraft(initialDraft ?? emptyAccidentReportDraft());
    onClose();
  };

  const save = async () => {
    const errors = accidentReportDraftErrors(draft);
    if (Object.keys(errors).length > 0) {
      formError.invalid("Bitte füllen Sie die markierten Felder aus.", errors);
      return;
    }
    setSaving(true);
    formError.clear();
    try {
      const fields = fieldsFromDraft(draft);
      const id = correctsReportId
        ? await studentAccidentReportsService.correct(
            studentId,
            correctsReportId,
            fields,
          )
        : await studentAccidentReportsService.create(studentId, fields);
      setDraft(emptyAccidentReportDraft());
      onCreated(id);
      if (openCreated) {
        router.push(`/students/${studentId}/unfallanzeigen/${id}`);
      }
    } catch (err) {
      logger.error("accident_report_create_failed", {
        student_id: studentId,
        error: err instanceof Error ? err.message : String(err),
      });
      await formError.show(err, { object: "die Unfallanzeige" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={close}
      title={correctsReportId ? "Korrektur anlegen" : "Unfall erfassen"}
      error={formError.error}
      closeDisabled={saving}
      isBackdropDismissDisabled
      footer={
        <EditActions
          onCancel={close}
          onSave={() => void save()}
          saving={saving}
          saveLabel={correctsReportId ? "Korrektur speichern" : "Speichern"}
        />
      }
    >
      <form
        ref={formRef}
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
        className="space-y-4"
      >
        <p className="text-sm text-gray-600">
          {correctsReportId
            ? "Die unterschriebene Fassung bleibt erhalten. Die Korrektur muss neu unterschrieben werden."
            : "Danach unterschreibt die Leitung die Unfallanzeige. Das PDF können Sie an die Unfallkasse weitergeben."}
        </p>
        <AccidentReportFormFields
          draft={draft}
          onChange={setDraft}
          fieldError={formError.fieldError}
          disabled={saving}
        />
      </form>
    </FormModal>
  );
}

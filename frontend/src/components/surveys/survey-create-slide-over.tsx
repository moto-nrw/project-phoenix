"use client";

/**
 * „Umfrage anlegen“ (#4781): ein Slide-over, weil das Formular mit
 * Empfängerwahl und Fragen breiter ist, als ein Modal trägt (BAUARTEN-SPEC
 * Bauart 2 Regel 4). Gespeichert wird ein Entwurf; danach führt die Seite
 * auf die Objektansicht, wo er veröffentlicht oder weiter bearbeitet wird.
 */

import { useRef, useState } from "react";

import { Button } from "~/components/ui/button";
import {
  SlideOver,
  SlideOverBody,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { useParentTargetLookups } from "~/components/announcements/parent-target-picker";
import { useApiFormError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  emptySurveyDraft,
  hasEditorErrors,
  surveyInputFromDraft,
  validateSurveyDraft,
  type SurveyEditorDraft,
  type SurveyEditorErrors,
} from "~/lib/survey-editor";
import { createSurvey } from "~/lib/surveys-api";
import type { Survey } from "~/lib/surveys-api";
import { SurveyEditorFields } from "./survey-editor-fields";

const logger = createLogger({ component: "SurveyCreate" });

export const SURVEY_FORM_CHECK_MESSAGE =
  "Bitte prüfen Sie die markierten Angaben.";

export function SurveyCreateSlideOver({
  onClose,
  onCreated,
}: Readonly<{
  onClose: () => void;
  onCreated: (survey: Survey) => void;
}>) {
  const [draft, setDraft] = useState<SurveyEditorDraft>(emptySurveyDraft);
  const [errors, setErrors] = useState<SurveyEditorErrors | null>(null);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);
  const lookups = useParentTargetLookups(draft.audience === "parents");

  const change = (next: SurveyEditorDraft) => {
    setDraft(next);
    // Nach dem ersten Versuch prüft das Formular mit, damit ein behobener
    // Fehler sofort verschwindet.
    if (errors) setErrors(validateSurveyDraft(next));
  };

  const saveDraft = async () => {
    const found = validateSurveyDraft(draft);
    setErrors(found);
    formErrors.clear();
    if (hasEditorErrors(found)) {
      formErrors.invalid(SURVEY_FORM_CHECK_MESSAGE);
      return;
    }
    setSaving(true);
    try {
      const created = await createSurvey(surveyInputFromDraft(draft));
      onCreated(created);
    } catch (err: unknown) {
      logger.error("survey_create_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, {
        object: "die Umfrage",
        retry: () => void saveDraft(),
      });
    } finally {
      setSaving(false);
    }
  };

  return (
    <SlideOver
      open
      onOpenChange={(open) => {
        if (!open && !saving) onClose();
      }}
    >
      <SlideOverContent widthClass="sm:w-[760px]" isBackdropDismissDisabled>
        <SlideOverHeader className="flex-row items-start justify-between gap-3">
          <div className="min-w-0">
            <SlideOverTitle>Neue Umfrage</SlideOverTitle>
          </div>
          <SlideOverCloseButton />
        </SlideOverHeader>
        <SlideOverBody error={formErrors.error}>
          <div ref={formRef}>
            <SurveyEditorFields
              draft={draft}
              onChange={change}
              errors={errors}
              disabled={saving}
              {...lookups}
            />
          </div>
        </SlideOverBody>
        <SlideOverFooter className="flex-row flex-wrap justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            disabled={saving}
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => void saveDraft()}
            isLoading={saving}
            loadingText="Wird gespeichert…"
          >
            Als Entwurf speichern
          </Button>
        </SlideOverFooter>
      </SlideOverContent>
    </SlideOver>
  );
}

"use client";

/**
 * „Umfrage anlegen“ (#4781): ein Slide-over, weil das Formular mit
 * Empfängerwahl und Fragen breiter ist, als ein Modal trägt (BAUARTEN-SPEC
 * Bauart 2 Regel 4). Gespeichert wird ein Entwurf; danach führt die Seite
 * auf die Objektansicht, wo er veröffentlicht oder weiter bearbeitet wird.
 */

import { useLayoutEffect, useRef, useState } from "react";

import { Button } from "~/components/ui/button";
import {
  SlideOver,
  SlideOverBody,
  SlideOverCancelButton,
  SlideOverContent,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
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
  // Locks the panel while the draft is created (#6593).
  const save = useFormModalPending();
  const saving = save.pending;
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);
  // „Wiederholen“ speichert den jetzt sichtbaren Entwurf, nicht den vom
  // Fehlversuch (#6503).
  const latestSaveDraft = useRef<() => Promise<void>>(async () => undefined);
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
    await save.run(async (op) => {
      try {
        const created = await createSurvey(surveyInputFromDraft(draft));
        // The page closes the panel and opens the new survey.
        if (op.isCurrent()) onCreated(created);
      } catch (err: unknown) {
        logger.error("survey_create_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        // Protokolliert; eine späte Ablehnung zeigt kein neuerer Vorgang.
        if (!op.isCurrent()) return;
        void formErrors.show(err, {
          object: "die Umfrage",
          retry: () => void latestSaveDraft.current(),
        });
      }
    });
  };
  useLayoutEffect(() => {
    latestSaveDraft.current = saveDraft;
  });

  return (
    <SlideOver
      open
      pending={save}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <SlideOverContent size="lg" isBackdropDismissDisabled>
        <SlideOverHeader>
          <SlideOverTitle>Neue Umfrage</SlideOverTitle>
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
        <SlideOverFooter>
          <SlideOverCancelButton>Abbrechen</SlideOverCancelButton>
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

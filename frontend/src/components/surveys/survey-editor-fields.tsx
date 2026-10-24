"use client";

/**
 * Die Felder einer Umfrage (#4781): Titel, Einleitung, Zielgruppe, anonym,
 * Teilnahme bis und die Fragen. Dasselbe Bauteil steht im Slide-over zum
 * Anlegen und im Bearbeiten-Zustand der Objektseite; den Fuß mit den
 * Knöpfen bringt die jeweilige Hülle mit.
 */

import { useState } from "react";
import { ArrowDown, ArrowUp, Plus, Trash2, X } from "lucide-react";

import { ParentTargetPicker } from "~/components/announcements/parent-target-picker";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { DatePicker } from "~/components/ui/date-picker";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import type { FormError } from "~/components/ui/form-error";
import { Input } from "~/components/ui/input";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import type { OverflowMenuItem } from "~/components/ui/page-header/OverflowMenu";
import { SegmentedControl } from "~/components/ui/segmented-control";
import type { SegmentedControlItem } from "~/components/ui/segmented-control";
import { Textarea } from "~/components/ui/textarea";
import { TimeField } from "~/components/ui/time-field";
import type { Group } from "~/lib/api";
import type { Activity } from "~/lib/activity-helpers";
import type { SurveyQuestionKind } from "~/lib/survey-form";
import {
  SURVEY_MAX_OPTIONS,
  SURVEY_MAX_QUESTIONS,
  SURVEY_OPTION_MAX_LENGTH,
  SURVEY_PROMPT_MAX_LENGTH,
  SURVEY_QUESTION_KIND_LABEL,
  SURVEY_TITLE_MAX_LENGTH,
  changeQuestionKind,
  moveItem,
  newQuestionDraft,
  type SurveyEditorDraft,
  type SurveyEditorErrors,
  type SurveyQuestionDraft,
} from "~/lib/survey-editor";
import type { SurveyAudience } from "~/lib/surveys-api";
import { SURVEY_MIN_RESPONSES } from "./survey-questionnaire";

const AUDIENCE_ITEMS: readonly SegmentedControlItem<SurveyAudience>[] = [
  { value: "parents", label: "Eltern" },
  { value: "staff", label: "Mitarbeitende" },
];

const KIND_ITEMS: readonly SegmentedControlItem<SurveyQuestionKind>[] = (
  ["single", "multiple", "scale", "text"] as const
).map((kind) => ({ value: kind, label: SURVEY_QUESTION_KIND_LABEL[kind] }));

const SECTION_TITLE =
  "text-xs font-semibold tracking-wide text-gray-500 uppercase";

export interface SurveyEditorFieldsProps {
  readonly draft: SurveyEditorDraft;
  readonly onChange: (next: SurveyEditorDraft) => void;
  /** Erst nach dem ersten Speichern-Versuch gesetzt. */
  readonly errors: SurveyEditorErrors | null;
  readonly groups: Group[];
  readonly activities: Activity[];
  readonly schoolClasses: string[];
  readonly lookupLoadError: FormError | null;
  readonly disabled?: boolean;
}

export function SurveyEditorFields({
  draft,
  onChange,
  errors,
  groups,
  activities,
  schoolClasses,
  lookupLoadError,
  disabled = false,
}: SurveyEditorFieldsProps) {
  const [studentNames, setStudentNames] = useState<Record<string, string>>({});
  const update = (patch: Partial<SurveyEditorDraft>) =>
    onChange({ ...draft, ...patch });
  const updateQuestion = (index: number, next: SurveyQuestionDraft) =>
    update({
      questions: draft.questions.map((question, i) =>
        i === index ? next : question,
      ),
    });

  return (
    <fieldset disabled={disabled} className="space-y-8">
      <section className="space-y-4">
        <Input
          label="Titel"
          name="title"
          value={draft.title}
          maxLength={SURVEY_TITLE_MAX_LENGTH}
          error={errors?.title}
          onChange={(event) => update({ title: event.target.value })}
          placeholder="z. B. Wie zufrieden sind Sie mit der OGS?"
        />
        <Textarea
          label="Einleitung (optional)"
          name="intro"
          rows={3}
          maxLength={2000}
          value={draft.intro}
          onChange={(event) => update({ intro: event.target.value })}
          placeholder="Wofür ist die Umfrage? Was passiert mit den Antworten?"
        />
      </section>

      <section className="space-y-4">
        <h3 className={SECTION_TITLE}>Wer soll antworten?</h3>
        <SegmentedControl
          items={AUDIENCE_ITEMS}
          value={draft.audience}
          onChange={(audience) => update({ audience })}
          ariaLabel="Wer soll antworten?"
          variant="joined"
        />
        {draft.audience === "parents" ? (
          <>
            <ParentTargetPicker
              targets={[...draft.targets]}
              groups={groups}
              activities={activities}
              schoolClasses={schoolClasses}
              studentNames={studentNames}
              onChange={(targets) => update({ targets })}
              onSetStudentName={(id, name) =>
                setStudentNames((prev) => ({ ...prev, [id]: name }))
              }
              kindLabel="diese Umfrage"
              heading="Welche Eltern sollen antworten?"
              allowPendingEnrollment={false}
              lookupLoadError={lookupLoadError}
            />
            <FieldErrorHint
              id="survey-targets-error"
              message={errors?.targets}
            />
            <ChoiceTile className="items-start gap-3 px-4 py-3">
              <Checkbox
                checked={draft.sendEmail}
                onChange={(event) =>
                  update({ sendEmail: event.target.checked })
                }
              />
              <span className="text-sm text-gray-800">
                <span className="block font-medium">
                  Zusätzlich per E-Mail benachrichtigen
                </span>
                <span className="block text-gray-500">
                  Beim Veröffentlichen bekommen die Eltern eine E-Mail mit dem
                  Hinweis auf die Umfrage.
                </span>
              </span>
            </ChoiceTile>
          </>
        ) : (
          <>
            <p className="text-sm text-gray-600">
              Alle Mitarbeitenden der OGS können antworten. Sie finden die
              Umfrage in moto unter „Umfragen“. Lehrkräfte, die nur das
              Schulportal nutzen, bekommen sie nicht.
            </p>
            <Alert
              type="info"
              announce="off"
              title="Vorher klären: Betriebsrat oder Personalrat"
              message="Eine Befragung der Beschäftigten kann mitbestimmungspflichtig sein. Bitte klären Sie das vor dem Veröffentlichen mit Ihrem Betriebsrat oder Personalrat. moto prüft das nicht."
            />
          </>
        )}
      </section>

      <section className="space-y-4">
        <h3 className={SECTION_TITLE}>Anonym und Frist</h3>
        <ChoiceTile
          selected={draft.anonymous}
          className="items-start gap-3 px-4 py-3"
        >
          <Checkbox
            checked={draft.anonymous}
            onChange={(event) => update({ anonymous: event.target.checked })}
          />
          <span className="text-sm text-gray-800">
            <span className="block font-medium">Anonym</span>
            <span className="block text-gray-500">
              {`Niemand sieht, wer was geantwortet hat. Die Auswertung erscheint erst nach dem Ende und ab ${SURVEY_MIN_RESPONSES} Antworten. Nach dem Veröffentlichen lässt sich das nicht mehr ändern.`}
            </span>
          </span>
        </ChoiceTile>
        <div className="grid gap-3 sm:grid-cols-2">
          <div>
            <span className="mb-1 block text-sm font-medium text-gray-700">
              Teilnahme bis
            </span>
            <DatePicker
              value={draft.closesDay}
              onChange={(closesDay) => update({ closesDay })}
              placeholder="Datum wählen"
              name="closes_at"
              invalid={Boolean(errors?.closesAt)}
              ariaDescribedBy={
                errors?.closesAt ? "survey-closes-at-error" : undefined
              }
              ariaLabel="Teilnahme bis, Datum"
            />
          </div>
          <TimeField
            label="Uhrzeit"
            value={draft.closesTime}
            onChange={(closesTime) => update({ closesTime })}
            hint="Uhrzeit im Format 18:00"
            placeholder="18:00"
            invalid={Boolean(errors?.closesAt)}
          />
        </div>
        <FieldErrorHint
          id="survey-closes-at-error"
          message={errors?.closesAt}
        />
      </section>

      <section className="space-y-4">
        <div>
          <h3 className={SECTION_TITLE}>Fragen</h3>
          <p className="mt-1 text-sm text-gray-600">
            {`Bis zu ${SURVEY_MAX_QUESTIONS} Fragen. Die Skala geht von 1 (trifft gar nicht zu) bis 5 (trifft voll zu).`}
          </p>
        </div>
        <ol className="space-y-4">
          {draft.questions.map((question, index) => (
            <QuestionEditor
              key={question.key}
              question={question}
              index={index}
              count={draft.questions.length}
              errors={errors?.perQuestion[question.key] ?? null}
              onChange={(next) => updateQuestion(index, next)}
              onMove={(to) =>
                update({ questions: moveItem(draft.questions, index, to) })
              }
              onRemove={() =>
                update({
                  questions: draft.questions.filter((_, i) => i !== index),
                })
              }
            />
          ))}
        </ol>
        <FieldErrorHint
          id="survey-questions-error"
          message={errors?.questions}
        />
        {draft.questions.length < SURVEY_MAX_QUESTIONS && (
          <Button
            type="button"
            variant="outline"
            size="md"
            className="gap-1.5"
            onClick={() =>
              update({ questions: [...draft.questions, newQuestionDraft()] })
            }
          >
            <Plus className="h-4 w-4" aria-hidden />
            Frage hinzufügen
          </Button>
        )}
      </section>
    </fieldset>
  );
}

function QuestionEditor({
  question,
  index,
  count,
  errors,
  onChange,
  onMove,
  onRemove,
}: Readonly<{
  question: SurveyQuestionDraft;
  index: number;
  count: number;
  errors: { prompt?: string; options?: string } | null;
  onChange: (next: SurveyQuestionDraft) => void;
  onMove: (to: number) => void;
  onRemove: () => void;
}>) {
  const number = index + 1;
  const choice = question.kind === "single" || question.kind === "multiple";
  const optionsErrorId = `survey-question-${question.key}-options-error`;
  const menuItems: OverflowMenuItem[] = [
    ...(index > 0
      ? [
          {
            label: "Nach oben",
            icon: <ArrowUp className="size-4" aria-hidden />,
            onClick: () => onMove(index - 1),
          },
        ]
      : []),
    ...(index < count - 1
      ? [
          {
            label: "Nach unten",
            icon: <ArrowDown className="size-4" aria-hidden />,
            onClick: () => onMove(index + 1),
          },
        ]
      : []),
    ...(count > 1
      ? [
          {
            label: "Frage entfernen",
            icon: <Trash2 className="size-4" aria-hidden />,
            destructive: true,
            onClick: onRemove,
          },
        ]
      : []),
  ];

  const setOption = (optionIndex: number, value: string) =>
    onChange({
      ...question,
      options: question.options.map((option, i) =>
        i === optionIndex ? value : option,
      ),
    });

  return (
    <li className="moto-content-surface space-y-4 rounded-xl border p-4">
      <div className="flex items-start gap-2">
        <div className="min-w-0 flex-1">
          <Input
            label={`Frage ${number}`}
            name={`survey-question-${number}`}
            value={question.prompt}
            maxLength={SURVEY_PROMPT_MAX_LENGTH}
            error={errors?.prompt}
            onChange={(event) =>
              onChange({ ...question, prompt: event.target.value })
            }
            placeholder={
              question.kind === "scale"
                ? "z. B. Mein Kind fühlt sich in der OGS wohl."
                : "z. B. Was sollen wir beibehalten?"
            }
          />
        </div>
        {menuItems.length > 0 && (
          <div className="pt-7">
            <OverflowMenu
              items={menuItems}
              ariaLabel={`Aktionen für Frage ${number}`}
            />
          </div>
        )}
      </div>

      <SegmentedControl
        items={KIND_ITEMS}
        value={question.kind}
        onChange={(kind) => onChange(changeQuestionKind(question, kind))}
        ariaLabel={`Art der Frage ${number}`}
        variant="joined"
      />

      {question.kind === "scale" && (
        <p className="text-sm text-gray-600">
          Wer antwortet, wählt einen Wert von 1 (trifft gar nicht zu) bis 5
          (trifft voll zu). Formulieren Sie die Frage deshalb als Aussage.
        </p>
      )}
      {question.kind === "text" && (
        <p className="text-sm text-gray-600">
          Wer antwortet, schreibt einen eigenen Text. In der Auswertung stehen
          die Texte ohne Namen und ohne Uhrzeit.
        </p>
      )}

      {choice && (
        <div>
          <span className="mb-1.5 block text-sm font-medium text-gray-700">
            Antworten
          </span>
          <ul className="space-y-2">
            {question.options.map((option, optionIndex) => (
              // Antworten haben vor dem Speichern keine ID; die Liste ist kurz
              // und wird an Ort und Stelle bearbeitet.
              // eslint-disable-next-line react/no-array-index-key
              <li
                key={optionIndex}
                className="inline-flex w-full items-center gap-2 rounded-lg"
              >
                <div className="min-w-0 flex-1">
                  <Input
                    controlSize="compact"
                    name={`survey-question-${number}-option-${optionIndex + 1}`}
                    value={option}
                    maxLength={SURVEY_OPTION_MAX_LENGTH}
                    aria-label={`Frage ${number}, Antwort ${optionIndex + 1}`}
                    aria-invalid={errors?.options ? true : undefined}
                    aria-describedby={
                      errors?.options ? optionsErrorId : undefined
                    }
                    placeholder={`Antwort ${optionIndex + 1}`}
                    onChange={(event) =>
                      setOption(optionIndex, event.target.value)
                    }
                  />
                </div>
                {question.options.length > 2 && (
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    aria-label={`Antwort ${optionIndex + 1} entfernen`}
                    onClick={() =>
                      onChange({
                        ...question,
                        options: question.options.filter(
                          (_, i) => i !== optionIndex,
                        ),
                      })
                    }
                  >
                    <X className="h-4 w-4" aria-hidden />
                  </Button>
                )}
              </li>
            ))}
          </ul>
          <FieldErrorHint id={optionsErrorId} message={errors?.options} />
          {question.options.length < SURVEY_MAX_OPTIONS && (
            <Button
              type="button"
              variant="outline"
              size="compact"
              className="mt-2 gap-1.5"
              onClick={() =>
                onChange({ ...question, options: [...question.options, ""] })
              }
            >
              <Plus className="h-4 w-4" aria-hidden />
              Antwort hinzufügen
            </Button>
          )}
        </div>
      )}

      <ChoiceTile className="items-center gap-3 px-3 py-2">
        <Checkbox
          checked={question.required}
          onChange={(event) =>
            onChange({ ...question, required: event.target.checked })
          }
        />
        <span className="text-sm text-gray-800">Pflichtfrage</span>
      </ChoiceTile>
    </li>
  );
}

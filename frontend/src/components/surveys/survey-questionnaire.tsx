"use client";

/**
 * Der Fragebogen einer Umfrage (#4781), geteilt von Mitarbeiterportal und
 * Elternportal. Die Texte kommen aus dem Namensraum `surveyForm`; das
 * Mitarbeiterportal bekommt ihn über `ShellIntlProvider`.
 *
 * Anonymität: der Hinweis, ob die Schule Namen sieht, steht VOR der ersten
 * Frage und vor dem Absenden. Die Antworten bleiben im Zustand dieses
 * Bauteils, bis die Person absendet. Sie gehen in kein Log, kein
 * Analyseereignis und keine Adresse; `data-analytics-block` hält sie aus
 * jeder Sitzungsaufzeichnung.
 */

import { useEffect, useId, useLayoutEffect, useRef, useState } from "react";
import { Check } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { Radio } from "~/components/ui/radio";
import { Skeleton } from "~/components/ui/skeleton";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import { berlinClockFromISO, formatBerlinDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  SURVEY_SCALE_VALUES,
  SURVEY_TEXT_MAX_LENGTH,
  buildSurveyAnswers,
  draftAnswer,
  missingRequiredQuestions,
  withSurveyScale,
  withSurveyText,
  sortedSurveyQuestions,
  toggleSurveyOption,
  type SurveyAnswersRequest,
  type SurveyDraft,
  type SurveyForm,
  type SurveyQuestion,
} from "~/lib/survey-form";

const logger = createLogger({ component: "SurveyQuestionnaire" });

/**
 * Ab so vielen Antworten zeigt die Schule eine anonyme Auswertung. Der Wert
 * steht im Vertrag fest (`min_responses`); der Fragebogen selbst trägt ihn
 * nicht.
 */
export const SURVEY_MIN_RESPONSES = 5;

type SurveyRespondentKind = "parent" | "staff";

/** „Teilnahme bis 12.11.2026, 18:00 Uhr“ in der Sprache der Person. */
export function useSurveyClosesAtLabel(): (closesAt: string) => string {
  const t = useTranslations("surveyForm");
  const locale = useLocale();
  return (closesAt) =>
    t("closesAt", {
      date: formatBerlinDate(closesAt, locale),
      time: berlinClockFromISO(closesAt),
    });
}

/**
 * Der deutliche Hinweis vor dem Antworten: anonym oder nicht, und was das
 * heißt. Eltern erfahren zusätzlich, dass eine Antwort je Elternkonto zählt.
 */
function SurveyAnonymityNotice({
  anonymous,
  respondent,
}: Readonly<{ anonymous: boolean; respondent: SurveyRespondentKind }>) {
  const t = useTranslations("surveyForm");
  const extra = respondent === "parent" ? ` ${t("parentOneAnswer")}` : "";
  return anonymous ? (
    <Alert
      type="info"
      announce="off"
      title={t("anonymousTitle")}
      message={`${t("anonymousText", { min: SURVEY_MIN_RESPONSES })}${extra}`}
    />
  ) : (
    <Alert
      type="warning"
      announce="off"
      title={t("notAnonymousTitle")}
      message={`${t("notAnonymousText")}${extra}`}
    />
  );
}

/** Der Zustand nach der Abgabe oder bei schon erfolgter Teilnahme. */
function SurveyDoneNote({
  title,
}: Readonly<{ title: string }>): React.ReactNode {
  const t = useTranslations("surveyForm");
  return (
    <div
      role="status"
      className="bg-moto-green-soft text-moto-green-strong flex items-start gap-3 rounded-xl px-4 py-3"
    >
      <Check className="mt-0.5 h-5 w-5 shrink-0" aria-hidden="true" />
      <div className="min-w-0">
        <p className="text-sm font-semibold">{title}</p>
        <p className="mt-0.5 text-sm">{t("thanksText")}</p>
      </div>
    </div>
  );
}

function QuestionField({
  question,
  index,
  total,
  draft,
  onChange,
  disabled,
  error,
  anonymous,
  name,
}: Readonly<{
  question: SurveyQuestion;
  index: number;
  total: number;
  draft: SurveyDraft;
  onChange: (next: SurveyDraft) => void;
  disabled: boolean;
  error: string | null;
  anonymous: boolean;
  /** The form name of the question's controls; the error path focuses it. */
  name: string;
}>) {
  const t = useTranslations("surveyForm");
  const baseId = useId();
  const errorId = `${baseId}-error`;
  const answer = draftAnswer(draft, question.id);
  const describedBy = error ? errorId : undefined;

  return (
    <fieldset
      className="space-y-3"
      aria-invalid={error ? true : undefined}
      aria-describedby={describedBy}
      disabled={disabled}
    >
      <legend className="w-full">
        <span className="sr-only">
          {t("questionLabel", { number: index + 1, total })}:{" "}
        </span>
        <span className="block text-base font-semibold text-gray-900">
          {question.prompt}
        </span>
        <span className="mt-0.5 flex flex-wrap gap-x-3 text-sm text-gray-500">
          {question.required && <span>{t("required")}</span>}
          {question.kind === "multiple" && <span>{t("multipleHint")}</span>}
          {question.kind === "scale" && <span>{t("scaleLegend")}</span>}
          {question.kind === "text" && anonymous && (
            <span>{t("anonymousTextHint")}</span>
          )}
        </span>
      </legend>

      {(question.kind === "single" || question.kind === "multiple") && (
        <div className="space-y-2">
          {question.options.map((option) => {
            const active = answer.optionIds.includes(option.id);
            return (
              <ChoiceTile
                key={option.id}
                selected={active}
                className="min-h-12 px-4 py-3 text-base has-[:disabled]:cursor-not-allowed"
              >
                {question.kind === "multiple" ? (
                  <Checkbox
                    name={name}
                    checked={active}
                    onChange={() =>
                      onChange(toggleSurveyOption(draft, question, option.id))
                    }
                  />
                ) : (
                  <Radio
                    name={name}
                    checked={active}
                    onChange={() =>
                      onChange(toggleSurveyOption(draft, question, option.id))
                    }
                  />
                )}
                <span>{option.label}</span>
              </ChoiceTile>
            );
          })}
        </div>
      )}

      {question.kind === "scale" && (
        <div>
          <div className="grid grid-cols-5 gap-2">
            {SURVEY_SCALE_VALUES.map((value) => (
              <ChoiceTile
                key={value}
                selected={answer.scaleValue === value}
                className="min-h-12 flex-col justify-center gap-1 px-2 py-2 text-base font-semibold has-[:disabled]:cursor-not-allowed"
              >
                <Radio
                  name={name}
                  checked={answer.scaleValue === value}
                  aria-label={
                    value === 1
                      ? `1, ${t("scaleLow")}`
                      : value === 5
                        ? `5, ${t("scaleHigh")}`
                        : String(value)
                  }
                  onChange={() =>
                    onChange(withSurveyScale(draft, question.id, value))
                  }
                />
                <span aria-hidden="true">{value}</span>
              </ChoiceTile>
            ))}
          </div>
          <div
            className="mt-1 flex justify-between gap-4 text-xs text-gray-500"
            aria-hidden="true"
          >
            <span>{t("scaleLow")}</span>
            <span className="text-right">{t("scaleHigh")}</span>
          </div>
        </div>
      )}

      {question.kind === "text" && (
        <Textarea
          id={`${baseId}-text`}
          name={name}
          aria-label={question.prompt}
          placeholder={t("textLabel")}
          rows={4}
          maxLength={SURVEY_TEXT_MAX_LENGTH}
          value={answer.text}
          onChange={(event) =>
            onChange(withSurveyText(draft, question.id, event.target.value))
          }
        />
      )}

      <FieldErrorHint id={errorId} message={error} />
    </fieldset>
  );
}

export interface SurveyAnswerPanelProps {
  /** Lädt den Fragebogen; ein Fehler steht im Panel mit Wiederholen. */
  readonly load: () => Promise<SurveyForm>;
  /** Schickt die Antworten einmal ab. */
  readonly submit: (body: SurveyAnswersRequest) => Promise<void>;
  readonly respondent: SurveyRespondentKind;
  /** Läuft nach der erfolgreichen Abgabe (Liste neu laden). */
  readonly onSubmitted?: () => void;
  /** Meldet, ob gerade abgeschickt wird (Dialog nicht schließen). */
  readonly onBusyChange?: (busy: boolean) => void;
  /** Ein neuer Wert lädt den Fragebogen neu. */
  readonly reloadKey?: string;
  /**
   * Die Einleitung zeigen. Im Elternportal steht sie schon als Text der
   * Mitteilung darüber.
   */
  readonly showIntro?: boolean;
}

/**
 * Laden, Hinweis, Fragen, Absenden und Danke in einem Bauteil. Beide Portale
 * betten es in ihren Dialog ein.
 */
export function SurveyAnswerPanel({
  load,
  submit,
  respondent,
  onSubmitted,
  onBusyChange,
  reloadKey = "",
  showIntro = true,
}: SurveyAnswerPanelProps) {
  const t = useTranslations("surveyForm");
  const closesAtLabel = useSurveyClosesAtLabel();
  const [form, setForm] = useState<SurveyForm | null>(null);
  const [draft, setDraft] = useState<SurveyDraft>({});
  const [attempted, setAttempted] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [reloadToken, setReloadToken] = useState(0);
  const loadErrors = useApiLoadError();
  const submitErrors = useApiFormError();
  const fieldPrefix = useId();
  const fieldName = (questionId: string) => `${fieldPrefix}-q${questionId}`;
  const { show: showLoadError, clear: clearLoadError } = loadErrors;
  const loadRef = useRef(load);
  useLayoutEffect(() => {
    loadRef.current = load;
  });

  useEffect(() => {
    let cancelled = false;
    setForm(null);
    setDraft({});
    setAttempted(false);
    setSubmitted(false);
    clearLoadError();
    loadRef
      .current()
      .then((loaded) => {
        if (!cancelled) setForm(loaded);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        logger.error("survey_form_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        void showLoadError(err, {
          object: t("loadObject"),
          retry: () => setReloadToken((n) => n + 1),
        });
      });
    return () => {
      cancelled = true;
    };
  }, [reloadKey, reloadToken, clearLoadError, showLoadError, t]);

  useEffect(() => {
    onBusyChange?.(submitting);
  }, [onBusyChange, submitting]);

  if (loadErrors.error) {
    return <LoadErrorAlert error={loadErrors.error} />;
  }
  if (!form) {
    return (
      <div className="space-y-3" aria-busy="true">
        <Skeleton className="h-16 w-full rounded-xl" />
        <Skeleton className="h-24 w-full rounded-xl" />
      </div>
    );
  }

  const questions = sortedSurveyQuestions(form.survey.questions);
  const closed = new Date(form.survey.closes_at) <= new Date();
  const missing = attempted ? missingRequiredQuestions(questions, draft) : [];
  const missingIds = new Set(missing.map((question) => question.id));
  const done = submitted || form.participated;
  const canAnswer = !done && !closed;

  const handleSubmit = async () => {
    setAttempted(true);
    submitErrors.clear();
    const [firstMissing] = missingRequiredQuestions(questions, draft);
    if (firstMissing) {
      // Marks nothing new (each question shows its own hint), only moves
      // focus to the first unanswered question.
      submitErrors.invalid(t("missingRequired"), {
        [fieldName(firstMissing.id)]: t("missingField"),
      });
      return;
    }
    setSubmitting(true);
    try {
      await submit(buildSurveyAnswers(questions, draft));
      setSubmitted(true);
      setDraft({});
      onSubmitted?.();
    } catch (err: unknown) {
      // Nur die Fehlermeldung, nie der Rumpf mit den Antworten.
      logger.error("survey_answers_submit_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void submitErrors.show(err, {
        object: t("submitObject"),
        retry: () => void handleSubmit(),
      });
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="space-y-5" data-analytics-block>
      <div className="space-y-3">
        <p className="text-sm text-gray-600">
          {closed ? t("closed") : closesAtLabel(form.survey.closes_at)}
        </p>
        <SurveyAnonymityNotice
          anonymous={form.survey.anonymous}
          respondent={respondent}
        />
        {showIntro && form.survey.intro && (
          <p className="text-base leading-7 whitespace-pre-line text-gray-800">
            {form.survey.intro}
          </p>
        )}
      </div>

      {done ? (
        <SurveyDoneNote
          title={submitted ? t("thanksTitle") : t("alreadyTitle")}
        />
      ) : closed ? null : (
        <form
          noValidate
          className="space-y-6"
          onSubmit={(event) => {
            event.preventDefault();
            void handleSubmit();
          }}
        >
          <FormErrorAlert message={submitErrors.error} />
          {questions.map((question, index) => (
            <QuestionField
              key={question.id}
              question={question}
              index={index}
              total={questions.length}
              draft={draft}
              onChange={setDraft}
              disabled={submitting}
              error={missingIds.has(question.id) ? t("missingField") : null}
              anonymous={form.survey.anonymous}
              name={fieldName(question.id)}
            />
          ))}
          <div className="space-y-3 border-t border-gray-100 pt-4">
            <p className="text-sm text-gray-600">{t("finalHint")}</p>
            {canAnswer && (
              <Button
                type="submit"
                size="md"
                isLoading={submitting}
                loadingText={t("submitting")}
                disabled={submitting}
              >
                {t("submit")}
              </Button>
            )}
          </div>
        </form>
      )}
    </div>
  );
}

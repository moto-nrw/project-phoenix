"use client";

/**
 * Die Auswertung einer Umfrage (#4781): nur Summen und Verteilungen.
 *
 * Schutzziel: Eine anonyme Umfrage unter der Mindestzahl zeigt keine einzige
 * Fragezahl, nur die Erklärung und die Zahl der Antworten. Freitexte stehen
 * ohne Namen und ohne Zeit, in der Reihenfolge des Backends (zufällig). Die
 * Antworten je Person gibt es nur bei nicht anonymen Umfragen, getrennt in
 * `SurveyRespondentsView`.
 */

import { ProgressBar } from "~/components/ui/progress-bar";
import { SectionCard } from "~/components/ui/section-card";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EmptyState } from "~/components/ui/empty-state";
import type {
  Survey,
  SurveyQuestionResult,
  SurveyResults,
  SurveyRespondent,
  SurveyScaleSummary,
} from "~/lib/surveys-api";
import { sortedSurveyQuestions } from "~/lib/survey-form";

const SCALE_LABEL: Record<number, string> = {
  1: "1 · trifft gar nicht zu",
  2: "2",
  3: "3",
  4: "4",
  5: "5 · trifft voll zu",
};

function formatSurveyAverage(value: number): string {
  return value.toLocaleString("de-DE", {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  });
}

function answersLabel(count: number): string {
  return `${count} ${count === 1 ? "Antwort" : "Antworten"}`;
}

function percent(count: number, total: number): string {
  if (total <= 0) return "0 %";
  return `${Math.round((count / total) * 100)} %`;
}

function DistributionRow({
  label,
  count,
  total,
}: Readonly<{ label: string; count: number; total: number }>) {
  return (
    <li className="space-y-1">
      <div className="flex items-baseline justify-between gap-3 text-sm">
        <span className="min-w-0 text-gray-800">{label}</span>
        <span className="shrink-0 text-gray-600 tabular-nums">
          {count} · {percent(count, total)}
        </span>
      </div>
      <ProgressBar
        value={count}
        max={total}
        label={`${label}: ${count} von ${total}`}
      />
    </li>
  );
}

function scaleValueLabel(value: string): string {
  return `Wert ${value}`;
}

function HighLow({ question }: Readonly<{ question: SurveyQuestionResult }>) {
  if (question.kind === "text") return null;
  const format = (values: string[]) =>
    values
      .map((value) =>
        question.kind === "scale" ? scaleValueLabel(value) : value,
      )
      .join(", ");
  if (question.highest.length === 0 && question.lowest.length === 0) {
    return null;
  }
  return (
    <DataGrid>
      {question.highest.length > 0 && (
        <DataField label="Stärken (am häufigsten gewählt)">
          {format(question.highest)}
        </DataField>
      )}
      {question.lowest.length > 0 && (
        <DataField label="Verbesserungsbedarf (am seltensten gewählt)">
          {format(question.lowest)}
        </DataField>
      )}
    </DataGrid>
  );
}

function QuestionResultCard({
  question,
  number,
}: Readonly<{ question: SurveyQuestionResult; number: number }>) {
  return (
    <SectionCard
      title={`Frage ${number}: ${question.prompt}`}
      description={answersLabel(question.answer_count)}
      headingLevel={3}
    >
      <div className="space-y-4">
        {(question.kind === "single" || question.kind === "multiple") && (
          <ul className="space-y-3">
            {question.options.map((option) => (
              <DistributionRow
                key={option.option_id}
                label={option.label}
                count={option.count}
                total={question.answer_count}
              />
            ))}
          </ul>
        )}
        {question.kind === "scale" && (
          <>
            <p className="text-sm text-gray-700">
              Durchschnitt:{" "}
              <span className="font-semibold text-gray-900">
                {question.average === null
                  ? "–"
                  : `${formatSurveyAverage(question.average)} von 5`}
              </span>
            </p>
            <ul className="space-y-3">
              {question.scale_counts.map((count, index) => (
                <DistributionRow
                  // Die Skala hat fest fünf Stufen; der Index ist der Wert.
                  // eslint-disable-next-line react/no-array-index-key
                  key={index}
                  label={SCALE_LABEL[index + 1] ?? String(index + 1)}
                  count={count}
                  total={question.answer_count}
                />
              ))}
            </ul>
          </>
        )}
        {question.kind === "text" &&
          (question.texts.length === 0 ? (
            <p className="text-sm text-gray-500">Noch keine Freitexte.</p>
          ) : (
            <ul className="space-y-2">
              {question.texts.map((text, index) => (
                // Freitexte haben bewusst keine ID: nichts soll auf die Person
                // zurückführen. Die Reihenfolge kommt zufällig vom Backend.
                // eslint-disable-next-line react/no-array-index-key
                <li
                  key={index}
                  className="rounded-lg bg-gray-50 px-3 py-2 text-sm whitespace-pre-line text-gray-800"
                >
                  {text}
                </li>
              ))}
            </ul>
          ))}
        <HighLow question={question} />
      </div>
    </SectionCard>
  );
}

function ScaleSummaryList({
  items,
}: Readonly<{ items: SurveyScaleSummary[] }>) {
  return (
    <ul className="space-y-1.5">
      {items.map((item) => (
        <li
          key={item.question_id}
          className="flex items-baseline justify-between gap-3 text-sm"
        >
          <span className="min-w-0 text-gray-800">{item.prompt}</span>
          <span className="shrink-0 font-semibold text-gray-900 tabular-nums">
            {formatSurveyAverage(item.average)}
          </span>
        </li>
      ))}
    </ul>
  );
}

export function SurveyResultsView({
  results,
}: Readonly<{ results: SurveyResults }>) {
  if (!results.available) {
    return (
      <SectionCard>
        <EmptyState
          title="Noch keine Auswertung"
          description={`Die Auswertung erscheint ab ${results.min_responses} Antworten, damit niemand erkennbar ist. Bisher: ${results.response_count}.`}
        />
      </SectionCard>
    );
  }

  const questions = [...results.questions].sort(
    (a, b) => a.position - b.position,
  );
  const hasOverview =
    results.strengths.length > 0 || results.improvements.length > 0;

  return (
    <div className="flex flex-col gap-6">
      {hasOverview && (
        <SectionCard
          title="Stärken und Verbesserungsbedarf"
          description="Die Skalenfragen mit dem höchsten und dem niedrigsten Durchschnitt (1 bis 5)."
        >
          <div className="grid gap-6 sm:grid-cols-2">
            <div>
              <h4 className="mb-2 text-sm font-semibold text-gray-900">
                Stärken
              </h4>
              {results.strengths.length > 0 ? (
                <ScaleSummaryList items={results.strengths} />
              ) : (
                <p className="text-sm text-gray-500">–</p>
              )}
            </div>
            <div>
              <h4 className="mb-2 text-sm font-semibold text-gray-900">
                Verbesserungsbedarf
              </h4>
              {results.improvements.length > 0 ? (
                <ScaleSummaryList items={results.improvements} />
              ) : (
                <p className="text-sm text-gray-500">–</p>
              )}
            </div>
          </div>
        </SectionCard>
      )}
      {questions.map((question, index) => (
        <QuestionResultCard
          key={question.question_id}
          question={question}
          number={index + 1}
        />
      ))}
    </div>
  );
}

/**
 * Antworten je Person, nur bei NICHT anonymen Umfragen. Die Optionen kommen
 * als IDs; die Beschriftung steht im Fragebogen.
 */
export function SurveyRespondentsView({
  survey,
  respondents,
}: Readonly<{ survey: Survey; respondents: SurveyRespondent[] }>) {
  const questions = sortedSurveyQuestions(survey.questions);
  if (respondents.length === 0) {
    return (
      <SectionCard>
        <EmptyState
          title="Noch keine Antworten"
          description="Sobald jemand teilnimmt, stehen die Antworten hier mit Namen."
        />
      </SectionCard>
    );
  }
  return (
    <div className="flex flex-col gap-4">
      {respondents.map((respondent, index) => {
        const byQuestion = new Map(
          respondent.answers.map((answer) => [answer.question_id, answer]),
        );
        return (
          <SectionCard
            // Personen kommen ohne ID; Name und Platz sind eindeutig genug.
            key={`${respondent.name}-${index}`}
            title={respondent.name}
            headingLevel={3}
            collapsible
            defaultCollapsed={respondents.length > 3}
          >
            <DataGrid columns={1}>
              {questions.map((question) => {
                const answer = byQuestion.get(question.id);
                const labels = new Map(
                  question.options.map((option) => [option.id, option.label]),
                );
                const value = !answer
                  ? "–"
                  : question.kind === "scale"
                    ? answer.scale_value === undefined
                      ? "–"
                      : `${answer.scale_value} von 5`
                    : question.kind === "text"
                      ? answer.text?.trim() || "–"
                      : answer.options
                          .map((id) => labels.get(id) ?? id)
                          .join(", ") || "–";
                return (
                  <DataField key={question.id} label={question.prompt}>
                    <span className="whitespace-pre-line">{value}</span>
                  </DataField>
                );
              })}
            </DataGrid>
          </SectionCard>
        );
      })}
    </div>
  );
}

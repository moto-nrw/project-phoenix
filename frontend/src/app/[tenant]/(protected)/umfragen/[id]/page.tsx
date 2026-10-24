"use client";

import { Suspense, useRef, useState } from "react";
import { useParams, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";

import { useParentTargetLookups } from "~/components/announcements/parent-target-picker";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { ForbiddenPage } from "~/components/ui/forbidden-page";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { SURVEY_FORM_CHECK_MESSAGE } from "~/components/surveys/survey-create-slide-over";
import { SurveyEditorFields } from "~/components/surveys/survey-editor-fields";
import {
  buildSurveyMenuItems,
  CloseSurveyDialog,
  DeleteSurveyDialog,
  PublishSurveyDialog,
  RemindSurveyDialog,
} from "~/components/surveys/survey-lifecycle-dialogs";
import {
  describeSurveyAudience,
  formatSurveyClosesAt,
  participantLabel,
  remindedMessage,
  SURVEY_LIST_KEY,
  SurveyAnonymityBadge,
  SurveyStatusBadge,
} from "~/components/surveys/survey-meta";
import {
  SurveyResultsView,
  SurveyRespondentsView,
} from "~/components/surveys/survey-results";
import {
  useApiErrorDisplay,
  useApiFormError,
  useToast,
} from "~/contexts/ToastContext";
import { useUpdateUrlParams } from "~/hooks/useUpdateUrlParams";
import { ApiError } from "~/lib/api-error";
import { hasPermission } from "~/lib/auth-utils";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import {
  SURVEY_QUESTION_KIND_LABEL,
  hasEditorErrors,
  surveyDraftFromSurvey,
  surveyInputFromDraft,
  validateSurveyDraft,
  type SurveyEditorDraft,
  type SurveyEditorErrors,
} from "~/lib/survey-editor";
import { sortedSurveyQuestions } from "~/lib/survey-form";
import {
  downloadSurveyResultsPdf,
  fetchSurvey,
  fetchSurveyRespondents,
  fetchSurveyResults,
  updateSurvey,
  type Survey,
  type SurveyRespondent,
  type SurveyResults,
} from "~/lib/surveys-api";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { useTenantRouter } from "~/lib/tenant-router";

const logger = createLogger({ component: "SurveyDetailPage" });

const BACK = {
  back: true,
  backHref: "/umfragen",
  backLabel: "Zurück zu den Umfragen",
} as const;

type Tab = "auswertung" | "fragebogen" | "personen";
type LifecycleAction = "publish" | "close" | "remind" | "delete" | null;

function surveyKey(id: string): string {
  return `survey-${id}`;
}

export default function SurveyDetailPage() {
  // useSearchParams (`?reiter=`, `?bearbeiten=`) braucht die Suspense-Grenze.
  return (
    <Suspense fallback={<TenantPage title="Umfrage" {...BACK} loading />}>
      <SurveyDetailContent />
    </Suspense>
  );
}

/**
 * Die Objektseite einer Umfrage (#4781), nur für `surveys:manage`. Reiter:
 * Auswertung (Summen und Verteilungen), Fragebogen (Angaben und Fragen; ein
 * Entwurf wird hier bearbeitet) und bei nicht anonymen Umfragen die
 * Antworten je Person.
 */
function SurveyDetailContent() {
  const { data: session, status: sessionStatus } = useSession();
  const canManage = hasPermission(session, "surveys:manage");
  const params = useParams();
  const surveyId = params.id as string;

  if (sessionStatus !== "loading" && !canManage) {
    return (
      <ForbiddenPage
        title="Umfrage"
        message="Umfragen anlegen und auswerten darf nur, wer dafür die Berechtigung hat. Bitte fragen Sie Ihre Leitung."
      />
    );
  }
  return <SurveyDetail surveyId={surveyId} />;
}

function SurveyDetail({ surveyId }: Readonly<{ surveyId: string }>) {
  const searchParams = useSearchParams();
  const updateUrlParams = useUpdateUrlParams();
  const router = useTenantRouter();
  const tenantMutate = useTenantMutate();
  const { success: toastSuccess } = useToast();
  const exportErrors = useApiErrorDisplay();
  const [action, setAction] = useState<LifecycleAction>(null);
  const [exporting, setExporting] = useState(false);

  const {
    data: survey,
    error: loadError,
    isLoading,
    mutate,
  } = useSWRAuth<Survey>(surveyKey(surveyId), () => fetchSurvey(surveyId), {
    revalidateOnFocus: false,
  });
  const notFound = loadError instanceof ApiError && loadError.status === 404;
  const shownLoadError = useSwrLoadError(
    notFound ? undefined : loadError,
    "die Umfrage",
    () => void mutate(),
  );

  const published = survey !== undefined && survey.status !== "draft";
  const results = useSWRAuth<SurveyResults>(
    published ? `survey-results-${surveyId}` : null,
    () => fetchSurveyResults(surveyId),
    { revalidateOnFocus: false },
  );
  const resultsError = useSwrLoadError(
    results.error,
    "die Auswertung",
    () => void results.mutate(),
  );

  const requestedTab = searchParams.get("reiter");
  const tab: Tab = !published
    ? "fragebogen"
    : requestedTab === "fragebogen"
      ? "fragebogen"
      : requestedTab === "personen" && survey && !survey.anonymous
        ? "personen"
        : "auswertung";

  const respondents = useSWRAuth<SurveyRespondent[]>(
    published && survey && !survey.anonymous && tab === "personen"
      ? `survey-respondents-${surveyId}`
      : null,
    () => fetchSurveyRespondents(surveyId),
    { revalidateOnFocus: false },
  );
  const respondentsError = useSwrLoadError(
    respondents.error,
    "die Antworten",
    () => void respondents.mutate(),
  );

  const editing =
    survey?.status === "draft" && searchParams.get("bearbeiten") === "1";

  const refresh = async () => {
    await Promise.all([
      mutate(),
      results.mutate(),
      tenantMutate(SURVEY_LIST_KEY),
    ]);
  };

  const awaitingErrorText = !!loadError && !notFound && !shownLoadError;
  if ((isLoading && !survey) || awaitingErrorText) {
    return <TenantPage title="Umfrage" {...BACK} statsLoading loading />;
  }
  if (loadError || !survey) {
    return (
      <TenantPage
        title="Umfrage"
        {...BACK}
        error={
          loadError && !notFound
            ? shownLoadError
            : "Diese Umfrage gibt es nicht mehr."
        }
      />
    );
  }

  const exportPdf = async () => {
    setExporting(true);
    try {
      await downloadSurveyResultsPdf(survey.id);
    } catch (err: unknown) {
      logger.error("survey_export_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void exportErrors.show(err, {
        object: "die Auswertung als PDF",
        retry: () => void exportPdf(),
      });
    } finally {
      setExporting(false);
    }
  };

  const menuItems = buildSurveyMenuItems(survey, {
    onPublish: () => setAction("publish"),
    onEdit: () => updateUrlParams({ bearbeiten: "1", reiter: null }),
    onRemind: () => setAction("remind"),
    onCloseSurvey: () => setAction("close"),
    onExport:
      results.data?.available && !exporting
        ? () => void exportPdf()
        : undefined,
    onDelete: () => setAction("delete"),
  });

  const stats = [
    describeSurveyAudience(survey),
    `Teilnahme bis ${formatSurveyClosesAt(survey.closes_at)}`,
    survey.status === "draft"
      ? null
      : participantLabel(survey.participant_count),
  ]
    .filter(Boolean)
    .join(" · ");

  const tabs = published
    ? {
        value: tab,
        onChange: (next: string) =>
          updateUrlParams({ reiter: next === "auswertung" ? null : next }),
        items: [
          { value: "auswertung", label: "Auswertung" },
          { value: "fragebogen", label: "Fragebogen" },
          ...(survey.anonymous
            ? []
            : [{ value: "personen", label: "Antworten je Person" }]),
        ],
        label: "Bereiche der Umfrage",
      }
    : undefined;

  return (
    <TenantPage
      title={survey.title}
      stats={stats}
      {...BACK}
      leading={
        <MotoDuotoneIcon
          icon={MOTO_CONCEPTS.polls.icon}
          tone={MOTO_CONCEPTS.polls.tone}
          size={40}
        />
      }
      actions={
        <>
          <SurveyStatusBadge status={survey.status} />
          <SurveyAnonymityBadge anonymous={survey.anonymous} />
          {!editing && (
            <OverflowMenu ariaLabel="Weitere Aktionen" items={menuItems} />
          )}
        </>
      }
      tabs={tabs}
      overlays={
        <>
          {action === "publish" && (
            <PublishSurveyDialog
              survey={survey}
              onClose={() => setAction(null)}
              onDone={refresh}
            />
          )}
          {action === "close" && (
            <CloseSurveyDialog
              survey={survey}
              onClose={() => setAction(null)}
              onDone={refresh}
            />
          )}
          {action === "remind" && (
            <RemindSurveyDialog
              survey={survey}
              onClose={() => setAction(null)}
              onReminded={(count) => toastSuccess(remindedMessage(count))}
            />
          )}
          {action === "delete" && (
            <DeleteSurveyDialog
              survey={survey}
              onClose={() => setAction(null)}
              onDone={async () => {
                await tenantMutate(SURVEY_LIST_KEY);
                router.push("/umfragen");
              }}
            />
          )}
        </>
      }
    >
      {tab === "auswertung" &&
        (resultsError ? (
          <SectionCard>
            <FormErrorAlert message={resultsError} />
          </SectionCard>
        ) : results.data ? (
          <SurveyResultsView results={results.data} />
        ) : (
          <SectionCard title="Auswertung" description="Wird geladen…" />
        ))}

      {tab === "personen" &&
        (respondentsError ? (
          <SectionCard>
            <FormErrorAlert message={respondentsError} />
          </SectionCard>
        ) : respondents.data ? (
          <SurveyRespondentsView
            survey={survey}
            respondents={respondents.data}
          />
        ) : (
          <SectionCard
            title="Antworten je Person"
            description="Wird geladen…"
          />
        ))}

      {tab === "fragebogen" &&
        (editing ? (
          <SurveyEditSection
            survey={survey}
            onCancel={() => updateUrlParams({ bearbeiten: null })}
            onSaved={async (saved) => {
              await mutate(saved, { revalidate: false });
              await tenantMutate(SURVEY_LIST_KEY);
              updateUrlParams({ bearbeiten: null });
            }}
          />
        ) : (
          <SurveyReadView survey={survey} />
        ))}
    </TenantPage>
  );
}

function SurveyReadView({ survey }: Readonly<{ survey: Survey }>) {
  const questions = sortedSurveyQuestions(survey.questions);
  return (
    <div className="flex flex-col gap-6">
      <SectionCard
        title="Angaben"
        description={
          survey.status === "draft"
            ? "Entwurf: Noch sieht niemand die Umfrage. Ändern Sie sie über das Menü oben mit „Bearbeiten“."
            : "Veröffentlicht: Die Angaben lassen sich nicht mehr ändern."
        }
      >
        <DataGrid>
          <DataField label="Wer antwortet">
            {describeSurveyAudience(survey)}
          </DataField>
          <DataField label="Anonym">
            {survey.anonymous ? "Ja" : "Nein, der Name steht bei der Antwort"}
          </DataField>
          <DataField label="Teilnahme bis">
            {formatSurveyClosesAt(survey.closes_at)}
          </DataField>
          {survey.audience === "parents" && (
            <DataField label="Zusätzlich per E-Mail">
              {survey.send_email ? "Ja" : "Nein"}
            </DataField>
          )}
          <DataField label="Einleitung" fullWidth>
            <span className="whitespace-pre-line">{survey.intro || "–"}</span>
          </DataField>
        </DataGrid>
      </SectionCard>
      <SectionCard title={`Fragen (${questions.length})`}>
        <ol className="space-y-4">
          {questions.map((question, index) => (
            <li key={question.id} className="space-y-1">
              <p className="text-sm font-semibold text-gray-900">
                {index + 1}. {question.prompt}
              </p>
              <p className="text-sm text-gray-500">
                {SURVEY_QUESTION_KIND_LABEL[question.kind]}
                {question.required ? " · Pflichtfrage" : " · freiwillig"}
              </p>
              {question.options.length > 0 && (
                <ul className="list-disc pl-5 text-sm text-gray-700">
                  {question.options.map((option) => (
                    <li key={option.id}>{option.label}</li>
                  ))}
                </ul>
              )}
            </li>
          ))}
        </ol>
      </SectionCard>
    </div>
  );
}

function SurveyEditSection({
  survey,
  onCancel,
  onSaved,
}: Readonly<{
  survey: Survey;
  onCancel: () => void;
  onSaved: (saved: Survey) => Promise<void>;
}>) {
  const [draft, setDraft] = useState<SurveyEditorDraft>(() =>
    surveyDraftFromSurvey(survey),
  );
  const [errors, setErrors] = useState<SurveyEditorErrors | null>(null);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);
  const lookups = useParentTargetLookups(draft.audience === "parents");

  const save = async () => {
    const found = validateSurveyDraft(draft);
    setErrors(found);
    formErrors.clear();
    if (hasEditorErrors(found)) {
      formErrors.invalid(SURVEY_FORM_CHECK_MESSAGE);
      return;
    }
    setSaving(true);
    try {
      const saved = await updateSurvey(survey.id, surveyInputFromDraft(draft));
      await onSaved(saved);
    } catch (err: unknown) {
      logger.error("survey_update_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, {
        object: "die Umfrage",
        retry: () => void save(),
      });
    } finally {
      setSaving(false);
    }
  };

  return (
    <SectionCard title="Entwurf bearbeiten">
      <div ref={formRef} className="space-y-6">
        <FormErrorAlert message={formErrors.error} />
        <SurveyEditorFields
          draft={draft}
          onChange={(next) => {
            setDraft(next);
            if (errors) setErrors(validateSurveyDraft(next));
          }}
          errors={errors}
          disabled={saving}
          {...lookups}
        />
        <EditActions
          onCancel={onCancel}
          onSave={() => void save()}
          saving={saving}
        />
      </div>
    </SectionCard>
  );
}

"use client";

import { Suspense, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { ListChecks, Plus } from "lucide-react";

import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { Button } from "~/components/ui/button";
import { DataTable } from "~/components/ui/data-table";
import type { DataTableColumn } from "~/components/ui/data-table";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import { TileCard } from "~/components/ui/tile-card";
import { SurveyAnswerModal } from "~/components/surveys/survey-answer-modal";
import { SurveyCreateSlideOver } from "~/components/surveys/survey-create-slide-over";
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
  SurveyAnonymityBadge,
  remindedMessage,
  SURVEY_LIST_KEY,
  SurveyStatusBadge,
} from "~/components/surveys/survey-meta";
import { useToast } from "~/contexts/ToastContext";
import { useUpdateUrlParams } from "~/hooks/useUpdateUrlParams";
import { hasPermission } from "~/lib/auth-utils";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth } from "~/lib/swr";
import {
  fetchSurveys,
  fetchSurveysToAnswer,
  type Survey,
  type SurveyToAnswer,
} from "~/lib/surveys-api";
import { useTenantRouter } from "~/lib/tenant-router";

const TO_ANSWER_KEY = "surveys-to-answer";

type View = "fuer-sie" | "alle";

type LifecycleAction = {
  kind: "publish" | "close" | "remind" | "delete";
  survey: Survey;
} | null;

function viewFromParam(value: string | null, canManage: boolean): View {
  if (!canManage) return "fuer-sie";
  return value === "fuer-sie" ? "fuer-sie" : "alle";
}

export default function SurveysPage() {
  // useSearchParams (Reiter `?ansicht=`) braucht die Suspense-Grenze.
  return (
    <Suspense fallback={null}>
      <SurveysContent />
    </Suspense>
  );
}

/**
 * Umfragen (#4781). Jede Person im Team beantwortet hier die Umfragen ihrer
 * Schule an das Team. Wer `surveys:manage` hat, legt außerdem Umfragen an
 * Eltern oder das Team an und sieht ihre Auswertung. Die Verwaltung sieht
 * nur Zahlen, nie wer (nicht) teilgenommen hat.
 */
function SurveysContent() {
  const { data: session } = useSession();
  const canManage = hasPermission(session, "surveys:manage");
  const searchParams = useSearchParams();
  const updateUrlParams = useUpdateUrlParams();
  const router = useTenantRouter();
  const { success: toastSuccess } = useToast();
  const view = viewFromParam(searchParams.get("ansicht"), canManage);

  const [creating, setCreating] = useState(false);
  const [answering, setAnswering] = useState<SurveyToAnswer | null>(null);
  const [action, setAction] = useState<LifecycleAction>(null);

  const toAnswer = useSWRAuth<SurveyToAnswer[]>(
    TO_ANSWER_KEY,
    fetchSurveysToAnswer,
    { revalidateOnFocus: false, keepPreviousData: true },
  );
  const surveys = useSWRAuth<Survey[]>(
    canManage ? SURVEY_LIST_KEY : null,
    fetchSurveys,
    { revalidateOnFocus: false, keepPreviousData: true },
  );
  const toAnswerError = useSwrLoadError(
    toAnswer.error,
    "die offenen Umfragen",
    () => void toAnswer.mutate(),
  );
  const surveysError = useSwrLoadError(
    surveys.error,
    "die Liste der Umfragen",
    () => void surveys.mutate(),
  );

  const openToAnswer = useMemo(
    () =>
      [...(toAnswer.data ?? [])].sort(
        (a, b) =>
          Number(a.participated) - Number(b.participated) ||
          a.closes_at.localeCompare(b.closes_at),
      ),
    [toAnswer.data],
  );
  const pendingCount = openToAnswer.filter((s) => !s.participated).length;
  const list = surveys.data ?? [];

  const objectPath = (survey: Survey) =>
    `/umfragen/${encodeURIComponent(survey.id)}`;

  const createButton = canManage ? (
    <Button
      type="button"
      variant="primary"
      size="md"
      onClick={() => setCreating(true)}
      className="gap-1.5"
    >
      <Plus className="h-4 w-4" aria-hidden />
      Umfrage anlegen
    </Button>
  ) : undefined;

  const refreshAll = async () => {
    await Promise.all([surveys.mutate(), toAnswer.mutate()]);
  };

  const active = view === "alle" ? surveys : toAnswer;
  const activeError = view === "alle" ? surveysError : toAnswerError;
  // Until the first answer arrives, the page is loading. Before the session is
  // known, useSWRAuth holds the key back and reports no request at all, so
  // isLoading alone would show "Keine offenen Umfragen" for a moment.
  const loading = active.data === undefined && !activeError;

  const stats =
    view === "alle"
      ? surveys.data === undefined
        ? null
        : `${list.length} ${list.length === 1 ? "Umfrage" : "Umfragen"} · ${list.filter((s) => s.status === "open").length} laufen`
      : toAnswer.data === undefined
        ? null
        : `${pendingCount} offen für Sie`;

  const columns: DataTableColumn<Survey>[] = [
    {
      key: "title",
      header: "Titel",
      stacked: "title",
      sortValue: (row) => row.title.toLowerCase(),
      render: (row) => (
        <div className="min-w-0">
          <p className="truncate font-semibold text-gray-900">{row.title}</p>
          <p className="mt-0.5 text-xs text-gray-500">
            {describeSurveyAudience(row)}
          </p>
        </div>
      ),
    },
    {
      key: "status",
      header: "Status",
      sortValue: (row) => row.status,
      render: (row) => (
        <div className="flex flex-wrap items-center gap-1.5">
          <SurveyStatusBadge status={row.status} />
          <SurveyAnonymityBadge anonymous={row.anonymous} />
        </div>
      ),
    },
    {
      key: "participants",
      header: "Teilnahme",
      sortValue: (row) => row.participant_count,
      render: (row) => (
        <span className="text-sm text-gray-700 tabular-nums">
          {row.status === "draft"
            ? "–"
            : participantLabel(row.participant_count)}
        </span>
      ),
    },
    {
      key: "closes",
      header: "Teilnahme bis",
      sortValue: (row) => row.closes_at,
      render: (row) => (
        <span className="text-sm text-gray-700">
          {formatSurveyClosesAt(row.closes_at)}
        </span>
      ),
    },
    {
      key: "actions",
      header: "",
      align: "right",
      render: (row) => (
        <div
          className="flex items-center justify-end"
          onClick={(e) => e.stopPropagation()}
          onKeyDown={(e) => e.stopPropagation()}
          role="presentation"
        >
          <OverflowMenu
            ariaLabel={`Aktionen für ${row.title}`}
            items={buildSurveyMenuItems(row, {
              onPublish: () => setAction({ kind: "publish", survey: row }),
              onView: () => router.push(objectPath(row)),
              onEdit: () => router.push(`${objectPath(row)}?bearbeiten=1`),
              onRemind: () => setAction({ kind: "remind", survey: row }),
              onCloseSurvey: () => setAction({ kind: "close", survey: row }),
              onDelete: () => setAction({ kind: "delete", survey: row }),
            })}
          />
        </div>
      ),
    },
  ];

  const empty =
    !loading && !activeError
      ? view === "alle" && list.length === 0
        ? {
            icon: <ListChecks className="h-12 w-12" aria-hidden />,
            title: "Noch keine Umfragen",
            description:
              "Fragen Sie Eltern oder das Team nach ihrer Meinung, auf Wunsch anonym.",
          }
        : view === "fuer-sie" && openToAnswer.length === 0
          ? {
              icon: <ListChecks className="h-12 w-12" aria-hidden />,
              title: "Keine offenen Umfragen",
              description:
                "Hier erscheinen Umfragen, die Ihre Schule an das Team stellt.",
            }
          : null
      : null;

  return (
    <TenantPage
      title="Umfragen"
      stats={stats}
      statsLoading={loading}
      info={
        canManage
          ? "Hier stellen Sie Eltern oder dem Team mehrere Fragen, auf Wunsch anonym. Für eine kurze Frage zu jedem Kind, zum Beispiel zum Sommerfest, nehmen Sie die Umfrage unter Eltern › Mitteilungen."
          : "Hier beantworten Sie Umfragen Ihrer Schule an das Team."
      }
      actions={createButton}
      tabs={
        canManage
          ? {
              value: view,
              onChange: (next) =>
                updateUrlParams({ ansicht: next === "alle" ? null : next }),
              items: [
                { value: "alle", label: "Alle Umfragen", badge: list.length },
                {
                  value: "fuer-sie",
                  label: "Für Sie",
                  badge: pendingCount,
                },
              ],
              label: "Ansicht",
            }
          : undefined
      }
      loading={loading}
      error={activeError}
      empty={empty}
      overlays={
        <>
          {creating && (
            <SurveyCreateSlideOver
              onClose={() => setCreating(false)}
              onCreated={(created) => {
                setCreating(false);
                void surveys.mutate();
                router.push(objectPath(created));
              }}
            />
          )}
          {answering && (
            <SurveyAnswerModal
              survey={answering}
              onClose={() => setAnswering(null)}
              onSubmitted={() => void refreshAll()}
            />
          )}
          {action?.kind === "publish" && (
            <PublishSurveyDialog
              survey={action.survey}
              onClose={() => setAction(null)}
              onDone={refreshAll}
            />
          )}
          {action?.kind === "close" && (
            <CloseSurveyDialog
              survey={action.survey}
              onClose={() => setAction(null)}
              onDone={refreshAll}
            />
          )}
          {action?.kind === "remind" && (
            <RemindSurveyDialog
              survey={action.survey}
              onClose={() => setAction(null)}
              onReminded={(count) => toastSuccess(remindedMessage(count))}
            />
          )}
          {action?.kind === "delete" && (
            <DeleteSurveyDialog
              survey={action.survey}
              onClose={() => setAction(null)}
              onDone={refreshAll}
            />
          )}
        </>
      }
    >
      {view === "alle" ? (
        <DataTable
          stackedOnMobile
          columns={columns}
          rows={list}
          getRowKey={(row) => row.id}
          defaultSortKey="closes"
          onRowClick={(row) => router.push(objectPath(row))}
          rowHasInteractiveControls
        />
      ) : (
        <ul className="space-y-3">
          {openToAnswer.map((survey) => (
            <li key={survey.id}>
              <TileCard
                onClick={() => setAnswering(survey)}
                ariaHasPopup="dialog"
              >
                <div className="flex min-w-0 items-center gap-2">
                  <MotoConceptIcon concept="polls" size={20} />
                  <h3 className="min-w-0 truncate text-base font-semibold text-gray-900">
                    {survey.title}
                  </h3>
                </div>
                {survey.intro && (
                  <p className="mt-1 line-clamp-2 text-sm text-gray-600">
                    {survey.intro}
                  </p>
                )}
                <div className="mt-2 flex flex-wrap items-center gap-2 text-sm text-gray-500">
                  {survey.participated ? (
                    <StatusBadge label="Teilgenommen" tone="green" />
                  ) : (
                    <StatusBadge label="Antwort offen" tone="orange" />
                  )}
                  <SurveyAnonymityBadge anonymous={survey.anonymous} />
                  <span>
                    Teilnahme bis {formatSurveyClosesAt(survey.closes_at)}
                  </span>
                </div>
              </TileCard>
            </li>
          ))}
        </ul>
      )}
    </TenantPage>
  );
}

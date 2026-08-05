"use client";

/**
 * Parents-portal feedback board (#1678).
 *
 * Same board the school's staff use — entries, votes, threads, statuses — but
 * addressed to the moto product team instead of the OGS, and pseudonymous:
 * guardians appear as "Elternteil" to everyone, including the product team.
 * Both facts are stated on the page, because a parent who mistakes this for a
 * channel to their school will write the wrong thing to the wrong people.
 *
 * The school separation is enforced in the database (actor-scoped RLS), not
 * here; this page only has to be honest about it.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";

import { SuggestionCard } from "~/components/suggestions/suggestion-card";
import { SuggestionForm } from "~/components/suggestions/suggestion-form";
import { EmptyState } from "~/components/ui/empty-state";
import { ConfirmationModal } from "~/components/ui/modal";
import { Skeleton } from "~/components/ui/skeleton";
import { CustomSelect } from "~/components/ui/custom-select";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ConceptPageHeader } from "~/components/ui/concept-section-header";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import type { Suggestion, SortOption } from "~/lib/suggestions-helpers";
import {
  createFeedback,
  deleteFeedback,
  fetchFeedback,
  fetchFeedbackSchools,
  parentFeedbackBoardApi,
  updateFeedback,
  PARENT_FEEDBACK_UNREAD_EVENT,
  type FeedbackSchool,
} from "~/lib/parent-feedback-api";

const logger = createLogger({ component: "ParentFeedbackPage" });

export default function ParentFeedbackPage() {
  const t = useTranslations("parentFeedback");
  const { success: toastSuccess, error: toastError } = useToast();

  const [schools, setSchools] = useState<FeedbackSchool[] | null>(null);
  const [schoolId, setSchoolId] = useState<string>("");
  const [posts, setPosts] = useState<Suggestion[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  const [boardReloadVersion, setBoardReloadVersion] = useState(0);

  const [sortBy, setSortBy] = useState<SortOption>("score");
  const [formOpen, setFormOpen] = useState(false);
  const [editPost, setEditPost] = useState<Suggestion | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Suggestion | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  const statusLabels = useMemo(
    () => ({
      open: t("statusOpen"),
      planned: t("statusPlanned"),
      in_progress: t("statusInProgress"),
      done: t("statusDone"),
      rejected: t("statusRejected"),
      need_info: t("statusNeedInfo"),
    }),
    [t],
  );

  const menuLabels = useMemo(
    () => ({
      edit: t("edit"),
      delete: t("delete"),
      actions: t("actions"),
    }),
    [t],
  );

  const loadSchools = useCallback(async () => {
    setLoading(true);
    setLoadError(false);
    setSchools(null);
    setSchoolId("");
    setPosts([]);

    try {
      const list = await fetchFeedbackSchools();
      setSchools(list);
      setSchoolId((current) =>
        list.some((school) => school.id === current)
          ? current
          : (list[0]?.id ?? ""),
      );
      setBoardReloadVersion((current) => current + 1);
      if (list.length === 0) setLoading(false);
    } catch (err) {
      logger.error("parent_feedback_schools_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setLoadError(true);
      setLoading(false);
    }
  }, []);

  // Load the boards the guardian may use, then pick the first one.
  useEffect(() => {
    void loadSchools();
  }, [loadSchools]);

  const reload = useCallback(async () => {
    if (!schoolId) return;
    const list = await fetchFeedback(schoolId, sortBy);
    setPosts(list);
  }, [schoolId, sortBy]);

  useEffect(() => {
    if (!schoolId) return;
    let active = true;
    setLoading(true);
    fetchFeedback(schoolId, sortBy)
      .then((list) => {
        if (active) {
          setPosts(list);
          setLoadError(false);
        }
      })
      .catch((err: unknown) => {
        logger.error("parent_feedback_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (active) setLoadError(true);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [schoolId, sortBy, boardReloadVersion]);

  const boardApi = useMemo(() => parentFeedbackBoardApi(schoolId), [schoolId]);

  const formApi = useMemo(
    () => ({
      create: (title: string, description: string) =>
        createFeedback(schoolId, title, description),
      update: (id: string, title: string, description: string) =>
        updateFeedback(schoolId, id, title, description),
    }),
    [schoolId],
  );

  const handleVoteChange = useCallback((updated: Suggestion) => {
    setPosts((current) =>
      current.map((post) => (post.id === updated.id ? updated : post)),
    );
  }, []);

  const handleDelete = useCallback(async () => {
    if (!deleteTarget) return;
    setIsDeleting(true);
    try {
      await deleteFeedback(schoolId, deleteTarget.id);
      toastSuccess(t("deleteSuccess"));
      setDeleteTarget(null);
      await reload();
      window.dispatchEvent(new CustomEvent(PARENT_FEEDBACK_UNREAD_EVENT));
    } catch (err) {
      logger.error("parent_feedback_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      toastError(t("deleteError"));
    } finally {
      setIsDeleting(false);
    }
  }, [deleteTarget, schoolId, reload, toastSuccess, toastError, t]);

  const handleFormSuccess = useCallback(() => {
    void reload();
  }, [reload]);

  const noSchools = schools !== null && schools.length === 0;
  const boardReady = Boolean(schoolId) && !loading && !loadError;

  // Same shell as the other parents-portal pages (see /parents/news): full
  // width up to max-w-7xl, header in its own card.
  return (
    <div className="mx-auto w-full max-w-7xl space-y-6">
      <header className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm sm:p-6">
        <ConceptPageHeader
          title={t("title")}
          eyebrow={t("kicker")}
          concept="feedback"
          subtitle={
            <>
              <p>{t("productTeamNotice")}</p>
              <p className="text-gray-500">{t("pseudonymNotice")}</p>
            </>
          }
        />
      </header>

      {loadError && (
        <div className="flex flex-wrap items-center gap-3">
          <Alert type="error" message={t("loadError")} />
          <Button
            type="button"
            onClick={() => void loadSchools()}
            variant="outline"
            size="compact"
          >
            {t("retry")}
          </Button>
        </div>
      )}

      {noSchools && (
        <EmptyState
          icon={<MotoConceptIcon concept="feedback" size={48} />}
          title={t("noSchoolsTitle")}
          description={t("noSchoolsDescription")}
        />
      )}

      {!noSchools && (
        <>
          <div className="flex flex-wrap items-end gap-3">
            {schools && schools.length > 1 && (
              <div className="min-w-48">
                <label
                  id="feedback-school-label"
                  htmlFor="feedback-school"
                  className="mb-1 block text-xs font-medium text-gray-500"
                >
                  {t("schoolLabel")}
                </label>
                <CustomSelect
                  id="feedback-school"
                  labelId="feedback-school-label"
                  value={schoolId}
                  onChange={setSchoolId}
                  options={schools.map((school) => ({
                    value: school.id,
                    label: school.name,
                  }))}
                />
              </div>
            )}
            <div className="min-w-40">
              <label
                id="feedback-sort-label"
                htmlFor="feedback-sort"
                className="mb-1 block text-xs font-medium text-gray-500"
              >
                {t("sortLabel")}
              </label>
              <CustomSelect
                id="feedback-sort"
                labelId="feedback-sort-label"
                value={sortBy}
                onChange={(value) => setSortBy(value as SortOption)}
                options={[
                  { value: "score", label: t("sortPopular") },
                  { value: "newest", label: t("sortNewest") },
                ]}
              />
            </div>
            {boardReady && (
              <Button
                type="button"
                onClick={() => {
                  setEditPost(null);
                  setFormOpen(true);
                }}
                className="ml-auto"
                size="compact"
              >
                {t("newEntry")}
              </Button>
            )}
          </div>

          {loading && (
            <div className="space-y-4">
              <Skeleton className="h-32 w-full rounded-2xl" />
              <Skeleton className="h-32 w-full rounded-2xl" />
            </div>
          )}

          {boardReady && posts.length === 0 && (
            <EmptyState
              icon={<MotoConceptIcon concept="feedback" size={48} />}
              title={t("emptyTitle")}
              description={t("emptyDescription")}
              action={
                <Button
                  type="button"
                  onClick={() => setFormOpen(true)}
                  size="compact"
                >
                  {t("newEntry")}
                </Button>
              }
            />
          )}

          {boardReady && posts.length > 0 && (
            <div className="space-y-4">
              {posts.map((post) => (
                <SuggestionCard
                  key={post.id}
                  suggestion={post}
                  currentAccountId=""
                  onEdit={(entry) => {
                    setEditPost(entry);
                    setFormOpen(true);
                  }}
                  onDelete={setDeleteTarget}
                  onVoteChange={handleVoteChange}
                  api={boardApi}
                  unreadRefreshEvent={PARENT_FEEDBACK_UNREAD_EVENT}
                  statusLabels={statusLabels}
                  menuLabels={menuLabels}
                />
              ))}
            </div>
          )}
        </>
      )}

      <SuggestionForm
        isOpen={formOpen}
        onClose={() => {
          setFormOpen(false);
          setEditPost(null);
        }}
        onSuccess={handleFormSuccess}
        editSuggestion={editPost}
        api={formApi}
        titlePlaceholder={t("titlePlaceholder")}
        descriptionPlaceholder={t("descriptionPlaceholder")}
        hint={
          <p className="rounded-lg bg-gray-50 p-3 text-sm text-gray-600">
            {t("formHint")}
          </p>
        }
      />

      <ConfirmationModal
        isOpen={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => {
          void handleDelete();
        }}
        title={t("deleteTitle")}
        confirmText={t("delete")}
        confirmButtonClass="bg-moto-red hover:bg-moto-red-hover"
        isConfirmLoading={isDeleting}
      >
        <p className="text-sm text-gray-600">{t("deleteBody")}</p>
      </ConfirmationModal>
    </div>
  );
}

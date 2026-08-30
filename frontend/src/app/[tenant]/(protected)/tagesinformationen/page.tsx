"use client";

import { Megaphone, Pencil, Trash2 } from "lucide-react";
import { useSession } from "next-auth/react";
import { useState } from "react";

import { StaffNoticeModal } from "~/components/staff-notices/staff-notice-modal";
import { TodayNoticeList } from "~/components/staff-notices/today-notice-list";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { EmptyState } from "~/components/ui/empty-state";
import { Loading } from "~/components/ui/loading";
import { StatusBadge } from "~/components/ui/status-badge";
import { getApiErrorMessage } from "~/lib/api-error-message";
import { hasEffectiveAdminScope } from "~/lib/auth-utils";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  createStaffNotice,
  deleteStaffNotice,
  describeRecurrence,
  fetchStaffNotices,
  fetchTodaysNotices,
  updateStaffNotice,
} from "~/lib/staff-notices-api";
import type { StaffNotice, StaffNoticeInput } from "~/lib/staff-notices-api";
import { useSWRAuth } from "~/lib/swr";

const logger = createLogger({ component: "TagesinformationenPage" });

/**
 * Tagesinformationen (#2180): interne Hinweise der Leitung an das Team.
 *
 * Eine Seite, zwei Rollen. Oben steht für ALLE Mitarbeitenden, was heute gilt
 * (mit Kenntnisnahme); darunter verwalten Admins den Bestand. Ein Hinweis ist
 * EINE Zeile mit einer Wiederholungsregel, keine Reihe von Tageseinträgen —
 * deshalb zeigt die Liste die Regel im Klartext ("Di · Woche B") statt einer
 * Terminliste. Die Route /api/staff-notices (Liste, Anlegen) ist im Backend
 * adminexklusiv; hier wird nur gespiegelt, was dort gilt.
 */
export default function TagesinformationenPage() {
  const { data: session } = useSession();
  const isAdmin = hasEffectiveAdminScope(session);

  const {
    data: todayData,
    error: todayError,
    isLoading: todayLoading,
    mutate: mutateToday,
  } = useSWRAuth<StaffNotice[]>("staff-notices-today", fetchTodaysNotices, {
    revalidateOnFocus: false,
  });

  const {
    data,
    error: noticesError,
    isLoading,
    mutate,
  } = useSWRAuth<StaffNotice[]>(
    isAdmin ? "staff-notices" : null,
    fetchStaffNotices,
    { revalidateOnFocus: false },
  );

  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<StaffNotice | null>(null);
  const [deleting, setDeleting] = useState<StaffNotice | null>(null);
  const [deleteError, setDeleteError] = useState("");
  const [deletePending, setDeletePending] = useState(false);
  const [listError, setListError] = useState("");

  const todayNotices = todayData ?? [];
  const notices = data ?? [];
  const visibleListError =
    listError ||
    (noticesError
      ? getApiErrorMessage(
          noticesError,
          "laden",
          "die Tagesinformationen",
          "Die Tagesinformationen konnten nicht geladen werden.",
        )
      : "");

  const save = async (input: StaffNoticeInput) => {
    if (editing) {
      await updateStaffNotice(editing.id, input);
    } else {
      await createStaffNotice(input);
    }
    await Promise.all([mutate(), mutateToday()]);
  };

  const confirmDelete = async () => {
    if (!deleting) return;
    setDeletePending(true);
    setDeleteError("");
    try {
      await deleteStaffNotice(deleting.id);
      await Promise.all([mutate(), mutateToday()]);
      setDeleting(null);
    } catch (err) {
      logger.error("staff_notice_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setDeleteError(
        getApiErrorMessage(
          err,
          "löschen",
          "die Tagesinformation",
          "Die Tagesinformation konnte nicht gelöscht werden.",
        ),
      );
    } finally {
      setDeletePending(false);
    }
  };

  return (
    <div className="w-full">
      <header className="moto-content-surface mb-4 rounded-2xl border p-5 shadow-sm backdrop-blur-md">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div className="min-w-0">
            <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
              Tagesinformationen
            </p>
            <h1 className="mt-1 text-xl font-bold text-gray-900 sm:text-2xl">
              Hinweise an das Team
            </h1>
            <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
              Interne Hinweise der Leitung für das ganze Team. Einmalig, für
              einen Zeitraum oder wiederkehrend an bestimmten Wochentagen.
            </p>
          </div>
          {isAdmin && (
            <Button
              type="button"
              variant="primary"
              size="md"
              onClick={() => {
                setEditing(null);
                setModalOpen(true);
              }}
            >
              Neue Tagesinformation
            </Button>
          )}
        </div>
      </header>

      {/* Heute: das, was jede Mitarbeiterin braucht. Steht deshalb vor der
          Verwaltung und ist die ganze Seite für Nicht-Admins. */}
      <section className="moto-content-surface mb-4 rounded-2xl border p-4 shadow-sm sm:p-5">
        <h2 className="text-base font-semibold text-gray-900">Heute</h2>
        {todayLoading && todayNotices.length === 0 ? (
          <Loading fullPage={false} />
        ) : todayError ? (
          // Ein Ladefehler darf nicht wie "keine Hinweise" aussehen: dann
          // verlässt sich jemand auf eine leere Tafel, die nur nicht geladen war.
          <div className="mt-3">
            <Alert
              type="error"
              message={getApiErrorMessage(
                todayError,
                "laden",
                "die Tagesinformationen",
                "Die Tagesinformationen konnten nicht geladen werden.",
              )}
            />
          </div>
        ) : todayNotices.length === 0 ? (
          <p className="mt-1 text-sm text-gray-600">
            Für heute liegen keine Hinweise vor.
          </p>
        ) : (
          <div className="mt-3">
            <TodayNoticeList notices={todayNotices} onChanged={mutateToday} />
          </div>
        )}
      </section>

      {isAdmin && (
        <h2 className="mb-3 text-base font-semibold text-gray-900">
          Alle Tagesinformationen
        </h2>
      )}

      {visibleListError && <Alert type="error" message={visibleListError} />}

      {!isAdmin ? null : isLoading && notices.length === 0 ? (
        <Loading fullPage={false} />
      ) : noticesError ? null : notices.length === 0 ? (
        <EmptyState
          icon={<Megaphone className="h-6 w-6" />}
          title="Noch keine Tagesinformationen"
          description="Hinweise wie „Jeden Dienstag ist die Turnhalle bis 15 Uhr belegt“ sehen damit alle Mitarbeitenden unter Kommunikation -> Tagesinformationen."
        />
      ) : (
        <ul className="space-y-3">
          {notices.map((notice) => (
            <li
              key={notice.id}
              className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-5"
            >
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-sm font-semibold text-gray-900">
                      {notice.title}
                    </span>
                    {notice.priority === "important" && (
                      <StatusBadge label="Wichtig" tone="orange" />
                    )}
                    {!notice.active && (
                      <StatusBadge label="Abgeschaltet" tone="gray" />
                    )}
                  </div>
                  {notice.body && (
                    <p className="mt-1 text-sm leading-relaxed whitespace-pre-line text-gray-600">
                      {notice.body}
                    </p>
                  )}
                  <p className="mt-2 text-xs text-gray-500">
                    {describeRecurrence(notice)} · ab{" "}
                    {formatDate(notice.valid_from)}
                    {notice.valid_until
                      ? ` bis ${formatDate(notice.valid_until)}`
                      : " (unbefristet)"}
                  </p>
                  {notice.requires_acknowledgement && (
                    <p className="mt-1 text-xs text-gray-500">
                      Kenntnisnahme verlangt ·{" "}
                      {notice.acknowledged_count === 1
                        ? "1 Person hat bestätigt"
                        : `${notice.acknowledged_count} Personen haben bestätigt`}
                    </p>
                  )}
                </div>

                <div className="flex flex-shrink-0 items-center gap-1">
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    aria-label="Bearbeiten"
                    onClick={() => {
                      setEditing(notice);
                      setModalOpen(true);
                    }}
                  >
                    <Pencil className="h-4 w-4" />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    aria-label="Löschen"
                    onClick={() => {
                      setDeleting(notice);
                      setDeleteError("");
                    }}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}

      <StaffNoticeModal
        isOpen={modalOpen}
        notice={editing}
        onClose={() => setModalOpen(false)}
        onSubmit={async (input) => {
          try {
            await save(input);
            setListError("");
          } catch (err) {
            logger.error("staff_notice_save_failed", {
              error: err instanceof Error ? err.message : String(err),
            });
            throw err;
          }
        }}
      />

      <ConfirmDeleteModal
        isOpen={deleting !== null}
        title="Tagesinformation löschen"
        description={
          deleting
            ? `„${deleting.title}“ wird für alle entfernt. Bereits erfasste Kenntnisnahmen gehen mit verloren.`
            : ""
        }
        gate={{ mode: "twoStep", firstStepLabel: "Löschen" }}
        loading={deletePending}
        error={deleteError}
        onConfirm={confirmDelete}
        onClose={() => setDeleting(null)}
      />
    </div>
  );
}

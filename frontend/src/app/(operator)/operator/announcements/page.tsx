"use client";

import { useState, useMemo, useCallback } from "react";
import { AnimatePresence, LayoutGroup, motion } from "framer-motion";
import useSWR from "swr";
import {
  PageHeaderWithSearch,
  type FilterConfig,
} from "~/components/ui/page-header";
import { Modal, ConfirmationModal } from "~/components/ui/modal";
import { Skeleton } from "~/components/ui/skeleton";
import { useOperatorAuth } from "~/lib/operator/auth-context";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { operatorAnnouncementsService } from "~/lib/operator/announcements-api";
import {
  TYPE_LABELS,
  TYPE_STYLES,
  SEVERITY_LABELS,
  SEVERITY_STYLES,
  ANNOUNCEMENT_STATUS_LABELS,
  ANNOUNCEMENT_STATUS_STYLES,
} from "~/lib/operator/announcements-helpers";
import type {
  Announcement,
  AnnouncementType,
  AnnouncementSeverity,
  CreateAnnouncementRequest,
  UpdateAnnouncementRequest,
} from "~/lib/operator/announcements-helpers";

function getRelativeTime(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const minutes = Math.floor(diff / 60000);
  if (minutes < 1) return "gerade eben";
  if (minutes < 60)
    return `vor ${minutes} ${minutes === 1 ? "Minute" : "Minuten"}`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `vor ${hours} ${hours === 1 ? "Stunde" : "Stunden"}`;
  const days = Math.floor(hours / 24);
  if (days < 7) return `vor ${days} ${days === 1 ? "Tag" : "Tagen"}`;
  const weeks = Math.floor(days / 7);
  if (weeks < 5) return `vor ${weeks} ${weeks === 1 ? "Woche" : "Wochen"}`;
  const months = Math.floor(days / 30);
  if (months < 12) return `vor ${months} ${months === 1 ? "Monat" : "Monaten"}`;
  const years = Math.floor(days / 365);
  return `vor ${years} ${years === 1 ? "Jahr" : "Jahren"}`;
}

interface FormData {
  title: string;
  content: string;
  type: AnnouncementType;
  severity: AnnouncementSeverity;
  version: string;
  expiresAt: string;
}

const EMPTY_FORM: FormData = {
  title: "",
  content: "",
  type: "announcement",
  severity: "info",
  version: "",
  expiresAt: "",
};

export default function OperatorAnnouncementsPage() {
  const { isAuthenticated } = useOperatorAuth();
  useSetBreadcrumb({ pageTitle: "Ankündigungen" });
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [formOpen, setFormOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<Announcement | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Announcement | null>(null);
  const [publishTarget, setPublishTarget] = useState<Announcement | null>(null);
  const [formData, setFormData] = useState<FormData>(EMPTY_FORM);
  const [isSaving, setIsSaving] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [isPublishing, setIsPublishing] = useState(false);

  const {
    data: announcements,
    isLoading,
    mutate,
  } = useSWR(isAuthenticated ? "operator-announcements" : null, () =>
    operatorAnnouncementsService.fetchAll(),
  );

  const filteredAnnouncements = useMemo(() => {
    if (!announcements) return [];
    if (statusFilter === "all") return announcements;
    return announcements.filter((a) => a.status === statusFilter);
  }, [announcements, statusFilter]);

  const openCreateForm = useCallback(() => {
    setEditTarget(null);
    setFormData(EMPTY_FORM);
    setFormOpen(true);
  }, []);

  const openEditForm = useCallback((announcement: Announcement) => {
    setEditTarget(announcement);
    setFormData({
      title: announcement.title,
      content: announcement.content,
      type: announcement.type,
      severity: announcement.severity,
      version: announcement.version ?? "",
      expiresAt: announcement.expiresAt
        ? new Date(announcement.expiresAt).toISOString().slice(0, 16)
        : "",
    });
    setFormOpen(true);
  }, []);

  const handleSave = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!formData.title.trim() || !formData.content.trim()) return;
      setIsSaving(true);
      try {
        if (editTarget) {
          const updateData: UpdateAnnouncementRequest = {
            title: formData.title,
            content: formData.content,
            type: formData.type,
            severity: formData.severity,
            version: formData.version || null,
            expires_at: formData.expiresAt || null,
          };
          await operatorAnnouncementsService.update(editTarget.id, updateData);
        } else {
          const createData: CreateAnnouncementRequest = {
            title: formData.title,
            content: formData.content,
            type: formData.type,
            severity: formData.severity,
            ...(formData.version && { version: formData.version }),
            ...(formData.expiresAt && { expires_at: formData.expiresAt }),
          };
          await operatorAnnouncementsService.create(createData);
        }
        setFormOpen(false);
        setEditTarget(null);
        await mutate();
      } catch (error) {
        console.error("Failed to save announcement:", error);
      } finally {
        setIsSaving(false);
      }
    },
    [formData, editTarget, mutate],
  );

  const handleDelete = useCallback(async () => {
    if (!deleteTarget) return;
    setIsDeleting(true);
    try {
      await operatorAnnouncementsService.delete(deleteTarget.id);
      setDeleteTarget(null);
      await mutate();
    } catch (error) {
      console.error("Failed to delete announcement:", error);
    } finally {
      setIsDeleting(false);
    }
  }, [deleteTarget, mutate]);

  const handlePublish = useCallback(async () => {
    if (!publishTarget) return;
    setIsPublishing(true);
    try {
      await operatorAnnouncementsService.publish(publishTarget.id);
      setPublishTarget(null);
      await mutate();
    } catch (error) {
      console.error("Failed to publish announcement:", error);
    } finally {
      setIsPublishing(false);
    }
  }, [publishTarget, mutate]);

  const filterConfigs: FilterConfig[] = [
    {
      id: "status",
      label: "Status",
      type: "dropdown",
      value: statusFilter,
      onChange: (value) => setStatusFilter(value as string),
      options: [
        { value: "all", label: "Alle" },
        ...Object.entries(ANNOUNCEMENT_STATUS_LABELS).map(([value, label]) => ({
          value,
          label,
        })),
      ],
    },
  ];

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        title="Ankündigungen"
        badge={
          announcements
            ? { count: announcements.length, label: "Gesamt" }
            : undefined
        }
        filters={filterConfigs}
        actionButton={
          <button
            type="button"
            onClick={openCreateForm}
            className="rounded-full bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700"
          >
            Neue Ankündigung
          </button>
        }
        mobileActionButton={
          <button
            type="button"
            onClick={openCreateForm}
            className="rounded-full bg-gray-900 p-2 text-white transition-colors hover:bg-gray-700"
            aria-label="Neue Ankündigung"
          >
            <svg
              className="h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M12 4v16m8-8H4"
              />
            </svg>
          </button>
        }
      />

      {isLoading && <AnnouncementSkeletons />}
      {!isLoading && filteredAnnouncements.length === 0 && (
        <div className="flex flex-col items-center gap-3 py-12 text-center">
          <svg
            className="h-12 w-12 text-gray-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={1.5}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M11 5.882V19.24a1.76 1.76 0 01-3.417.592l-2.147-6.15M18 13a3 3 0 100-6M5.436 13.683A4.001 4.001 0 017 6h1.832c4.1 0 7.625-1.234 9.168-3v14c-1.543-1.766-5.067-3-9.168-3H7a3.988 3.988 0 01-1.564-.317z"
            />
          </svg>
          <p className="text-lg font-medium text-gray-900">
            Keine Ankündigungen
          </p>
          <p className="text-sm text-gray-500">
            Erstellen Sie eine neue Ankündigung, um Nutzer zu informieren.
          </p>
          <button
            type="button"
            onClick={openCreateForm}
            className="mt-2 rounded-full bg-gray-900 px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700"
          >
            Neue Ankündigung
          </button>
        </div>
      )}

      {!isLoading && filteredAnnouncements.length > 0 && (
        <LayoutGroup>
          <div className="mt-4 space-y-4">
            <AnimatePresence>
              {filteredAnnouncements.map((announcement) => (
                <motion.div
                  key={announcement.id}
                  layout
                  transition={{ type: "spring", stiffness: 500, damping: 35 }}
                >
                  <AnnouncementCard
                    announcement={announcement}
                    onEdit={openEditForm}
                    onDelete={setDeleteTarget}
                    onPublish={setPublishTarget}
                  />
                </motion.div>
              ))}
            </AnimatePresence>
          </div>
        </LayoutGroup>
      )}

      {/* Create/Edit Form Modal */}
      <Modal
        isOpen={formOpen}
        onClose={() => {
          setFormOpen(false);
          setEditTarget(null);
        }}
        title={editTarget ? "Ankündigung bearbeiten" : "Neue Ankündigung"}
        footer={
          <>
            <button
              type="button"
              onClick={() => {
                setFormOpen(false);
                setEditTarget(null);
              }}
              className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
            >
              Abbrechen
            </button>
            <button
              type="button"
              onClick={(e) => void handleSave(e)}
              disabled={
                isSaving || !formData.title.trim() || !formData.content.trim()
              }
              className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {isSaving
                ? "Wird gespeichert..."
                : editTarget
                  ? "Speichern"
                  : "Erstellen"}
            </button>
          </>
        }
      >
        <form
          onSubmit={(e) => void handleSave(e)}
          className="space-y-4"
          id="announcement-form"
        >
          {/* Title */}
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">
              Titel
            </label>
            <input
              type="text"
              value={formData.title}
              onChange={(e) =>
                setFormData((prev) => ({ ...prev, title: e.target.value }))
              }
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
              required
            />
          </div>

          {/* Content */}
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">
              Inhalt
            </label>
            <textarea
              value={formData.content}
              onChange={(e) =>
                setFormData((prev) => ({ ...prev, content: e.target.value }))
              }
              rows={5}
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
              required
            />
          </div>

          {/* Type */}
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">
              Typ
            </label>
            <div className="flex gap-2">
              {(
                Object.entries(TYPE_LABELS) as [AnnouncementType, string][]
              ).map(([value, label]) => (
                <button
                  key={value}
                  type="button"
                  onClick={() =>
                    setFormData((prev) => ({ ...prev, type: value }))
                  }
                  className={`rounded-lg px-3 py-1.5 text-sm font-medium transition-colors ${
                    formData.type === value
                      ? "bg-gray-900 text-white"
                      : "bg-gray-100 text-gray-700 hover:bg-gray-200"
                  }`}
                >
                  {label}
                </button>
              ))}
            </div>
          </div>

          {/* Severity */}
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">
              Dringlichkeit
            </label>
            <select
              value={formData.severity}
              onChange={(e) =>
                setFormData((prev) => ({
                  ...prev,
                  severity: e.target.value as AnnouncementSeverity,
                }))
              }
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
            >
              {Object.entries(SEVERITY_LABELS).map(([value, label]) => (
                <option key={value} value={value}>
                  {label}
                </option>
              ))}
            </select>
          </div>

          {/* Version (only for release type) */}
          {formData.type === "release" && (
            <div>
              <label className="mb-1 block text-sm font-medium text-gray-700">
                Version
              </label>
              <input
                type="text"
                value={formData.version}
                onChange={(e) =>
                  setFormData((prev) => ({ ...prev, version: e.target.value }))
                }
                placeholder="z.B. 2.1.0"
                className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
              />
            </div>
          )}

          {/* Expires at */}
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">
              Ablaufdatum (optional)
            </label>
            <input
              type="datetime-local"
              value={formData.expiresAt}
              onChange={(e) =>
                setFormData((prev) => ({ ...prev, expiresAt: e.target.value }))
              }
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
            />
          </div>
        </form>
      </Modal>

      {/* Delete confirmation */}
      <ConfirmationModal
        isOpen={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => void handleDelete()}
        title="Ankündigung löschen?"
        confirmText="Löschen"
        confirmButtonClass="bg-red-500 hover:bg-red-600"
        isConfirmLoading={isDeleting}
      >
        <p className="text-sm text-gray-600">
          Die Ankündigung &quot;{deleteTarget?.title}&quot; wird unwiderruflich
          gelöscht.
        </p>
      </ConfirmationModal>

      {/* Publish confirmation */}
      <ConfirmationModal
        isOpen={!!publishTarget}
        onClose={() => setPublishTarget(null)}
        onConfirm={() => void handlePublish()}
        title="Ankündigung veröffentlichen?"
        confirmText="Veröffentlichen"
        confirmButtonClass="bg-green-600 hover:bg-green-700"
        isConfirmLoading={isPublishing}
      >
        <p className="text-sm text-gray-600">
          Die Ankündigung &quot;{publishTarget?.title}&quot; wird für alle
          Nutzer sichtbar.
        </p>
      </ConfirmationModal>
    </div>
  );
}

function AnnouncementCard({
  announcement,
  onEdit,
  onDelete,
  onPublish,
}: {
  readonly announcement: Announcement;
  readonly onEdit: (a: Announcement) => void;
  readonly onDelete: (a: Announcement) => void;
  readonly onPublish: (a: Announcement) => void;
}) {
  return (
    <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-5 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-150">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0 flex-1">
          <h3 className="text-base font-semibold text-gray-900">
            {announcement.title}
          </h3>
          <div className="mt-1 flex flex-wrap items-center gap-2">
            <span
              className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${TYPE_STYLES[announcement.type]}`}
            >
              {TYPE_LABELS[announcement.type]}
            </span>
            <span
              className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${SEVERITY_STYLES[announcement.severity]}`}
            >
              {SEVERITY_LABELS[announcement.severity]}
            </span>
            <span
              className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${ANNOUNCEMENT_STATUS_STYLES[announcement.status]}`}
            >
              {ANNOUNCEMENT_STATUS_LABELS[announcement.status]}
            </span>
            {announcement.version && (
              <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
                v{announcement.version}
              </span>
            )}
          </div>
        </div>
      </div>
      <p className="mt-2 line-clamp-2 text-sm text-gray-600">
        {announcement.content}
      </p>
      <div className="mt-3 flex items-center justify-between">
        <span className="text-xs text-gray-500">
          {getRelativeTime(announcement.createdAt)}
          {announcement.publishedAt &&
            ` · Veröffentlicht ${getRelativeTime(announcement.publishedAt)}`}
        </span>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => onEdit(announcement)}
            className="rounded-lg px-3 py-1.5 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-100"
          >
            Bearbeiten
          </button>
          {announcement.status === "draft" && (
            <button
              type="button"
              onClick={() => onPublish(announcement)}
              className="rounded-lg bg-green-50 px-3 py-1.5 text-xs font-medium text-green-700 transition-colors hover:bg-green-100"
            >
              Veröffentlichen
            </button>
          )}
          <button
            type="button"
            onClick={() => onDelete(announcement)}
            className="rounded-lg px-3 py-1.5 text-xs font-medium text-red-600 transition-colors hover:bg-red-50"
          >
            Löschen
          </button>
        </div>
      </div>
    </div>
  );
}

function AnnouncementSkeletons() {
  return (
    <div className="mt-4 space-y-4">
      {Array.from({ length: 3 }, (_, i) => (
        <div
          key={i}
          className="rounded-3xl border border-gray-100/50 bg-white/90 p-5 shadow-[0_8px_30px_rgb(0,0,0,0.12)]"
        >
          <div className="space-y-3">
            <Skeleton className="h-5 w-3/5 rounded" />
            <div className="flex gap-2">
              <Skeleton className="h-5 w-20 rounded-full" />
              <Skeleton className="h-5 w-16 rounded-full" />
              <Skeleton className="h-5 w-24 rounded-full" />
            </div>
            <Skeleton className="h-4 w-full rounded" />
            <Skeleton className="h-4 w-4/5 rounded" />
          </div>
        </div>
      ))}
    </div>
  );
}

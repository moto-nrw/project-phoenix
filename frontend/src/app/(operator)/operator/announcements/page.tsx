"use client";

import { useState, useMemo, useCallback, useRef, useEffect } from "react";
import { AnimatePresence, LayoutGroup, motion } from "framer-motion";
import useSWR from "swr";
import { MoreVertical, Pencil, Trash2, Send, Check } from "lucide-react";
import {
  PageHeaderWithSearch,
  type FilterConfig,
} from "~/components/ui/page-header";
import { Modal, ConfirmationModal } from "~/components/ui/modal";
import { Skeleton } from "~/components/ui/skeleton";
import { DatePicker } from "~/components/ui/date-picker";
import { useOperatorAuth } from "~/lib/operator/auth-context";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { operatorAnnouncementsService } from "~/lib/operator/announcements-api";
import {
  TYPE_LABELS,
  TYPE_TEXT_COLORS,
  SEVERITY_LABELS,
  ANNOUNCEMENT_STATUS_LABELS,
  SYSTEM_ROLE_LABELS,
} from "~/lib/operator/announcements-helpers";
import type {
  Announcement,
  AnnouncementType,
  AnnouncementSeverity,
  SystemRole,
  AnnouncementStats,
  CreateAnnouncementRequest,
  UpdateAnnouncementRequest,
} from "~/lib/operator/announcements-helpers";
import { AnnouncementViewsAccordion } from "~/components/operator/announcement-views-accordion";
import { getRelativeTime } from "~/lib/format-utils";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OperatorAnnouncementsPage" });

interface FormData {
  title: string;
  content: string;
  type: AnnouncementType;
  severity: AnnouncementSeverity;
  version: string;
  expiresAt: string;
  targetRoles: SystemRole[];
}

const EMPTY_FORM: FormData = {
  title: "",
  content: "",
  type: "announcement",
  severity: "info",
  version: "",
  expiresAt: "",
  targetRoles: [],
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
  const [severityDropdownOpen, setSeverityDropdownOpen] = useState(false);
  const severityDropdownRef = useRef<HTMLDivElement>(null);

  const {
    data: announcements,
    isLoading,
    mutate,
  } = useSWR(
    isAuthenticated ? "operator-announcements" : null,
    () => operatorAnnouncementsService.fetchAll(),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const filteredAnnouncements = useMemo(() => {
    if (!announcements) return [];
    if (statusFilter === "all") return announcements;
    return announcements.filter((a) => a.status === statusFilter);
  }, [announcements, statusFilter]);

  // Close severity dropdown on click outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (
        severityDropdownRef.current &&
        !severityDropdownRef.current.contains(event.target as Node)
      ) {
        setSeverityDropdownOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

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
      expiresAt: announcement.expiresAt ?? "",
      targetRoles: announcement.targetRoles,
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
            target_roles: formData.targetRoles,
          };
          await operatorAnnouncementsService.update(editTarget.id, updateData);
        } else {
          const createData: CreateAnnouncementRequest = {
            title: formData.title,
            content: formData.content,
            type: formData.type,
            severity: formData.severity,
            target_roles: formData.targetRoles,
            ...(formData.version && { version: formData.version }),
            ...(formData.expiresAt && { expires_at: formData.expiresAt }),
          };
          await operatorAnnouncementsService.create(createData);
        }
        setFormOpen(false);
        setEditTarget(null);
        await mutate();
      } catch (error) {
        logger.error("announcement_save_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
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
      logger.error("announcement_delete_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
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
      logger.error("announcement_publish_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
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
          announcements && announcements.length > 0 ? (
            <button
              type="button"
              onClick={openCreateForm}
              className="rounded-full bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700"
            >
              Neue Ankündigung
            </button>
          ) : undefined
        }
        mobileActionButton={
          announcements && announcements.length > 0 ? (
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
          ) : undefined
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
            <label
              htmlFor="announcement-title"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Titel
            </label>
            <input
              id="announcement-title"
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
            <label
              htmlFor="announcement-content"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Inhalt
            </label>
            <textarea
              id="announcement-content"
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
            <span
              id="announcement-type-label"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Typ
            </span>
            <div
              className="flex flex-wrap gap-2"
              role="group"
              aria-labelledby="announcement-type-label"
            >
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
            <span
              id="announcement-severity-label"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Dringlichkeit
            </span>
            <div
              className="relative"
              ref={severityDropdownRef}
              aria-labelledby="announcement-severity-label"
            >
              <button
                type="button"
                onClick={() => setSeverityDropdownOpen(!severityDropdownOpen)}
                className={`flex w-full items-center justify-between rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-all ${
                  severityDropdownOpen
                    ? "border-gray-300 bg-gray-50"
                    : "hover:bg-gray-50"
                }`}
              >
                <span className="text-gray-900">
                  {SEVERITY_LABELS[formData.severity]}
                </span>
                <svg
                  className={`h-4 w-4 text-gray-400 transition-transform ${
                    severityDropdownOpen ? "rotate-180" : ""
                  }`}
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M19 9l-7 7-7-7"
                  />
                </svg>
              </button>
              {severityDropdownOpen && (
                <div className="absolute top-full left-0 z-[10001] mt-1 w-full rounded-xl border border-gray-200 bg-white py-1 shadow-lg">
                  {Object.entries(SEVERITY_LABELS).map(([value, label]) => (
                    <button
                      key={value}
                      type="button"
                      onClick={() => {
                        setFormData((prev) => ({
                          ...prev,
                          severity: value as AnnouncementSeverity,
                        }));
                        setSeverityDropdownOpen(false);
                      }}
                      className={`flex w-full items-center px-4 py-2 text-left text-sm transition-colors hover:bg-gray-50 ${
                        formData.severity === value
                          ? "bg-gray-50 font-medium text-gray-900"
                          : "text-gray-700"
                      }`}
                    >
                      {label}
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>

          {/* Version (only for release type) */}
          {formData.type === "release" && (
            <div>
              <label
                htmlFor="announcement-version"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                Version
              </label>
              <input
                id="announcement-version"
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
            <span
              id="announcement-expires-label"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Ablaufdatum (optional)
            </span>
            <DatePicker
              value={formData.expiresAt ? new Date(formData.expiresAt) : null}
              onChange={(date) =>
                setFormData((prev) => ({
                  ...prev,
                  expiresAt: date ? date.toISOString() : "",
                }))
              }
              placeholder="Datum auswählen"
            />
          </div>

          {/* Target Roles */}
          <div>
            <span
              id="announcement-roles-label"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Zielgruppen
            </span>
            <p className="mb-2 text-xs text-gray-500">
              Leer = Alle Benutzer sehen die Ankündigung
            </p>
            <div
              className="flex flex-wrap gap-3"
              role="group"
              aria-labelledby="announcement-roles-label"
            >
              {(["admin", "user", "guardian"] as const).map((role) => {
                const isChecked = formData.targetRoles.includes(role);
                return (
                  <button
                    key={role}
                    type="button"
                    onClick={() => {
                      if (isChecked) {
                        setFormData((prev) => ({
                          ...prev,
                          targetRoles: prev.targetRoles.filter(
                            (r) => r !== role,
                          ),
                        }));
                      } else {
                        setFormData((prev) => ({
                          ...prev,
                          targetRoles: [...prev.targetRoles, role],
                        }));
                      }
                    }}
                    className={`flex items-center gap-2 rounded-lg border px-3 py-2 text-sm transition-all ${
                      isChecked
                        ? "border-[#83CD2D] bg-[#83CD2D]/10 text-gray-900"
                        : "border-gray-200 bg-white text-gray-600 hover:border-gray-300 hover:bg-gray-50"
                    }`}
                  >
                    <span
                      className={`flex h-4 w-4 items-center justify-center rounded border transition-all ${
                        isChecked
                          ? "border-[#83CD2D] bg-[#83CD2D]"
                          : "border-gray-300 bg-white"
                      }`}
                    >
                      {isChecked && (
                        <Check className="h-3 w-3 text-white" strokeWidth={3} />
                      )}
                    </span>
                    {SYSTEM_ROLE_LABELS[role]}
                  </button>
                );
              })}
            </div>
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
        confirmButtonClass="bg-gradient-to-br from-[#83CD2D] to-[#70b525] hover:shadow-lg"
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
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  // Close menu on click outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setMenuOpen(false);
      }
    }
    if (menuOpen) {
      document.addEventListener("mousedown", handleClickOutside);
      return () =>
        document.removeEventListener("mousedown", handleClickOutside);
    }
  }, [menuOpen]);

  return (
    <div className="relative rounded-3xl border border-gray-100/50 bg-white/90 p-5 pr-12 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-150">
      {/* Kebab menu - absolute top right */}
      <div className="absolute top-3 right-3" ref={menuRef}>
        <button
          type="button"
          onClick={() => setMenuOpen(!menuOpen)}
          className="rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600"
          aria-label="Menü öffnen"
        >
          <MoreVertical className="h-5 w-5" />
        </button>

        {menuOpen && (
          <div className="absolute top-full right-0 z-50 mt-1 w-40 rounded-xl border border-gray-100 bg-white py-1 shadow-lg">
            <button
              type="button"
              onClick={() => {
                setMenuOpen(false);
                onEdit(announcement);
              }}
              className="flex w-full items-center gap-2 px-4 py-2 text-left text-sm text-gray-700 transition-colors hover:bg-gray-50"
            >
              <Pencil className="h-4 w-4" />
              Bearbeiten
            </button>
            <button
              type="button"
              onClick={() => {
                setMenuOpen(false);
                onDelete(announcement);
              }}
              className="flex w-full items-center gap-2 px-4 py-2 text-left text-sm text-red-600 transition-colors hover:bg-red-50"
            >
              <Trash2 className="h-4 w-4" />
              Löschen
            </button>
          </div>
        )}
      </div>

      {/* Type label */}
      <p
        className={`mb-2 text-xs font-medium tracking-wide uppercase ${TYPE_TEXT_COLORS[announcement.type]}`}
      >
        {TYPE_LABELS[announcement.type]}
      </p>

      {/* Title with draft badge */}
      <div className="flex items-center gap-2">
        <h3 className="text-base font-semibold text-gray-900">
          {announcement.title}
        </h3>
        {announcement.status === "draft" && (
          <span className="rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-700">
            Entwurf
          </span>
        )}
      </div>

      {/* Meta line: version and timestamp */}
      <div className="mt-1 flex items-center gap-2 text-xs text-gray-500">
        {announcement.version && (
          <>
            <span>v{announcement.version}</span>
            <span className="text-gray-300">·</span>
          </>
        )}
        <span>{getRelativeTime(announcement.createdAt)}</span>
      </div>

      {/* Target roles display */}
      {announcement.targetRoles.length > 0 && (
        <div className="mt-1.5 flex items-center gap-1.5 text-xs text-gray-400">
          <svg
            className="h-3.5 w-3.5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
            />
          </svg>
          <span>
            {announcement.targetRoles
              .map((r) => SYSTEM_ROLE_LABELS[r])
              .join(", ")}
          </span>
        </div>
      )}

      {/* Content preview */}
      <p className="mt-2 line-clamp-2 text-sm text-gray-600">
        {announcement.content}
      </p>

      {/* Footer with publish button for drafts */}
      {announcement.status === "draft" && (
        <div className="mt-4 -mr-7 flex justify-end">
          <button
            type="button"
            onClick={() => onPublish(announcement)}
            className="group flex items-center gap-2 rounded-xl bg-gradient-to-r from-[#83CD2D] to-[#6db823] px-5 py-2 text-sm font-medium text-white shadow-md transition-all hover:shadow-lg hover:brightness-105 active:scale-[0.98]"
          >
            <span>Veröffentlichen</span>
            <Send className="h-4 w-4 transition-transform group-hover:translate-x-0.5" />
          </button>
        </div>
      )}

      {/* Published timestamp */}
      {announcement.status === "published" && announcement.publishedAt && (
        <p className="mt-3 text-xs text-gray-400">
          Veröffentlicht {getRelativeTime(announcement.publishedAt)}
        </p>
      )}

      {/* Views accordion at the bottom */}
      {announcement.status === "published" && (
        <AnnouncementViewsAccordionWrapper announcementId={announcement.id} />
      )}
    </div>
  );
}

function AnnouncementViewsAccordionWrapper({
  announcementId,
}: {
  readonly announcementId: string;
}) {
  const { data: stats } = useSWR<AnnouncementStats>(
    `announcement-stats-${announcementId}`,
    () => operatorAnnouncementsService.fetchStats(announcementId),
    { refreshInterval: 30000 },
  );

  if (!stats || (stats.seen_count === 0 && stats.dismissed_count === 0)) {
    return null;
  }

  return (
    <AnnouncementViewsAccordion
      announcementId={announcementId}
      dismissedCount={stats.dismissed_count}
    />
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

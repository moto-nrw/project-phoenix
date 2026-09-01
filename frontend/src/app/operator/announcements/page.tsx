"use client";

import { useState, useMemo, useCallback, useRef, useEffect } from "react";
import { AnimatePresence, LayoutGroup, motion } from "framer-motion";
// eslint-disable-next-line no-restricted-imports -- operator pages use useOperatorAuth, not NextAuth
import useSWR from "swr";
import { Pencil, Trash2, Send, Check } from "lucide-react";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import type { FilterConfig } from "~/components/ui/page-header/types";
import { Modal, ConfirmationModal } from "~/components/ui/modal";
import { Skeleton } from "~/components/ui/skeleton";
import { SkeletonRegion } from "~/components/ui/page-skeletons";
import { DatePicker } from "~/components/ui/date-picker";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { operatorAnnouncementsService } from "~/lib/operator/announcements-api";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
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
import type { Organization, School } from "~/lib/operator/provisioning-helpers";
import { AnnouncementViewsAccordion } from "~/components/operator/announcement-views-accordion";
import { getRelativeTime } from "~/lib/format-utils";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { useCurrentTimestamp } from "~/lib/hooks/use-current-timestamp";

const logger = createLogger({ component: "OperatorAnnouncementsPage" });

interface FormData {
  title: string;
  content: string;
  type: AnnouncementType;
  severity: AnnouncementSeverity;
  version: string;
  expiresAt: string;
  targetRoles: SystemRole[];
  targetOrgIds: string[];
  targetTenantIds: string[];
}

const EMPTY_FORM: FormData = {
  title: "",
  content: "",
  type: "announcement",
  severity: "info",
  version: "",
  expiresAt: "",
  targetRoles: [],
  targetOrgIds: [],
  targetTenantIds: [],
};

export default function OperatorAnnouncementsPage() {
  useSetBreadcrumb({ pageTitle: "Ankündigungen" });
  const { success: toastSuccess, error: toastError } = useToast();
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

  // SWR key is unconditional — OperatorAuthGuard already ensures
  // this page only renders when the session is authenticated.
  // Using useSession().status here caused data loss during token refresh
  // (status briefly becomes "loading" → SWR key becomes null → data cleared).
  const {
    data: announcements,
    isLoading,
    mutate,
  } = useSWR("operator-announcements", () =>
    operatorAnnouncementsService.fetchAll(),
  );

  const { data: organizations } = useSWR(
    "operator-organizations",
    () => operatorProvisioningService.listOrganizations(),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const { data: schools } = useSWR(
    "operator-schools",
    () => operatorProvisioningService.listSchools(),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  // Soft-deleted orgs stay in `organizations` so the card can still resolve their
  // names for historical targeting, but the picker below must hide them so new
  // announcements cannot be targeted at trashed orgs.
  const activeOrganizations = useMemo(
    () => organizations?.filter((o) => o.deletedAt == null) ?? [],
    [organizations],
  );

  const deletedOrgIdSet = useMemo(
    () =>
      new Set(
        (organizations ?? [])
          .filter((o) => o.deletedAt != null)
          .map((o) => o.id),
      ),
    [organizations],
  );

  // Historical targets (deleted orgs, or tenants whose school no longer appears
  // in the picker) MUST NOT be silently stripped from the form. In the backend,
  // empty target_org_ids/target_tenant_ids means "visible globally", so pruning
  // a scoped announcement down to [] would republish it to every tenant.
  // Instead, preserve them and surface a warning so the operator can see the
  // situation; the backend rejects NEW additions of deleted-org/school IDs via
  // CountByIDs, and allows existing IDs through via a diff on update.

  // Filter schools by selected orgs (if any orgs selected, show only their schools).
  // Soft-deleted schools are excluded from the picker — the backend rejects new
  // tenant targets that reference deleted schools via CountByIDs. Historical
  // targets on existing announcements are preserved separately (see prune effect
  // below and staleTenantTargets warning).
  const availableSchools = useMemo(() => {
    if (!schools) return [];
    const active = schools.filter((s) => s.deletedAt == null);
    if (formData.targetOrgIds.length === 0) return active;
    return active.filter((s) =>
      formData.targetOrgIds.includes(s.organizationId),
    );
  }, [schools, formData.targetOrgIds]);

  // Prune orphaned tenant selections when schools data loads or org selection changes.
  // This covers the race condition where org toggles happen before schools are fetched.
  const orgIdKey = formData.targetOrgIds.join(",");
  useEffect(() => {
    if (!schools || orgIdKey === "") return;
    setFormData((prev) => {
      const schoolsInSelectedOrgs = new Set(
        schools
          .filter(
            (s) =>
              s.deletedAt == null &&
              prev.targetOrgIds.includes(s.organizationId),
          )
          .map((s) => s.id),
      );
      // Keep IDs that are (a) in a selected org's active schools, (b) unknown
      // entirely (not in schools), or (c) soft-deleted. The latter two are
      // historical targets — stripping them would silently widen the announcement
      // to global scope, since empty arrays mean "all tenants" in the backend.
      const pruned = prev.targetTenantIds.filter((tid) => {
        if (schoolsInSelectedOrgs.has(tid)) return true;
        const school = schools.find((s) => s.id === tid);
        if (!school) return true;
        return school.deletedAt != null;
      });
      if (pruned.length === prev.targetTenantIds.length) return prev;
      return { ...prev, targetTenantIds: pruned };
    });
  }, [schools, orgIdKey]);

  const filteredAnnouncements = useMemo(() => {
    if (!announcements) return [];
    if (statusFilter === "all") return announcements;
    return announcements.filter((a) => a.status === statusFilter);
  }, [announcements, statusFilter]);

  // Identify selected targets that reference soft-deleted orgs or orphaned
  // schools (school soft-deleted, or its org soft-deleted). These IDs stay in
  // the form so save does not widen scope, but operators need to see them.
  const staleOrgTargets = useMemo(() => {
    if (!organizations) return [] as { id: string; name: string }[];
    return formData.targetOrgIds
      .filter((id) => deletedOrgIdSet.has(id))
      .map((id) => ({
        id,
        name: organizations.find((o) => o.id === id)?.name ?? `#${id}`,
      }));
  }, [formData.targetOrgIds, deletedOrgIdSet, organizations]);

  const staleTenantTargets = useMemo(() => {
    if (!schools) return [] as { id: string; name: string }[];
    return formData.targetTenantIds
      .map((tid) => {
        const school = schools.find((s) => s.id === tid);
        if (!school) return { id: tid, name: `#${tid}`, stale: true };
        if (school.deletedAt != null) {
          return { id: tid, name: school.name, stale: true };
        }
        if (deletedOrgIdSet.has(school.organizationId)) {
          return { id: tid, name: school.name, stale: true };
        }
        return { id: tid, name: school.name, stale: false };
      })
      .filter((t) => t.stale)
      .map(({ id, name }) => ({ id, name }));
  }, [formData.targetTenantIds, schools, deletedOrgIdSet]);

  const hasStaleTargets =
    staleOrgTargets.length > 0 || staleTenantTargets.length > 0;

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
    // Preserve historical targets verbatim — do NOT filter out IDs that now
    // point at soft-deleted orgs/schools. Stripping them would convert a
    // scoped announcement into a globally-visible one the moment the operator
    // saves any unrelated change, since empty target arrays mean "no filter"
    // in the backend. A warning banner surfaces the situation instead.
    setFormData({
      title: announcement.title,
      content: announcement.content,
      type: announcement.type,
      severity: announcement.severity,
      version: announcement.version ?? "",
      expiresAt: announcement.expiresAt ?? "",
      targetRoles: announcement.targetRoles,
      targetOrgIds: announcement.targetOrgIds ?? [],
      targetTenantIds: announcement.targetTenantIds ?? [],
    });
    setFormOpen(true);
  }, []);

  const handleSave = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!formData.title.trim() || !formData.content.trim()) return;
      setIsSaving(true);
      try {
        // Submit target IDs verbatim. Dropping deleted-org/school IDs here
        // would silently widen a scoped announcement to global (empty arrays
        // mean "visible to all" in the backend). The backend enforces the
        // soft-delete invariant server-side by rejecting NEW additions via
        // CountByIDs; historical targets pass through via a diff on update.
        const targetOrgIdsNum = formData.targetOrgIds
          .map((id) => parseInt(id, 10))
          .filter((id) => !isNaN(id));
        const targetTenantIdsNum = formData.targetTenantIds
          .map((id) => parseInt(id, 10))
          .filter((id) => !isNaN(id));

        if (editTarget) {
          const updateData: UpdateAnnouncementRequest = {
            title: formData.title,
            content: formData.content,
            type: formData.type,
            severity: formData.severity,
            version: formData.version || null,
            expires_at: formData.expiresAt || null,
            target_roles: formData.targetRoles,
            target_org_ids: targetOrgIdsNum,
            target_tenant_ids: targetTenantIdsNum,
          };
          await operatorAnnouncementsService.update(editTarget.id, updateData);
        } else {
          const createData: CreateAnnouncementRequest = {
            title: formData.title,
            content: formData.content,
            type: formData.type,
            severity: formData.severity,
            target_roles: formData.targetRoles,
            target_org_ids: targetOrgIdsNum,
            target_tenant_ids: targetTenantIdsNum,
            ...(formData.version && { version: formData.version }),
            ...(formData.expiresAt && { expires_at: formData.expiresAt }),
          };
          await operatorAnnouncementsService.create(createData);
        }
        toastSuccess(
          editTarget ? "Ankündigung gespeichert" : "Ankündigung erstellt",
        );
        setFormOpen(false);
        setEditTarget(null);
        // Revalidation is best-effort — don't let it mask the successful save
        mutate().catch((err) => {
          logger.warn("revalidation_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
        });
      } catch (error) {
        const msg = error instanceof Error ? error.message : String(error);
        logger.error("announcement_save_failed", { error: msg });
        toastError(`Fehler: ${msg}`);
      } finally {
        setIsSaving(false);
      }
    },
    [formData, editTarget, mutate, toastSuccess, toastError],
  );

  const handleDelete = useCallback(async () => {
    if (!deleteTarget) return;
    setIsDeleting(true);
    try {
      await operatorAnnouncementsService.delete(deleteTarget.id);
      toastSuccess("Ankündigung gelöscht");
      setDeleteTarget(null);
      // Revalidation is best-effort — don't let it mask the successful delete
      mutate().catch((err) => {
        logger.warn("revalidation_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    } catch (error) {
      const msg = error instanceof Error ? error.message : String(error);
      logger.error("announcement_delete_failed", { error: msg });
      toastError(`Fehler beim Löschen: ${msg}`);
    } finally {
      setIsDeleting(false);
    }
  }, [deleteTarget, mutate, toastSuccess, toastError]);

  const handlePublish = useCallback(async () => {
    if (!publishTarget) return;
    setIsPublishing(true);
    try {
      await operatorAnnouncementsService.publish(publishTarget.id);
      toastSuccess("Ankündigung veröffentlicht");
      setPublishTarget(null);
      // Revalidation is best-effort — don't let it mask the successful publish
      mutate().catch((err) => {
        logger.warn("revalidation_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    } catch (error) {
      const msg = error instanceof Error ? error.message : String(error);
      logger.error("announcement_publish_failed", { error: msg });
      toastError(`Fehler beim Veröffentlichen: ${msg}`);
    } finally {
      setIsPublishing(false);
    }
  }, [publishTarget, mutate, toastSuccess, toastError]);

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
        concept="announcements"
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

      {isLoading && (
        <SkeletonRegion label="Ankündigungen werden geladen">
          <AnnouncementSkeletons />
        </SkeletonRegion>
      )}
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
                    organizations={organizations}
                    schools={schools}
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
              maxLength={200}
              className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:outline-none"
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
              maxLength={5000}
              className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:outline-none"
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
                <div className="absolute top-full left-0 z-[10001] mt-1 w-full overflow-hidden rounded-xl border border-gray-200 bg-white py-1 shadow-lg">
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
                maxLength={255}
                className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:outline-none"
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
                        ? "border-moto-green bg-moto-green/10 text-gray-900"
                        : "border-gray-200 bg-white text-gray-600 hover:border-gray-300 hover:bg-gray-50"
                    }`}
                  >
                    <span
                      className={`flex h-4 w-4 items-center justify-center rounded border transition-all ${
                        isChecked
                          ? "border-moto-green bg-moto-green"
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

          {/* Deleted-target warning: surfaces historical org/tenant IDs that
              reference soft-deleted entities so the operator sees them before
              saving. These IDs are preserved verbatim to avoid widening scope. */}
          {hasStaleTargets && (
            <div
              role="alert"
              className="border-moto-orange/40 bg-moto-orange/10 rounded-lg border p-3 text-sm text-gray-800"
            >
              <p className="font-medium">
                Diese Ankündigung enthält gelöschte Ziele
              </p>
              <p className="mt-1 text-xs text-gray-700">
                Die folgenden Organisationen/Schulen wurden gelöscht und sind in
                den Auswahllisten ausgeblendet. Sie bleiben als Ziel erhalten,
                damit die Ankündigung nicht versehentlich global sichtbar wird,
                und tauchen wieder auf, falls das Ziel wiederhergestellt wird.
                Entfernen Sie ein Ziel explizit, wenn die Ankündigung nach einer
                Wiederherstellung nicht mehr für dieses Ziel gelten soll.
              </p>
              {staleOrgTargets.length > 0 && (
                <div className="mt-2 text-xs">
                  <span className="font-medium">Gelöschte Organisationen:</span>
                  <ul className="mt-1 space-y-1">
                    {staleOrgTargets.map((t) => (
                      <li
                        key={t.id}
                        className="flex items-center justify-between gap-2"
                      >
                        <span>{t.name}</span>
                        <button
                          type="button"
                          onClick={() =>
                            setFormData((prev) => ({
                              ...prev,
                              targetOrgIds: prev.targetOrgIds.filter(
                                (id) => id !== t.id,
                              ),
                            }))
                          }
                          className="text-moto-orange hover:bg-moto-orange/20 rounded px-2 py-0.5 text-xs font-medium"
                          aria-label={`${t.name} aus Zielen entfernen`}
                        >
                          Entfernen
                        </button>
                      </li>
                    ))}
                  </ul>
                </div>
              )}
              {staleTenantTargets.length > 0 && (
                <div className="mt-2 text-xs">
                  <span className="font-medium">Gelöschte Schulen:</span>
                  <ul className="mt-1 space-y-1">
                    {staleTenantTargets.map((t) => (
                      <li
                        key={t.id}
                        className="flex items-center justify-between gap-2"
                      >
                        <span>{t.name}</span>
                        <button
                          type="button"
                          onClick={() =>
                            setFormData((prev) => ({
                              ...prev,
                              targetTenantIds: prev.targetTenantIds.filter(
                                (id) => id !== t.id,
                              ),
                            }))
                          }
                          className="text-moto-orange hover:bg-moto-orange/20 rounded px-2 py-0.5 text-xs font-medium"
                          aria-label={`${t.name} aus Zielen entfernen`}
                        >
                          Entfernen
                        </button>
                      </li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
          )}

          {/* Target Organizations */}
          <div>
            <span
              id="announcement-orgs-label"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Organisationen
            </span>
            <p className="mb-2 text-xs text-gray-500">
              Leer = Alle Organisationen
            </p>
            <div
              className="flex flex-wrap gap-3"
              role="group"
              aria-labelledby="announcement-orgs-label"
            >
              {activeOrganizations.map((org) => {
                const orgId = org.id;
                const isChecked = formData.targetOrgIds.includes(orgId);
                return (
                  <button
                    key={org.id}
                    type="button"
                    onClick={() => {
                      if (isChecked) {
                        setFormData((prev) => ({
                          ...prev,
                          targetOrgIds: prev.targetOrgIds.filter(
                            (id) => id !== orgId,
                          ),
                          // Remove tenant selections that belonged to this org
                          targetTenantIds: schools
                            ? prev.targetTenantIds.filter(
                                (tid) =>
                                  !schools.some(
                                    (s) =>
                                      s.id === tid &&
                                      s.organizationId === orgId,
                                  ),
                              )
                            : prev.targetTenantIds,
                        }));
                      } else {
                        setFormData((prev) => {
                          const newOrgIds = [...prev.targetOrgIds, orgId];
                          return {
                            ...prev,
                            targetOrgIds: newOrgIds,
                            // Prune tenant selections to only schools belonging to selected orgs
                            targetTenantIds: schools
                              ? prev.targetTenantIds.filter((tid) =>
                                  schools.some(
                                    (s) =>
                                      s.id === tid &&
                                      newOrgIds.includes(s.organizationId),
                                  ),
                                )
                              : prev.targetTenantIds,
                          };
                        });
                      }
                    }}
                    className={`flex items-center gap-2 rounded-lg border px-3 py-2 text-sm transition-all ${
                      isChecked
                        ? "border-moto-green bg-moto-green/10 text-gray-900"
                        : "border-gray-200 bg-white text-gray-600 hover:border-gray-300 hover:bg-gray-50"
                    }`}
                  >
                    <span
                      className={`flex h-4 w-4 items-center justify-center rounded border transition-all ${
                        isChecked
                          ? "border-moto-green bg-moto-green"
                          : "border-gray-300 bg-white"
                      }`}
                    >
                      {isChecked && (
                        <Check className="h-3 w-3 text-white" strokeWidth={3} />
                      )}
                    </span>
                    {org.name}
                  </button>
                );
              })}
            </div>
          </div>

          {/* Target Schools/Tenants */}
          <div>
            <span
              id="announcement-tenants-label"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Schulen
            </span>
            <p className="mb-2 text-xs text-gray-500">
              Leer = Alle Schulen
              {formData.targetOrgIds.length > 0
                ? " der ausgewählten Organisationen"
                : ""}
            </p>
            <div
              className="flex flex-wrap gap-3"
              role="group"
              aria-labelledby="announcement-tenants-label"
            >
              {availableSchools.map((school) => {
                const schoolId = school.id;
                const isChecked = formData.targetTenantIds.includes(schoolId);
                return (
                  <button
                    key={school.id}
                    type="button"
                    onClick={() => {
                      if (isChecked) {
                        setFormData((prev) => ({
                          ...prev,
                          targetTenantIds: prev.targetTenantIds.filter(
                            (id) => id !== schoolId,
                          ),
                        }));
                      } else {
                        setFormData((prev) => ({
                          ...prev,
                          targetTenantIds: [...prev.targetTenantIds, schoolId],
                        }));
                      }
                    }}
                    className={`flex items-center gap-2 rounded-lg border px-3 py-2 text-sm transition-all ${
                      isChecked
                        ? "border-moto-green bg-moto-green/10 text-gray-900"
                        : "border-gray-200 bg-white text-gray-600 hover:border-gray-300 hover:bg-gray-50"
                    }`}
                  >
                    <span
                      className={`flex h-4 w-4 items-center justify-center rounded border transition-all ${
                        isChecked
                          ? "border-moto-green bg-moto-green"
                          : "border-gray-300 bg-white"
                      }`}
                    >
                      {isChecked && (
                        <Check className="h-3 w-3 text-white" strokeWidth={3} />
                      )}
                    </span>
                    {school.name}
                  </button>
                );
              })}
              {availableSchools.length === 0 && (
                <p className="text-xs text-gray-400 italic">
                  Keine Schulen verfügbar
                </p>
              )}
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
        confirmButtonClass="bg-moto-red hover:bg-moto-red-hover"
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
        confirmButtonClass="bg-moto-green hover:bg-moto-green-hover hover:shadow-lg"
        isConfirmLoading={isPublishing}
      >
        <p className="text-sm text-gray-600">
          {publishTarget &&
          ((publishTarget.targetOrgIds?.length ?? 0) > 0 ||
            (publishTarget.targetTenantIds?.length ?? 0) > 0)
            ? `Die Ankündigung "${publishTarget.title}" wird für die ausgewählten Organisationen/Schulen sichtbar.`
            : `Die Ankündigung "${publishTarget?.title}" wird für alle Nutzer sichtbar.`}
        </p>
      </ConfirmationModal>
    </div>
  );
}

function AnnouncementCard({
  announcement,
  organizations,
  schools,
  onEdit,
  onDelete,
  onPublish,
}: {
  readonly announcement: Announcement;
  readonly organizations?: Organization[];
  readonly schools?: School[];
  readonly onEdit: (a: Announcement) => void;
  readonly onDelete: (a: Announcement) => void;
  readonly onPublish: (a: Announcement) => void;
}) {
  const currentTimestamp = useCurrentTimestamp();
  const [expanded, setExpanded] = useState(false);
  const [isClamped, setIsClamped] = useState(false);
  const contentRef = useRef<HTMLParagraphElement>(null);

  useEffect(() => {
    const el = contentRef.current;
    if (el) {
      setIsClamped(el.scrollHeight > el.clientHeight);
    }
  }, [announcement.content]);

  return (
    <div className="moto-content-surface relative rounded-2xl border p-5 pr-12 transition-all duration-150">
      {/* Kebab menu - absolute top right */}
      <div className="absolute top-3 right-3">
        <OverflowMenu
          ariaLabel="Menü öffnen"
          items={[
            {
              label: "Bearbeiten",
              icon: <Pencil className="h-4 w-4" />,
              onClick: () => onEdit(announcement),
            },
            {
              label: "Löschen",
              icon: <Trash2 className="h-4 w-4" />,
              destructive: true,
              onClick: () => onDelete(announcement),
            },
          ]}
        />
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
          <span className="bg-moto-amber-soft text-moto-amber-strong rounded-full px-2 py-0.5 text-xs font-medium">
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

      {/* Target organizations display */}
      {(announcement.targetOrgIds?.length ?? 0) > 0 && organizations && (
        <div className="mt-1 flex items-center gap-1.5 text-xs text-gray-400">
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
              d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
            />
          </svg>
          <span>
            {announcement.targetOrgIds
              .map(
                (id) =>
                  organizations.find((o) => o.id === id)?.name ?? `Org ${id}`,
              )
              .join(", ")}
          </span>
        </div>
      )}

      {/* Target tenants/schools display */}
      {(announcement.targetTenantIds?.length ?? 0) > 0 && schools && (
        <div className="mt-1 flex items-center gap-1.5 text-xs text-gray-400">
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
              d="M12 14l9-5-9-5-9 5 9 5zm0 0l6.16-3.422a12.083 12.083 0 01.665 6.479A11.952 11.952 0 0012 20.055a11.952 11.952 0 00-6.824-2.998 12.078 12.078 0 01.665-6.479L12 14zm-4 6v-7.5l4-2.222"
            />
          </svg>
          <span>
            {announcement.targetTenantIds
              .map(
                (id) =>
                  schools.find((s) => s.id === id)?.name ?? `Schule ${id}`,
              )
              .join(", ")}
          </span>
        </div>
      )}

      {/* Content preview */}
      <p
        ref={contentRef}
        className={`mt-2 text-sm text-gray-600 ${expanded ? "" : "line-clamp-2"}`}
      >
        {announcement.content}
      </p>
      {(isClamped || expanded) && (
        <button
          type="button"
          onClick={() => setExpanded((prev) => !prev)}
          className="mt-1 text-xs font-medium text-gray-500 transition-colors hover:text-gray-700"
        >
          {expanded ? "Weniger anzeigen" : "Mehr anzeigen"}
        </button>
      )}

      {/* Footer with publish button for drafts */}
      {announcement.status === "draft" && (
        <div className="mt-4 -mr-7 flex justify-end">
          <button
            type="button"
            onClick={() => onPublish(announcement)}
            className="group bg-moto-green hover:bg-moto-green-hover flex items-center gap-2 rounded-xl px-5 py-2 text-sm font-medium text-white shadow-md transition-all hover:shadow-lg active:scale-[0.98]"
          >
            <span>Veröffentlichen</span>
            <Send className="h-4 w-4 transition-transform group-hover:translate-x-0.5" />
          </button>
        </div>
      )}

      {/* Published + expired timestamps */}
      {announcement.status !== "draft" && announcement.publishedAt && (
        <div className="mt-3 flex flex-wrap items-center gap-x-2 text-xs text-gray-400">
          <span>
            Veröffentlicht {getRelativeTime(announcement.publishedAt)}
          </span>
          {announcement.expiresAt && (
            <>
              <span className="text-gray-300">·</span>
              <span>
                {new Date(announcement.expiresAt).getTime() < currentTimestamp
                  ? `Abgelaufen ${getRelativeTime(announcement.expiresAt)}`
                  : `Läuft ab am ${new Date(announcement.expiresAt).toLocaleDateString("de-DE", { timeZone: "Europe/Berlin" })}`}
              </span>
            </>
          )}
        </div>
      )}

      {/* Views accordion at the bottom */}
      {announcement.status !== "draft" && (
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
        <div key={i} className="moto-content-surface rounded-2xl border p-5">
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

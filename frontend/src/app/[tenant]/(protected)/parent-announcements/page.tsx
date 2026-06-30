"use client";

import { Suspense, useEffect, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";
import {
  BarChart3,
  Check,
  Megaphone,
  Pencil,
  Plus,
  Send,
  Trash2,
  Undo2,
} from "lucide-react";

import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { FilterConfig, ActiveFilter } from "~/components/ui/page-header";
import { DataTable } from "~/components/ui/data-table";
import type { DataTableColumn } from "~/components/ui/data-table";
import { FormModal } from "~/components/ui/form-modal";
import { Modal } from "~/components/ui/modal";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Checkbox } from "~/components/ui/checkbox";
import { DatePicker } from "~/components/ui/date-picker";
import { Loading } from "~/components/ui/loading";
import { InfoCard, InfoItem } from "~/components/ui/info-card";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { useSWRAuth } from "~/lib/swr";
import { groupService, studentService } from "~/lib/api";
import type { Group, Student } from "~/lib/api";
import { fetchActivities } from "~/lib/activity-api";
import type { Activity } from "~/lib/activity-helpers";
import {
  createAnnouncement,
  deleteAnnouncement,
  fetchAnnouncements,
  fetchAnnouncementStats,
  publishAnnouncement,
  unpublishAnnouncement,
  updateAnnouncement,
} from "~/lib/parent-announcements-api";
import type {
  Announcement,
  AnnouncementInput,
  AnnouncementPriority,
  AnnouncementStats,
  AnnouncementStatus,
  AnnouncementTarget,
  AnnouncementTargetType,
} from "~/lib/parent-announcements-api";

const logger = createLogger({ component: "ParentAnnouncementsPage" });

const PRIORITY_OPTIONS: ReadonlyArray<{
  value: AnnouncementPriority;
  label: string;
}> = [
  { value: "info", label: "Info" },
  { value: "important", label: "Wichtig" },
];

const STATUS_META: Record<AnnouncementStatus, { label: string }> = {
  draft: { label: "Entwurf" },
  published: { label: "Veröffentlicht" },
  expired: { label: "Abgelaufen" },
};

const TARGET_TYPE_PLURAL: Record<AnnouncementTargetType, string> = {
  school_all: "Ganze Schule",
  pending_enrollment: "Offene Anmeldungen",
  class: "Klassen",
  group: "Gruppen",
  activity_group: "AGs",
  student: "Kinder",
};

/** Stable identity for a target so duplicates are rejected and React keys are unique. */
function targetKey(target: AnnouncementTarget): string {
  return `${target.target_type}:${target.ref_id ?? target.ref_text ?? ""}`;
}

/** Compact, name-free summary for the list table (e.g. "2 Gruppen, 1 Klasse"). */
function summarizeTargets(targets: AnnouncementTarget[]): string {
  if (targets.length === 0) return "—";
  if (targets.some((t) => t.target_type === "school_all"))
    return "Ganze Schule";

  const counts = new Map<AnnouncementTargetType, number>();
  for (const t of targets) {
    counts.set(t.target_type, (counts.get(t.target_type) ?? 0) + 1);
  }
  return [...counts.entries()]
    .map(([type, count]) => {
      if (type === "school_all" || type === "pending_enrollment") {
        return TARGET_TYPE_PLURAL[type];
      }
      return `${count} ${TARGET_TYPE_PLURAL[type]}`;
    })
    .join(", ");
}

/** End of the chosen calendar day as an instant. expires_at is a TIMESTAMPTZ. */
function endOfDayISO(date: Date): string {
  return new Date(
    date.getFullYear(),
    date.getMonth(),
    date.getDate(),
    23,
    59,
    59,
  ).toISOString();
}

function Pill({ label }: { label: string }) {
  return (
    <span className="inline-flex items-center rounded-full bg-gray-100 px-3 py-1 text-xs font-medium text-gray-700">
      {label}
    </span>
  );
}

export default function ParentAnnouncementsPage() {
  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <ParentAnnouncementsContent />
    </Suspense>
  );
}

function ParentAnnouncementsContent() {
  const [searchTerm, setSearchTerm] = useState("");
  const [statusFilter, setStatusFilter] = useState<"all" | AnnouncementStatus>(
    "all",
  );

  const [isFormOpen, setIsFormOpen] = useState(false);
  const [editing, setEditing] = useState<Announcement | null>(null);
  const [statsFor, setStatsFor] = useState<Announcement | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Announcement | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const [pendingActionId, setPendingActionId] = useState<string | null>(null);

  const {
    data: announcements,
    isLoading,
    error: loadError,
    mutate,
  } = useSWRAuth<Announcement[]>(
    "parent-announcements-list",
    () => fetchAnnouncements(true),
    { keepPreviousData: true, revalidateOnFocus: false },
  );

  // Targeting data sources — fetched only while the form is open.
  const { data: groups } = useSWRAuth<Group[]>(
    isFormOpen ? "parent-announcements-groups" : null,
    () => groupService.getGroups(),
    { revalidateOnFocus: false },
  );
  const { data: activities } = useSWRAuth<Activity[]>(
    isFormOpen ? "parent-announcements-activities" : null,
    () => fetchActivities(),
    { revalidateOnFocus: false },
  );
  const { data: schoolClasses } = useSWRAuth<string[]>(
    isFormOpen ? "parent-announcements-classes" : null,
    () => studentService.getSchoolClasses(),
    { revalidateOnFocus: false },
  );

  const list = useMemo(() => announcements ?? [], [announcements]);

  const filtered = useMemo(() => {
    const term = searchTerm.trim().toLowerCase();
    return list.filter((a) => {
      if (statusFilter !== "all" && a.status !== statusFilter) return false;
      if (term && !a.title.toLowerCase().includes(term)) return false;
      return true;
    });
  }, [list, searchTerm, statusFilter]);

  const filterConfigs: FilterConfig[] = useMemo(
    () => [
      {
        id: "status",
        label: "Status",
        type: "grid",
        value: statusFilter,
        onChange: (value) =>
          setStatusFilter(value as "all" | AnnouncementStatus),
        options: [
          { value: "all", label: "Alle", icon: "M4 6h16M4 12h16M4 18h16" },
          {
            value: "draft",
            label: "Entwürfe",
            icon: "M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z",
          },
          {
            value: "published",
            label: "Veröffentlicht",
            icon: "M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z",
          },
          {
            value: "expired",
            label: "Abgelaufen",
            icon: "M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z",
          },
        ],
      },
    ],
    [statusFilter],
  );

  const activeFilters: ActiveFilter[] = useMemo(() => {
    const result: ActiveFilter[] = [];
    if (searchTerm) {
      result.push({
        id: "search",
        label: `"${searchTerm}"`,
        onRemove: () => setSearchTerm(""),
      });
    }
    if (statusFilter !== "all") {
      result.push({
        id: "status",
        label: STATUS_META[statusFilter].label,
        onRemove: () => setStatusFilter("all"),
      });
    }
    return result;
  }, [searchTerm, statusFilter]);

  const openCreate = () => {
    setEditing(null);
    setIsFormOpen(true);
  };

  const openEdit = (announcement: Announcement) => {
    setEditing(announcement);
    setIsFormOpen(true);
  };

  const closeForm = () => {
    setIsFormOpen(false);
    setEditing(null);
  };

  const runStatusAction = async (
    announcement: Announcement,
    action: "publish" | "unpublish",
  ) => {
    setPendingActionId(announcement.id);
    try {
      if (action === "publish") {
        await publishAnnouncement(announcement.id);
      } else {
        await unpublishAnnouncement(announcement.id);
      }
      await mutate();
    } catch (err) {
      logger.error("announcement_status_action_failed", {
        action,
        error: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setPendingActionId(null);
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    setDeleteError("");
    try {
      await deleteAnnouncement(deleteTarget.id);
      await mutate();
      setDeleteTarget(null);
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Elternmitteilung konnte nicht gelöscht werden";
      setDeleteError(message);
      logger.error("announcement_delete_failed", { error: message });
    } finally {
      setDeleting(false);
    }
  };

  const columns: DataTableColumn<Announcement>[] = [
    {
      key: "title",
      header: "Titel",
      sortValue: (row) => row.title.toLowerCase(),
      render: (row) => (
        <div className="min-w-0">
          <p className="truncate font-semibold text-gray-900">{row.title}</p>
          <p className="mt-0.5 line-clamp-1 text-xs text-gray-500">
            {row.body}
          </p>
        </div>
      ),
    },
    {
      key: "priority",
      header: "Priorität",
      render: (row) =>
        row.priority === "important" ? (
          <Pill label="Wichtig" />
        ) : (
          <span className="text-xs text-gray-400">Info</span>
        ),
    },
    {
      key: "status",
      header: "Status",
      sortValue: (row) => row.status,
      render: (row) => <Pill label={STATUS_META[row.status].label} />,
    },
    {
      key: "targets",
      header: "Zielgruppen",
      render: (row) => (
        <span className="text-sm text-gray-700">
          {summarizeTargets(row.targets)}
        </span>
      ),
    },
    {
      key: "dates",
      header: "Veröffentlicht / Ablauf",
      render: (row) => (
        <div className="text-xs text-gray-600">
          <p>
            {row.published_at
              ? `Veröffentlicht ${formatDate(row.published_at)}`
              : "Noch nicht veröffentlicht"}
          </p>
          {row.expires_at && <p>Läuft ab {formatDate(row.expires_at)}</p>}
        </div>
      ),
    },
    {
      key: "actions",
      header: "",
      align: "right",
      render: (row) => (
        <div className="flex justify-end gap-1">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={() => openEdit(row)}
            aria-label="Bearbeiten"
            title="Bearbeiten"
          >
            <Pencil className="size-4" aria-hidden />
          </Button>
          {row.status === "draft" && (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              disabled={pendingActionId === row.id}
              onClick={() => void runStatusAction(row, "publish")}
              aria-label="Veröffentlichen"
              title="Veröffentlichen"
            >
              <Send className="size-4" aria-hidden />
            </Button>
          )}
          {row.status === "published" && (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              disabled={pendingActionId === row.id}
              onClick={() => void runStatusAction(row, "unpublish")}
              aria-label="Zurückziehen"
              title="Zurückziehen"
            >
              <Undo2 className="size-4" aria-hidden />
            </Button>
          )}
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={() => setStatsFor(row)}
            aria-label="Statistik"
            title="Statistik"
          >
            <BarChart3 className="size-4" aria-hidden />
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={() => {
              setDeleteError("");
              setDeleteTarget(row);
            }}
            aria-label="Löschen"
            title="Löschen"
            className="hover:text-[#FF3130]"
          >
            <Trash2 className="size-4" aria-hidden />
          </Button>
        </div>
      ),
    },
  ];

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        title="Elternmitteilungen"
        badge={{
          icon: <Megaphone className="h-5 w-5 text-gray-600" aria-hidden />,
          count: filtered.length,
        }}
        search={{
          value: searchTerm,
          onChange: setSearchTerm,
          placeholder: "Titel suchen...",
        }}
        filters={filterConfigs}
        activeFilters={activeFilters}
        onClearAllFilters={() => {
          setSearchTerm("");
          setStatusFilter("all");
        }}
        actionButton={
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={openCreate}
            aria-label="Neue Elternmitteilung erstellen"
            className="gap-1.5"
          >
            <Plus className="h-4 w-4" aria-hidden />
            Elternmitteilung
          </Button>
        }
      />

      {loadError && (
        <div className="mb-4 rounded-lg border border-[#FF3130]/30 bg-[#FF3130]/10 p-4 text-sm text-[#CC2626]">
          Elternmitteilungen konnten nicht geladen werden.
        </div>
      )}

      {isLoading ? (
        <Loading fullPage={false} />
      ) : filtered.length === 0 ? (
        <div className="py-12 text-center">
          <div className="flex flex-col items-center gap-4">
            <Megaphone className="h-12 w-12 text-gray-400" aria-hidden />
            <div>
              <h3 className="text-lg font-medium text-gray-900">
                Keine Elternmitteilungen
              </h3>
              <p className="text-gray-600">
                Erstellen Sie eine neue Mitteilung für die Eltern.
              </p>
            </div>
          </div>
        </div>
      ) : (
        <DataTable
          columns={columns}
          rows={filtered}
          getRowKey={(row) => row.id}
          defaultSortKey="title"
        />
      )}

      {isFormOpen && (
        <AnnouncementFormModal
          announcement={editing}
          groups={groups ?? []}
          activities={activities ?? []}
          schoolClasses={schoolClasses ?? []}
          onClose={closeForm}
          onSaved={async () => {
            await mutate();
            closeForm();
          }}
        />
      )}

      {statsFor && (
        <StatsModal announcement={statsFor} onClose={() => setStatsFor(null)} />
      )}

      {deleteTarget && (
        <ConfirmDeleteModal
          isOpen={Boolean(deleteTarget)}
          title="Elternmitteilung löschen"
          description={
            <>
              Möchten Sie die Elternmitteilung
              <span className="font-medium text-gray-900">
                {" "}
                „{deleteTarget.title}“{" "}
              </span>
              wirklich löschen? Dies kann nicht rückgängig gemacht werden.
            </>
          }
          gate={{ mode: "twoStep" }}
          onConfirm={confirmDelete}
          onClose={() => {
            setDeleteTarget(null);
            setDeleteError("");
          }}
          loading={deleting}
          error={deleteError}
        />
      )}
    </div>
  );
}

interface AnnouncementFormModalProps {
  readonly announcement: Announcement | null;
  readonly groups: Group[];
  readonly activities: Activity[];
  readonly schoolClasses: string[];
  readonly onClose: () => void;
  readonly onSaved: () => Promise<void> | void;
}

function AnnouncementFormModal({
  announcement,
  groups,
  activities,
  schoolClasses,
  onClose,
  onSaved,
}: AnnouncementFormModalProps) {
  const isEdit = announcement !== null;

  const [title, setTitle] = useState(announcement?.title ?? "");
  const [body, setBody] = useState(announcement?.body ?? "");
  const [priority, setPriority] = useState<AnnouncementPriority>(
    announcement?.priority ?? "info",
  );
  const [requiresAck, setRequiresAck] = useState(
    announcement?.requires_acknowledgement ?? false,
  );
  const [sendEmail, setSendEmail] = useState(announcement?.send_email ?? false);
  const [expiresAt, setExpiresAt] = useState<Date | null>(
    announcement?.expires_at ? new Date(announcement.expires_at) : null,
  );
  const [targets, setTargets] = useState<AnnouncementTarget[]>(
    announcement?.targets ?? [],
  );
  const [studentNames, setStudentNames] = useState<Record<string, string>>({});

  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState("");

  const handleSubmit = async () => {
    if (!title.trim()) {
      setFormError("Bitte einen Titel eingeben.");
      return;
    }
    if (!body.trim()) {
      setFormError("Bitte einen Text eingeben.");
      return;
    }
    if (targets.length === 0) {
      setFormError("Bitte mindestens eine Zielgruppe hinzufügen.");
      return;
    }

    const input: AnnouncementInput = {
      title: title.trim(),
      body: body.trim(),
      priority,
      requires_acknowledgement: requiresAck,
      send_email: sendEmail,
      expires_at: expiresAt ? endOfDayISO(expiresAt) : null,
      targets,
    };

    setSubmitting(true);
    setFormError("");
    try {
      if (isEdit && announcement) {
        await updateAnnouncement(announcement.id, input);
      } else {
        await createAnnouncement(input);
      }
      await onSaved();
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Elternmitteilung konnte nicht gespeichert werden";
      setFormError(message);
      logger.error("announcement_save_failed", {
        mode: isEdit ? "update" : "create",
        error: message,
      });
    } finally {
      setSubmitting(false);
    }
  };

  const footer = (
    <>
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={onClose}
        disabled={submitting}
      >
        Abbrechen
      </Button>
      <Button
        type="button"
        size="md"
        onClick={() => void handleSubmit()}
        isLoading={submitting}
        loadingText="Wird gespeichert..."
      >
        {isEdit ? "Speichern" : "Erstellen"}
      </Button>
    </>
  );

  return (
    <FormModal
      isOpen
      onClose={onClose}
      title={isEdit ? "Elternmitteilung bearbeiten" : "Neue Elternmitteilung"}
      footer={footer}
      size="xl"
    >
      <div className="space-y-5">
        <Input
          label="Titel"
          name="announcement-title"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="z. B. Sommerfest am Freitag"
        />

        <div>
          <label
            htmlFor="announcement-body"
            className="mb-2 block text-sm font-medium text-gray-700"
          >
            Text
          </label>
          <textarea
            id="announcement-body"
            value={body}
            onChange={(e) => setBody(e.target.value)}
            rows={4}
            maxLength={4000}
            placeholder="Inhalt der Mitteilung..."
            className="block w-full rounded-lg border-0 bg-white px-4 py-3 text-base text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400"
          />
        </div>

        <div>
          <span
            id="announcement-priority-label"
            className="mb-1.5 block text-sm font-medium text-gray-700"
          >
            Priorität
          </span>
          <div
            className="flex flex-wrap gap-2"
            role="group"
            aria-labelledby="announcement-priority-label"
          >
            {PRIORITY_OPTIONS.map((opt) => (
              <button
                key={opt.value}
                type="button"
                onClick={() => setPriority(opt.value)}
                className={`rounded-lg px-3 py-1.5 text-sm font-medium transition-colors ${
                  priority === opt.value
                    ? "bg-gray-900 text-white"
                    : "bg-gray-100 text-gray-700 hover:bg-gray-200"
                }`}
              >
                {opt.label}
              </button>
            ))}
          </div>
        </div>

        <div>
          <span className="mb-1.5 block text-sm font-medium text-gray-700">
            Ablaufdatum (optional)
          </span>
          <DatePicker
            value={expiresAt}
            onChange={setExpiresAt}
            placeholder="Kein Ablaufdatum"
            dropdownPlacement="down"
          />
        </div>

        <div className="space-y-3">
          <label
            htmlFor="announcement-ack"
            className="flex cursor-pointer items-start gap-3"
          >
            <Checkbox
              id="announcement-ack"
              checked={requiresAck}
              onChange={(e) => setRequiresAck(e.target.checked)}
            />
            <span>
              <span className="block text-sm font-medium text-gray-700">
                Lesebestätigung erforderlich
              </span>
              <span className="block text-xs text-gray-500">
                Eltern bestätigen ausdrücklich, dass sie die Mitteilung gelesen
                haben.
              </span>
            </span>
          </label>

          <label
            htmlFor="announcement-email"
            className="flex cursor-pointer items-start gap-3"
          >
            <Checkbox
              id="announcement-email"
              checked={sendEmail}
              onChange={(e) => setSendEmail(e.target.checked)}
            />
            <span>
              <span className="block text-sm font-medium text-gray-700">
                Eltern zusätzlich per E-Mail benachrichtigen
              </span>
              <span className="block text-xs text-gray-500">
                Beim Veröffentlichen erhalten die erreichten Eltern eine E-Mail.
                Die Mitteilung erscheint immer auch im Elternportal.
              </span>
            </span>
          </label>
        </div>

        <TargetingSection
          targets={targets}
          groups={groups}
          activities={activities}
          schoolClasses={schoolClasses}
          studentNames={studentNames}
          onChange={setTargets}
          onSetStudentName={(id, name) =>
            setStudentNames((prev) => ({ ...prev, [id]: name }))
          }
        />

        {formError && (
          <p role="alert" className="text-sm text-[#CC2626]">
            {formError}
          </p>
        )}
      </div>
    </FormModal>
  );
}

/** Toggle a value in/out of a string list (used by the targeting pills). */
function toggleValue(values: string[], v: string): string[] {
  return values.includes(v) ? values.filter((x) => x !== v) : [...values, v];
}

/**
 * Multi-select toggle pill, mirroring the operator announcement modal's target
 * selector (a bordered button with an inline check-box; active = brand green).
 * The whole targeting UX uses this so it reads identically to the sibling
 * operator feature.
 */
function CheckPill({
  label,
  active,
  onClick,
}: {
  readonly label: string;
  readonly active: boolean;
  readonly onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex w-full min-w-0 items-center gap-2 rounded-lg border px-3 py-2 text-left text-sm transition-all ${
        active
          ? "border-gray-900 bg-gray-50 text-gray-900"
          : "border-gray-200 bg-white text-gray-600 hover:border-gray-300 hover:bg-gray-50"
      }`}
    >
      <span
        className={`flex h-4 w-4 shrink-0 items-center justify-center rounded border transition-all ${
          active ? "border-gray-900 bg-gray-900" : "border-gray-300 bg-white"
        }`}
      >
        {active && <Check className="h-3 w-3 text-white" strokeWidth={3} />}
      </span>
      <span className="truncate">{label}</span>
    </button>
  );
}

/** A labelled group of targeting pills (label + wrapping pill row). */
function PillCategory({
  label,
  children,
}: {
  readonly label: string;
  readonly children: ReactNode;
}) {
  return (
    <div>
      <span className="mb-1.5 block text-sm font-medium text-gray-700">
        {label}
      </span>
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">{children}</div>
    </div>
  );
}

interface TargetingSectionProps {
  readonly targets: AnnouncementTarget[];
  readonly groups: Group[];
  readonly activities: Activity[];
  readonly schoolClasses: string[];
  readonly studentNames: Record<string, string>;
  readonly onChange: (targets: AnnouncementTarget[]) => void;
  readonly onSetStudentName: (id: string, name: string) => void;
}

function TargetingSection({
  targets,
  groups,
  activities,
  schoolClasses,
  studentNames,
  onChange,
  onSetStudentName,
}: TargetingSectionProps) {
  // Single source of truth is `targets`; each control derives its selection
  // from it and rebuilds it on change.
  const schoolAll = targets.some((t) => t.target_type === "school_all");
  const pendingEnrollment = targets.some(
    (t) => t.target_type === "pending_enrollment",
  );
  const selectedClasses = targets
    .filter((t) => t.target_type === "class")
    .map((t) => t.ref_text ?? "");
  const selectedGroups = targets
    .filter((t) => t.target_type === "group")
    .map((t) => t.ref_id ?? "");
  const selectedActivities = targets
    .filter((t) => t.target_type === "activity_group")
    .map((t) => t.ref_id ?? "");
  const selectedStudents = targets.filter((t) => t.target_type === "student");

  const toggleSimple = (
    targetType: "school_all" | "pending_enrollment",
    on: boolean,
  ) => {
    const without = targets.filter((t) => t.target_type !== targetType);
    onChange(on ? [...without, { target_type: targetType }] : without);
  };

  const setCategory = (
    targetType: "class" | "group" | "activity_group",
    values: string[],
  ) => {
    const others = targets.filter((t) => t.target_type !== targetType);
    const next = values.map<AnnouncementTarget>((v) =>
      targetType === "class"
        ? { target_type: "class", ref_text: v }
        : { target_type: targetType, ref_id: v },
    );
    onChange([...others, ...next]);
  };

  const addStudent = (student: Student) => {
    onSetStudentName(student.id, student.name);
    if (selectedStudents.some((t) => t.ref_id === student.id)) return;
    onChange([...targets, { target_type: "student", ref_id: student.id }]);
  };

  const removeStudent = (id: string) => {
    onChange(
      targets.filter((t) => !(t.target_type === "student" && t.ref_id === id)),
    );
  };

  // Students are too many to list, so this stays a debounced search; the chosen
  // children render as removable chips below it.
  const [studentSearch, setStudentSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const searchTimeout = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    if (searchTimeout.current) clearTimeout(searchTimeout.current);
    searchTimeout.current = setTimeout(
      () => setDebouncedSearch(studentSearch.trim()),
      300,
    );
    return () => {
      if (searchTimeout.current) clearTimeout(searchTimeout.current);
    };
  }, [studentSearch]);

  const { data: studentResults, isLoading: studentsLoading } = useSWRAuth<
    Student[]
  >(
    debouncedSearch.length >= 2
      ? `parent-announcements-student-search-${debouncedSearch}`
      : null,
    async () => {
      const result = await studentService.getStudents({
        search: debouncedSearch,
        pageSize: 20,
      });
      return result.students;
    },
    { revalidateOnFocus: false },
  );

  const fieldLabel = "mb-1.5 block text-sm font-medium text-gray-700";

  return (
    <div className="moto-content-surface rounded-2xl border p-4">
      <h4 className="text-sm font-semibold text-gray-900">Zielgruppen</h4>
      <p className="mt-0.5 text-xs text-gray-500">
        Wählen Sie aus, wer diese Mitteilung erhält. Mindestens eine Zielgruppe
        ist erforderlich.
      </p>

      <div className="mt-3 space-y-4">
        <PillCategory label="Allgemein">
          <CheckPill
            label="Ganze Schule"
            active={schoolAll}
            onClick={() => toggleSimple("school_all", !schoolAll)}
          />
          <CheckPill
            label="Offene Anmeldungen"
            active={pendingEnrollment}
            onClick={() =>
              toggleSimple("pending_enrollment", !pendingEnrollment)
            }
          />
        </PillCategory>

        {!schoolAll && (
          <>
            {schoolClasses.length > 0 && (
              <PillCategory label="Klassen">
                {schoolClasses.map((c) => (
                  <CheckPill
                    key={c}
                    label={c}
                    active={selectedClasses.includes(c)}
                    onClick={() =>
                      setCategory("class", toggleValue(selectedClasses, c))
                    }
                  />
                ))}
              </PillCategory>
            )}
            {groups.length > 0 && (
              <PillCategory label="Gruppen">
                {groups.map((g) => (
                  <CheckPill
                    key={g.id}
                    label={g.name}
                    active={selectedGroups.includes(g.id)}
                    onClick={() =>
                      setCategory("group", toggleValue(selectedGroups, g.id))
                    }
                  />
                ))}
              </PillCategory>
            )}
            {activities.length > 0 && (
              <PillCategory label="AGs / Betreuung">
                {activities.map((a) => (
                  <CheckPill
                    key={a.id}
                    label={a.name}
                    active={selectedActivities.includes(a.id)}
                    onClick={() =>
                      setCategory(
                        "activity_group",
                        toggleValue(selectedActivities, a.id),
                      )
                    }
                  />
                ))}
              </PillCategory>
            )}

            <div>
              <span className={fieldLabel}>Einzelne Kinder</span>
              {selectedStudents.length > 0 && (
                <div className="mb-2 grid grid-cols-2 gap-2 sm:grid-cols-3">
                  {selectedStudents.map((t) => (
                    <CheckPill
                      key={targetKey(t)}
                      label={
                        t.ref_id ? (studentNames[t.ref_id] ?? "Kind") : "Kind"
                      }
                      active
                      onClick={() => t.ref_id && removeStudent(t.ref_id)}
                    />
                  ))}
                </div>
              )}
              <Input
                name="announcement-student-search"
                controlSize="compact"
                value={studentSearch}
                onChange={(e) => setStudentSearch(e.target.value)}
                placeholder="Kind suchen (mind. 2 Zeichen)..."
              />
              {debouncedSearch.length >= 2 && (
                <div className="mt-2 max-h-40 overflow-y-auto rounded-lg border border-gray-200">
                  {studentsLoading ? (
                    <p className="px-3 py-2 text-sm text-gray-500">
                      Wird gesucht...
                    </p>
                  ) : (studentResults?.length ?? 0) === 0 ? (
                    <p className="px-3 py-2 text-sm text-gray-500">
                      Keine Kinder gefunden.
                    </p>
                  ) : (
                    studentResults?.map((student) => (
                      <button
                        key={student.id}
                        type="button"
                        onClick={() => addStudent(student)}
                        className="flex w-full items-center justify-between px-3 py-2 text-left text-sm text-gray-700 transition-colors hover:bg-gray-50"
                      >
                        <span className="truncate">{student.name}</span>
                        {student.school_class && (
                          <span className="ml-2 shrink-0 text-xs text-gray-400">
                            {student.school_class}
                          </span>
                        )}
                      </button>
                    ))
                  )}
                </div>
              )}
            </div>
          </>
        )}
      </div>
    </div>
  );
}

interface StatsModalProps {
  readonly announcement: Announcement;
  readonly onClose: () => void;
}

function StatsModal({ announcement, onClose }: StatsModalProps) {
  const [stats, setStats] = useState<AnnouncementStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError("");
    fetchAnnouncementStats(announcement.id)
      .then((result) => {
        if (!cancelled) setStats(result);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        const message =
          err instanceof Error
            ? err.message
            : "Statistik konnte nicht geladen werden";
        setError(message);
        logger.error("announcement_stats_failed", { error: message });
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [announcement.id]);

  return (
    <Modal isOpen onClose={onClose} title="Statistik">
      {loading ? (
        <Loading fullPage={false} />
      ) : error ? (
        <p className="text-sm text-[#CC2626]">{error}</p>
      ) : stats ? (
        <InfoCard
          title={announcement.title}
          icon={<BarChart3 className="h-5 w-5" aria-hidden />}
        >
          <InfoItem
            label="Gelesen"
            value={`${stats.read_count} von ${stats.target_count} gelesen`}
          />
          {announcement.requires_acknowledgement && (
            <InfoItem
              label="Lesebestätigungen"
              value={`${stats.acknowledged_count} von ${stats.target_count} bestätigt`}
            />
          )}
        </InfoCard>
      ) : null}
    </Modal>
  );
}

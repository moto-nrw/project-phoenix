"use client";

import { Suspense, useEffect, useMemo, useRef, useState } from "react";
import {
  BarChart3,
  BellRing,
  Check,
  ExternalLink,
  ListChecks,
  Megaphone,
  Pencil,
  Plus,
  Send,
  Trash2,
  Undo2,
} from "lucide-react";

import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import type {
  FilterConfig,
  ActiveFilter,
} from "~/components/ui/page-header/types";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import type { OverflowMenuItem } from "~/components/ui/page-header/OverflowMenu";
import { DataTable } from "~/components/ui/data-table";
import type { DataTableColumn } from "~/components/ui/data-table";
import { FormModal } from "~/components/ui/form-modal";
import { Modal, ConfirmationModal } from "~/components/ui/modal";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Checkbox } from "~/components/ui/checkbox";
import { DatePicker } from "~/components/ui/date-picker";
import { Loading } from "~/components/ui/loading";
import { MultiCheckboxSelect } from "~/components/ui/multi-checkbox-select";
import { WizardStepper } from "~/components/ui/wizard-stepper";
import { SegmentedControl } from "~/components/ui/segmented-control";
import type { SegmentedControlItem } from "~/components/ui/segmented-control";
import { LinkifiedText } from "~/components/ui/linkified-text";
import { LOCATION_COLORS } from "~/lib/location-helper";
import {
  berlinDayFromISO,
  endOfBerlinDayISO,
  formatBerlinDate,
  formatDate,
} from "~/lib/date-helpers";
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
  fetchAnnouncementRecipients,
  fetchAnnouncementStats,
  fetchPollChildren,
  fetchPollResults,
  isPoll,
  publishAnnouncement,
  remindUnanswered,
  unpublishAnnouncement,
  updateAnnouncement,
} from "~/lib/parent-announcements-api";
import type {
  Announcement,
  AnnouncementInput,
  AnnouncementPriority,
  AnnouncementRecipient,
  AnnouncementResponseType,
  AnnouncementStats,
  AnnouncementStatus,
  AnnouncementTarget,
  AnnouncementTargetType,
  PollChild,
  PollResults,
} from "~/lib/parent-announcements-api";

const logger = createLogger({ component: "ParentAnnouncementsPage" });

const PRIORITY_OPTIONS: ReadonlyArray<{
  value: AnnouncementPriority;
  label: string;
}> = [
  { value: "info", label: "Info" },
  { value: "important", label: "Wichtig" },
];

const STATUS_META: Record<
  AnnouncementStatus,
  { label: string; color: string }
> = {
  draft: { label: "Entwurf", color: LOCATION_COLORS.UNKNOWN },
  published: { label: "Veröffentlicht", color: LOCATION_COLORS.GROUP_ROOM },
  expired: { label: "Abgelaufen", color: LOCATION_COLORS.SCHOOLYARD },
};

/**
 * The two things this page manages. Both are the same entity in the backend
 * (an Umfrage is an announcement with answer options, #1371) — the split is a
 * UI one, because writing an information and asking a question are different
 * jobs with different follow-up work (Statistik vs Auswertung).
 */
type AnnouncementKind = "announcement" | "poll";

const KIND_ITEMS: ReadonlyArray<SegmentedControlItem<AnnouncementKind>> = [
  { value: "announcement", label: "Mitteilungen" },
  { value: "poll", label: "Umfragen" },
];

const KIND_COPY: Record<
  AnnouncementKind,
  {
    title: string;
    action: string;
    ariaLabel: string;
    emptyTitle: string;
    emptyBody: string;
  }
> = {
  announcement: {
    title: "Elternmitteilungen",
    action: "Mitteilung",
    ariaLabel: "Neue Elternmitteilung erstellen",
    emptyTitle: "Keine Mitteilungen",
    emptyBody: "Erstellen Sie eine neue Mitteilung für die Eltern.",
  },
  poll: {
    title: "Elternumfragen",
    action: "Umfrage",
    ariaLabel: "Neue Umfrage erstellen",
    emptyTitle: "Keine Umfragen",
    emptyBody:
      "Stellen Sie den Eltern eine Frage, zum Beispiel ob ihr Kind zum Sommerfest kommt.",
  },
};

const RESPONSE_TYPE_LABEL: Record<AnnouncementResponseType, string> = {
  none: "Keine Rückmeldung",
  single_choice: "Eine Antwort",
  multi_choice: "Mehrere Antworten",
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
  if (targets.some((t) => t.target_type === "school_all")) {
    // Whole-school subsumes the class/group/student targets, but pending
    // enrollments are separate applicants the backend still notifies — so
    // surface that combination instead of collapsing it to "Ganze Schule".
    return targets.some((t) => t.target_type === "pending_enrollment")
      ? "Ganze Schule, Offene Anmeldungen"
      : "Ganze Schule";
  }

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

/** Returns a German validation error for a non-empty link, or null when ok. */
function linkError(raw: string): string | null {
  const trimmed = raw.trim();
  if (!trimmed) return null;
  try {
    const parsed = new URL(trimmed);
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
      return "Der Link muss mit https:// (oder http://) beginnen.";
    }
    return null;
  } catch {
    return "Bitte einen vollständigen Link angeben (z. B. https://...).";
  }
}

function StatusPill({ status }: { status: AnnouncementStatus }) {
  const meta = STATUS_META[status];
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full bg-gray-50 px-3 py-1 text-xs font-medium">
      <span
        aria-hidden
        className="inline-block h-1.5 w-1.5 rounded-full"
        style={{ backgroundColor: meta.color }}
      />
      <span style={{ color: meta.color }}>{meta.label}</span>
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
  const [kind, setKind] = useState<AnnouncementKind>("announcement");
  const [searchTerm, setSearchTerm] = useState("");
  const [statusFilter, setStatusFilter] = useState<"all" | AnnouncementStatus>(
    "all",
  );

  const [isFormOpen, setIsFormOpen] = useState(false);
  const [editing, setEditing] = useState<Announcement | null>(null);
  const [detailFor, setDetailFor] = useState<Announcement | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Announcement | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const [publishTarget, setPublishTarget] = useState<Announcement | null>(null);
  const [publishError, setPublishError] = useState("");
  const [unpublishTarget, setUnpublishTarget] = useState<Announcement | null>(
    null,
  );
  const [unpublishError, setUnpublishError] = useState("");
  const [pendingActionId, setPendingActionId] = useState<string | null>(null);
  // Confirmation for the poll reminder, shown above the list because the detail
  // modal closes right after sending.
  const [reminderNotice, setReminderNotice] = useState("");

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

  // Targeting data sources — fetched while the form is open (pickers) or a
  // detail view is open (to label target chips with real names).
  const needsLookups = isFormOpen || detailFor !== null;
  const { data: groups } = useSWRAuth<Group[]>(
    needsLookups ? "parent-announcements-groups" : null,
    () => groupService.getGroups(),
    { revalidateOnFocus: false },
  );
  const { data: activities } = useSWRAuth<Activity[]>(
    needsLookups ? "parent-announcements-activities" : null,
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
      if (isPoll(a) !== (kind === "poll")) return false;
      if (statusFilter !== "all" && a.status !== statusFilter) return false;
      if (term && !a.title.toLowerCase().includes(term)) return false;
      return true;
    });
  }, [list, kind, searchTerm, statusFilter]);

  // Counts for the tab labels: both lists come from one fetch, so switching
  // tabs costs nothing and the user sees where the work is.
  const kindCounts = useMemo(
    () => ({
      announcement: list.filter((a) => !isPoll(a)).length,
      poll: list.filter((a) => isPoll(a)).length,
    }),
    [list],
  );

  const kindItems = useMemo(
    () =>
      KIND_ITEMS.map((item) => ({
        ...item,
        label: `${item.label} (${kindCounts[item.value]})`,
      })),
    [kindCounts],
  );

  const copy = KIND_COPY[kind];

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

  const confirmPublish = async () => {
    if (!publishTarget) return;
    setPendingActionId(publishTarget.id);
    setPublishError("");
    try {
      await publishAnnouncement(publishTarget.id);
      await mutate();
      setPublishTarget(null);
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Elternmitteilung konnte nicht veröffentlicht werden";
      setPublishError(message);
      logger.error("announcement_publish_failed", { error: message });
    } finally {
      setPendingActionId(null);
    }
  };

  const confirmUnpublish = async () => {
    if (!unpublishTarget) return;
    setPendingActionId(unpublishTarget.id);
    setUnpublishError("");
    try {
      await unpublishAnnouncement(unpublishTarget.id);
      await mutate();
      setUnpublishTarget(null);
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Elternmitteilung konnte nicht zurückgezogen werden";
      setUnpublishError(message);
      logger.error("announcement_unpublish_failed", { error: message });
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
          <div className="flex min-w-0 items-center gap-2">
            <p className="truncate font-semibold text-gray-900">{row.title}</p>
            {row.priority === "important" && (
              <span className="inline-flex shrink-0 items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-semibold text-gray-700">
                Wichtig
              </span>
            )}
          </div>
          <p className="mt-0.5 line-clamp-1 text-xs text-gray-500">
            {row.body}
          </p>
        </div>
      ),
    },
    {
      key: "status",
      header: "Status",
      sortValue: (row) => row.status,
      render: (row) => <StatusPill status={row.status} />,
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
      header:
        kind === "poll" ? "Veröffentlicht / Frist" : "Veröffentlicht / Ablauf",
      render: (row) => (
        <div className="text-xs text-gray-600">
          <p>
            {row.published_at
              ? `Veröffentlicht ${formatDate(row.published_at)}`
              : "Noch nicht veröffentlicht"}
          </p>
          {row.response_deadline ? (
            <p>Antwort bis {formatBerlinDate(row.response_deadline)}</p>
          ) : (
            row.expires_at && <p>Läuft ab {formatBerlinDate(row.expires_at)}</p>
          )}
        </div>
      ),
    },
    {
      key: "actions",
      header: "",
      align: "right",
      render: (row) => (
        // stopPropagation keeps button/menu clicks from also triggering the
        // row click (which opens the detail view).
        <div
          className="flex items-center justify-end gap-1"
          onClick={(e) => e.stopPropagation()}
          onKeyDown={(e) => e.stopPropagation()}
          role="presentation"
        >
          {row.status === "draft" && (
            <Button
              type="button"
              variant="ghost"
              size="compact"
              disabled={pendingActionId === row.id}
              onClick={() => {
                setPublishError("");
                setPublishTarget(row);
              }}
              className="gap-1.5 text-[#669f21] hover:text-[#4d7719]"
            >
              <Send className="size-4" aria-hidden />
              Veröffentlichen
            </Button>
          )}
          <OverflowMenu
            items={buildMenuItems(row)}
            ariaLabel={`Aktionen für ${row.title}`}
          />
        </div>
      ),
    },
  ];

  // One clear inline action per row (publish a draft); details open via row/
  // card click. Editing is draft-only — published announcements are immutable
  // (Zurückziehen first, then edit the draft).
  function buildMenuItems(row: Announcement): OverflowMenuItem[] {
    const menuItems: OverflowMenuItem[] = [
      {
        label: "Anzeigen",
        icon: <BarChart3 className="size-4" aria-hidden />,
        onClick: () => setDetailFor(row),
      },
    ];
    if (row.status === "draft") {
      menuItems.push({
        label: "Bearbeiten",
        icon: <Pencil className="size-4" aria-hidden />,
        onClick: () => openEdit(row),
      });
    }
    if (row.status === "published") {
      menuItems.push({
        label: "Zurückziehen",
        icon: <Undo2 className="size-4" aria-hidden />,
        onClick: () => {
          setUnpublishError("");
          setUnpublishTarget(row);
        },
        disabled: pendingActionId === row.id,
      });
    }
    menuItems.push({
      label: "Löschen",
      icon: <Trash2 className="size-4" aria-hidden />,
      destructive: true,
      onClick: () => {
        setDeleteError("");
        setDeleteTarget(row);
      },
    });
    return menuItems;
  }

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        title={copy.title}
        badge={{
          icon:
            kind === "poll" ? (
              <ListChecks className="h-5 w-5 text-gray-600" aria-hidden />
            ) : (
              <Megaphone className="h-5 w-5 text-gray-600" aria-hidden />
            ),
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
            aria-label={copy.ariaLabel}
            className="gap-1.5"
          >
            <Plus className="h-4 w-4" aria-hidden />
            {copy.action}
          </Button>
        }
      />

      <div className="mb-4">
        <SegmentedControl
          items={kindItems}
          value={kind}
          onChange={setKind}
          ariaLabel="Mitteilungen oder Umfragen"
        />
      </div>

      {loadError && (
        <div className="mb-4 rounded-lg border border-[#FF3130]/30 bg-[#FF3130]/10 p-4 text-sm text-[#CC2626]">
          Elternmitteilungen konnten nicht geladen werden.
        </div>
      )}

      {reminderNotice && (
        // Opaque tints, not alpha on the brand green: the page background is a
        // dotted pattern, and a translucent banner lets the dots show through,
        // which reads as a rendering glitch. The hexes are #83CD2D composited
        // over white at 10% (fill) and 40% (border).
        <div className="mb-4 rounded-lg border border-[#CDEBAB] bg-[#F3FAEA] p-4 text-sm text-[#4d7719]">
          {reminderNotice}
        </div>
      )}

      {isLoading ? (
        <Loading fullPage={false} />
      ) : filtered.length === 0 ? (
        <div className="py-12 text-center">
          <div className="flex flex-col items-center gap-4">
            {kind === "poll" ? (
              <ListChecks className="h-12 w-12 text-gray-400" aria-hidden />
            ) : (
              <Megaphone className="h-12 w-12 text-gray-400" aria-hidden />
            )}
            <div>
              <h3 className="text-lg font-medium text-gray-900">
                {copy.emptyTitle}
              </h3>
              <p className="text-gray-600">{copy.emptyBody}</p>
            </div>
          </div>
        </div>
      ) : (
        <>
          {/* Mobile: card list (the DataTable would only scroll sideways). */}
          <ul className="space-y-3 md:hidden">
            {filtered.map((row) => (
              <li key={row.id}>
                <AnnouncementCard
                  announcement={row}
                  pending={pendingActionId === row.id}
                  menuItems={buildMenuItems(row)}
                  onOpen={() => setDetailFor(row)}
                  onPublish={() => {
                    setPublishError("");
                    setPublishTarget(row);
                  }}
                />
              </li>
            ))}
          </ul>
          <div className="hidden md:block">
            <DataTable
              columns={columns}
              rows={filtered}
              getRowKey={(row) => row.id}
              defaultSortKey="title"
              onRowClick={(row) => setDetailFor(row)}
            />
          </div>
        </>
      )}

      {isFormOpen && (
        <AnnouncementFormModal
          announcement={editing}
          kind={editing ? (isPoll(editing) ? "poll" : "announcement") : kind}
          groups={groups ?? []}
          activities={activities ?? []}
          schoolClasses={schoolClasses ?? []}
          onClose={closeForm}
          onRefresh={async () => {
            await mutate();
          }}
          onSaved={async () => {
            await mutate();
            closeForm();
          }}
        />
      )}

      {detailFor && (
        <DetailModal
          announcement={detailFor}
          groups={groups ?? []}
          activities={activities ?? []}
          onClose={() => setDetailFor(null)}
          onReminded={(count) => {
            setReminderNotice(
              count === 0
                ? "Alle erreichten Kinder haben bereits geantwortet — es wurde niemand erinnert."
                : `${count} ${count === 1 ? "Elternteil wurde" : "Eltern wurden"} an die offene Umfrage erinnert.`,
            );
            setDetailFor(null);
          }}
        />
      )}

      {publishTarget && (
        <ConfirmationModal
          isOpen={Boolean(publishTarget)}
          onClose={() => {
            setPublishTarget(null);
            setPublishError("");
          }}
          onConfirm={() => void confirmPublish()}
          title="Elternmitteilung veröffentlichen"
          confirmText="Jetzt veröffentlichen"
          cancelText="Abbrechen"
          isConfirmLoading={pendingActionId === publishTarget.id}
        >
          <div className="space-y-2 text-sm text-gray-700">
            <p>
              „{publishTarget.title}“ wird für{" "}
              <span className="font-medium">
                {summarizeTargets(publishTarget.targets)}
              </span>{" "}
              sichtbar.
            </p>
            {publishTarget.send_email && (
              <p>
                Die erreichten Eltern werden zusätzlich per E-Mail
                benachrichtigt.
              </p>
            )}
            <p className="text-xs text-gray-500">
              Nach dem Veröffentlichen kann die Mitteilung nicht mehr bearbeitet
              werden.
            </p>
            {publishError && (
              <p role="alert" className="text-sm text-[#CC2626]">
                {publishError}
              </p>
            )}
          </div>
        </ConfirmationModal>
      )}

      {unpublishTarget && (
        <ConfirmationModal
          isOpen={Boolean(unpublishTarget)}
          onClose={() => {
            setUnpublishTarget(null);
            setUnpublishError("");
          }}
          onConfirm={() => void confirmUnpublish()}
          title="Elternmitteilung zurückziehen"
          confirmText="Zurückziehen"
          cancelText="Abbrechen"
          isConfirmLoading={pendingActionId === unpublishTarget.id}
        >
          <div className="space-y-2 text-sm text-gray-700">
            <p>
              „{unpublishTarget.title}“ wird für die Eltern nicht mehr sichtbar
              und kehrt in den Entwurfsstatus zurück.
            </p>
            <p className="text-xs text-gray-500">
              Noch nicht versendete E-Mail-Benachrichtigungen werden
              abgebrochen.
            </p>
            {unpublishError && (
              <p role="alert" className="text-sm text-[#CC2626]">
                {unpublishError}
              </p>
            )}
          </div>
        </ConfirmationModal>
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
  /**
   * Which of the two things is being written. Decided by the caller (the active
   * tab for a new entry, the entry's own type when editing) so the modal never
   * has to guess and a poll can never silently lose its options.
   */
  readonly kind: AnnouncementKind;
  readonly groups: Group[];
  readonly activities: Activity[];
  readonly schoolClasses: string[];
  readonly onClose: () => void;
  readonly onSaved: () => Promise<void> | void;
  /**
   * Refresh the list WITHOUT closing the modal — used when the draft was saved
   * but publishing failed, so the new/updated draft appears in the list while
   * the publish error stays visible for the user to retry.
   */
  readonly onRefresh: () => Promise<void> | void;
}

const WIZARD_STEPS = ["Inhalt", "Empfänger"] as const;

/**
 * Two-step wizard, like writing an e-mail: first the content, then who
 * receives it — with "Als Entwurf speichern" and "Veröffentlichen" as the
 * final, explicit choices. Only drafts ever reach this modal (published
 * announcements are immutable).
 */
function AnnouncementFormModal({
  announcement,
  kind,
  groups,
  activities,
  schoolClasses,
  onClose,
  onSaved,
  onRefresh,
}: AnnouncementFormModalProps) {
  const isEdit = announcement !== null;
  const isPollForm = kind === "poll";
  const [step, setStep] = useState(0);
  // Tracks the id of the draft once it exists in the backend. Seeded from an
  // existing draft when editing; set after a create so that a publish-retry
  // (the modal stays open when publishing fails) updates that same draft
  // instead of creating a duplicate.
  const [persistedId, setPersistedId] = useState<string | null>(
    announcement?.id ?? null,
  );

  const [title, setTitle] = useState(announcement?.title ?? "");
  const [body, setBody] = useState(announcement?.body ?? "");
  const [linkUrl, setLinkUrl] = useState(announcement?.link_url ?? "");
  const [priority, setPriority] = useState<AnnouncementPriority>(
    announcement?.priority ?? "info",
  );
  const [requiresAck, setRequiresAck] = useState(
    announcement?.requires_acknowledgement ?? false,
  );
  const [sendEmail, setSendEmail] = useState(announcement?.send_email ?? false);
  // Both cut-offs were written as the END of a Berlin day, so they have to be
  // read back as that Berlin day — a plain `new Date(...)` shows the following
  // day east of Berlin and re-saving would move the date by one day.
  const [expiresAt, setExpiresAt] = useState<Date | null>(
    announcement?.expires_at ? berlinDayFromISO(announcement.expires_at) : null,
  );
  const [targets, setTargets] = useState<AnnouncementTarget[]>(
    announcement?.targets ?? [],
  );
  const [studentNames, setStudentNames] = useState<Record<string, string>>({});

  // Poll state. A fresh poll starts as the question schools ask most often, so
  // the common case is two clicks away instead of two text fields.
  const [multiChoice, setMultiChoice] = useState(
    announcement?.response_type === "multi_choice",
  );
  // Each answer is its own row, mirroring the "Feste Auswahlzeiten" editor in
  // the enrollment form builder: an input plus a remove button, added through
  // an explicit button. Answers are a fixed set the parents pick from, so they
  // have to look like a set — not like free text.
  const [optionRows, setOptionRows] = useState<string[]>(() => {
    const existing = announcement?.options?.map((o) => o.label) ?? [];
    if (existing.length > 0) return existing;
    return isPollForm ? ["Ja", "Nein"] : [];
  });
  const [deadline, setDeadline] = useState<Date | null>(
    announcement?.response_deadline
      ? berlinDayFromISO(announcement.response_deadline)
      : null,
  );

  // The persisted set: trimmed, blanks dropped. Rows stay editable as typed.
  const options = useMemo(
    () => optionRows.map((row) => row.trim()).filter(Boolean),
    [optionRows],
  );

  const setOptionAt = (index: number, value: string) =>
    setOptionRows((prev) => prev.map((row, i) => (i === index ? value : row)));
  const addOption = () => setOptionRows((prev) => [...prev, ""]);
  const removeOptionAt = (index: number) =>
    setOptionRows((prev) => prev.filter((_, i) => i !== index));

  const [submitting, setSubmitting] = useState<"draft" | "publish" | null>(
    null,
  );
  const [formError, setFormError] = useState("");

  const validateContent = (): boolean => {
    if (!title.trim()) {
      setFormError("Bitte einen Titel eingeben.");
      return false;
    }
    if (!body.trim()) {
      setFormError("Bitte einen Text eingeben.");
      return false;
    }
    const linkProblem = linkError(linkUrl);
    if (linkProblem) {
      setFormError(linkProblem);
      return false;
    }
    if (isPollForm) {
      if (options.length < 2) {
        setFormError("Bitte mindestens zwei Antwortmöglichkeiten angeben.");
        return false;
      }
      if (options.length > 10) {
        setFormError("Bitte höchstens zehn Antwortmöglichkeiten angeben.");
        return false;
      }
      const seen = new Set(options.map((o) => o.toLowerCase()));
      if (seen.size !== options.length) {
        setFormError("Antwortmöglichkeiten dürfen sich nicht wiederholen.");
        return false;
      }
      if (deadline && expiresAt && deadline > expiresAt) {
        setFormError(
          "Die Antwortfrist darf nicht nach dem Ablaufdatum liegen — sonst können Eltern nicht mehr antworten.",
        );
        return false;
      }
    }
    setFormError("");
    return true;
  };

  const goNext = () => {
    if (validateContent()) setStep(1);
  };

  const handleSubmit = async (publish: boolean) => {
    if (!validateContent()) {
      setStep(0);
      return;
    }
    if (targets.length === 0) {
      setFormError("Bitte mindestens eine Zielgruppe hinzufügen.");
      return;
    }

    const trimmedLink = linkUrl.trim();
    const input: AnnouncementInput = {
      title: title.trim(),
      body: body.trim(),
      priority,
      link_url: trimmedLink ? trimmedLink : null,
      // A poll never also carries a read confirmation (the field is hidden for
      // polls, so a value left over from a converted draft must not leak).
      requires_acknowledgement: isPollForm ? false : requiresAck,
      send_email: sendEmail,
      expires_at: expiresAt ? endOfBerlinDayISO(expiresAt) : null,
      targets,
      response_type: isPollForm
        ? multiChoice
          ? "multi_choice"
          : "single_choice"
        : "none",
      // The chosen day is the LAST day parents can answer, so the cut-off is
      // its end — not midnight, which would close the poll a day early.
      response_deadline:
        isPollForm && deadline ? endOfBerlinDayISO(deadline) : null,
      options: isPollForm ? options : undefined,
    };

    setSubmitting(publish ? "publish" : "draft");
    setFormError("");
    try {
      const saved = persistedId
        ? await updateAnnouncement(persistedId, input)
        : await createAnnouncement(input);
      setPersistedId(saved.id);
      if (publish) {
        try {
          await publishAnnouncement(saved.id);
        } catch (err) {
          // The draft is saved — surface only the publish failure so nothing is
          // lost. Refresh the list so the draft appears, but keep THIS modal
          // open so the error stays visible and the user can retry publishing.
          const message =
            err instanceof Error
              ? err.message
              : "Elternmitteilung konnte nicht veröffentlicht werden";
          setFormError(
            `Als Entwurf gespeichert, aber das Veröffentlichen ist fehlgeschlagen: ${message}`,
          );
          logger.error("announcement_publish_failed", { error: message });
          await onRefresh();
          return;
        }
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
      setSubmitting(null);
    }
  };

  const footer =
    step === 0 ? (
      <>
        <Button type="button" variant="outline" size="md" onClick={onClose}>
          Abbrechen
        </Button>
        <Button type="button" size="md" onClick={goNext}>
          Weiter
        </Button>
      </>
    ) : (
      <>
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={() => setStep(0)}
          disabled={submitting !== null}
        >
          Zurück
        </Button>
        <Button
          type="button"
          variant="secondary"
          size="md"
          onClick={() => void handleSubmit(false)}
          isLoading={submitting === "draft"}
          loadingText="Wird gespeichert..."
          disabled={submitting === "publish"}
        >
          Als Entwurf speichern
        </Button>
        <Button
          type="button"
          size="md"
          onClick={() => void handleSubmit(true)}
          isLoading={submitting === "publish"}
          loadingText="Wird veröffentlicht..."
          disabled={submitting === "draft"}
          className="gap-1.5"
        >
          <Send className="size-4" aria-hidden />
          Veröffentlichen
        </Button>
      </>
    );

  return (
    <FormModal
      isOpen
      onClose={onClose}
      title={
        isPollForm
          ? isEdit
            ? "Umfrage bearbeiten"
            : "Neue Umfrage"
          : isEdit
            ? "Elternmitteilung bearbeiten"
            : "Neue Elternmitteilung"
      }
      footer={footer}
      size="xl"
    >
      <div className="space-y-4">
        <WizardStepper steps={WIZARD_STEPS} current={step} />

        {step === 0 ? (
          // Sections with quiet uppercase headers, every control on the full
          // width — the same form language as the Vertretungsplan slide-over.
          // No nested cards and no per-option input rows: at this size they
          // read as clutter, not as structure.
          <div className="space-y-6">
            <section className="space-y-4">
              <Input
                label={isPollForm ? "Frage" : "Titel"}
                name="announcement-title"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder={
                  isPollForm
                    ? "z. B. Kommt Ihr Kind zur Murmelparty?"
                    : "z. B. Sommerfest am Freitag"
                }
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
                  rows={5}
                  maxLength={4000}
                  placeholder="Inhalt der Mitteilung... Links im Text werden für Eltern klickbar."
                  className="block w-full rounded-lg border-0 bg-white px-4 py-3 text-base text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400"
                />
              </div>

              <Input
                label="Link (optional)"
                name="announcement-link"
                type="url"
                value={linkUrl}
                onChange={(e) => setLinkUrl(e.target.value)}
                placeholder="https://..."
              />
            </section>

            {isPollForm && (
              <section className="space-y-3">
                <h3 className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                  Antwortmöglichkeiten
                </h3>
                <div>
                  <ul className="space-y-2">
                    {optionRows.map((option, index) => (
                      // Rows have no id before saving; the list is short and
                      // edited in place, so the index is the stable key.
                      // eslint-disable-next-line react/no-array-index-key
                      <li key={index} className="flex items-center gap-2">
                        {/* Input's className lands on the control, not on its
                            wrapper, so the wrapper carries the flex sizing. */}
                        <div className="min-w-0 flex-1">
                          <Input
                            controlSize="compact"
                            name={`announcement-option-${index}`}
                            value={option}
                            onChange={(e) => setOptionAt(index, e.target.value)}
                            placeholder={`Antwort ${index + 1}`}
                            aria-label={`Antwort ${index + 1}`}
                            maxLength={120}
                          />
                        </div>
                        <button
                          type="button"
                          onClick={() => removeOptionAt(index)}
                          aria-label={`Antwort ${index + 1} entfernen`}
                          className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border border-[#FF3130]/20 bg-white text-[#CC2626] shadow-sm transition-colors hover:bg-[#FF3130]/10 focus-visible:ring-2 focus-visible:ring-[#FF3130]/30 focus-visible:outline-none"
                        >
                          <Trash2 className="h-4 w-4" aria-hidden />
                        </button>
                      </li>
                    ))}
                  </ul>

                  <button
                    type="button"
                    onClick={addOption}
                    disabled={optionRows.length >= 10}
                    className="mt-3 inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <Plus className="h-4 w-4" aria-hidden />
                    Antwort hinzufügen
                  </button>

                  <p className="mt-2 text-xs text-gray-500">
                    Zwei bis zehn Antworten. Eltern antworten für jedes Kind
                    einzeln.
                  </p>
                </div>

                <label
                  htmlFor="announcement-multi"
                  className="flex cursor-pointer items-start gap-3"
                >
                  <Checkbox
                    id="announcement-multi"
                    checked={multiChoice}
                    onChange={(e) => setMultiChoice(e.target.checked)}
                  />
                  <span className="text-sm text-gray-800">
                    <span className="block">Mehrfachauswahl erlauben</span>
                    <span className="block text-xs text-gray-500">
                      Eltern können pro Kind mehrere Antworten auswählen.
                    </span>
                  </span>
                </label>
              </section>
            )}

            <section className="space-y-3">
              <h3 className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                Zeitraum
              </h3>
              <div className="grid gap-4 sm:grid-cols-2">
                {isPollForm && (
                  <div>
                    <span className="mb-1.5 block text-sm font-medium text-gray-700">
                      Antwortfrist (optional)
                    </span>
                    <DatePicker
                      value={deadline}
                      onChange={setDeadline}
                      placeholder="Keine Frist"
                      dropdownPlacement="down"
                    />
                    <p className="mt-1.5 text-xs text-gray-500">
                      Danach ist die Umfrage geschlossen, bleibt aber lesbar.
                    </p>
                  </div>
                )}
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
                  <p className="mt-1.5 text-xs text-gray-500">
                    Danach wird {isPollForm ? "die Umfrage" : "die Mitteilung"}{" "}
                    für Eltern ausgeblendet.
                  </p>
                </div>
              </div>
            </section>

            <section className="space-y-3">
              <h3 className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                Veröffentlichung
              </h3>

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

              {/* A poll answer already IS the confirmation — offering a second,
                  weaker "gelesen" checkbox on top only muddies the result. */}
              {!isPollForm && (
                <label
                  htmlFor="announcement-ack"
                  className="flex cursor-pointer items-start gap-3"
                >
                  <Checkbox
                    id="announcement-ack"
                    checked={requiresAck}
                    onChange={(e) => setRequiresAck(e.target.checked)}
                  />
                  <span className="text-sm text-gray-800">
                    <span className="block">Lesebestätigung erforderlich</span>
                    <span className="block text-xs text-gray-500">
                      Eltern bestätigen ausdrücklich, dass sie die Mitteilung
                      gelesen haben.
                    </span>
                  </span>
                </label>
              )}

              <label
                htmlFor="announcement-email"
                className="flex cursor-pointer items-start gap-3"
              >
                <Checkbox
                  id="announcement-email"
                  checked={sendEmail}
                  onChange={(e) => setSendEmail(e.target.checked)}
                />
                <span className="text-sm text-gray-800">
                  <span className="block">
                    Eltern zusätzlich per E-Mail benachrichtigen
                  </span>
                  <span className="block text-xs text-gray-500">
                    Beim Veröffentlichen erhalten die erreichten Eltern eine
                    E-Mail mit Titel und Link ins Elternportal.
                  </span>
                </span>
              </label>
            </section>
          </div>
        ) : (
          <TargetingStep
            targets={targets}
            groups={groups}
            activities={activities}
            schoolClasses={schoolClasses}
            studentNames={studentNames}
            onChange={setTargets}
            onSetStudentName={(id, name) =>
              setStudentNames((prev) => ({ ...prev, [id]: name }))
            }
            kindLabel={isPollForm ? "diese Umfrage" : "diese Mitteilung"}
            allowPendingEnrollment={!isPollForm}
          />
        )}

        {formError && (
          <p role="alert" className="text-sm text-[#CC2626]">
            {formError}
          </p>
        )}
      </div>
    </FormModal>
  );
}

/**
 * Broad-audience toggle pill (mirrors the operator announcement modal's
 * selector): a bordered button with an inline check-box.
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

interface TargetingStepProps {
  readonly targets: AnnouncementTarget[];
  readonly groups: Group[];
  readonly activities: Activity[];
  readonly schoolClasses: string[];
  readonly studentNames: Record<string, string>;
  readonly onChange: (targets: AnnouncementTarget[]) => void;
  readonly onSetStudentName: (id: string, name: string) => void;
  /** Names the thing being addressed in the heading (Mitteilung vs Umfrage). */
  readonly kindLabel: string;
  /**
   * Offene Anmeldungen reach applicants who have no enrolled child yet — so a
   * poll would show them a question they cannot answer (answers are per child).
   * Polls therefore hide the option; the backend refuses it as well.
   */
  readonly allowPendingEnrollment: boolean;
}

function TargetingStep({
  targets,
  groups,
  activities,
  schoolClasses,
  studentNames,
  onChange,
  onSetStudentName,
  kindLabel,
  allowPendingEnrollment,
}: TargetingStepProps) {
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
    <div className="space-y-4">
      <div>
        <h4 className="text-sm font-semibold text-gray-900">
          Wer soll {kindLabel} erhalten?
        </h4>
        <p className="mt-0.5 text-xs text-gray-500">
          Mehrere Zielgruppen lassen sich kombinieren; jedes Elternteil erhält
          die Mitteilung höchstens einmal.
        </p>
      </div>

      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
        <CheckPill
          label="Ganze Schule"
          active={schoolAll}
          onClick={() => toggleSimple("school_all", !schoolAll)}
        />
        {allowPendingEnrollment && (
          <CheckPill
            label="Offene Anmeldungen"
            active={pendingEnrollment}
            onClick={() =>
              toggleSimple("pending_enrollment", !pendingEnrollment)
            }
          />
        )}
      </div>

      {!schoolAll && (
        <>
          <div className="grid gap-3 sm:grid-cols-3">
            <div>
              <span className={fieldLabel}>Klassen</span>
              <MultiCheckboxSelect
                ariaLabel="Klassen auswählen"
                value={selectedClasses}
                options={schoolClasses.map((c) => ({ value: c, label: c }))}
                onChange={(values) => setCategory("class", values)}
                emptyLabel="Keine ausgewählt"
                unavailableLabel="Keine Klassen"
                searchable
                searchPlaceholder="Klasse suchen..."
              />
            </div>
            <div>
              <span className={fieldLabel}>Gruppen</span>
              <MultiCheckboxSelect
                ariaLabel="Gruppen auswählen"
                value={selectedGroups}
                options={groups.map((g) => ({ value: g.id, label: g.name }))}
                onChange={(values) => setCategory("group", values)}
                emptyLabel="Keine ausgewählt"
                unavailableLabel="Keine Gruppen"
                searchable
                searchPlaceholder="Gruppe suchen..."
              />
            </div>
            <div>
              <span className={fieldLabel}>AGs / Betreuung</span>
              <MultiCheckboxSelect
                ariaLabel="AGs auswählen"
                value={selectedActivities}
                options={activities.map((a) => ({
                  value: a.id,
                  label: a.name,
                }))}
                onChange={(values) => setCategory("activity_group", values)}
                emptyLabel="Keine ausgewählt"
                unavailableLabel="Keine AGs"
                searchable
                searchPlaceholder="AG suchen..."
              />
            </div>
          </div>

          <div>
            <span className={fieldLabel}>Einzelne Kinder</span>
            {selectedStudents.length > 0 && (
              <div className="mb-2 flex flex-wrap gap-2">
                {selectedStudents.map((t) => (
                  <button
                    key={targetKey(t)}
                    type="button"
                    onClick={() => t.ref_id && removeStudent(t.ref_id)}
                    title="Entfernen"
                    className="inline-flex items-center gap-1.5 rounded-full border border-gray-300 bg-gray-50 px-3 py-1 text-sm text-gray-800 transition-colors hover:border-gray-400 hover:bg-gray-100"
                  >
                    <span className="max-w-48 truncate">
                      {t.ref_id ? (studentNames[t.ref_id] ?? "Kind") : "Kind"}
                    </span>
                    <span aria-hidden className="text-gray-400">
                      ×
                    </span>
                  </button>
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

      <p className="rounded-lg bg-gray-50 px-3 py-2 text-sm text-gray-700">
        Ausgewählt:{" "}
        <span className="font-medium">{summarizeTargets(targets)}</span>
      </p>
    </div>
  );
}

/** Mobile list card: same data as a table row, tap opens the detail view. */
function AnnouncementCard({
  announcement,
  pending,
  menuItems,
  onOpen,
  onPublish,
}: {
  readonly announcement: Announcement;
  readonly pending: boolean;
  readonly menuItems: OverflowMenuItem[];
  readonly onOpen: () => void;
  readonly onPublish: () => void;
}) {
  return (
    <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
      <button
        type="button"
        onClick={onOpen}
        className="block w-full min-w-0 p-4 pb-3 text-left transition-colors active:bg-gray-50"
      >
        <div className="flex flex-wrap items-center gap-1.5">
          <StatusPill status={announcement.status} />
          {announcement.priority === "important" && (
            <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-semibold text-gray-700">
              Wichtig
            </span>
          )}
        </div>
        <p className="mt-2 line-clamp-2 leading-snug font-semibold text-gray-900">
          {announcement.title}
        </p>
        <p className="mt-1 line-clamp-2 text-sm leading-5 text-gray-600">
          {announcement.body}
        </p>
        <p className="mt-2 text-xs text-gray-500">
          {summarizeTargets(announcement.targets)}
          {announcement.published_at
            ? ` · ${formatDate(announcement.published_at)}`
            : " · Entwurf"}
          {announcement.expires_at &&
            ` · bis ${formatBerlinDate(announcement.expires_at)}`}
        </p>
      </button>
      <div className="flex items-center justify-between gap-2 border-t border-gray-100 bg-gray-50/60 px-2 py-1.5">
        {announcement.status === "draft" ? (
          <Button
            type="button"
            variant="ghost"
            size="compact"
            disabled={pending}
            onClick={onPublish}
            className="gap-1.5 text-[#669f21] hover:text-[#4d7719]"
          >
            <Send className="size-4" aria-hidden />
            Veröffentlichen
          </Button>
        ) : (
          <Button
            type="button"
            variant="ghost"
            size="compact"
            onClick={onOpen}
            className="gap-1.5 text-gray-600"
          >
            <BarChart3 className="size-4" aria-hidden />
            Details
          </Button>
        )}
        <OverflowMenu
          items={menuItems}
          ariaLabel={`Aktionen für ${announcement.title}`}
        />
      </div>
    </div>
  );
}

/** Chips describing the audience, with real names where the lookups know them. */
function targetChips(
  targets: AnnouncementTarget[],
  groups: Group[],
  activities: Activity[],
): string[] {
  if (targets.some((t) => t.target_type === "school_all")) {
    // Whole-school subsumes class/group/student targets; pending enrollments
    // are additional recipients the backend still reaches, so keep that chip.
    const schoolChips = ["Ganze Schule"];
    if (targets.some((t) => t.target_type === "pending_enrollment"))
      schoolChips.push("Offene Anmeldungen");
    return schoolChips;
  }
  const groupNames = new Map(groups.map((g) => [g.id, g.name]));
  const activityNames = new Map(activities.map((a) => [a.id, a.name]));
  const chips: string[] = [];
  let studentCount = 0;
  for (const t of targets) {
    switch (t.target_type) {
      case "pending_enrollment":
        chips.push("Offene Anmeldungen");
        break;
      case "class":
        chips.push(`Klasse ${t.ref_text ?? "?"}`);
        break;
      case "group":
        chips.push(groupNames.get(t.ref_id ?? "") ?? "Gruppe");
        break;
      case "activity_group":
        chips.push(activityNames.get(t.ref_id ?? "") ?? "AG");
        break;
      case "student":
        studentCount++;
        break;
      case "school_all":
        break;
    }
  }
  if (studentCount > 0) {
    chips.push(
      studentCount === 1
        ? "1 einzelnes Kind"
        : `${studentCount} einzelne Kinder`,
    );
  }
  return chips;
}

const RECIPIENT_STATUS_META: Record<
  AnnouncementRecipient["status"],
  { label: string; color: string }
> = {
  acknowledged: { label: "Bestätigt", color: LOCATION_COLORS.GROUP_ROOM },
  read: { label: "Gelesen", color: LOCATION_COLORS.OTHER_ROOM },
  pending: { label: "Ausstehend", color: LOCATION_COLORS.UNKNOWN },
};

const RECIPIENT_SORT: Record<AnnouncementRecipient["status"], number> = {
  pending: 0,
  read: 1,
  acknowledged: 2,
};

interface RecipientListProps {
  readonly recipients: AnnouncementRecipient[];
  readonly showStatus: boolean;
  readonly statusFilter: "all" | AnnouncementRecipient["status"];
  readonly onStatusFilter: (
    value: "all" | AnnouncementRecipient["status"],
  ) => void;
  readonly nameFilter: string;
  readonly onNameFilter: (value: string) => void;
}

/**
 * The per-guardian read/ack list, built to stay usable at big-school scale:
 * status chips with counts (the "Ausstehend" chip is the chase list), a name
 * search, and a capped, scrolling list.
 */
function RecipientList({
  recipients,
  showStatus,
  statusFilter,
  onStatusFilter,
  nameFilter,
  onNameFilter,
}: RecipientListProps) {
  const counts = {
    all: recipients.length,
    pending: 0,
    read: 0,
    acknowledged: 0,
  };
  for (const rcpt of recipients) counts[rcpt.status]++;

  const term = nameFilter.trim().toLowerCase();
  const filtered = recipients.filter((rcpt) => {
    if (showStatus && statusFilter !== "all" && rcpt.status !== statusFilter)
      return false;
    if (
      term &&
      !`${rcpt.first_name} ${rcpt.last_name}`.toLowerCase().includes(term)
    )
      return false;
    return true;
  });

  const chips: ReadonlyArray<{
    value: "all" | AnnouncementRecipient["status"];
    label: string;
  }> = [
    { value: "all", label: `Alle (${counts.all})` },
    { value: "pending", label: `Ausstehend (${counts.pending})` },
    { value: "read", label: `Gelesen (${counts.read})` },
    { value: "acknowledged", label: `Bestätigt (${counts.acknowledged})` },
  ];

  return (
    <div className="mt-2 space-y-2">
      {showStatus && (
        <div className="flex flex-wrap gap-1.5">
          {chips.map((chip) => (
            <button
              key={chip.value}
              type="button"
              onClick={() => onStatusFilter(chip.value)}
              className={`rounded-full px-3 py-1 text-xs font-medium transition-colors ${
                statusFilter === chip.value
                  ? "bg-gray-900 text-white"
                  : "bg-gray-100 text-gray-700 hover:bg-gray-200"
              }`}
            >
              {chip.label}
            </button>
          ))}
        </div>
      )}
      {recipients.length > 8 && (
        <Input
          name="recipient-search"
          controlSize="compact"
          value={nameFilter}
          onChange={(e) => onNameFilter(e.target.value)}
          placeholder="Name suchen..."
        />
      )}
      {filtered.length === 0 ? (
        <p className="rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-500">
          Keine Treffer.
        </p>
      ) : (
        <ul className="max-h-56 divide-y divide-gray-100 overflow-y-auto rounded-lg border border-gray-200">
          {filtered.map((rcpt) => (
            <li
              key={rcpt.account_id}
              className="flex items-center justify-between gap-2 px-3 py-2"
            >
              <span className="min-w-0 truncate text-sm text-gray-800">
                {`${rcpt.first_name} ${rcpt.last_name}`.trim() || "Ohne Namen"}
              </span>
              {showStatus && (
                <span
                  className="inline-flex shrink-0 items-center gap-1.5 rounded-full bg-gray-50 px-2.5 py-0.5 text-xs font-medium"
                  style={{ color: RECIPIENT_STATUS_META[rcpt.status].color }}
                >
                  <span
                    aria-hidden
                    className="inline-block h-1.5 w-1.5 rounded-full"
                    style={{
                      backgroundColor: RECIPIENT_STATUS_META[rcpt.status].color,
                    }}
                  />
                  {RECIPIENT_STATUS_META[rcpt.status].label}
                </span>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

interface DetailModalProps {
  readonly announcement: Announcement;
  readonly groups: Group[];
  readonly activities: Activity[];
  readonly onClose: () => void;
  /** Called with the number of guardians reached by a poll reminder. */
  readonly onReminded: (count: number) => void;
}

/**
 * Poll evaluation: one bar per option over the children with an eligible
 * respondent, plus the full per-child list so staff can distinguish open
 * answers from children who currently cannot answer. Counts are children, not
 * parents — that is the number the school plans with.
 */
function PollResultsPanel({
  announcement,
  onReminded,
}: {
  readonly announcement: Announcement;
  readonly onReminded: (count: number) => void;
}) {
  const [results, setResults] = useState<PollResults | null>(null);
  const [children, setChildren] = useState<PollChild[] | null>(null);
  const [error, setError] = useState("");
  const [onlyOpen, setOnlyOpen] = useState(false);
  const [reminding, setReminding] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setError("");
    Promise.all([
      fetchPollResults(announcement.id),
      fetchPollChildren(announcement.id),
    ])
      .then(([resultsData, childrenData]) => {
        if (cancelled) return;
        setResults(resultsData);
        setChildren(childrenData);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        const message =
          err instanceof Error
            ? err.message
            : "Umfrageergebnis konnte nicht geladen werden";
        setError(message);
        logger.error("announcement_poll_results_failed", { error: message });
      });
    return () => {
      cancelled = true;
    };
  }, [announcement.id]);

  const deadlinePassed =
    announcement.response_deadline !== undefined &&
    new Date(announcement.response_deadline) <= new Date();
  const canRemind =
    announcement.status === "published" &&
    !deadlinePassed &&
    (results?.answered_count ?? 0) < (results?.child_count ?? 0);

  const handleRemind = async () => {
    setReminding(true);
    setError("");
    try {
      const count = await remindUnanswered(announcement.id);
      onReminded(count);
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Erinnerung konnte nicht gesendet werden";
      setError(message);
      logger.error("announcement_poll_reminder_failed", { error: message });
    } finally {
      setReminding(false);
    }
  };

  const visibleChildren = (children ?? []).filter(
    (child) =>
      !onlyOpen || (child.can_answer && child.answer_labels.length === 0),
  );

  return (
    <div>
      <h4 className="mb-1.5 flex items-center gap-1.5 text-sm font-semibold text-gray-900">
        <ListChecks className="h-4 w-4 text-gray-500" aria-hidden />
        Auswertung
      </h4>

      {error && <p className="mb-2 text-sm text-[#CC2626]">{error}</p>}

      {results === null || children === null ? (
        error ? null : (
          <Loading fullPage={false} />
        )
      ) : (
        <>
          <p className="text-sm text-gray-700">
            {results.answered_count} von {results.child_count}{" "}
            {results.child_count === 1 ? "Kind" : "Kindern"} beantwortet
          </p>
          {results.target_child_count > results.child_count && (
            <p className="mt-1 text-xs text-gray-500">
              {results.target_child_count - results.child_count}{" "}
              {results.target_child_count - results.child_count === 1
                ? "weiteres Zielkind hat"
                : "weitere Zielkinder haben"}{" "}
              derzeit keine antwortberechtigte Person.
            </p>
          )}

          <ul className="mt-3 space-y-2">
            {results.options.map((option) => {
              const share =
                results.child_count > 0
                  ? Math.round((option.count / results.child_count) * 100)
                  : 0;
              return (
                <li key={option.option_id}>
                  <div className="flex items-baseline justify-between gap-3">
                    <span className="truncate text-sm text-gray-800">
                      {option.label}
                    </span>
                    <span className="shrink-0 text-sm font-semibold text-gray-900">
                      {option.count}
                    </span>
                  </div>
                  <div className="mt-1 h-2 w-full overflow-hidden rounded-full bg-gray-100">
                    <div
                      className="h-full rounded-full"
                      style={{
                        width: `${share}%`,
                        backgroundColor: LOCATION_COLORS.GROUP_ROOM,
                      }}
                    />
                  </div>
                </li>
              );
            })}
          </ul>

          {children.length > 0 && (
            <div className="mt-4">
              <div className="mb-2 flex items-center justify-between gap-3">
                <span className="text-xs font-medium text-gray-700">
                  Antworten pro Kind
                </span>
                <Button
                  type="button"
                  variant="ghost"
                  size="compact"
                  onClick={() => setOnlyOpen((prev) => !prev)}
                >
                  {onlyOpen ? "Alle anzeigen" : "Nur offene"}
                </Button>
              </div>
              <ul className="max-h-56 divide-y divide-gray-100 overflow-y-auto rounded-lg border border-gray-200">
                {visibleChildren.map((child) => (
                  <li
                    key={child.student_id}
                    className="flex items-center justify-between gap-3 px-3 py-2"
                  >
                    <span className="min-w-0 truncate text-sm text-gray-800">
                      {child.first_name} {child.last_name}
                      {child.school_class && (
                        <span className="text-gray-500">
                          {" "}
                          · {child.school_class}
                        </span>
                      )}
                    </span>
                    {child.answer_labels.length > 0 ? (
                      <span className="shrink-0 text-xs font-medium text-[#4d7719]">
                        {child.answer_labels.join(", ")}
                      </span>
                    ) : child.can_answer ? (
                      <span className="shrink-0 text-xs text-gray-500">
                        Offen
                      </span>
                    ) : (
                      <span className="shrink-0 text-xs text-gray-500">
                        Nicht beantwortbar
                      </span>
                    )}
                  </li>
                ))}
              </ul>
            </div>
          )}

          {canRemind && (
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => void handleRemind()}
              isLoading={reminding}
              loadingText="Wird gesendet..."
              className="mt-4 gap-1.5"
            >
              <BellRing className="size-4" aria-hidden />
              Eltern ohne Antwort erinnern
            </Button>
          )}
        </>
      )}
    </div>
  );
}

/**
 * Read-only detail view: since published announcements are immutable, this is
 * the only place staff can re-read the full text. Includes reach stats and the
 * per-guardian read/ack list (pending first — the people to chase).
 */
function DetailModal({
  announcement,
  groups,
  activities,
  onClose,
  onReminded,
}: DetailModalProps) {
  const [stats, setStats] = useState<AnnouncementStats | null>(null);
  const [recipients, setRecipients] = useState<AnnouncementRecipient[] | null>(
    null,
  );
  const [error, setError] = useState("");
  const [statusFilter, setStatusFilter] = useState<
    "all" | AnnouncementRecipient["status"]
  >("all");
  const [nameFilter, setNameFilter] = useState("");

  // A published poll renders its Auswertung instead of the read/ack statistics
  // (see below), so it must not pay for the two requests behind them either.
  const showReadStats = !(
    isPoll(announcement) && announcement.status !== "draft"
  );

  useEffect(() => {
    if (!showReadStats) return;
    let cancelled = false;
    setError("");
    Promise.all([
      fetchAnnouncementStats(announcement.id),
      fetchAnnouncementRecipients(announcement.id),
    ])
      .then(([statsResult, recipientsResult]) => {
        if (cancelled) return;
        setStats(statsResult);
        setRecipients(
          [...recipientsResult].sort(
            (a, b) =>
              RECIPIENT_SORT[a.status] - RECIPIENT_SORT[b.status] ||
              a.last_name.localeCompare(b.last_name) ||
              a.first_name.localeCompare(b.first_name),
          ),
        );
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        const message =
          err instanceof Error
            ? err.message
            : "Statistik konnte nicht geladen werden";
        setError(message);
        logger.error("announcement_detail_failed", { error: message });
      });
    return () => {
      cancelled = true;
    };
  }, [announcement.id, showReadStats]);

  const isPublished = announcement.status !== "draft";
  const poll = isPoll(announcement);
  const chips = targetChips(announcement.targets, groups, activities);

  return (
    <Modal
      isOpen
      onClose={onClose}
      title={announcement.title}
      widthClass="mx-4 w-[calc(100%-2rem)] max-w-2xl"
    >
      <div className="space-y-5">
        <div className="flex flex-wrap items-center gap-2">
          <StatusPill status={announcement.status} />
          {announcement.priority === "important" && (
            <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-semibold text-gray-700">
              Wichtig
            </span>
          )}
          <span className="text-xs text-gray-500">
            {announcement.published_at
              ? `Veröffentlicht ${formatDate(announcement.published_at)}`
              : "Noch nicht veröffentlicht"}
            {announcement.expires_at &&
              ` · Läuft ab ${formatBerlinDate(announcement.expires_at)}`}
          </span>
        </div>

        <p className="text-sm leading-6 whitespace-pre-line text-gray-800">
          <LinkifiedText text={announcement.body} />
        </p>

        {announcement.link_url && (
          <a
            href={announcement.link_url}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex max-w-full items-center gap-1.5 text-sm font-medium text-[#5080D8] underline underline-offset-2 hover:text-[#3f68b5]"
          >
            <ExternalLink className="h-4 w-4 shrink-0" aria-hidden />
            <span className="truncate">{announcement.link_url}</span>
          </a>
        )}

        <div>
          <h4 className="mb-1.5 text-sm font-semibold text-gray-900">
            Zielgruppen
          </h4>
          <div className="flex flex-wrap gap-1.5">
            {chips.map((chip) => (
              <span
                key={chip}
                className="inline-flex items-center rounded-full bg-gray-100 px-3 py-1 text-xs font-medium text-gray-700"
              >
                {chip}
              </span>
            ))}
          </div>
          <p className="mt-2 text-xs text-gray-500">
            {poll
              ? RESPONSE_TYPE_LABEL[announcement.response_type]
              : announcement.requires_acknowledgement
                ? "Lesebestätigung erforderlich"
                : "Keine Lesebestätigung erforderlich"}
            {" · "}
            {announcement.send_email
              ? "E-Mail-Benachrichtigung aktiviert"
              : "Keine E-Mail-Benachrichtigung"}
            {poll && announcement.response_deadline && (
              <>
                {" "}
                · Antwort bis {formatBerlinDate(announcement.response_deadline)}
              </>
            )}
          </p>
        </div>

        {poll && (
          <PollResultsPanel
            announcement={announcement}
            onReminded={onReminded}
          />
        )}

        {/* A published poll shows its Auswertung instead of the read/ack
            statistics: the two count different things (children vs guardian
            accounts), and two different denominators in one modal is a support
            ticket waiting to happen. A DRAFT poll still shows the reach, which
            is the number staff check before publishing. */}
        {showReadStats && (
          <div>
            <h4 className="mb-1.5 flex items-center gap-1.5 text-sm font-semibold text-gray-900">
              <BarChart3 className="h-4 w-4 text-gray-500" aria-hidden />
              {isPublished ? "Statistik" : "Aktuelle Reichweite"}
            </h4>
            {error ? (
              <p className="text-sm text-[#CC2626]">{error}</p>
            ) : stats === null || recipients === null ? (
              <Loading fullPage={false} />
            ) : (
              <>
                <p className="text-sm text-gray-700">
                  {isPublished ? (
                    <>
                      {stats.read_count} von {stats.target_count} gelesen
                      {announcement.requires_acknowledgement &&
                        ` · ${stats.acknowledged_count} von ${stats.target_count} bestätigt`}
                    </>
                  ) : (
                    <>
                      Erreicht aktuell {stats.target_count}{" "}
                      {stats.target_count === 1 ? "Elternteil" : "Eltern"}.
                    </>
                  )}
                </p>
                {recipients.length > 0 && (
                  <RecipientList
                    recipients={recipients}
                    showStatus={isPublished}
                    statusFilter={statusFilter}
                    onStatusFilter={setStatusFilter}
                    nameFilter={nameFilter}
                    onNameFilter={setNameFilter}
                  />
                )}
              </>
            )}
          </div>
        )}
      </div>
    </Modal>
  );
}

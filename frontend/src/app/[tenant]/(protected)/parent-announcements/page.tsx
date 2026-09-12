"use client";

import { Suspense, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import {
  Check,
  ListChecks,
  Megaphone,
  Plus,
  Paperclip,
  Send,
  Trash2,
} from "lucide-react";

import { TenantPage } from "~/components/ui/tenant-page";
import type {
  FilterConfig,
  ActiveFilter,
} from "~/components/ui/page-header/types";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { DataTable } from "~/components/ui/data-table";
import { StatusBadge } from "~/components/ui/status-badge";
import type { DataTableColumn } from "~/components/ui/data-table";
import { Button } from "~/components/ui/button";
import {
  SlideOver,
  SlideOverBody,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { Alert } from "~/components/ui/alert";
import { useFormError } from "~/components/ui/form-error";
import { Input } from "~/components/ui/input";
import { Checkbox } from "~/components/ui/checkbox";
import { DatePicker } from "~/components/ui/date-picker";
import {
  AnnouncementStatusBadge,
  KIND_PARAM,
  kindFromParam,
  kindOf,
  STATUS_LABEL,
  summarizeTargets,
} from "~/components/announcements/announcement-meta";
import type { AnnouncementKind } from "~/components/announcements/announcement-meta";
import {
  buildAnnouncementMenuItems,
  DeleteAnnouncementDialog,
  PublishAnnouncementDialog,
  UnpublishAnnouncementDialog,
} from "~/components/announcements/announcement-lifecycle-dialogs";
import { useUpdateUrlParams } from "~/hooks/useUpdateUrlParams";
import { useTenantRouter } from "~/lib/tenant-router";
import { MultiCheckboxSelect } from "~/components/ui/multi-checkbox-select";
import { WizardStepper } from "~/components/ui/wizard-stepper";
import { SegmentedControl } from "~/components/ui/segmented-control";
import type { SegmentedControlItem } from "~/components/ui/segmented-control";
import { AttachmentList } from "~/components/ui/attachment-list";
import { formatBytes } from "~/lib/files-api";
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
  fetchAnnouncements,
  publishAnnouncement,
  updateAnnouncement,
  announcementAttachmentDownloadUrl,
  deleteAnnouncementAttachment,
  fetchAnnouncementAttachments,
  uploadAnnouncementAttachment,
} from "~/lib/parent-announcements-api";
import type {
  Announcement,
  AnnouncementAttachment,
  AnnouncementEmailAudience,
  AnnouncementInput,
  AnnouncementPriority,
  AnnouncementStatus,
  AnnouncementTarget,
} from "~/lib/parent-announcements-api";

const logger = createLogger({ component: "ParentAnnouncementsPage" });

const PRIORITY_OPTIONS: ReadonlyArray<{
  value: AnnouncementPriority;
  label: string;
}> = [
  { value: "info", label: "Info" },
  { value: "important", label: "Wichtig" },
];

const KIND_ITEMS: ReadonlyArray<SegmentedControlItem<AnnouncementKind>> = [
  { value: "announcement", label: "Mitteilungen" },
  { value: "letter", label: "Elternbriefe" },
  { value: "poll", label: "Umfragen" },
];

function announcementStatusFilterFromParam(
  value: string | null,
): "all" | AnnouncementStatus {
  return value === "draft" || value === "published" || value === "expired"
    ? value
    : "all";
}

/**
 * Who receives the e-mail. Deliberately a value choice, not a content panel, so
 * SegmentedControl is the right control (ui/Tabs is for panels).
 */
const EMAIL_AUDIENCE_ITEMS: ReadonlyArray<
  SegmentedControlItem<AnnouncementEmailAudience>
> = [
  { value: "portal_only", label: "Nur mit Portalzugang" },
  { value: "all_contacts", label: "Alle Bezugspersonen" },
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
  letter: {
    title: "Elternbriefe",
    action: "Elternbrief",
    ariaLabel: "Neuen Elternbrief erstellen",
    emptyTitle: "Keine Elternbriefe",
    emptyBody:
      "Ein Elternbrief geht gleichzeitig ins Elternportal und per E-Mail und wird von den Eltern bestätigt.",
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

/** Stable identity for a target so duplicates are rejected and React keys are unique. */
function targetKey(target: AnnouncementTarget): string {
  return `${target.target_type}:${target.ref_id ?? target.ref_text ?? ""}`;
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
    return "Bitte einen vollständigen Link angeben (z. B. https://…).";
  }
}

export default function ParentAnnouncementsPage() {
  // useSearchParams (Reiter und `?bearbeiten=`) braucht die Suspense-Grenze.
  return (
    <Suspense fallback={null}>
      <ParentAnnouncementsContent />
    </Suspense>
  );
}

function ParentAnnouncementsContent() {
  const searchParams = useSearchParams();
  const updateUrlParams = useUpdateUrlParams();
  const router = useTenantRouter();
  // Der Reiter steht in der Adresse (`?art=`), damit die Objektseite den
  // Rückweg auf denselben Reiter setzen kann (#3115).
  const kind = kindFromParam(searchParams.get("art"));
  const setKind = (next: AnnouncementKind) =>
    updateUrlParams({ art: next === "announcement" ? null : KIND_PARAM[next] });
  const [searchTerm, setSearchTerm] = useState(
    () => searchParams.get("search") ?? "",
  );
  const [statusFilter, setStatusFilter] = useState<"all" | AnnouncementStatus>(
    () => announcementStatusFilterFromParam(searchParams.get("status")),
  );

  const [isFormOpen, setIsFormOpen] = useState(false);
  const [editing, setEditing] = useState<Announcement | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Announcement | null>(null);
  const [publishTarget, setPublishTarget] = useState<Announcement | null>(null);
  const [unpublishTarget, setUnpublishTarget] = useState<Announcement | null>(
    null,
  );

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

  // Targeting data sources — fetched while the form is open (pickers).
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

  // Die Objektseite (#3115) schickt „Bearbeiten" mit `?bearbeiten=<id>`
  // hierher, weil der Assistent auf der Liste wohnt. Der Parameter wird nach
  // dem Öffnen entfernt, damit Neuladen den Assistenten nicht erneut öffnet.
  const editRequestId = searchParams.get("bearbeiten");
  useEffect(() => {
    if (!editRequestId || announcements === undefined) return;
    const requested = announcements.find((a) => a.id === editRequestId);
    if (requested && requested.status === "draft" && !requested.system_kind) {
      setEditing(requested);
      setIsFormOpen(true);
    }
    updateUrlParams({ bearbeiten: null });
  }, [announcements, editRequestId, updateUrlParams]);

  // Die Objektseite einer Mitteilung, mit dem aktuellen Reiter, der Suche und
  // dem Statusfilter als Rückweg.
  const collectionReferrer = useMemo(() => {
    const query = new URLSearchParams(searchParams);
    query.set("art", KIND_PARAM[kind]);
    if (searchTerm) query.set("search", searchTerm);
    else query.delete("search");
    if (statusFilter !== "all") query.set("status", statusFilter);
    else query.delete("status");
    const serialized = query.toString();
    return serialized
      ? `/parent-announcements?${serialized}`
      : "/parent-announcements";
  }, [kind, searchParams, searchTerm, statusFilter]);
  const objectPath = (announcement: Announcement) =>
    `/parent-announcements/${encodeURIComponent(announcement.id)}?from=${encodeURIComponent(
      collectionReferrer,
    )}`;

  const filtered = useMemo(() => {
    const term = searchTerm.trim().toLowerCase();
    return list.filter((a) => {
      if (kindOf(a) !== kind) return false;
      if (statusFilter !== "all" && a.status !== statusFilter) return false;
      if (term && !a.title.toLowerCase().includes(term)) return false;
      return true;
    });
  }, [list, kind, searchTerm, statusFilter]);

  // Counts for the tab labels: both lists come from one fetch, so switching
  // tabs costs nothing and the user sees where the work is.
  const kindCounts = useMemo(
    () => ({
      announcement: list.filter((a) => kindOf(a) === "announcement").length,
      letter: list.filter((a) => kindOf(a) === "letter").length,
      poll: list.filter((a) => kindOf(a) === "poll").length,
    }),
    [list],
  );

  const copy = KIND_COPY[kind];

  // Statuszeile unter dem Seitentitel, allein aus der geladenen Liste:
  // wie viele Einträge der aktiven Art es gibt und wie viele davon
  // veröffentlicht sind.
  const kindSummary = (() => {
    const ofKind = list.filter((entry) => kindOf(entry) === kind);
    const published = ofKind.filter(
      (entry) => entry.status === "published",
    ).length;
    const noun =
      kind === "poll"
        ? ofKind.length === 1
          ? "Umfrage"
          : "Umfragen"
        : kind === "letter"
          ? ofKind.length === 1
            ? "Elternbrief"
            : "Elternbriefe"
          : ofKind.length === 1
            ? "Mitteilung"
            : "Mitteilungen";
    return `${ofKind.length} ${noun} · ${published} veröffentlicht`;
  })();

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
        label: STATUS_LABEL[statusFilter],
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

  const columns: DataTableColumn<Announcement>[] = [
    {
      key: "title",
      header: "Titel",
      sortValue: (row) => row.title.toLowerCase(),
      render: (row) => (
        <div className="min-w-0">
          <div className="flex min-w-0 items-center gap-2">
            <p className="truncate font-semibold text-gray-900">{row.title}</p>
            {row.system_kind === "care_cancellation" && (
              <StatusBadge
                label="Ausfall"
                tone="red"
                title="Automatisch beim Absagen eines Termins erstellt"
              />
            )}
            {row.priority === "important" &&
              row.system_kind !== "care_cancellation" && (
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
      render: (row) => <AnnouncementStatusBadge status={row.status} />,
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
        // stopPropagation keeps menu clicks from also triggering the row
        // click (which opens the detail view).
        <div
          className="flex items-center justify-end"
          onClick={(e) => e.stopPropagation()}
          onKeyDown={(e) => e.stopPropagation()}
          role="presentation"
        >
          <OverflowMenu
            items={buildAnnouncementMenuItems(row, {
              onPublish: () => setPublishTarget(row),
              onView: () => router.push(objectPath(row)),
              onEdit: () => openEdit(row),
              onUnpublish: () => setUnpublishTarget(row),
              onDelete: () => setDeleteTarget(row),
            })}
            ariaLabel={`Aktionen für ${row.title}`}
          />
        </div>
      ),
    },
  ];

  return (
    <TenantPage
      title={copy.title}
      stats={kindSummary}
      statsLoading={isLoading && !announcements}
      actions={
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
      search={{
        value: searchTerm,
        onChange: setSearchTerm,
        placeholder: "Titel suchen…",
      }}
      filters={filterConfigs}
      activeFilters={activeFilters}
      onClearAllFilters={() => {
        setSearchTerm("");
        setStatusFilter("all");
      }}
      // Mitteilungen und Umfragen sind zwei Listen derselben Seite, also
      // Seitenreiter — nicht ein Filter in der Suchzeile.
      tabs={{
        value: kind,
        onChange: (value) => setKind(value as AnnouncementKind),
        items: KIND_ITEMS.map((item) => ({
          value: item.value,
          label: item.label,
          badge: kindCounts[item.value],
        })),
        label: "Mitteilungen oder Umfragen",
      }}
      loading={isLoading}
      error={
        loadError
          ? {
              message: "Elternmitteilungen konnten nicht geladen werden.",
              keepContent: announcements !== undefined,
            }
          : null
      }
      empty={
        !isLoading && filtered.length === 0
          ? {
              icon:
                kind === "poll" ? (
                  <ListChecks className="h-12 w-12" aria-hidden />
                ) : (
                  <Megaphone className="h-12 w-12" aria-hidden />
                ),
              title: copy.emptyTitle,
              description: copy.emptyBody,
              action: (
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
              ),
            }
          : null
      }
      overlays={
        <>
          {isFormOpen && (
            <AnnouncementFormModal
              announcement={editing}
              kind={
                // kindOf, not a poll/announcement ternary: editing a letter must keep
                // its mode, otherwise saving the draft would silently downgrade it to
                // a plain Mitteilung and drop the mandatory channels.
                editing ? kindOf(editing) : kind
              }
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

          {publishTarget && (
            <PublishAnnouncementDialog
              announcement={publishTarget}
              onClose={() => setPublishTarget(null)}
              onDone={async () => {
                await mutate();
              }}
            />
          )}

          {unpublishTarget && (
            <UnpublishAnnouncementDialog
              announcement={unpublishTarget}
              onClose={() => setUnpublishTarget(null)}
              onDone={async () => {
                await mutate();
              }}
            />
          )}

          {deleteTarget && (
            <DeleteAnnouncementDialog
              announcement={deleteTarget}
              onClose={() => setDeleteTarget(null)}
              onDone={async () => {
                await mutate();
              }}
            />
          )}
        </>
      }
    >
      <DataTable
        stackedOnMobile
        columns={columns}
        rows={filtered}
        getRowKey={(row) => row.id}
        defaultSortKey="title"
        // Die Zeile öffnet die Objektseite (BAUARTEN-SPEC Bauart 1 Regel 2);
        // „Anzeigen" im Aktionsmenü tut dasselbe per Tastatur. Deshalb darf
        // die Zeile mit ihren eigenen Aktionen nicht zusätzlich als
        // Schaltfläche angekündigt werden.
        onRowClick={(row) => router.push(objectPath(row))}
        rowHasInteractiveControls
      />
    </TenantPage>
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
 * Anhänge (#2890). Both limits mirror the backend
 * (models/filestore.MaxAnnouncementAttachments, api/filestore.maxFile) and are
 * stated in the form BEFORE a file is picked — an error message after a failed
 * upload arrives too late to help.
 */
const MAX_ATTACHMENTS = 5;
// Als runde Zahl geschrieben, nicht über formatBytes: „25 MB" ist die Grenze,
// die jemand im Kopf behält, „25.0 MB" liest sich wie eine Messung.
const MAX_ATTACHMENT_MB = 25;
const MAX_ATTACHMENT_BYTES = MAX_ATTACHMENT_MB * 1024 * 1024;
const ACCEPTED_ATTACHMENT_TYPES = ".pdf,.docx,.xlsx,.pptx,.png,.jpg,.jpeg";
// Sagt, was zu tun ist, nicht nur dass es nicht geht. Steht sowohl im
// Anhangsbereich einer veröffentlichten Mitteilung als auch in der Meldung,
// falls jemand die Schaltfläche doch noch erwischt.
const ATTACHMENTS_LOCKED_HINT =
  "Die Mitteilung ist veröffentlicht. Die Dateien stehen jetzt fest. Ziehen Sie die Mitteilung zurück, wenn Sie etwas ändern möchten.";

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
  // An Elternbrief (#2384) is the same entity with both channels mandatory: the
  // backend forces them and a DB constraint guarantees them, so the form shows
  // them as fixed facts rather than as choices that could be unticked.
  const isLetterForm = kind === "letter";
  const [step, setStep] = useState(0);
  // Tracks the id of the draft once it exists in the backend. Seeded from an
  // existing draft when editing; set after a create so that a publish-retry
  // (the modal stays open when publishing fails) updates that same draft
  // instead of creating a duplicate.
  const [persistedId, setPersistedId] = useState<string | null>(
    announcement?.id ?? null,
  );

  // Anhänge (#2890). Eine schon hochgeladene Datei liegt beim Backend
  // (existingAttachments), eine gerade ausgewählte noch nicht (pendingFiles):
  // beim Anlegen gibt es die Mitteilung ja erst nach dem Speichern. Beide
  // Listen stehen untereinander, damit die schreibende Person nicht zwischen
  // "schon da" und "kommt noch" unterscheiden muss.
  const [existingAttachments, setExistingAttachments] = useState<
    AnnouncementAttachment[]
  >([]);
  const [pendingFiles, setPendingFiles] = useState<File[]>([]);
  const [attachmentError, setAttachmentError] = useState("");
  const [attachmentBusy, setAttachmentBusy] = useState(false);
  // Ob die Anhänge dieser Mitteilung noch änderbar sind. Die Antwort der
  // Liste sagt es: ein Entwurf, der zwischenzeitlich anderswo veröffentlicht
  // wurde, ist es nicht mehr. Ohne diesen Zustand bietet der Assistent weiter
  // "Datei auswählen" und ein Kreuz an, die dann beide mit 409 scheitern.
  const [attachmentsEditable, setAttachmentsEditable] = useState(true);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Beim Bearbeiten eines Entwurfs die schon hochgeladenen Dateien nachladen.
  // Scheitert das, sagt der Anhangsbereich das auch: eine leere Liste liest
  // sich sonst als „es hängt nichts dran", und die schreibende Person hängt
  // eine Datei ein zweites Mal an oder hält eine vorhandene für gelöscht.
  useEffect(() => {
    if (!persistedId) return;
    let cancelled = false;
    void fetchAnnouncementAttachments(persistedId)
      .then((list) => {
        if (!cancelled) {
          setExistingAttachments(list.attachments);
          setAttachmentsEditable(list.editable);
          setAttachmentError("");
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setAttachmentError(
            "Die vorhandenen Dateien konnten nicht geladen werden. Bitte öffnen Sie die Mitteilung noch einmal.",
          );
        }
        logger.error("announcement_attachments_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      cancelled = true;
    };
  }, [persistedId]);

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
  // Who gets the mail — a separate question from who sees the announcement in
  // the portal. The narrow option is the default so an oversight never
  // discloses the text to someone deliberately kept out of the portal.
  const [emailAudience, setEmailAudience] = useState<AnnouncementEmailAudience>(
    announcement?.email_audience ?? "portal_only",
  );
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
  const [formError, setFormError] = useFormError();

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
          "Die Antwortfrist darf nicht nach dem Ablaufdatum liegen, sonst können Eltern nicht mehr antworten.",
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

  const attachmentCount = existingAttachments.length + pendingFiles.length;

  const addFiles = (files: FileList | null) => {
    if (!files || files.length === 0) return;
    if (!attachmentsEditable) {
      setAttachmentError(ATTACHMENTS_LOCKED_HINT);
      return;
    }
    setAttachmentError("");
    const room = MAX_ATTACHMENTS - attachmentCount;
    const picked = Array.from(files);
    if (picked.length > room) {
      setAttachmentError(
        `Es sind höchstens ${MAX_ATTACHMENTS} Dateien je Mitteilung möglich.`,
      );
    }
    const accepted: File[] = [];
    for (const file of picked.slice(0, Math.max(room, 0))) {
      if (file.size > MAX_ATTACHMENT_BYTES) {
        setAttachmentError(
          `„${file.name}“ ist zu groß. Erlaubt sind bis zu ${MAX_ATTACHMENT_MB} MB je Datei.`,
        );
        continue;
      }
      accepted.push(file);
    }
    if (accepted.length > 0) setPendingFiles((prev) => [...prev, ...accepted]);
  };

  const removePendingFile = (index: number) => {
    setAttachmentError("");
    setPendingFiles((prev) => prev.filter((_, i) => i !== index));
  };

  const removeExistingAttachment = async (attachmentId: string) => {
    if (!persistedId) return;
    if (!attachmentsEditable) {
      setAttachmentError(ATTACHMENTS_LOCKED_HINT);
      return;
    }
    setAttachmentBusy(true);
    setAttachmentError("");
    try {
      await deleteAnnouncementAttachment(persistedId, attachmentId);
      setExistingAttachments((prev) =>
        prev.filter((a) => a.id !== attachmentId),
      );
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Der Anhang konnte nicht entfernt werden";
      setAttachmentError(message);
      logger.error("announcement_attachment_delete_failed", { error: message });
    } finally {
      setAttachmentBusy(false);
    }
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
      // A letter always carries both channels; the backend forces them anyway,
      // but sending the true values keeps the payload honest.
      requires_acknowledgement: isPollForm
        ? false
        : isLetterForm || requiresAck,
      send_email: isLetterForm || sendEmail,
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
      delivery_mode: isLetterForm ? "letter" : "standard",
      // A broad e-mail audience belongs only to letters; standard announcements
      // always retain the existing portal-only delivery scope.
      email_audience: isLetterForm ? emailAudience : "portal_only",
    };

    setSubmitting(publish ? "publish" : "draft");
    setFormError("");
    try {
      const saved = persistedId
        ? await updateAnnouncement(persistedId, input)
        : await createAnnouncement(input);
      setPersistedId(saved.id);

      // Anhänge vor dem Veröffentlichen hochladen: danach ist die Mitteilung
      // unveränderlich und das Backend lehnt jeden weiteren Anhang ab. Der
      // Entwurf ist an dieser Stelle schon gespeichert, deshalb bleibt bei
      // einem Fehlschlag nur der Upload offen — und in der Warteliste stehen
      // dann genau die Dateien, die noch nicht oben sind, so dass ein zweiter
      // Versuch nichts erneut auswählen und nichts doppelt hochladen muss.
      if (pendingFiles.length > 0) {
        try {
          for (const file of pendingFiles) {
            const uploaded = await uploadAnnouncementAttachment(saved.id, file);
            // Jede Datei verlässt die Warteliste sofort nach ihrem Upload,
            // nicht erst am Ende der Schleife: scheitert eine spätere, lädt der
            // zweite Versuch sonst die schon übertragenen erneut hoch — der
            // Anhang läge doppelt an, und die Höchstzahl wäre schneller
            // erreicht als die Person Dateien ausgewählt hat.
            setPendingFiles((prev) => prev.filter((entry) => entry !== file));
            if (uploaded) {
              setExistingAttachments((prev) => [...prev, uploaded]);
            }
          }
        } catch (err) {
          const message =
            err instanceof Error
              ? err.message
              : "Die Datei konnte nicht hochgeladen werden";
          setFormError(
            `Als Entwurf gespeichert, aber ein Anhang konnte nicht hochgeladen werden: ${message}`,
          );
          logger.error("announcement_attachment_upload_failed", {
            error: message,
          });
          await onRefresh();
          return;
        }
      }

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
          loadingText="Wird gespeichert…"
          disabled={submitting === "publish"}
        >
          Als Entwurf speichern
        </Button>
        <Button
          type="button"
          size="md"
          onClick={() => void handleSubmit(true)}
          isLoading={submitting === "publish"}
          loadingText="Wird veröffentlicht…"
          disabled={submitting === "draft"}
          className="gap-1.5"
        >
          <Send className="size-4" aria-hidden />
          Veröffentlichen
        </Button>
      </>
    );

  return (
    <SlideOver
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <SlideOverContent widthClass="sm:w-[860px]">
        <SlideOverHeader className="flex-row items-start justify-between gap-3">
          <div className="min-w-0">
            <SlideOverTitle>
              {isPollForm
                ? isEdit
                  ? "Umfrage bearbeiten"
                  : "Neue Umfrage"
                : isLetterForm
                  ? isEdit
                    ? "Elternbrief bearbeiten"
                    : "Neuer Elternbrief"
                  : isEdit
                    ? "Elternmitteilung bearbeiten"
                    : "Neue Elternmitteilung"}
            </SlideOverTitle>
          </div>
          <SlideOverCloseButton />
        </SlideOverHeader>
        {/* Der Fehler des Formulars steht oben im Rumpf, nicht unten über
            dem Footer (Bauart 2 Regel 5, #3113). */}
        <SlideOverBody error={formError} className="space-y-4">
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
                    placeholder="Inhalt der Mitteilung… Links im Text werden für Eltern klickbar."
                    className="block w-full rounded-lg border-0 bg-white px-4 py-3 text-base text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400"
                  />
                </div>

                <Input
                  label="Link (optional)"
                  name="announcement-link"
                  type="url"
                  value={linkUrl}
                  onChange={(e) => setLinkUrl(e.target.value)}
                  placeholder="https://…"
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
                              onChange={(e) =>
                                setOptionAt(index, e.target.value)
                              }
                              placeholder={`Antwort ${index + 1}`}
                              aria-label={`Antwort ${index + 1}`}
                              maxLength={120}
                            />
                          </div>
                          <button
                            type="button"
                            onClick={() => removeOptionAt(index)}
                            aria-label={`Antwort ${index + 1} entfernen`}
                            className="border-moto-red/20 text-moto-red-strong hover:bg-moto-red/10 focus-visible:ring-moto-red/30 inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border bg-white shadow-sm transition-colors focus-visible:ring-2 focus-visible:outline-none"
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
                      Danach wird{" "}
                      {isPollForm
                        ? "die Umfrage"
                        : isLetterForm
                          ? "der Elternbrief"
                          : "die Mitteilung"}{" "}
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

                {/* An Elternbrief is defined by both channels being mandatory, so
                  they are stated as facts instead of offered as choices. Shown,
                  not hidden: the author must see what will happen. */}
                {isLetterForm && (
                  <div className="rounded-lg border border-gray-200 bg-gray-50 p-4">
                    <p className="text-sm font-medium text-gray-800">
                      Beim Veröffentlichen passiert automatisch:
                    </p>
                    <ul className="mt-2 space-y-1 text-sm text-gray-700">
                      <li>Der Brief erscheint vollständig im Elternportal.</li>
                      <li>
                        Die Bezugspersonen bekommen den Brieftext per E-Mail.
                      </li>
                      <li>
                        Eltern bestätigen den Brief im Elternportal. Eine
                        Bestätigung pro Kind genügt.
                      </li>
                    </ul>
                  </div>
                )}

                {/* A poll answer already IS the confirmation — offering a second,
                  weaker "gelesen" checkbox on top only muddies the result. */}
                {!isPollForm && !isLetterForm && (
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
                      <span className="block">
                        Lesebestätigung erforderlich
                      </span>
                      <span className="block text-xs text-gray-500">
                        Eltern bestätigen ausdrücklich, dass sie die Mitteilung
                        gelesen haben.
                      </span>
                    </span>
                  </label>
                )}

                {!isLetterForm && (
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
                )}

                {(isLetterForm || sendEmail) && (
                  <div>
                    <p className="mb-2 text-sm font-medium text-gray-800">
                      Wer erhält die E-Mail?
                    </p>
                    <SegmentedControl
                      items={
                        isLetterForm
                          ? EMAIL_AUDIENCE_ITEMS
                          : EMAIL_AUDIENCE_ITEMS.slice(0, 1)
                      }
                      value={emailAudience}
                      onChange={setEmailAudience}
                      ariaLabel="E-Mail-Empfänger"
                      fullWidth
                    />
                    <p className="mt-2 text-xs text-gray-500">
                      {emailAudience === "all_contacts"
                        ? "Auch Bezugspersonen ohne Portal-Zugang bekommen die E-Mail. Sie können den Brief nicht in moto bestätigen. Geht es um Gesundheit oder andere sensible Angaben zu einem Kind? Dann wählen Sie die andere Option."
                        : "Nur Bezugspersonen mit Portal-Zugang bekommen die E-Mail. Alle anderen sehen Sie danach in der Empfängerliste."}
                    </p>
                  </div>
                )}
              </section>

              <section className="flex flex-col gap-3 border-t border-gray-200 pt-4">
                <div>
                  <p className="flex items-center gap-2 text-sm font-medium text-gray-800">
                    <Paperclip className="h-4 w-4 text-gray-400" aria-hidden />
                    Dateien anhängen
                  </p>
                  <p className="mt-1 text-xs text-gray-500">
                    Die Dateien sehen genau die Eltern, die auch die Mitteilung
                    bekommen – nicht alle Eltern der Schule. Sie liegen im
                    Elternportal zum Herunterladen bereit und gehen nicht per
                    E-Mail mit.
                  </p>
                  <p className="mt-1 text-xs text-gray-500">
                    Erlaubt sind PDF, DOCX, XLSX, PPTX, PNG und JPEG. Bis zu{" "}
                    {MAX_ATTACHMENT_MB} MB je Datei, höchstens {MAX_ATTACHMENTS}{" "}
                    Dateien.
                  </p>
                </div>

                {existingAttachments.length > 0 && (
                  <AttachmentList
                    attachments={existingAttachments}
                    downloadUrl={(attachmentId) =>
                      announcementAttachmentDownloadUrl(
                        persistedId ?? "",
                        attachmentId,
                      )
                    }
                    onRemove={
                      attachmentsEditable
                        ? (attachmentId) =>
                            void removeExistingAttachment(attachmentId)
                        : undefined
                    }
                    busy={attachmentBusy}
                  />
                )}

                {pendingFiles.length > 0 && (
                  <ul className="flex flex-col gap-2">
                    {pendingFiles.map((file, index) => (
                      <li
                        key={`${file.name}-${index}`}
                        className="flex items-center gap-3 rounded-md border border-dashed border-gray-300 bg-gray-50 px-3 py-2"
                      >
                        <Paperclip
                          className="h-4 w-4 shrink-0 text-gray-400"
                          aria-hidden
                        />
                        <span className="min-w-0 flex-1 truncate text-sm text-gray-900">
                          {file.name}
                        </span>
                        <span className="shrink-0 text-xs text-gray-500">
                          {formatBytes(file.size)}
                        </span>
                        <Button
                          type="button"
                          variant="ghost"
                          size="compact"
                          onClick={() => removePendingFile(index)}
                        >
                          Entfernen
                        </Button>
                      </li>
                    ))}
                    <li className="text-xs text-gray-500">
                      Diese Dateien werden mit dem Speichern hochgeladen.
                    </li>
                  </ul>
                )}

                {attachmentsEditable ? (
                  <div>
                    <input
                      ref={fileInputRef}
                      type="file"
                      multiple
                      accept={ACCEPTED_ATTACHMENT_TYPES}
                      className="hidden"
                      onChange={(e) => {
                        addFiles(e.target.files);
                        e.target.value = "";
                      }}
                    />
                    <Button
                      type="button"
                      variant="outline"
                      size="md"
                      disabled={
                        attachmentBusy || attachmentCount >= MAX_ATTACHMENTS
                      }
                      onClick={() => fileInputRef.current?.click()}
                    >
                      Datei auswählen
                    </Button>
                    {attachmentCount >= MAX_ATTACHMENTS && (
                      <p className="mt-2 text-xs text-gray-500">
                        Die Höchstzahl ist erreicht. Entfernen Sie eine Datei,
                        um eine andere anzuhängen.
                      </p>
                    )}
                  </div>
                ) : (
                  <p className="text-xs text-gray-500">
                    {ATTACHMENTS_LOCKED_HINT}
                  </p>
                )}

                {attachmentError && (
                  <Alert type="error" message={attachmentError} />
                )}
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
              kindLabel={
                isPollForm
                  ? "diese Umfrage"
                  : isLetterForm
                    ? "diesen Elternbrief"
                    : "diese Mitteilung"
              }
              allowPendingEnrollment={!isPollForm && !isLetterForm}
            />
          )}
        </SlideOverBody>
        <SlideOverFooter className="flex-row flex-wrap justify-end gap-2">
          {footer}
        </SlideOverFooter>
      </SlideOverContent>
    </SlideOver>
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
                searchPlaceholder="Klasse suchen…"
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
                searchPlaceholder="Gruppe suchen…"
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
                searchPlaceholder="AG suchen…"
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
              placeholder="Kind suchen (mind. 2 Zeichen)…"
            />
            {debouncedSearch.length >= 2 && (
              <div className="mt-2 max-h-40 overflow-y-auto rounded-lg border border-gray-200">
                {studentsLoading ? (
                  <p className="px-3 py-2 text-sm text-gray-500">
                    Wird gesucht…
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

/** Chips describing the audience, with real names where the lookups know them. */

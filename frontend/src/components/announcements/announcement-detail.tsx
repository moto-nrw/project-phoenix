"use client";

import { useEffect, useState } from "react";
import {
  BellRing,
  ExternalLink,
  ListChecks,
  Megaphone,
  Send,
} from "lucide-react";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { Input } from "~/components/ui/input";
import { LinkifiedText } from "~/components/ui/linkified-text";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import {
  SegmentedControl,
  type SegmentedControlItem,
} from "~/components/ui/segmented-control";
import { LetterStatusPanel } from "~/components/announcements/letter-status-panel";
import type { Group } from "~/lib/api";
import type { Activity } from "~/lib/activity-helpers";
import { formatBerlinDate } from "~/lib/date-helpers";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import {
  fetchAnnouncementRecipients,
  fetchAnnouncementStats,
  fetchPollChildren,
  fetchPollResults,
  isLetter,
  isPoll,
  remindUnanswered,
} from "~/lib/parent-announcements-api";
import type {
  Announcement,
  AnnouncementRecipient,
  AnnouncementStats,
  PollChild,
  PollResults,
} from "~/lib/parent-announcements-api";
import { RESPONSE_TYPE_LABEL, targetChips } from "./announcement-meta";

const logger = createLogger({ component: "AnnouncementDetail" });

const RECIPIENT_STATUS_META: Record<
  AnnouncementRecipient["status"],
  { label: string; tone: StatusBadgeTone }
> = {
  acknowledged: { label: "Bestätigt", tone: "green" },
  read: { label: "Gelesen", tone: "blue" },
  pending: { label: "Ausstehend", tone: "gray" },
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
export function RecipientList({
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

  const statusItems: ReadonlyArray<
    SegmentedControlItem<"all" | AnnouncementRecipient["status"]>
  > = [
    { value: "all", label: `Alle (${counts.all})` },
    { value: "pending", label: `Ausstehend (${counts.pending})` },
    { value: "read", label: `Gelesen (${counts.read})` },
    { value: "acknowledged", label: `Bestätigt (${counts.acknowledged})` },
  ];

  return (
    <div className="mt-2 space-y-2">
      {showStatus && (
        <SegmentedControl
          items={statusItems}
          value={statusFilter}
          onChange={onStatusFilter}
          variant="pills"
          ariaLabel="Status der Empfänger"
          className="max-w-full overflow-x-auto"
        />
      )}
      {recipients.length > 8 && (
        <Input
          name="recipient-search"
          controlSize="compact"
          value={nameFilter}
          onChange={(e) => onNameFilter(e.target.value)}
          placeholder="Name suchen…"
        />
      )}
      {filtered.length === 0 ? (
        <p className="rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-500">
          Keine Treffer.
        </p>
      ) : (
        <ul className="max-h-72 divide-y divide-gray-100 overflow-y-auto rounded-lg border border-gray-200">
          {filtered.map((rcpt) => (
            <li
              key={rcpt.account_id}
              className="flex items-center justify-between gap-2 px-3 py-2"
            >
              <span className="min-w-0 truncate text-sm text-gray-800">
                {`${rcpt.first_name} ${rcpt.last_name}`.trim() || "Ohne Namen"}
              </span>
              {showStatus && (
                <StatusBadge
                  label={RECIPIENT_STATUS_META[rcpt.status].label}
                  tone={RECIPIENT_STATUS_META[rcpt.status].tone}
                />
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
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
    <SectionCard
      title="Auswertung"
      icon={ListChecks}
      description="Gezählt werden Kinder, nicht Konten: die Zahl, mit der die Schule plant."
    >
      {error && <p className="text-moto-red-strong mb-2 text-sm">{error}</p>}

      {results === null || children === null ? (
        error ? null : (
          <SkeletonRegion label="Umfrageergebnisse werden geladen…">
            <ListSkeleton rows={4} avatar={false} />
          </SkeletonRegion>
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
              <ul className="max-h-72 divide-y divide-gray-100 overflow-y-auto rounded-lg border border-gray-200">
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
                      <span className="text-moto-green-strong shrink-0 text-xs font-medium">
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
              loadingText="Wird gesendet…"
              className="mt-4 gap-1.5"
            >
              <BellRing className="size-4" aria-hidden />
              Eltern ohne Antwort erinnern
            </Button>
          )}
        </>
      )}
    </SectionCard>
  );
}

interface AnnouncementDetailProps {
  readonly announcement: Announcement;
  readonly groups: Group[];
  readonly activities: Activity[];
  /** Called with the number of guardians reached by a poll reminder. */
  readonly onReminded: (count: number) => void;
}

/**
 * Der Inhalt der Objektseite /parent-announcements/[id] (BAUARTEN-SPEC
 * Bauart 2, #3115). Veröffentlichte Mitteilungen sind unveränderlich; hier
 * liest das Personal den vollständigen Text nach und sieht die Reichweite,
 * die Statistik oder die Auswertung, je nach Art.
 */
export function AnnouncementDetail({
  announcement,
  groups,
  activities,
  onReminded,
}: AnnouncementDetailProps) {
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
    (isPoll(announcement) || isLetter(announcement)) &&
    announcement.status !== "draft"
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
  const letter = isLetter(announcement);
  const chips = targetChips(announcement.targets, groups, activities);

  return (
    <div className="space-y-4 sm:space-y-6">
      <SectionCard
        title={poll ? "Umfrage" : letter ? "Elternbrief" : "Mitteilung"}
        icon={poll ? ListChecks : Megaphone}
      >
        <p className="text-sm leading-6 whitespace-pre-line text-gray-800">
          <LinkifiedText text={announcement.body} />
        </p>

        {announcement.link_url && (
          <a
            href={announcement.link_url}
            target="_blank"
            rel="noopener noreferrer"
            className="text-moto-blue hover:text-moto-blue-hover mt-3 inline-flex max-w-full items-center gap-1.5 text-sm font-medium underline underline-offset-2"
          >
            <ExternalLink className="h-4 w-4 shrink-0" aria-hidden />
            <span className="truncate">{announcement.link_url}</span>
          </a>
        )}
      </SectionCard>

      <SectionCard title="Zielgruppen und Zustellung" icon={Send}>
        <DataGrid>
          <DataField label="Zielgruppen" fullWidth>
            <span className="flex flex-wrap gap-1.5">
              {chips.map((chip) => (
                <StatusBadge key={chip} label={chip} tone="gray" />
              ))}
            </span>
          </DataField>
          <DataField label={poll ? "Antwortart" : "Lesebestätigung"}>
            {poll
              ? RESPONSE_TYPE_LABEL[announcement.response_type]
              : letter
                ? "Erforderlich (Elternbrief)"
                : announcement.requires_acknowledgement
                  ? "Erforderlich"
                  : "Nicht erforderlich"}
          </DataField>
          <DataField label="E-Mail an die Eltern">
            {letter
              ? announcement.email_audience === "all_contacts"
                ? "An alle Bezugspersonen"
                : "An Bezugspersonen mit Portalzugang"
              : announcement.send_email
                ? "Wird versendet"
                : "Wird nicht versendet"}
          </DataField>
          {poll && announcement.response_deadline && (
            <DataField label="Antwort bis">
              {formatBerlinDate(announcement.response_deadline)}
            </DataField>
          )}
        </DataGrid>
      </SectionCard>

      {poll && (
        <PollResultsPanel announcement={announcement} onReminded={onReminded} />
      )}

      {/* A published Elternbrief shows the recipient matrix instead of the
        generic read/ack statistics: it counts CHILDREN and reports both
        channels separately, while the generic panel counts guardian
        accounts. Two different denominators on one page is a support
        ticket waiting to happen. */}
      {letter && isPublished && (
        <SectionCard title="Status">
          <LetterStatusPanel
            announcementId={announcement.id}
            canAct={announcement.status === "published"}
            emailAudience={announcement.email_audience}
          />
        </SectionCard>
      )}

      {/* A published poll shows its Auswertung instead of the read/ack
        statistics: the two count different things (children vs guardian
        accounts). A DRAFT poll still shows the reach, which is the number
        staff check before publishing. */}
      {showReadStats && (
        <SectionCard title={isPublished ? "Statistik" : "Aktuelle Reichweite"}>
          {error ? (
            <p className="text-moto-red-strong text-sm">{error}</p>
          ) : stats === null || recipients === null ? (
            <SkeletonRegion label="Statistik wird geladen…">
              <ListSkeleton rows={4} avatar={false} />
            </SkeletonRegion>
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
        </SectionCard>
      )}
    </div>
  );
}

/** Platzhalter der Objektseite, solange die Mitteilung lädt. */
export function AnnouncementDetailSkeleton() {
  return (
    <div
      className="space-y-4 sm:space-y-6"
      role="status"
      aria-label="Mitteilung wird geladen"
      data-testid="announcement-detail-skeleton"
    >
      <SectionCard title="Mitteilung">
        <Skeleton className="h-4 w-full" />
        <Skeleton className="mt-2 h-4 w-11/12" />
        <Skeleton className="mt-2 h-4 w-2/3" />
      </SectionCard>
      <SectionCard title="Zielgruppen und Zustellung">
        <Skeleton className="h-4 w-1/2" />
        <Skeleton className="mt-2 h-4 w-1/3" />
      </SectionCard>
    </div>
  );
}

"use client";

/**
 * Aggregierte Eltern-Anfragenliste (#2432): EINE Liste über alle vier
 * Anfragearten (Stammdaten, Betreuungszeiten, Angebote, Abwesenheiten)
 * statt vier gestapelter Abschnitte. Suche und Filter wirken serverseitig;
 * nachgeladen wird über den Keyset-Cursor des Aggregations-Endpunkts.
 *
 * Die Komponente rendert je nach `view` entweder entscheidbare Karten (die
 * bestehenden Entscheiden-Abläufe leben in den per-Art-Item-Komponenten)
 * oder die read-only Historie-Karten. Der Aufrufer remountet sie beim
 * Umschalten Offen ↔ Historie (key={view}), wie zuvor die Einzelsektionen.
 */

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { TrayIcon } from "@phosphor-icons/react/ssr";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { EmptyState } from "~/components/ui/empty-state";
import { ConfirmationModal } from "~/components/ui/modal";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { CareExitModal } from "~/components/students/care-exit-modal";
import { CareRequestReviewItem } from "~/components/students/care-request-review-item";
import { StudentDeletionModal } from "~/components/students/student-deletion-modal";
import { ExcusedRequestReviewItem } from "~/components/students/excused-request-review-item";
import { MasterDataReviewItem } from "~/components/students/master-data-review-item";
import { OfferingRequestReviewItem } from "~/components/students/offering-request-review-item";
import { EnrollmentRequestItem } from "~/components/students/enrollment-request-item";
import { FamilyProtectionControl } from "~/components/students/family-protection-control";
import { RequestHistoryItem } from "~/components/students/request-history-item";
import {
  RequestReviewCard,
  RequestRowHeader,
} from "~/components/students/request-review-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { Textarea } from "~/components/ui/textarea";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  fetchCareWithdrawals,
  type CareWithdrawalCompletion,
} from "~/lib/care-exit-api";
import {
  type AggregatedHistoryRequest,
  type AggregatedOpenRequest,
  type AggregatedRequestParams,
  type AggregatedRequestStatus,
  type AggregatedRequestType,
  type RequestReviewMetadata,
  bulkApproveParentRequests,
  listAggregatedOpenRequests,
  listAggregatedRequestHistory,
  listEnrollmentChangeRequests,
} from "~/lib/change-request-list-api";
import {
  createFeedState,
  takeMergedPage,
  type FeedState,
  type FeedSource,
} from "~/lib/request-feed";

const logger = createLogger({ component: "AggregatedRequestList" });
const WITHDRAWAL_PAGE_SIZE = 25;

export interface AggregatedRequestFilters {
  readonly search: string;
  /**
   * Nur die Einträge dieses Kindes — das Änderungsprotokoll der Kinderkartei
   * (#2437). Ohne Angabe: alle Kinder, die die Person sehen darf.
   */
  readonly studentId?: string;
  /**
   * Darf der Aggregator über die vier Kinderdaten-Arten abgefragt werden? Er
   * verlangt users:update oder users:absence; wer nur Anmeldungsänderungen
   * entscheidet, bekäme sonst für die ganze Liste einen 403. Ohne Angabe ja —
   * er ist die Hauptquelle der Liste.
   */
  readonly includeAggregated?: boolean;
  /**
   * Dürfen Anmeldungsänderungen mitgeladen werden? Sie hängen an config:manage
   * und kommen aus einem eigenen Endpunkt; ohne das Recht bleibt die Quelle
   * weg, statt der Seite einen 403 einzuhandeln.
   */
  readonly includeEnrollment?: boolean;
  /** Offene Komplett-Abmeldungen; verlangt users:delete. */
  readonly includeCareWithdrawals?: boolean;
  readonly canManageFamilyProtection?: boolean;
  /** Leer = alle Arten. */
  readonly types: readonly AggregatedRequestType[];
  /** Nur Historie; leer = alle Status. */
  readonly statuses: readonly AggregatedRequestStatus[];
  /** Nur Historie, YYYY-MM-DD. */
  readonly from?: string;
  /** Nur Historie, YYYY-MM-DD. */
  readonly to?: string;
}

type AnyItem = AggregatedOpenRequest | AggregatedHistoryRequest;

type OpenCase = {
  readonly key: string;
  readonly studentID?: string;
  readonly studentName: string;
  groupName?: string;
  urgentToday: boolean;
  familyProtected?: boolean;
  readonly items: AggregatedOpenRequest[];
  readonly withdrawals: CareWithdrawalCompletion[];
};

function itemKey(item: AnyItem): string {
  return `${item.request_type}:${item.data.id}`;
}

function hasReviewMetadata(
  item: AggregatedOpenRequest,
): item is Exclude<AggregatedOpenRequest, { request_type: "enrollment" }> {
  return "student_id" in item;
}

function addOpenCase(
  cases: Map<string, OpenCase>,
  item: AggregatedOpenRequest,
  key: string,
  studentName: string,
  metadata?: Partial<RequestReviewMetadata>,
) {
  const existing = cases.get(key);
  if (existing) {
    existing.items.push(item);
    existing.urgentToday ||= metadata?.urgent_today === true;
    if (metadata?.group_name) existing.groupName = metadata.group_name;
    if (metadata?.family_protected === true) existing.familyProtected = true;
    return;
  }
  cases.set(key, {
    key,
    studentID: key.startsWith("request:") ? undefined : key,
    studentName,
    groupName: metadata?.group_name,
    urgentToday: metadata?.urgent_today === true,
    familyProtected: metadata?.family_protected,
    items: [item],
    withdrawals: [],
  });
}

function addEnrollmentCases(
  cases: Map<string, OpenCase>,
  item: Extract<AggregatedOpenRequest, { request_type: "enrollment" }>,
) {
  if (item.data.children?.length) {
    item.data.children.forEach((child) =>
      addOpenCase(
        cases,
        item,
        child.student_id ?? `request:enrollment-child:${child.case_id}`,
        child.name,
      ),
    );
    return;
  }
  const childIDs = item.data.child_ids ?? [];
  if (childIDs.length) {
    childIDs.forEach((id, index) =>
      addOpenCase(cases, item, id, item.data.child_names[index] ?? "Anmeldung"),
    );
    return;
  }
  addOpenCase(
    cases,
    item,
    `request:enrollment:${item.data.id}`,
    item.data.child_names?.join(", ") || "Anmeldung",
  );
}

function addOpenItem(
  cases: Map<string, OpenCase>,
  item: AggregatedOpenRequest,
) {
  const wire = item as AggregatedOpenRequest & Partial<RequestReviewMetadata>;
  if (typeof wire.student_id === "string") {
    addOpenCase(
      cases,
      item,
      wire.student_id,
      wire.student_name ?? "Anfrage",
      wire,
    );
  } else if (item.request_type === "enrollment") {
    addEnrollmentCases(cases, item);
  } else {
    addOpenCase(
      cases,
      item,
      `request:${item.request_type}:${item.data.id}`,
      "Anfrage",
    );
  }
}

function addWithdrawalCase(
  cases: Map<string, OpenCase>,
  row: CareWithdrawalCompletion,
) {
  const existing = cases.get(row.studentId);
  if (existing) {
    existing.withdrawals.push(row);
    existing.urgentToday ||= row.urgency === "overdue";
    return;
  }
  cases.set(row.studentId, {
    key: row.studentId,
    studentID: row.studentId,
    studentName: `${row.firstName} ${row.lastName}`.trim(),
    urgentToday: row.urgency === "overdue",
    items: [],
    withdrawals: [row],
  });
}

function groupOpenCases(
  items: readonly AnyItem[],
  withdrawals: readonly CareWithdrawalCompletion[],
): OpenCase[] {
  const cases = new Map<string, OpenCase>();
  items.forEach((item) => addOpenItem(cases, item as AggregatedOpenRequest));
  withdrawals.forEach((row) => addWithdrawalCase(cases, row));
  return [...cases.values()].sort((left, right) => {
    if (left.urgentToday !== right.urgentToday)
      return left.urgentToday ? -1 : 1;
    const byTime = (right.items[0]?.occurred_at ?? "").localeCompare(
      left.items[0]?.occurred_at ?? "",
    );
    return byTime || left.studentName.localeCompare(right.studentName, "de");
  });
}

const REQUEST_TYPE_LABELS: Record<AggregatedRequestType, string> = {
  master_data: "Stammdaten",
  care_schedule: "Betreuungszeiten",
  offering: "Angebote",
  excused: "Abwesenheit",
  direct_correction: "Direkt-Korrektur",
  enrollment: "Anmeldung",
  care_withdrawal: "Abmeldung",
};

function requestDates(item: AggregatedOpenRequest): readonly string[] {
  if (item.request_type === "excused") return item.data.dates ?? [];
  if (item.request_type === "offering") return [item.data.effective_from];
  return [];
}

function caseDates(childCase: OpenCase): string[] {
  const dates = childCase.items.flatMap(requestDates);
  dates.push(...childCase.withdrawals.map((row) => row.firstBookinglessDay));
  return [...new Set(dates)].sort();
}

function conflictEntries(item: AggregatedOpenRequest): [string, string][] {
  if (item.request_type === "master_data") {
    return [
      [
        `master:${item.data.target}:${item.data.field_key}`,
        JSON.stringify(item.data.new_value),
      ],
    ];
  }
  if (item.request_type === "care_schedule") {
    return (item.data.diff ?? []).map((line) => [
      `care:${line.weekday ?? line.label}:${line.care_kind ?? "value"}`,
      line.new,
    ]);
  }
  if (item.request_type === "offering") {
    return (item.data.diff ?? []).map((line) => [
      `offering:${line.offering_id}`,
      line.new,
    ]);
  }
  if (item.request_type === "excused") {
    return (item.data.dates ?? []).map((date) => [
      date,
      item.data.absence_status,
    ]);
  }
  return [];
}

function hasCaseConflict(childCase: OpenCase): boolean {
  const values = new Map<string, string>();
  for (const item of childCase.items) {
    for (const [key, value] of conflictEntries(item)) {
      const previous = values.get(key);
      if (previous !== undefined && previous !== value) return true;
      values.set(key, value);
    }
  }
  return false;
}

function OpenCaseSummary({ childCase }: Readonly<{ childCase: OpenCase }>) {
  const dates = caseDates(childCase);
  const needsIndividualReview = childCase.items.some(
    (item) => hasReviewMetadata(item) && !item.bulk_eligible,
  );
  const conflict = hasCaseConflict(childCase);
  const parts = [
    childCase.groupName ? `Gruppe: ${childCase.groupName}` : "",
    dates.length > 0
      ? `Betrifft: ${dates.map((date) => formatDate(date)).join(", ")}`
      : "",
    needsIndividualReview ? "Einzeln prüfen" : "",
    conflict ? "Wünsche widersprechen sich" : "Keine Widersprüche",
  ].filter(Boolean);
  return <p className="mt-1 text-xs text-gray-500">{parts.join(" · ")}</p>;
}

function OpenRequestContent({
  request,
  view,
  onDecided,
}: Readonly<{
  request: AggregatedOpenRequest;
  view: "open" | "history";
  onDecided: (notice: string) => void;
}>) {
  switch (request.request_type) {
    case "enrollment":
      return <EnrollmentRequestItem row={request.data} view={view} />;
    case "master_data":
      return <MasterDataReviewItem row={request.data} onDecided={onDecided} />;
    case "care_schedule":
      return <CareRequestReviewItem row={request.data} onDecided={onDecided} />;
    case "offering":
      return (
        <OfferingRequestReviewItem row={request.data} onDecided={onDecided} />
      );
    case "excused":
      return (
        <ExcusedRequestReviewItem row={request.data} onDecided={onDecided} />
      );
  }
}

function OpenRequestRow({
  request,
  selected,
  onSelectionChange,
  onDecided,
}: Readonly<{
  request: AggregatedOpenRequest;
  selected: boolean;
  onSelectionChange: (key: string, checked: boolean) => void;
  onDecided: (key: string, notice: string) => void;
}>) {
  const key = itemKey(request);
  const content = (
    <OpenRequestContent
      request={request}
      view="open"
      onDecided={(notice) => onDecided(key, notice)}
    />
  );
  if (!hasReviewMetadata(request)) return content;
  const disabledReason = request.bulk_eligible
    ? undefined
    : (request.bulk_ineligible_reason ??
      "Diese Anfrage muss einzeln geprüft werden.");
  return (
    <div className="border-t border-gray-100 first:border-t-0">
      <div className="flex items-start gap-3 px-3 py-2 sm:px-4">
        <label
          htmlFor={`bulk-request-${key}`}
          className="flex min-h-11 shrink-0 items-center gap-2 text-sm text-gray-700"
        >
          <Checkbox
            id={`bulk-request-${key}`}
            aria-label={`${request.student_name} für Sammelfreigabe auswählen`}
            checked={selected}
            disabled={!request.bulk_eligible}
            onChange={(event) => onSelectionChange(key, event.target.checked)}
          />
          <span className="sr-only">Für Sammelfreigabe auswählen</span>
        </label>
        <div className="min-w-0 flex-1">
          {disabledReason ? (
            <p className="mb-1 text-xs text-gray-600">{disabledReason}</p>
          ) : null}
          {content}
        </div>
      </div>
    </div>
  );
}

function OpenCaseCard({
  childCase,
  canManageFamilyProtection,
  selected,
  onSelectionChange,
  onDecided,
  onProtectionChanged,
  finishWithdrawal,
  removeWithdrawal,
}: Readonly<{
  childCase: OpenCase;
  canManageFamilyProtection: boolean;
  selected: ReadonlySet<string>;
  onSelectionChange: (key: string, checked: boolean) => void;
  onDecided: (key: string, notice: string) => void;
  onProtectionChanged: (studentID: string, enabled: boolean) => void;
  finishWithdrawal: (row: CareWithdrawalCompletion) => void;
  removeWithdrawal: (row: CareWithdrawalCompletion) => void;
}>) {
  const typeLabels = [
    ...new Set(
      childCase.items.map((item) => REQUEST_TYPE_LABELS[item.request_type]),
    ),
  ];
  if (childCase.withdrawals.length > 0) typeLabels.push("Abmeldung");
  const requestCount = childCase.items.length + childCase.withdrawals.length;
  return (
    <article className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
      <header className="flex flex-wrap items-center justify-between gap-2 px-4 py-3">
        <div className="min-w-0">
          <h3 className="truncate font-semibold text-gray-900">
            {childCase.studentName}
          </h3>
          <p className="text-sm text-gray-600">
            <span>
              {requestCount === 1 ? "1 Wunsch" : `${requestCount} Wünsche`}
            </span>
            {typeLabels.length > 0 ? ` · ${typeLabels.join(", ")}` : ""}
          </p>
          <OpenCaseSummary childCase={childCase} />
        </div>
        <div className="flex flex-wrap items-center justify-end gap-2">
          {childCase.urgentToday ? (
            <StatusBadge tone="red" label="Heute betroffen" />
          ) : null}
          {childCase.studentID && childCase.familyProtected !== undefined ? (
            <FamilyProtectionControl
              studentId={childCase.studentID}
              canManage={canManageFamilyProtection}
              initialEnabled={childCase.familyProtected}
              compact
              onChanged={(enabled) =>
                onProtectionChanged(childCase.studentID!, enabled)
              }
            />
          ) : null}
        </div>
      </header>
      <div>
        {childCase.items.map((request) => (
          <OpenRequestRow
            key={itemKey(request)}
            request={request}
            selected={selected.has(itemKey(request))}
            onSelectionChange={onSelectionChange}
            onDecided={onDecided}
          />
        ))}
        {childCase.withdrawals.map((row) => (
          <OpenWithdrawalCard
            key={`care_withdrawal:${row.id}`}
            row={row}
            finish={finishWithdrawal}
            remove={removeWithdrawal}
          />
        ))}
      </div>
    </article>
  );
}

function OpenCaseGroup(
  props: Readonly<
    { id: string; title: string; cases: readonly OpenCase[] } & Omit<
      Parameters<typeof OpenCaseCard>[0],
      "childCase"
    >
  >,
) {
  if (props.cases.length === 0) return null;
  return (
    <section aria-labelledby={props.id} className="space-y-2">
      <h2 id={props.id} className="text-sm font-semibold text-gray-900">
        {props.title}
      </h2>
      {props.cases.map((childCase) => (
        <OpenCaseCard key={childCase.key} {...props} childCase={childCase} />
      ))}
    </section>
  );
}

/** Wie viele Zeilen eine Seite der Historie zeigt. */
const PAGE_SIZE = 25;

async function takeInitialFeed(
  sources: readonly FeedSource<AnyItem>[],
  feed: FeedState<AnyItem>,
  _view: "open" | "history",
) {
  return takeMergedPage(sources, feed, PAGE_SIZE);
}

function BulkApprovalPanel({
  count,
  reason,
  setReason,
  open,
}: Readonly<{
  count: number;
  reason: string;
  setReason: (value: string) => void;
  open: () => void;
}>) {
  return (
    <div className="moto-content-surface space-y-3 rounded-2xl border p-4 shadow-sm">
      <div>
        <h2 className="font-semibold text-gray-900">Sammelfreigabe</h2>
        <p className="text-sm text-gray-600">
          Wählen Sie mindestens zwei einfache Anfragen aus. Entweder werden alle
          freigegeben oder keine.
        </p>
        <p className="mt-1 text-sm text-gray-600">
          Ohne ausdrückliche Freigabe sehen andere Sorgeberechtigte nur den
          wirksamen Stand. Name, Hinweise und Begründungen der einreichenden
          Person bleiben privat.
        </p>
      </div>
      <label
        htmlFor="bulk-approval-reason"
        className="block space-y-1 text-sm font-medium text-gray-800"
      >
        <span>Gemeinsame Begründung</span>
        <Textarea
          id="bulk-approval-reason"
          value={reason}
          onChange={(event) => setReason(event.target.value)}
          rows={2}
          placeholder="Warum können diese Anfragen freigegeben werden?"
        />
      </label>
      <Button
        type="button"
        variant="primary"
        size="md"
        disabled={count < 2 || reason.trim() === ""}
        onClick={open}
      >
        {count} freigeben
      </Button>
    </div>
  );
}

function OpenWithdrawalCard({
  row,
  finish,
  remove,
}: Readonly<{
  row: CareWithdrawalCompletion;
  finish: (row: CareWithdrawalCompletion) => void;
  remove: (row: CareWithdrawalCompletion) => void;
}>) {
  const name = `${row.firstName} ${row.lastName}`.trim();
  const overdue = row.urgency === "overdue";
  return (
    <RequestReviewCard
      type="care_withdrawal"
      typeLabel="Abmeldung"
      childName={name}
      summary={`Keine Betreuungstage ab ${formatDate(row.firstBookinglessDay)}`}
      badge={
        <StatusBadge
          tone={overdue ? "red" : "orange"}
          label={overdue ? "Überfällig" : "Geplant"}
        />
      }
      history={{
        kind: "readonly",
        label: overdue ? "Überfällig" : "Geplant",
        tone: overdue ? "red" : "orange",
      }}
      action={
        <div className="flex flex-wrap gap-1">
          <Button
            type="button"
            variant="ghost"
            size="compact"
            onClick={() => finish(row)}
          >
            Betreuung beenden
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="compact"
            onClick={() => remove(row)}
          >
            Kind sofort löschen
          </Button>
        </div>
      }
    >
      <p className="text-sm text-gray-600">
        Für dieses Kind ist kein Betreuungstag mehr gebucht. Beenden Sie jetzt
        die Betreuung.
      </p>
    </RequestReviewCard>
  );
}

function HistoryWithdrawalCard({
  row,
}: Readonly<{ row: CareWithdrawalCompletion }>) {
  const deleted = row.outcome === "deleted";
  const childName =
    deleted || row.studentId === ""
      ? "Gelöschtes Kind"
      : `${row.firstName} ${row.lastName}`.trim();
  return (
    <RequestReviewCard
      type="care_withdrawal"
      typeLabel="Abmeldung"
      childName={childName}
      summary={deleted ? "Kind sofort gelöscht" : "Betreuung beendet"}
      badge={
        <StatusBadge
          tone="gray"
          label={deleted ? "Gelöscht" : "Abgeschlossen"}
        />
      }
      history={{
        kind: "readonly",
        label: deleted ? "Gelöscht" : "Abgeschlossen",
        tone: "gray",
      }}
    >
      {row.resolvedAt && (
        <p className="text-sm text-gray-600">
          Erledigt am {formatDate(row.resolvedAt)}
        </p>
      )}
    </RequestReviewCard>
  );
}

function HistoryRequestList({
  items,
  withdrawals,
}: Readonly<{
  items: readonly AnyItem[];
  withdrawals: readonly CareWithdrawalCompletion[];
}>) {
  return (
    <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
      <RequestRowHeader view="history" />
      {withdrawals.map((row) => (
        <HistoryWithdrawalCard key={`care_withdrawal:${row.id}`} row={row} />
      ))}
      {items.map((item) =>
        item.request_type === "enrollment" ? (
          <EnrollmentRequestItem
            key={itemKey(item)}
            row={item.data}
            view="history"
          />
        ) : (
          <RequestHistoryItem
            key={itemKey(item)}
            item={item as AggregatedHistoryRequest}
          />
        ),
      )}
    </div>
  );
}

function RequestEmptyState({
  view,
  hasMore,
  hasActiveFilters,
}: Readonly<{
  view: "open" | "history";
  hasMore: boolean;
  hasActiveFilters: boolean;
}>) {
  const title = hasMore
    ? "Hier ist noch nichts gefunden."
    : view === "open"
      ? "Keine offenen Anfragen."
      : "Noch keine entschiedenen Anfragen.";
  const description = hasMore
    ? "Ältere Einträge sind noch nicht geladen. Mit „Weitere Einträge laden“ weitersuchen."
    : hasActiveFilters
      ? "Für die aktuelle Suche und Filter gibt es keine Treffer."
      : undefined;
  return (
    <EmptyState
      icon={<TrayIcon size={32} aria-hidden="true" />}
      title={title}
      description={description}
      variant="compact"
    />
  );
}

function CareExitDialog({
  row,
  close,
  finished,
}: Readonly<{
  row: CareWithdrawalCompletion | null;
  close: () => void;
  finished: (row: CareWithdrawalCompletion) => void;
}>) {
  if (!row) return null;
  return (
    <CareExitModal
      isOpen
      studentIds={[row.studentId]}
      completionId={row.id}
      firstBookinglessDay={row.firstBookinglessDay}
      onClose={close}
      onFinished={() => finished(row)}
    />
  );
}

function DeletionDialog({
  row,
  close,
  deleted,
}: Readonly<{
  row: CareWithdrawalCompletion | null;
  close: () => void;
  deleted: (row: CareWithdrawalCompletion) => void;
}>) {
  if (!row) return null;
  return (
    <StudentDeletionModal
      isOpen
      studentId={row.studentId}
      completionId={row.id}
      displayName={`${row.firstName} ${row.lastName}`.trim()}
      onClose={close}
      onDeleted={() => deleted(row)}
    />
  );
}

function DeletionWarningDialog({
  row,
  close,
  confirm,
}: Readonly<{
  row: CareWithdrawalCompletion | null;
  close: () => void;
  confirm: (row: CareWithdrawalCompletion) => void;
}>) {
  if (!row) return null;
  return (
    <ConfirmationModal
      isOpen
      onClose={close}
      onConfirm={() => confirm(row)}
      title="Kind sofort löschen"
      confirmText="Löschen prüfen"
      cancelText="Zurück"
      mobileSheet
    >
      <p className="text-sm text-gray-600">
        Das Kind wird sofort gelöscht. Auch ein späterer letzter Betreuungstag
        wird nicht abgewartet.
      </p>
    </ConfirmationModal>
  );
}

function BulkConfirmationDialog({
  open,
  count,
  reason,
  saving,
  close,
  confirm,
}: Readonly<{
  open: boolean;
  count: number;
  reason: string;
  saving: boolean;
  close: () => void;
  confirm: () => void;
}>) {
  if (!open) return null;
  return (
    <ConfirmationModal
      isOpen
      onClose={close}
      onConfirm={confirm}
      title="Sammelfreigabe bestätigen"
      confirmText="Alles freigeben"
      cancelText="Zurück"
      isConfirmLoading={saving}
      isDismissDisabled={saving}
      mobileSheet
    >
      <div className="space-y-2 text-sm text-gray-700">
        <p>
          Alle {count} Anfragen werden gemeinsam freigegeben. Wenn eine Anfrage
          nicht mehr passt, wird keine freigegeben.
        </p>
        <p>
          <span className="font-medium">Begründung:</span> {reason.trim()}
        </p>
      </div>
    </ConfirmationModal>
  );
}

function useRequestSources(
  view: "open" | "history",
  filters: AggregatedRequestFilters,
) {
  return useMemo<FeedSource<AnyItem>[]>(() => {
    const params: AggregatedRequestParams = {
      search: filters.search,
      studentId: filters.studentId,
      ...(view === "history"
        ? { statuses: filters.statuses, from: filters.from, to: filters.to }
        : {}),
    };
    const wantsType = (type: AggregatedRequestType) =>
      filters.types.length === 0 || filters.types.includes(type);
    const aggregatedTypes = filters.types.filter(
      (type) => type !== "enrollment" && type !== "care_withdrawal",
    );
    const sources: FeedSource<AnyItem>[] = [];
    if (
      filters.includeAggregated !== false &&
      (filters.types.length === 0 || aggregatedTypes.length > 0)
    ) {
      const aggregatedParams = { ...params, types: aggregatedTypes };
      sources.push({
        key: "aggregated",
        fetchPage: (cursor) =>
          view === "history"
            ? listAggregatedRequestHistory({ ...aggregatedParams, cursor })
            : listAggregatedOpenRequests({ ...aggregatedParams, cursor }),
      });
    }
    if (filters.includeEnrollment && wantsType("enrollment")) {
      sources.push({
        key: "enrollment",
        fetchPage: (cursor) =>
          listEnrollmentChangeRequests(view, { ...params, cursor }),
      });
    }
    return sources;
  }, [filters, view]);
}

function useFeedLifecycle(
  sources: readonly FeedSource<AnyItem>[],
  view: "open" | "history",
) {
  const feedRef = useRef(createFeedState<AnyItem>(sources));
  const generationRef = useRef(0);
  const loadingRef = useRef(false);
  const loadMoreRef = useRef(false);
  const start = useCallback(() => {
    const feed = createFeedState<AnyItem>(sources);
    const generation = ++generationRef.current;
    feedRef.current = feed;
    loadingRef.current = true;
    const page = takeInitialFeed(sources, feed, view).finally(() => {
      if (generation === generationRef.current) loadingRef.current = false;
    });
    return {
      generation,
      page,
      isCurrent: () => generation === generationRef.current,
    };
  }, [sources, view]);
  return { feedRef, generationRef, loadingRef, loadMoreRef, start };
}

function useInitialFeed(
  start: ReturnType<typeof useFeedLifecycle>["start"],
  setItems: (items: AnyItem[]) => void,
  setHasMore: (value: boolean) => void,
  setLoading: (value: boolean) => void,
  setError: (value: string | null) => void,
) {
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    const { page, isCurrent } = start();
    void page
      .then((result) => {
        if (cancelled || !isCurrent()) return;
        setItems(result.items);
        setHasMore(result.hasMore);
        setLoading(false);
      })
      .catch((err: unknown) => {
        if (cancelled || !isCurrent()) return;
        logger.warn("aggregated_request_list_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setError("Anfragen konnten nicht geladen werden.");
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [setError, setHasMore, setItems, setLoading, start]);
}

function useMergedRequestFeed(
  sources: readonly FeedSource<AnyItem>[],
  view: "open" | "history",
) {
  const [items, setItems] = useState<AnyItem[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const lifecycle = useFeedLifecycle(sources, view);
  useInitialFeed(lifecycle.start, setItems, setHasMore, setLoading, setError);
  const reload = useCallback(async () => {
    const { generation, page } = lifecycle.start();
    try {
      const result = await page;
      if (generation !== lifecycle.generationRef.current) return;
      setItems(result.items);
      setHasMore(result.hasMore);
      setLoading(false);
    } catch (err) {
      if (generation !== lifecycle.generationRef.current) return;
      logger.warn("aggregated_request_list_reload_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setLoading(false);
    }
  }, [lifecycle]);
  const loadMore = useCallback(async () => {
    if (
      !hasMore ||
      lifecycle.loadingRef.current ||
      lifecycle.loadMoreRef.current
    )
      return;
    const generation = lifecycle.generationRef.current;
    lifecycle.loadMoreRef.current = true;
    setLoadingMore(true);
    setError(null);
    try {
      const page = await takeMergedPage(
        sources,
        lifecycle.feedRef.current,
        PAGE_SIZE,
      );
      if (generation !== lifecycle.generationRef.current) return;
      setItems((current) => [...current, ...page.items]);
      setHasMore(page.hasMore);
    } catch (err) {
      if (generation === lifecycle.generationRef.current) {
        logger.warn("aggregated_request_list_load_more_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setError("Weitere Anfragen konnten nicht geladen werden.");
      }
    } finally {
      lifecycle.loadMoreRef.current = false;
      setLoadingMore(false);
    }
  }, [hasMore, lifecycle, sources]);
  return {
    items,
    setItems,
    hasMore,
    loading,
    loadingMore,
    error,
    setError,
    reload,
    loadMore,
  };
}

async function fetchWithdrawalPage(
  view: "open" | "history",
  filters: AggregatedRequestFilters,
  page: number,
) {
  if (
    !filters.includeCareWithdrawals ||
    (filters.types.length > 0 && !filters.types.includes("care_withdrawal"))
  )
    return { items: [], total: 0, page, pageSize: WITHDRAWAL_PAGE_SIZE };
  return fetchCareWithdrawals({
    search: filters.search,
    studentId: filters.studentId,
    page,
    pageSize: WITHDRAWAL_PAGE_SIZE,
    ...(view === "history" ? { state: "resolved" as const } : {}),
  });
}

function useWithdrawalFeed(
  view: "open" | "history",
  filters: AggregatedRequestFilters,
  reportError: (message: string) => void,
) {
  const [items, setItems] = useState<CareWithdrawalCompletion[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const nextPageRef = useRef(2);
  const generationRef = useRef(0);
  const load = useCallback(async () => {
    const generation = ++generationRef.current;
    try {
      const page = await fetchWithdrawalPage(view, filters, 1);
      if (generation !== generationRef.current) return;
      setItems(page.items);
      setHasMore(page.items.length < page.total);
      nextPageRef.current = 2;
    } catch (err) {
      if (generation !== generationRef.current) return;
      logger.warn("care_withdrawal_list_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      reportError("Abmeldungen konnten nicht geladen werden.");
    }
  }, [filters, reportError, view]);
  useEffect(() => {
    let cancelled = false;
    setLoading(filters.includeCareWithdrawals === true);
    void load().finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [filters.includeCareWithdrawals, load]);
  const loadMore = useCallback(async () => {
    if (!hasMore || loadingMore) return;
    setLoadingMore(true);
    try {
      const pageNumber = nextPageRef.current;
      const page = await fetchWithdrawalPage(view, filters, pageNumber);
      setItems((current) => [...current, ...page.items]);
      setHasMore(pageNumber * WITHDRAWAL_PAGE_SIZE < page.total);
      nextPageRef.current = pageNumber + 1;
    } catch {
      reportError("Weitere Abmeldungen konnten nicht geladen werden.");
    } finally {
      setLoadingMore(false);
    }
  }, [filters, hasMore, loadingMore, reportError, view]);
  return { items, setItems, loading, loadingMore, hasMore, load, loadMore };
}

export function AggregatedRequestList({
  view,
  filters,
}: Readonly<{
  view: "open" | "history";
  filters: AggregatedRequestFilters;
}>) {
  const sources = useRequestSources(view, filters);
  const feed = useMergedRequestFeed(sources, view);
  const { items, setItems, hasMore, loading, loadingMore, error, setError } =
    feed;
  const withdrawalsFeed = useWithdrawalFeed(view, filters, setError);
  const {
    items: withdrawals,
    setItems: setWithdrawals,
    loading: withdrawalsLoading,
    loadingMore: withdrawalsLoadingMore,
    hasMore: withdrawalsHaveMore,
    load: loadWithdrawals,
    loadMore: loadMoreWithdrawals,
  } = withdrawalsFeed;
  const [notice, setNotice] = useState<string | null>(null);
  const [selectedForBulk, setSelectedForBulk] = useState<Set<string>>(
    () => new Set(),
  );
  const [bulkReason, setBulkReason] = useState("");
  const [bulkConfirmOpen, setBulkConfirmOpen] = useState(false);
  const [bulkSaving, setBulkSaving] = useState(false);
  const [careExitWithdrawal, setCareExitWithdrawal] =
    useState<CareWithdrawalCompletion | null>(null);
  const [deletionWithdrawal, setDeletionWithdrawal] =
    useState<CareWithdrawalCompletion | null>(null);
  const [deletionWarningWithdrawal, setDeletionWarningWithdrawal] =
    useState<CareWithdrawalCompletion | null>(null);
  // Set while THIS list dispatches change-requests-refresh so its own listener
  // (below) doesn't refetch — it already removed the decided row optimistically.
  // dispatchEvent is synchronous, so the flag only has to cover that one call.
  const suppressSelfReloadRef = useRef(false);

  // Refetch ohne Spinner, wenn eine Entscheidung anderswo fällt: Entscheidungen
  // in diesem Fenster senden change-requests-refresh, Entscheidungen anderswo
  // kommen als SSE-abgeleitetes messages-unread-refresh bzw. beim Fokuswechsel
  // an. Nur die offene Arbeitsliste braucht das — die Historie mountet beim
  // Umschalten frisch.
  useEffect(() => {
    if (view !== "open") return;
    const handler = () => {
      if (suppressSelfReloadRef.current) return;
      void feed.reload();
      void loadWithdrawals();
    };
    const onFocus = () => {
      void feed.reload();
      void loadWithdrawals();
    };
    const onVisibility = () => {
      if (typeof document !== "undefined" && !document.hidden) {
        void feed.reload();
        void loadWithdrawals();
      }
    };
    window.addEventListener("change-requests-refresh", handler);
    window.addEventListener("messages-unread-refresh", handler);
    window.addEventListener("focus", onFocus);
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      window.removeEventListener("change-requests-refresh", handler);
      window.removeEventListener("messages-unread-refresh", handler);
      window.removeEventListener("focus", onFocus);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [view, feed, loadWithdrawals]);

  // Nach einer Entscheidung: Zeile entfernen, Hinweis zeigen und das
  // Badge/die Geschwister-Ansichten über change-requests-refresh anstoßen.
  // Der eigene Listener ist währenddessen unterdrückt — die Zeile ist schon weg.
  const handleDecided = useCallback(
    (key: string, decidedNotice: string) => {
      setItems((prev) => prev.filter((item) => itemKey(item) !== key));
      setSelectedForBulk((current) => {
        if (!current.has(key)) return current;
        const next = new Set(current);
        next.delete(key);
        return next;
      });
      suppressSelfReloadRef.current = true;
      window.dispatchEvent(new Event("change-requests-refresh"));
      suppressSelfReloadRef.current = false;
      setNotice(decidedNotice);
    },
    [setItems],
  );

  const openCases = useMemo(
    () => (view === "open" ? groupOpenCases(items, withdrawals) : []),
    [items, view, withdrawals],
  );
  const selectedBulkItems = useMemo(
    () =>
      items.filter(
        (item): item is AggregatedOpenRequest =>
          view === "open" && selectedForBulk.has(itemKey(item)),
      ),
    [items, selectedForBulk, view],
  );

  const confirmBulkApproval = useCallback(async () => {
    const refs = selectedBulkItems.flatMap((item) => {
      if (
        !hasReviewMetadata(item) ||
        (item.request_type !== "master_data" && item.request_type !== "excused")
      ) {
        return [];
      }
      return [
        {
          kind: item.request_type,
          id: item.data.id,
          expected_version: item.expected_version,
        },
      ];
    });
    setBulkSaving(true);
    setError(null);
    try {
      const count = await bulkApproveParentRequests(refs, bulkReason.trim());
      const selectedKeys = new Set(selectedForBulk);
      setItems((current) =>
        current.filter((item) => !selectedKeys.has(itemKey(item))),
      );
      setSelectedForBulk(new Set());
      setBulkReason("");
      setBulkConfirmOpen(false);
      setNotice(`${count} Anfragen wurden freigegeben.`);
      suppressSelfReloadRef.current = true;
      window.dispatchEvent(new Event("change-requests-refresh"));
      suppressSelfReloadRef.current = false;
    } catch (err: unknown) {
      logger.warn("parent_request_bulk_approval_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setBulkConfirmOpen(false);
      setError(
        err instanceof Error
          ? err.message
          : "Die Sammelfreigabe konnte nicht gespeichert werden.",
      );
      await feed.reload();
    } finally {
      setBulkSaving(false);
    }
  }, [
    bulkReason,
    feed,
    selectedBulkItems,
    selectedForBulk,
    setError,
    setItems,
  ]);

  const handleWithdrawalFinished = useCallback(
    (row: CareWithdrawalCompletion, deleted = false) => {
      setWithdrawals((current) => current.filter((item) => item.id !== row.id));
      setNotice(
        deleted ? "Das Kind wurde gelöscht." : "Die Betreuung wurde beendet.",
      );
      window.dispatchEvent(new Event("change-requests-refresh"));
    },
    [setWithdrawals],
  );

  const handleBulkSelection = useCallback((key: string, checked: boolean) => {
    setSelectedForBulk((current) => {
      const next = new Set(current);
      if (checked) next.add(key);
      else next.delete(key);
      return next;
    });
  }, []);

  const handleProtectionChanged = useCallback(
    (studentID: string, enabled: boolean) => {
      setItems((current) =>
        current.map((item) =>
          "student_id" in item && item.student_id === studentID
            ? { ...item, family_protected: enabled }
            : item,
        ),
      );
      setNotice(
        enabled
          ? "Der Familienschutz ist jetzt aktiv."
          : "Der Familienschutz ist jetzt aus.",
      );
    },
    [setItems],
  );

  if (loading || withdrawalsLoading) {
    return (
      <SkeletonRegion label="Anfragen werden geladen">
        <ListSkeleton rows={3} avatar={false} />
      </SkeletonRegion>
    );
  }

  // filters.studentId zählt bewusst NICHT als aktiver Filter: im
  // Änderungsprotokoll eines Kindes ist es der Kontext, kein Suchkriterium.
  // "Keine Treffer für Suche und Filter" wäre dort nur verwirrend.
  const hasActiveFilters =
    filters.search.trim() !== "" ||
    filters.types.length > 0 ||
    filters.statuses.length > 0 ||
    Boolean(filters.from) ||
    Boolean(filters.to);
  const visibleWithdrawals =
    view === "history"
      ? withdrawals.filter((row) => {
          const resolvedDate = row.resolvedAt?.slice(0, 10);
          return (
            resolvedDate !== undefined &&
            (!filters.from || resolvedDate >= filters.from) &&
            (!filters.to || resolvedDate <= filters.to)
          );
        })
      : withdrawals;

  return (
    <div className="space-y-3">
      {error && <Alert type="error" message={error} />}
      {notice && <Alert type="success" message={notice} />}
      {items.length === 0 && visibleWithdrawals.length === 0 && !error ? (
        <RequestEmptyState
          view={view}
          hasMore={hasMore}
          hasActiveFilters={hasActiveFilters}
        />
      ) : view === "open" ? (
        <>
          <OpenCaseGroup
            id="request-group-urgent"
            title="Heute wichtig"
            cases={openCases.filter((childCase) => childCase.urgentToday)}
            canManageFamilyProtection={Boolean(
              filters.canManageFamilyProtection,
            )}
            selected={selectedForBulk}
            onSelectionChange={handleBulkSelection}
            onDecided={handleDecided}
            onProtectionChanged={handleProtectionChanged}
            finishWithdrawal={setCareExitWithdrawal}
            removeWithdrawal={setDeletionWarningWithdrawal}
          />
          <BulkApprovalPanel
            count={selectedBulkItems.length}
            reason={bulkReason}
            setReason={setBulkReason}
            open={() => setBulkConfirmOpen(true)}
          />
          <OpenCaseGroup
            id="request-group-later"
            title="Später"
            cases={openCases.filter((childCase) => !childCase.urgentToday)}
            canManageFamilyProtection={Boolean(
              filters.canManageFamilyProtection,
            )}
            selected={selectedForBulk}
            onSelectionChange={handleBulkSelection}
            onDecided={handleDecided}
            onProtectionChanged={handleProtectionChanged}
            finishWithdrawal={setCareExitWithdrawal}
            removeWithdrawal={setDeletionWarningWithdrawal}
          />
        </>
      ) : (
        <HistoryRequestList items={items} withdrawals={visibleWithdrawals} />
      )}
      {(hasMore || withdrawalsHaveMore) && (
        <div className="flex justify-center pt-1">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() => {
              if (hasMore) void feed.loadMore();
              if (withdrawalsHaveMore) void loadMoreWithdrawals();
            }}
            disabled={loadingMore || withdrawalsLoadingMore}
          >
            {loadingMore || withdrawalsLoadingMore
              ? "Wird geladen…"
              : "Weitere Einträge laden"}
          </Button>
        </div>
      )}
      <CareExitDialog
        row={careExitWithdrawal}
        close={() => setCareExitWithdrawal(null)}
        finished={(row) => {
          setCareExitWithdrawal(null);
          handleWithdrawalFinished(row);
        }}
      />
      <DeletionDialog
        row={deletionWithdrawal}
        close={() => setDeletionWithdrawal(null)}
        deleted={(row) => {
          setDeletionWithdrawal(null);
          handleWithdrawalFinished(row, true);
        }}
      />
      <DeletionWarningDialog
        row={deletionWarningWithdrawal}
        close={() => setDeletionWarningWithdrawal(null)}
        confirm={(row) => {
          setDeletionWithdrawal(row);
          setDeletionWarningWithdrawal(null);
        }}
      />
      <BulkConfirmationDialog
        open={bulkConfirmOpen}
        count={selectedBulkItems.length}
        reason={bulkReason}
        saving={bulkSaving}
        close={() => setBulkConfirmOpen(false)}
        confirm={() => void confirmBulkApproval()}
      />
    </div>
  );
}

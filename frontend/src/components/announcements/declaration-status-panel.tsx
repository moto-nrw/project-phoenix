"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { BellRing, CheckCircle2, Download } from "lucide-react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { InfoCard } from "~/components/ui/info-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
import type { SegmentedControlItem } from "~/components/ui/segmented-control";
import { StatusBadge } from "~/components/ui/status-badge";
import type { StatusBadgeTone } from "~/components/ui/status-badge";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { formatChatDateTime } from "~/lib/date-helpers";
import { formatBytes } from "~/lib/files-api";
import { createLogger } from "~/lib/logger";
import {
  downloadDeclarationExport,
  fetchDeclarationStatus,
  remindUnanswered,
} from "~/lib/parent-announcements-api";
import type {
  DeclarationAction,
  DeclarationChild,
  DeclarationChildState,
  DeclarationStatus,
  DeclarationSubmission,
  DeclarationVersion,
} from "~/lib/parent-announcements-api";

const logger = createLogger({ component: "DeclarationStatusPanel" });

/**
 * State of one child for the CURRENT version (#3430). Declined and revoked
 * share the red tone on purpose: both mean „keine Zustimmung", the label
 * tells them apart. No-signer and expired share gray: nothing more can
 * happen for that child without the school.
 */
const DECLARATION_STATE_META: Record<
  DeclarationChildState,
  { label: string; tone: StatusBadgeTone; title?: string }
> = {
  open: { label: "Offen", tone: "orange" },
  partial: {
    label: "Teilweise beantwortet",
    tone: "blue",
    title:
      "Einige, aber noch nicht alle sorgeberechtigten Personen haben geantwortet.",
  },
  agreed: { label: "Zugestimmt", tone: "green" },
  declined: { label: "Abgelehnt", tone: "red" },
  revoked: { label: "Widerrufen", tone: "red" },
  no_signer: {
    label: "Niemand kann antworten",
    tone: "gray",
    title:
      "Keine sorgeberechtigte Person dieses Kindes hat ein Eltern-Konto. Eine Erinnerung ändert daran nichts.",
  },
  expired: { label: "Frist abgelaufen", tone: "gray" },
};

const DECLARATION_ACTION_LABEL: Record<DeclarationAction, string> = {
  agreed: "Zugestimmt",
  declined: "Abgelehnt",
  revoked: "Widerrufen",
};

const GUARDIAN_ROLE_LABEL: Record<string, string> = {
  primary_guardian: "Hauptberechtigt",
  legal_guardian: "Erziehungsberechtigt",
  co_guardian: "Mitberechtigt",
};

/**
 * Order of the child list (#3430): children with a submission first, then the
 * ones still waiting, and those nobody can submit for last. At a school with
 * a hundred children the last group would otherwise bury the relevant ones.
 */
const STATE_GROUP: Record<DeclarationChildState, number> = {
  agreed: 0,
  declined: 0,
  revoked: 0,
  partial: 0,
  open: 1,
  expired: 1,
  no_signer: 2,
};

/** Sorted copy: by group (see STATE_GROUP), then last name, first name. */
export function sortDeclarationChildren(
  children: readonly DeclarationChild[],
): DeclarationChild[] {
  return [...children].sort(
    (a, b) =>
      (STATE_GROUP[a.state] ?? 1) - (STATE_GROUP[b.state] ?? 1) ||
      a.last_name.localeCompare(b.last_name, "de") ||
      a.first_name.localeCompare(b.first_name, "de"),
  );
}

type ChildFilter = "all" | "open";

const CHILD_FILTERS: ReadonlyArray<SegmentedControlItem<ChildFilter>> = [
  { value: "all", label: "Alle Kinder" },
  { value: "open", label: "Nur offene" },
];

/** „12.09.2026, 08:00 Uhr" in the school's timezone. */
function formatDateTime(iso: string): string {
  return `${formatChatDateTime(iso)} Uhr`;
}

function fullName(first: string, last: string): string {
  return `${first} ${last}`.trim() || "Unbekannt";
}

/**
 * Status of an Einverständnis (#3430), counted per child: who agreed or
 * declined, who is still open, and the frozen versions. Checksums stay out
 * of the screen (they are in the CSV); one sentence says whether everything
 * stored is unchanged.
 * The history lists every action, also those for older versions, because
 * a school may have to show later what was agreed to and when.
 */
export function DeclarationStatusPanel({
  announcementId,
  canAct,
}: {
  readonly announcementId: string;
  /** Reminders are only offered while the Einverständnis is live. */
  readonly canAct: boolean;
}) {
  const [status, setStatus] = useState<DeclarationStatus | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  const loadError = useApiLoadError();
  const showLoadError = loadError.show;
  const clearLoadError = loadError.clear;
  const { show: showError } = useApiErrorDisplay();
  const { success: toastSuccess } = useToast();
  const [busy, setBusy] = useState<"remind" | "pdf" | "csv" | null>(null);
  const [childFilter, setChildFilter] = useState<ChildFilter>("all");
  // „Wiederholen“ ruft Laden bzw. die Aktion mit dem dann aktuellen Stand auf.
  const latestLoadRef = useRef<() => void>(() => undefined);
  const latestRemindRef = useRef<() => void>(() => undefined);
  const latestDownloadRef = useRef<(format: "pdf" | "csv") => void>(
    () => undefined,
  );

  const load = useCallback(async () => {
    try {
      setStatus(await fetchDeclarationStatus(announcementId));
      setLoadFailed(false);
      clearLoadError();
    } catch (error) {
      logger.error("declaration_status_load_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      setLoadFailed(true);
      void showLoadError(error, {
        object: "die Übersicht der Antworten",
        retry: () => latestLoadRef.current(),
      });
    }
  }, [announcementId, showLoadError, clearLoadError]);
  useLayoutEffect(() => {
    latestLoadRef.current = () => void load();
  });

  useEffect(() => {
    void load();
  }, [load]);

  const children = useMemo(() => {
    const all = sortDeclarationChildren(status?.children ?? []);
    // "Nur offene" means what the reminder reaches: open or partly done.
    return childFilter === "open"
      ? all.filter((c) => c.state === "open" || c.state === "partial")
      : all;
  }, [status, childFilter]);

  const remind = async () => {
    setBusy("remind");
    try {
      const count = await remindUnanswered(announcementId);
      toastSuccess(
        count === 0
          ? "Für alle Kinder liegt eine Antwort vor. Es wurde niemand erinnert."
          : `${count} ${count === 1 ? "Person wurde" : "Personen wurden"} erinnert.`,
      );
      await load();
    } catch (error) {
      logger.error("declaration_remind_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      void showError(error, {
        object: "das Senden der Erinnerung",
        retry: () => latestRemindRef.current(),
      });
    } finally {
      setBusy(null);
    }
  };

  const download = async (format: "pdf" | "csv") => {
    setBusy(format);
    try {
      await downloadDeclarationExport(announcementId, format);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      logger.error("declaration_export_failed", { format, error: message });
      void showError(error, {
        object: "das Erstellen der Datei",
        retry: () => latestDownloadRef.current(format),
      });
    } finally {
      setBusy(null);
    }
  };
  useLayoutEffect(() => {
    latestRemindRef.current = () => void remind();
    latestDownloadRef.current = (format) => void download(format);
  });

  if (loadFailed && !status) {
    // Bis der Katalogtext da ist, bleibt die Ladezeile stehen.
    return loadError.error ? (
      <LoadErrorAlert error={loadError.error} />
    ) : (
      <div className="text-sm text-gray-500">Stand wird geladen …</div>
    );
  }
  if (!status) {
    return <div className="text-sm text-gray-500">Stand wird geladen …</div>;
  }

  const s = status.summary;
  const done = s.agreed + s.declined + s.revoked;
  const waiting = s.open + s.partial;
  const answerable = s.children_total - s.no_signer;

  return (
    <div className="space-y-4">
      <InfoCard
        title="Antworten"
        icon={<CheckCircle2 className="h-5 w-5" aria-hidden />}
      >
        <div className="flex flex-wrap items-baseline gap-x-2">
          <span className="text-3xl font-semibold text-gray-900 tabular-nums">
            {done}
          </span>
          <span className="text-lg text-gray-500 tabular-nums">
            / {answerable}
          </span>
          <span className="text-sm text-gray-600">Kinder mit Antwort</span>
        </div>
        <p className="mt-1 text-sm text-gray-600">
          {waiting > 0
            ? `Für ${waiting} ${waiting === 1 ? "Kind" : "Kinder"} fehlt noch eine Antwort.`
            : s.expired > 0
              ? `Für ${s.expired} ${s.expired === 1 ? "Kind" : "Kinder"} ist die Frist abgelaufen.`
              : "Für alle Kinder, bei denen das möglich ist, liegt eine Antwort vor."}
        </p>
        {s.no_signer > 0 && (
          <p className="mt-1 text-sm text-gray-600">
            Für {s.no_signer} {s.no_signer === 1 ? "Kind" : "Kinder"} kann
            niemand antworten. Keine sorgeberechtigte Person hat ein
            Eltern-Konto.
          </p>
        )}

        <div className="mt-4">
          <DataGrid columns={4}>
            <DataField label="Zugestimmt">
              <span className="tabular-nums">{s.agreed}</span>
            </DataField>
            <DataField label="Abgelehnt">
              <span className="tabular-nums">{s.declined}</span>
            </DataField>
            {(status.revocable || s.revoked > 0) && (
              <DataField label="Widerrufen">
                <span className="tabular-nums">{s.revoked}</span>
              </DataField>
            )}
            <DataField label="Offen">
              <span className="tabular-nums">{s.open}</span>
            </DataField>
            {status.signers === "all" && (
              <DataField label="Teilweise beantwortet">
                <span className="tabular-nums">{s.partial}</span>
              </DataField>
            )}
            {s.expired > 0 && (
              <DataField label="Frist abgelaufen">
                <span className="tabular-nums">{s.expired}</span>
              </DataField>
            )}
          </DataGrid>
        </div>
      </InfoCard>

      <div className="flex flex-wrap gap-2">
        {canAct && (
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() => void remind()}
            disabled={busy !== null || waiting === 0}
            isLoading={busy === "remind"}
            loadingText="Wird gesendet …"
            className="gap-1.5"
          >
            <BellRing className="h-4 w-4" aria-hidden />
            Offene erinnern
          </Button>
        )}
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={() => void download("pdf")}
          disabled={busy !== null}
          isLoading={busy === "pdf"}
          loadingText="Wird erstellt …"
          className="gap-1.5"
        >
          <Download className="h-4 w-4" aria-hidden />
          Bericht als PDF
        </Button>
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={() => void download("csv")}
          disabled={busy !== null}
          isLoading={busy === "csv"}
          loadingText="Wird erstellt …"
          className="gap-1.5"
        >
          <Download className="h-4 w-4" aria-hidden />
          Verlauf als CSV
        </Button>
      </div>

      {/* Ein Neuladen nach einer Aktion ist gescheitert: der alte Stand
          bleibt sichtbar, der Fehler steht darüber. */}
      <LoadErrorAlert error={loadFailed ? loadError.error : null} />

      <section>
        <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
          <h3 className="text-sm font-semibold text-gray-900">Kinder</h3>
          <SegmentedControl
            items={CHILD_FILTERS}
            value={childFilter}
            onChange={setChildFilter}
            ariaLabel="Kinder filtern"
          />
        </div>
        {children.length === 0 ? (
          <p className="text-sm text-gray-500">
            {childFilter === "open"
              ? "Für alle Kinder, bei denen das möglich ist, liegt eine Antwort vor."
              : "Dieses Einverständnis erreicht derzeit kein Kind."}
          </p>
        ) : (
          <ul className="divide-y divide-gray-100 rounded-lg border border-gray-200 bg-white">
            {children.map((child) => (
              <ChildRow key={child.student_id} child={child} />
            ))}
          </ul>
        )}
      </section>

      <VersionSection
        current={status.current_version}
        versions={status.versions}
        integrityOk={status.integrity_ok}
      />

      <HistorySection submissions={status.submissions} />
    </div>
  );
}

function ChildRow({ child }: { readonly child: DeclarationChild }) {
  const meta = DECLARATION_STATE_META[child.state] ?? {
    label: child.state,
    tone: "gray" as StatusBadgeTone,
  };
  return (
    <li className="flex flex-wrap items-start justify-between gap-2 px-4 py-3">
      <div className="min-w-0">
        <p className="truncate text-sm text-gray-900">
          {fullName(child.first_name, child.last_name)}
          {child.school_class && (
            <span className="ml-2 text-xs text-gray-500">
              {child.school_class}
            </span>
          )}
        </p>
        {child.signers.length > 0 && (
          <ul className="mt-0.5 space-y-0.5">
            {child.signers.map((signer) => (
              <li key={signer.account_id} className="text-xs text-gray-500">
                {fullName(signer.first_name, signer.last_name)}:{" "}
                {signer.action
                  ? `${DECLARATION_ACTION_LABEL[signer.action]}${
                      signer.submitted_at
                        ? ` am ${formatDateTime(signer.submitted_at)}`
                        : ""
                    }`
                  : "noch keine Antwort"}
              </li>
            ))}
          </ul>
        )}
      </div>
      <StatusBadge label={meta.label} tone={meta.tone} title={meta.title} />
    </li>
  );
}

function VersionSection({
  current,
  versions,
  integrityOk,
}: {
  readonly current: DeclarationVersion | null;
  readonly versions: DeclarationVersion[];
  readonly integrityOk: boolean;
}) {
  const older = versions.filter((v) => v.id !== current?.id);
  return (
    <section>
      <h3 className="mb-1 text-sm font-semibold text-gray-900">Fassung</h3>
      <p className="mb-2 text-xs text-gray-500">
        Beim Veröffentlichen hält moto Text und Dateien fest.
      </p>
      <div className="mb-2">
        {integrityOk ? (
          <p className="text-sm text-gray-700">
            Text und Antworten sind seit der Veröffentlichung unverändert.
          </p>
        ) : (
          <Alert
            type="warning"
            message="Achtung: Mindestens ein gespeicherter Eintrag wurde nachträglich verändert. Bitte wenden Sie sich an den moto-Support."
          />
        )}
      </div>
      {current ? (
        <div className="rounded-lg border border-gray-200 bg-white p-4">
          <DataGrid>
            <DataField label="Aktuelle Fassung">
              Fassung {current.version_no}
              {current.published_at &&
                `, veröffentlicht am ${formatDateTime(current.published_at)}`}
            </DataField>
            {current.attachments.length > 0 && (
              <DataField label="Dateien" fullWidth>
                <ul className="space-y-1">
                  {current.attachments.map((file) => (
                    <li
                      key={`${file.filename}-${file.sha256}`}
                      className="text-sm text-gray-900"
                    >
                      {file.filename}{" "}
                      <span className="text-xs text-gray-500">
                        ({formatBytes(file.size_bytes)})
                      </span>
                    </li>
                  ))}
                </ul>
              </DataField>
            )}
          </DataGrid>
        </div>
      ) : (
        <p className="text-sm text-gray-500">
          Noch keine Fassung. Sie entsteht beim Veröffentlichen.
        </p>
      )}
      {older.length > 0 && (
        <p className="mt-2 text-xs text-gray-500">
          Frühere{" "}
          {older.length === 1
            ? `Fassung: ${older[0]!.version_no}`
            : `Fassungen: ${older.map((v) => v.version_no).join(", ")}`}
          . Antworten dazu stehen weiter im Verlauf, zählen aber nicht mehr für
          den aktuellen Stand.
        </p>
      )}
    </section>
  );
}

function HistorySection({
  submissions,
}: {
  readonly submissions: DeclarationSubmission[];
}) {
  return (
    <section>
      <h3 className="mb-1 text-sm font-semibold text-gray-900">Verlauf</h3>
      <p className="mb-2 text-xs text-gray-500">
        Jede Antwort, auch zu früheren Fassungen. Einträge lassen sich nicht
        ändern oder löschen.
      </p>
      {submissions.length === 0 ? (
        <p className="text-sm text-gray-500">Noch keine Antworten.</p>
      ) : (
        <ul className="max-h-96 divide-y divide-gray-100 overflow-y-auto rounded-lg border border-gray-200 bg-white">
          {submissions.map((entry) => (
            <li key={entry.id} className="px-4 py-3 text-sm">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="min-w-0 text-gray-900">
                  <span className="font-medium">{entry.signer_name}</span>
                  {" für "}
                  {fullName(entry.student_first_name, entry.student_last_name)}
                </span>
                <StatusBadge
                  label={DECLARATION_ACTION_LABEL[entry.action] ?? entry.action}
                  tone="gray"
                />
              </div>
              <p className="mt-0.5 text-xs text-gray-500">
                {formatDateTime(entry.submitted_at)} · Fassung{" "}
                {entry.version_no}
                {GUARDIAN_ROLE_LABEL[entry.guardian_role]
                  ? ` · ${GUARDIAN_ROLE_LABEL[entry.guardian_role]}`
                  : ""}
                {entry.password_confirmed ? " · mit Passwort" : ""}
              </p>
              {!entry.integrity_ok && (
                <p className="text-moto-red-strong mt-1 text-xs font-semibold">
                  Nachträglich verändert
                </p>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

"use client";

// Verlauf des Träger-Büros (#5474): wer wann eine Person eingeladen, eine
// Einladung zurückgenommen, Rollen geändert oder eine Person entfernt hat.
// Nur zum Lesen, neueste zuerst, seitenweise.

import { useCallback, useEffect, useRef, useState } from "react";
import { History } from "lucide-react";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { useApiErrorDisplay, useApiLoadError } from "~/contexts/ToastContext";
import { formatBerlinDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  fetchTraegerAccountHistory,
  type TraegerAccountChangeAction,
  type TraegerAccountHistoryEntry,
} from "~/lib/traeger-api";
import { TRAEGER_ROLE_LABELS, type TraegerRole } from "~/lib/traeger-roles";

const logger = createLogger({ component: "TraegerAccountHistory" });

const ACTION_LABELS: Readonly<Record<TraegerAccountChangeAction, string>> = {
  invitation_created: "Eingeladen",
  invitation_revoked: "Einladung zurückgenommen",
  invitation_accepted: "Einladung angenommen",
  roles_changed: "Rollen geändert",
  member_removed: "Entfernt",
};

const BERLIN_TIME = new Intl.DateTimeFormat("de-DE", {
  timeZone: "Europe/Berlin",
  hour: "2-digit",
  minute: "2-digit",
});

function roleNames(roles: readonly TraegerRole[]): string {
  return roles.map((role) => TRAEGER_ROLE_LABELS[role]).join(", ") || "keine";
}

/** The roles a change names, in one line. */
function historyDetail(entry: TraegerAccountHistoryEntry): string {
  switch (entry.action) {
    case "roles_changed":
      return `Vorher: ${roleNames(entry.rolesBefore)}. Jetzt: ${roleNames(entry.rolesAfter)}.`;
    case "member_removed":
      return `Bisher: ${roleNames(entry.rolesBefore)}.`;
    case "invitation_revoked":
      return `Rolle der Einladung: ${roleNames(entry.rolesBefore)}.`;
    default:
      return `Rolle: ${roleNames(entry.rolesAfter)}.`;
  }
}

const COLUMNS: DataTableColumn<TraegerAccountHistoryEntry>[] = [
  {
    key: "occurredAt",
    header: "Zeitpunkt",
    render: (entry) => (
      <span className="whitespace-nowrap text-gray-900">
        {formatBerlinDate(entry.occurredAt)},{" "}
        {BERLIN_TIME.format(new Date(entry.occurredAt))} Uhr
      </span>
    ),
    stacked: "meta",
  },
  {
    key: "action",
    header: "Was",
    render: (entry) => (
      <span>
        <span className="font-medium text-gray-900">
          {ACTION_LABELS[entry.action]}
        </span>
        <span className="block text-gray-600">{historyDetail(entry)}</span>
      </span>
    ),
    stacked: "title",
  },
  {
    key: "target",
    header: "Betrifft",
    render: (entry) => (
      <span className="wrap-anywhere">{entry.targetEmail || "–"}</span>
    ),
    stacked: "field",
  },
  {
    key: "actor",
    header: "Von",
    render: (entry) => (
      <span className="wrap-anywhere">
        {entry.actorEmail || "Gelöschtes Konto"}
      </span>
    ),
    stacked: "field",
  },
];

/**
 * The history section of the page „Personen“. `version` changes after every
 * change on the page; the history then reads its first page again.
 */
export function TraegerAccountHistory({
  version,
}: Readonly<{ version: number }>) {
  const [entries, setEntries] = useState<readonly TraegerAccountHistoryEntry[]>(
    [],
  );
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const historyLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = historyLoad;
  const { show: showActionError } = useApiErrorDisplay();
  // A reload drops the answer of an older read, also of „Ältere laden“.
  const generation = useRef(0);

  const load = useCallback(async () => {
    const current = ++generation.current;
    setLoading(true);
    // A pending „Ältere laden“ of the older read no longer counts.
    setLoadingMore(false);
    try {
      const page = await fetchTraegerAccountHistory();
      if (current !== generation.current) return;
      setEntries(page.entries);
      setHasMore(page.hasMore);
      clearLoadError();
    } catch (err) {
      if (current !== generation.current) return;
      logger.error("traeger_account_history_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "den Verlauf",
        retry: () => void load(),
      });
    } finally {
      if (current === generation.current) setLoading(false);
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load, version]);

  const loadMore = async () => {
    const last = entries.at(-1);
    if (!last) return;
    const current = generation.current;
    setLoadingMore(true);
    try {
      const page = await fetchTraegerAccountHistory(last.id);
      if (current !== generation.current) return;
      setEntries((shown) => [...shown, ...page.entries]);
      setHasMore(page.hasMore);
    } catch (err) {
      if (current !== generation.current) return;
      logger.error("traeger_account_history_load_more_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showActionError(err, { object: "die älteren Einträge" });
    } finally {
      if (current === generation.current) setLoadingMore(false);
    }
  };

  return (
    <SectionCard
      title="Verlauf"
      description="Wer wann Personen eingeladen, Rollen geändert oder entfernt hat. Nur zum Lesen."
    >
      {historyLoad.error ? (
        <LoadErrorAlert error={historyLoad.error} actionLayout="responsive" />
      ) : (
        <div className="flex flex-col gap-4">
          <DataTable
            columns={COLUMNS}
            rows={[...entries]}
            getRowKey={(entry) => entry.id}
            isLoading={loading}
            loadingRowCount={3}
            stackedOnMobile
            emptyState={
              <EmptyState
                icon={<History className="h-12 w-12" aria-hidden="true" />}
                title="Noch keine Änderungen im Träger-Büro."
              />
            }
          />
          {hasMore && !loading ? (
            <div className="flex justify-center">
              <Button
                type="button"
                variant="outline"
                size="md"
                isLoading={loadingMore}
                loadingText="Lädt …"
                onClick={() => void loadMore()}
              >
                Ältere Einträge laden
              </Button>
            </div>
          ) : null}
        </div>
      )}
    </SectionCard>
  );
}

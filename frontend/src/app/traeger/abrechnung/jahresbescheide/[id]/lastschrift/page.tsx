"use client";

// Die Lastschrift der Raten eines Jahreslaufs (#4697, ADR 0181): je
// Fälligkeitsmonat eine PAIN.008-Datei für die Bank. Eine eigene Seite wie
// „Schreiben“ beim Monatslauf; der Jahreslauf verlinkt sie im Kopf.

import { useParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { NoticeSepaDebits } from "~/components/traeger/notice-sepa-debits";
import { EmptyState } from "~/components/ui/empty-state";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type CareNoticeRunDetail,
  fetchCareNoticeRun,
} from "~/lib/traeger-care-notices-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerNoticeDebitsPage" });

export default function TraegerNoticeDebitsPage() {
  const { id: runId } = useParams<{ id: string }>();
  const [run, setRun] = useState<CareNoticeRunDetail | null>(null);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;

  const load = useCallback(async () => {
    try {
      setRun(await fetchCareNoticeRun(runId));
      clearLoadError();
    } catch (err) {
      logger.error("traeger_notice_debits_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "der Jahreslauf",
        retry: () => void load(),
      });
    }
  }, [runId, clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const issued = run?.status === "released" && run.previewCount > 0;

  return (
    <TenantPage
      title={
        run ? `Lastschrift der Raten ${run.year}` : "Lastschrift der Raten"
      }
      back
      backHref={traegerPath(`/traeger/abrechnung/jahresbescheide/${runId}`)}
      backLabel="Zurück zum Jahreslauf"
      stats={
        run ? (
          <span className="text-sm text-gray-600 tabular-nums">
            {formatDate(run.periodFrom)} bis {formatDate(run.periodUntil)}
          </span>
        ) : null
      }
      statsLoading={!run && !pageLoad.error}
      error={pageLoad.error}
      loading={!pageLoad.error && !run}
    >
      {run ? (
        issued ? (
          <NoticeSepaDebits
            periodFrom={run.periodFrom}
            periodUntil={run.periodUntil}
          />
        ) : (
          <EmptyState
            title="Noch keine Lastschrift"
            description={
              run.status === "released"
                ? "Dieser Jahreslauf hat keinen Bescheid. Es gibt nichts abzubuchen."
                : "Die Datei für die Bank gibt es, sobald Sie die Bescheide auf der Seite des Jahreslaufs ausgestellt haben."
            }
          />
        )
      ) : null}
    </TenantPage>
  );
}

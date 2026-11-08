"use client";

// Seite 1 eines Jahresbescheids (#6219) mit dem Briefkopf des Trägers: vor
// dem Ausstellen der Entwurf, danach der ausgestellte Bescheid. Drucken
// nutzt den Druck des Browsers. Jedes Öffnen steht im Protokoll des Laufs.

import { Printer } from "lucide-react";
import { useParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { CareNoticeDocument } from "~/components/traeger/care-notice-document";
import { NOTICE_KIND_LABELS } from "~/components/traeger/care-notice-format";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type CareNoticeLetter,
  fetchCareNoticeLetter,
} from "~/lib/traeger-care-notices-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerCareNoticePage" });

export default function TraegerCareNoticePage() {
  const params = useParams<{ id: string; contractId: string }>();
  const { id: runId, contractId } = params;
  const [letter, setLetter] = useState<CareNoticeLetter | null>(null);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;

  const load = useCallback(async () => {
    try {
      setLetter(await fetchCareNoticeLetter(runId, contractId));
      clearLoadError();
    } catch (err) {
      logger.error("traeger_care_notice_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "der Bescheid",
        retry: () => void load(),
      });
    }
  }, [runId, contractId, clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const notice = letter?.notice;
  const draft = notice?.number === "";

  return (
    <TenantPage
      title={
        notice
          ? `${NOTICE_KIND_LABELS[notice.kind]} ${notice.childFirstName} ${notice.childLastName}`
          : "Bescheid"
      }
      back
      backHref={traegerPath(`/traeger/abrechnung/jahresbescheide/${runId}`)}
      backLabel="Zurück zum Jahreslauf"
      stats={
        notice ? (
          <span className="text-sm text-gray-600">
            {draft
              ? "Entwurf: Nummer und Datum kommen beim Ausstellen."
              : `Nr. ${notice.number} vom ${notice.issuedOn ? formatDate(notice.issuedOn) : "–"}`}
          </span>
        ) : null
      }
      statsLoading={!letter && !pageLoad.error}
      actions={
        letter && !draft ? (
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() => window.print()}
          >
            <Printer className="mr-2 h-4 w-4" aria-hidden="true" />
            Drucken
          </Button>
        ) : undefined
      }
      error={pageLoad.error}
      loading={!pageLoad.error && !letter}
    >
      {letter ? (
        <>
          {notice?.amendedBy ? (
            <Alert
              type="info"
              message={`Dieser Bescheid gilt nicht mehr für alle Monate. Der Änderungsbescheid Nr. ${notice.amendedBy.number} ersetzt ihn.`}
            />
          ) : null}
          <CareNoticeDocument letter={letter} />
        </>
      ) : null}
    </TenantPage>
  );
}

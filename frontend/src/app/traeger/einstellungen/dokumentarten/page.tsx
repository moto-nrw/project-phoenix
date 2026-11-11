"use client";

// Dokumentart je Leistung (#4688): Die Abrechnung wählt, ob ein Lauf eine
// Rechnung, einen Gebührenbescheid oder (Ferien) eine Bestätigung mit
// Rechnung ausstellt. Der Gebührenbescheid druckt die
// Rechtsbehelfsbelehrung der Vorlage „Bescheid“; ohne sie wird er nicht
// freigegeben. Wählen darf, wer das Recht „Abrechnung“ hat.

import { useCallback, useEffect, useRef, useState } from "react";
import { useSession } from "next-auth/react";
import { DocumentKindCard } from "~/components/traeger/document-kind-card";
import { Alert } from "~/components/ui/alert";
import { ButtonLink } from "~/components/ui/button";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import { contributionsConcept } from "~/lib/concepts/contributions";
import { documentTemplatesConcept } from "~/lib/concepts/documentTemplates";
import { holidayWeeksConcept } from "~/lib/concepts/holidayWeeks";
import { createLogger } from "~/lib/logger";
import {
  type DocumentKindOverview,
  fetchDocumentKinds,
  mergeSavedService,
} from "~/lib/traeger-document-kinds-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerDocumentKindsPage" });

const BILLING_RIGHT = "traeger_billing:manage";
const NOTICE_TEMPLATE_PATH =
  "/traeger/einstellungen/briefkopf/vorlagen/bescheid";

const INFO =
  "Hier legen Sie fest, welches Schreiben die Familien je Leistung bekommen. Die Wahl gilt ab der nächsten Freigabe. Schon ausgestellte Schreiben bleiben, wie sie sind.";

function useDocumentKinds(enabled: boolean) {
  const [overview, setOverview] = useState<DocumentKindOverview | null>(null);
  const loadGeneration = useRef(0);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;

  const load = useCallback(async () => {
    const generation = ++loadGeneration.current;
    try {
      const next = await fetchDocumentKinds();
      if (generation !== loadGeneration.current) return;
      setOverview(next);
      clearLoadError();
    } catch (err) {
      if (generation !== loadGeneration.current) return;
      logger.error("traeger_document_kinds_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Dokumentarten",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    if (enabled) {
      void load();
      return;
    }
    loadGeneration.current++;
    setOverview(null);
  }, [enabled, load]);

  return { overview, setOverview, error: pageLoad.error };
}

export default function TraegerDocumentKindsPage() {
  const { data: session } = useSession();
  const canBill = hasPermission(session ?? null, BILLING_RIGHT);
  const canEditTemplates = hasPermission(
    session ?? null,
    "traeger_settings:manage",
  );
  const remedyInstruction = canEditTemplates
    ? "Tragen Sie sie zuerst in der Vorlage ein."
    : "Bitten Sie eine Person mit dem Recht „Einstellungen“, sie einzutragen.";
  const { overview, setOverview, error } = useDocumentKinds(canBill);
  const notice = overview?.services.some(
    (service) => service.documentKind === "fee_notice",
  );

  return (
    <TenantPage
      title="Dokumentarten"
      info={INFO}
      error={canBill ? error : null}
      loading={canBill && !overview && !error}
    >
      {canBill ? null : (
        <Alert
          type="info"
          message="Die Dokumentarten wählt die Rolle „Abrechnung“."
        />
      )}
      {overview ? (
        <>
          {overview.services.map((service) => (
            <DocumentKindCard
              key={service.service}
              service={service}
              concept={
                service.service === "holiday"
                  ? holidayWeeksConcept
                  : contributionsConcept
              }
              onSaved={(next) =>
                setOverview((current) =>
                  mergeSavedService(current, next, service.service),
                )
              }
            />
          ))}
          <SectionCard
            title="Rechtsbehelfsbelehrung"
            description="Steht auf jedem Gebührenbescheid. Den Text ändert, wer das Recht „Einstellungen“ hat."
            concept={documentTemplatesConcept}
            action={
              <ButtonLink
                href={traegerPath(NOTICE_TEMPLATE_PATH)}
                variant={
                  overview.legalRemedySet || !canEditTemplates
                    ? "outline"
                    : "primary"
                }
                size="md"
                tooltip="Öffnet die Vorlage der Rechtsbehelfsbelehrung für die Bescheide."
              >
                {overview.legalRemedySet || !canEditTemplates
                  ? "Vorlage ansehen"
                  : "Belehrung eintragen"}
              </ButtonLink>
            }
          >
            {overview.legalRemedySet ? (
              <Alert
                type="success"
                message={`Ihre Vorlage „Bescheid“ hat eine Rechtsbehelfsbelehrung (Version ${overview.feeNoticeTemplateVersion}).`}
              />
            ) : (
              <Alert
                type={notice ? "warning" : "info"}
                title="Noch keine Rechtsbehelfsbelehrung"
                message={
                  notice
                    ? `Ohne Belehrung können Sie keinen Gebührenbescheid freigeben. ${remedyInstruction}`
                    : "Sie brauchen sie erst, wenn eine Leistung Gebührenbescheide ausstellt."
                }
              />
            )}
          </SectionCard>
        </>
      ) : null}
    </TenantPage>
  );
}

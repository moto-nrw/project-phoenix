"use client";

// Konten und Kostenstellen (#4821): auf welches Erlöskonto moto die Beiträge
// jeder Leistung und jedes Angebots bucht, welche Kostenstelle (KOST1) eine
// Schule und die Ferienbetreuung haben und welche zweite Kostenstelle (KOST2)
// ein Angebot hat, jeweils ab einem Tag. Beim Ausstellen hält jede
// Rechnungsposition ihre Werte fest; DATEV-Datei und Sollstellung buchen
// damit. Nur das Recht „Finanz-Export“ sieht die Seite.

import { useCallback, useEffect, useState } from "react";
import {
  CostCentersCard,
  RevenueAccountsCard,
} from "~/components/traeger/booking-accounts-cards";
import {
  costCenterOn,
  costCenterTimeline,
} from "~/components/traeger/booking-accounts-format";
import { DATEV_SETTINGS_PATH } from "~/components/traeger/finance-export-datev";
import { Alert } from "~/components/ui/alert";
import { ButtonLink } from "~/components/ui/button";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { useApiLoadError, useToast } from "~/contexts/ToastContext";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { createLogger } from "~/lib/logger";
import {
  addCostCenter,
  type BookingAccounts,
  deleteCostCenter,
  fetchBookingAccounts,
  saveRevenueAccount,
} from "~/lib/traeger-finance-exports-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerBookingAccountsPage" });

const EXPORTS_PATH = "/traeger/abrechnung/exporte";

const INFO =
  "Ihr Steuerbüro bucht die Beiträge auf Erlöskonten und Kostenstellen. Legen Sie hier fest, welche Leistung wohin gehört. Eine Rechnung hält die Werte beim Ausstellen fest. Eine Änderung gilt nur für neue Rechnungen.";

export default function TraegerBookingAccountsPage() {
  const today = useBerlinToday();
  const [page, setPage] = useState<BookingAccounts | null>(null);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const { success } = useToast();

  const load = useCallback(async () => {
    try {
      setPage(await fetchBookingAccounts());
      clearLoadError();
    } catch (err) {
      logger.error("traeger_booking_accounts_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Konten und Kostenstellen",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const schoolsWithCenter = page
    ? page.schools.filter(
        (school) =>
          costCenterOn(
            costCenterTimeline(page.costCenters, "school", school.id),
            today,
          )?.code,
      ).length
    : 0;

  return (
    <TenantPage
      title="Konten und Kostenstellen"
      info={INFO}
      back
      backHref={traegerPath(EXPORTS_PATH)}
      backLabel="Zurück zu den Exporten"
      stats={
        page ? (
          <TenantPageStats
            items={[
              {
                value: page.standardAccount || "fehlt",
                label: "Standard-Erlöskonto",
              },
              {
                value: `${schoolsWithCenter} von ${page.schools.length}`,
                label:
                  page.schools.length === 1
                    ? "Schule mit Kostenstelle"
                    : "Schulen mit Kostenstelle",
              },
            ]}
          />
        ) : null
      }
      statsLoading={!page && !pageLoad.error}
      error={pageLoad.error}
      loading={!page && !pageLoad.error}
    >
      {page ? (
        <>
          {page.standardAccount ? null : (
            <Alert
              type="warning"
              title="Das Standard-Erlöskonto fehlt"
              message="Ohne Standard-Erlöskonto gibt es keine DATEV-Datei. Tragen Sie es in den DATEV-Werten ein."
              action={
                <ButtonLink
                  href={traegerPath(DATEV_SETTINGS_PATH)}
                  variant="ghost_tone"
                  size="compact"
                  tooltip="Öffnet die DATEV-Werte in den Einstellungen."
                >
                  DATEV-Werte eintragen
                </ButtonLink>
              }
            />
          )}
          <RevenueAccountsCard
            page={page}
            onSave={async (scope, scopeId, account) => {
              setPage(await saveRevenueAccount(scope, scopeId, account));
              success(
                account
                  ? "Das Erlöskonto ist gespeichert."
                  : "Das eigene Erlöskonto ist entfernt.",
              );
            }}
          />
          <CostCentersCard
            kind="kost1"
            page={page}
            today={today}
            onAdd={async (scope, scopeId, validFrom, code) => {
              setPage(await addCostCenter({ scope, scopeId, validFrom, code }));
              success("Die Kostenstelle ist gespeichert.");
            }}
            onDeleted={async (id) => setPage(await deleteCostCenter(id))}
          />
          <CostCentersCard
            kind="kost2"
            page={page}
            today={today}
            onAdd={async (scope, scopeId, validFrom, code) => {
              setPage(await addCostCenter({ scope, scopeId, validFrom, code }));
              success("Die Kostenstelle ist gespeichert.");
            }}
            onDeleted={async (id) => setPage(await deleteCostCenter(id))}
          />
        </>
      ) : null}
    </TenantPage>
  );
}

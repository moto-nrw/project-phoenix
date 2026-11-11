"use client";

// Abrechnungsvorgaben des Trägers (#5685): was für jede Abrechnung gilt.
// Bankverbindungen (#4653) sieht jede Rolle mit „Übersicht lesen“, ändern darf
// sie, wer „Einstellungen“ hat. Rechnungsnummern der Ferien (#4668) und der
// Monatsbeiträge (#4684), Ermäßigungen (#4664), Kostenträger (#5627) und
// Zahlung und Rechnungen je Leistung (#4689) und die Lastschrift mit
// Gläubiger-ID (#4695) gehören zum Recht „Abrechnung“.

import { useCallback, useEffect, useRef, useState } from "react";
import { useSession } from "next-auth/react";
import { BillingTermsCard } from "~/components/traeger/billing-terms-card";
import { ContributionRulesCard } from "~/components/traeger/contribution-rules-card";
import { CostBearersCard } from "~/components/traeger/cost-bearers-card";
import { metadataLoadError } from "~/components/traeger/metadata-load-error";
import { NumberRangeCard } from "~/components/traeger/number-range-card";
import { SepaCreditorCard } from "~/components/traeger/sepa-creditor-card";
import { TraegerBankAccountsCard } from "~/components/traeger/traeger-bank-accounts-card";
import { useTraegerMasterData } from "~/components/traeger/use-traeger-master-data";
import { useTraegerSettingsSchema } from "~/components/traeger/use-traeger-resources";
import { Alert } from "~/components/ui/alert";
import { TenantPage } from "~/components/ui/tenant-page";
import { useRefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { useApiLoadError } from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import { createLogger } from "~/lib/logger";
import {
  fetchCareNumberRange,
  fetchHolidayNumberRange,
  type NumberRange,
} from "~/lib/traeger-billing-api";
import {
  type BillingTerms,
  fetchBillingTerms,
} from "~/lib/traeger-billing-terms-api";
import {
  type ContributionRule,
  type CostBearer,
  fetchContributionRules,
  fetchCostBearers,
} from "~/lib/traeger-contributions-api";
import { type SepaCreditor, fetchSepaCreditor } from "~/lib/traeger-sepa-api";

const logger = createLogger({ component: "TraegerAbrechnungsvorgabenPage" });

const BILLING_RIGHT = "traeger_billing:manage";

/** The Träger's switch of the SEPA mandate in the parents portal (#4696). */
const ONLINE_MANDATE_KEY = "parent_billing.online_mandate";
const SETTINGS_RIGHT = "traeger_settings:manage";

const INFO = "Hier stehen die Vorgaben für Ihre Abrechnung.";
const CONTRIBUTION_INFO =
  "So entsteht der Ferienbeitrag: Jede gebuchte Woche kostet den Preis ihres Zeitmodells. Den Preis legen Sie im Ferienzeitraum fest. Danach gelten die Regeln auf dieser Seite, in dieser Reihenfolge. Beim Bestätigen einer Anmeldung wird der Beitrag festgehalten. Spätere Änderungen gelten nur für neue Bestätigungen.";

interface BillingDefaults {
  readonly rules: ContributionRule[];
  readonly bearers: CostBearer[];
  readonly numberRange: NumberRange;
  readonly careNumberRange: NumberRange;
  readonly terms: BillingTerms;
  readonly creditor: SepaCreditor;
}

function useBillingDefaults(enabled: boolean) {
  const [data, setData] = useState<BillingDefaults | null>(null);
  const [failure, setFailure] = useState<unknown>();
  const enabledRef = useRef(enabled);
  const loadGeneration = useRef(0);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;

  const read = useCallback(async () => {
    if (!enabledRef.current) return;
    const generation = ++loadGeneration.current;
    try {
      const [rules, bearers, numberRange, careNumberRange, terms, creditor] =
        await Promise.all([
          fetchContributionRules(),
          fetchCostBearers(),
          fetchHolidayNumberRange(),
          fetchCareNumberRange(),
          fetchBillingTerms(),
          fetchSepaCreditor(),
        ]);
      if (!enabledRef.current || generation !== loadGeneration.current) return;
      setData({
        rules,
        bearers,
        numberRange,
        careNumberRange,
        terms,
        creditor,
      });
      setFailure(undefined);
      clearLoadError();
    } catch (err) {
      if (!enabledRef.current || generation !== loadGeneration.current) return;
      setFailure(err);
      throw err;
    }
  }, [clearLoadError]);

  const load = useCallback(async () => {
    try {
      await read();
    } catch (err) {
      logger.error("traeger_billing_defaults_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Abrechnungsvorgaben",
        retry: () => void load(),
      });
    }
  }, [read, showLoadError]);

  useEffect(() => {
    enabledRef.current = enabled;
    if (enabled) {
      void load();
      return;
    }
    // A permission update can arrive after the data did. Invalidate both the
    // cache and requests already in flight before rendering the restricted view.
    loadGeneration.current++;
    setData(null);
    setFailure(undefined);
    clearLoadError();
  }, [enabled, load, clearLoadError]);

  return { data, failure, error: pageLoad.error, load, read };
}

export default function TraegerAbrechnungsvorgabenPage() {
  const { data: session } = useSession();
  const canBill = hasPermission(session ?? null, BILLING_RIGHT);
  const canEditBank = hasPermission(session ?? null, SETTINGS_RIGHT);
  const master = useTraegerMasterData("die Bankverbindungen");
  const billing = useBillingDefaults(canBill);
  const settings = useTraegerSettingsSchema();
  const afterBankWrite = useRefreshAfterWrite(
    "die Anzeige der Bankverbindungen",
    master.read,
  );
  const afterBillingWrite = useRefreshAfterWrite(
    "die Anzeige der Abrechnungsvorgaben",
    billing.read,
  );
  const onlineMandate = settings.data?.tabs
    .flatMap((tab) => tab.categories)
    .flatMap((category) => category.items)
    .find((item) => item.key === ONLINE_MANDATE_KEY)?.value;
  const error = metadataLoadError(
    master.error ??
      afterBankWrite.error ??
      (canBill
        ? (billing.error ?? afterBillingWrite.error ?? settings.loadError)
        : null) ??
      null,
    master.loaded || (canBill && billing.data !== null),
    master.failure,
    canBill ? billing.failure : undefined,
    canBill ? settings.error : undefined,
  );

  return (
    <TenantPage
      title="Abrechnungsvorgaben"
      info={canBill ? `${INFO} ${CONTRIBUTION_INFO}` : INFO}
      error={error}
      loading={
        !error &&
        (master.loading || !master.loaded || (canBill && !billing.data))
      }
    >
      {master.loaded ? (
        <>
          {canEditBank ? null : (
            <Alert
              type="info"
              message="Die Bankverbindungen können Sie nur ansehen. Ändern kann, wer im Träger-Büro das Recht „Einstellungen“ hat."
            />
          )}
          {canBill ? null : (
            <Alert
              type="info"
              message="Zahlung, Rechnungsnummern, Ermäßigungen und Kostenträger pflegt die Rolle „Abrechnung“."
            />
          )}
          <TraegerBankAccountsCard
            accounts={master.masterData.bankAccounts}
            canEdit={canEditBank}
            onChanged={afterBankWrite.refresh}
          />
        </>
      ) : null}
      {canBill && billing.data ? (
        <>
          <BillingTermsCard
            data={billing.data.terms}
            onChanged={afterBillingWrite.refresh}
          />
          <SepaCreditorCard
            creditor={billing.data.creditor}
            onlineMandate={
              typeof onlineMandate === "boolean" ? onlineMandate : null
            }
            onChanged={afterBillingWrite.refresh}
          />
          <NumberRangeCard
            numberRange={billing.data.numberRange}
            onChanged={afterBillingWrite.refresh}
          />
          <NumberRangeCard
            service="care"
            numberRange={billing.data.careNumberRange}
            onChanged={afterBillingWrite.refresh}
          />
          <ContributionRulesCard
            kind="sibling"
            rules={billing.data.rules.filter((rule) => rule.kind === "sibling")}
            onChanged={afterBillingWrite.refresh}
          />
          <ContributionRulesCard
            kind="benefits_exemption"
            rules={billing.data.rules.filter(
              (rule) => rule.kind === "benefits_exemption",
            )}
            onChanged={afterBillingWrite.refresh}
          />
          <CostBearersCard
            bearers={billing.data.bearers}
            onChanged={afterBillingWrite.refresh}
          />
        </>
      ) : null}
    </TenantPage>
  );
}

"use client";

// Briefkopf des Trägers (#4653, ADR 0112): Logo, Absender, Farbe, Fußzeile,
// Ansprechpartner und Bankverbindungen. Alle Dokumente des Trägers nutzen
// diese Angaben; die Vorschau zeigt Kopf und Fuß eines Briefs. Lesen darf,
// wer „Übersicht lesen“ hat, ändern nur, wer „Einstellungen“ hat.

import { useCallback, useEffect, useState } from "react";
import { useSession } from "next-auth/react";
import { SettingsCategory } from "~/components/settings/settings-category";
import { TraegerBankAccountsCard } from "~/components/traeger/traeger-bank-accounts-card";
import { TraegerContactCard } from "~/components/traeger/traeger-contact-card";
import { TraegerLetterheadPreview } from "~/components/traeger/traeger-letterhead-preview";
import { TraegerLogoCard } from "~/components/traeger/traeger-logo-card";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { CardSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { SectionCard } from "~/components/ui/section-card";
import { useApiLoadError } from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import { createLogger } from "~/lib/logger";
import type { ResolvedSetting, SchemaTab } from "~/lib/settings-api";
import {
  clearTraegerSettingValue,
  fetchTraegerMasterData,
  fetchTraegerSettingsSchema,
  saveTraegerSettingValue,
  type TraegerMasterData,
  traegerLogoSrc,
} from "~/lib/traeger-api";
import { TRAEGER_BRANDING_TAB } from "~/lib/traeger-branding";

const logger = createLogger({ component: "TraegerBriefkopfPage" });

/** The Träger right that changes the letterhead (ADR 0084 section 4). */
const SETTINGS_RIGHT = "traeger_settings:manage";

const EMPTY: TraegerMasterData = {
  contact: { name: "", phone: "", email: "" },
  logo: null,
  bankAccounts: [],
};

function brandingValue(tab: SchemaTab | null, key: string): string {
  const item: ResolvedSetting | undefined = tab?.categories
    .flatMap((category) => category.items)
    .find((candidate) => candidate.key === key);
  return typeof item?.value === "string" ? item.value : "";
}

export default function TraegerBriefkopfPage() {
  const { data: session } = useSession();
  const canEdit = hasPermission(session ?? null, SETTINGS_RIGHT);
  const [masterData, setMasterData] = useState<TraegerMasterData>(EMPTY);
  const [branding, setBranding] = useState<SchemaTab | null>(null);
  const [loading, setLoading] = useState(true);
  // The cards show only after one complete read: empty forms after a failed
  // load would overwrite stored values on save.
  const [loaded, setLoaded] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;

  const read = useCallback(async () => {
    const [data, schema] = await Promise.all([
      fetchTraegerMasterData(),
      fetchTraegerSettingsSchema(),
    ]);
    setMasterData(data);
    setBranding(
      schema.tabs.find((tab) => tab.key === TRAEGER_BRANDING_TAB) ?? null,
    );
    setLoaded(true);
    clearLoadError();
  }, [clearLoadError]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      await read();
    } catch (err) {
      logger.error("traeger_briefkopf_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "der Briefkopf",
        retry: () => void load(),
      });
    } finally {
      setLoading(false);
    }
  }, [read, showLoadError]);

  // After a change the page reads again; a failed refresh keeps the shown
  // values and says so.
  const refresh = useCallback(async () => {
    try {
      await read();
    } catch (err) {
      logger.warn("traeger_briefkopf_refresh_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "der Briefkopf",
        retry: () => void load(),
      });
    }
  }, [load, read, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  // Save and reset throw the ApiError of a failed request; the field shows
  // it on the shared error path.
  const saveSetting = useCallback(
    async (key: string, value: unknown) => {
      await saveTraegerSettingValue(key, value);
      void refresh();
    },
    [refresh],
  );
  const resetSetting = useCallback(
    async (key: string) => {
      await clearTraegerSettingValue(key);
      void refresh();
    },
    [refresh],
  );

  const defaultAccount =
    masterData.bankAccounts.find((account) => account.isDefault) ?? null;

  return (
    <div className="w-full space-y-6">
      <div>
        <h1 className="text-xl font-semibold text-gray-900 sm:text-2xl">
          Briefkopf
        </h1>
        <p className="mt-1 text-sm text-gray-500">
          Diese Angaben stehen auf allen Briefen und Rechnungen Ihres Trägers.
        </p>
        {!canEdit ? (
          <p className="mt-1 text-sm text-gray-500">
            Sie können die Angaben nur ansehen. Ändern kann, wer im Träger-Büro
            das Recht „Einstellungen“ hat.
          </p>
        ) : null}
      </div>

      <LoadErrorAlert error={pageLoad.error} />

      {loading ? (
        <SkeletonRegion label="Briefkopf wird geladen" className="space-y-6">
          <CardSkeleton rows={4} />
          <CardSkeleton rows={3} />
        </SkeletonRegion>
      ) : null}

      {!loading && loaded ? (
        <>
          <SectionCard
            title="Vorschau"
            description="So sehen Kopf und Fuß eines Briefs aus. Die Vorschau ändert sich mit Ihren Angaben."
          >
            <TraegerLetterheadPreview
              logoSrc={masterData.logo ? traegerLogoSrc(masterData.logo) : null}
              letterheadText={brandingValue(
                branding,
                "branding.letterhead_text",
              )}
              footerText={brandingValue(branding, "branding.footer_text")}
              color={brandingValue(branding, "branding.primary_color")}
              contact={masterData.contact}
              defaultAccount={defaultAccount}
            />
          </SectionCard>

          <TraegerLogoCard
            logo={masterData.logo}
            canEdit={canEdit}
            onChanged={refresh}
          />

          {branding?.categories.map((category) => (
            <SettingsCategory
              key={category.key}
              category={category}
              onSave={saveSetting}
              onReset={resetSetting}
              onSchemaRefresh={() => void refresh()}
              audience="traeger"
            />
          ))}

          <TraegerContactCard
            contact={masterData.contact}
            canEdit={canEdit}
            onSaved={refresh}
          />

          <TraegerBankAccountsCard
            accounts={masterData.bankAccounts}
            canEdit={canEdit}
            onChanged={refresh}
          />
        </>
      ) : null}
    </div>
  );
}

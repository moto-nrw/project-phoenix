"use client";

// Briefkopf des Trägers (#4653, ADR 0112): Logo, Absender, Farbe und Fußzeile.
// Alle Dokumente des Trägers nutzen diese Angaben; die Vorschau zeigt Kopf und
// Fuß eines Briefs, auch mit Ansprechperson und Bankverbindung aus den anderen
// Einstellungen (#5685). Darunter führt „Bestätigung und Rechnung“ direkt zum
// ganzen Schreiben, das ein Ferien-Lauf verschickt, mit Text und
// Rechnungstabelle (#5691), und „Weitere Vorlagen“ zu den übrigen
// Dokumentarten (#4677). Lesen darf, wer „Übersicht lesen“ hat, ändern nur, wer
// „Einstellungen“ hat.

import { useCallback } from "react";
import { useSession } from "next-auth/react";
import { SettingsCategory } from "~/components/settings/settings-category";
import { TraegerLetterheadPreview } from "~/components/traeger/traeger-letterhead-preview";
import { TraegerLogoCard } from "~/components/traeger/traeger-logo-card";
import { Alert } from "~/components/ui/alert";
import { ButtonLink } from "~/components/ui/button";
import { metadataLoadError } from "~/components/traeger/metadata-load-error";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { hasPermission } from "~/lib/auth-utils";
import type { ResolvedSetting, SchemaTab } from "~/lib/settings-api";
import {
  clearTraegerSettingValue,
  saveTraegerSettingValue,
  traegerLogoSrc,
} from "~/lib/traeger-api";
import { billingConcept } from "~/lib/concepts/billing";
import { documentTemplatesConcept } from "~/lib/concepts/documentTemplates";
import { traegerPath } from "~/lib/traeger-url";
import { useRefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { useTraegerMasterData } from "~/components/traeger/use-traeger-master-data";
import { useTraegerSettingsSchema } from "~/components/traeger/use-traeger-resources";
import { TRAEGER_BRANDING_TAB } from "~/lib/traeger-branding";

/** The Träger right that changes the letterhead (ADR 0084 section 4). */
const SETTINGS_RIGHT = "traeger_settings:manage";

function brandingValue(tab: SchemaTab | null, key: string): string {
  const item: ResolvedSetting | undefined = tab?.categories
    .flatMap((category) => category.items)
    .find((candidate) => candidate.key === key);
  return typeof item?.value === "string" ? item.value : "";
}

export default function TraegerBriefkopfPage() {
  const { data: session } = useSession();
  const canEdit = hasPermission(session ?? null, SETTINGS_RIGHT);
  const master = useTraegerMasterData("der Briefkopf");
  const afterLogoWrite = useRefreshAfterWrite(
    "die Anzeige des Logos",
    master.read,
  );
  const { masterData, loaded, loading, refresh: refreshMaster } = master;
  const {
    data: schema,
    isLoading: schemaLoading,
    loadError: schemaError,
    error: schemaFailure,
    mutate: refreshSchema,
  } = useTraegerSettingsSchema();
  const branding =
    schema?.tabs.find((tab) => tab.key === TRAEGER_BRANDING_TAB) ?? null;

  // After a change the page reads again; a failed refresh keeps the shown
  // values and says so.
  const refresh = useCallback(async () => {
    await Promise.all([
      refreshMaster(),
      // The shared schema reader owns its error and retry, not master data.
      refreshSchema().catch(() => undefined),
    ]);
  }, [refreshMaster, refreshSchema]);

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

  // Ein fehlgeschlagenes Nachladen lässt die gezeigten Angaben stehen; die
  // Meldung steht dann über ihnen statt an ihrer Stelle.
  const error = metadataLoadError(
    master.error ?? afterLogoWrite.error ?? schemaError,
    loaded && schema !== undefined,
    master.failure,
    schemaFailure,
  );

  return (
    <TenantPage
      title="Briefkopf und Vorlagen"
      info="Diese Angaben stehen auf allen Briefen und Rechnungen Ihres Trägers."
      error={error}
      loading={loading || schemaLoading}
    >
      {loaded && schema ? (
        <>
          {!canEdit ? (
            <Alert
              type="info"
              message="Sie können die Angaben nur ansehen. Ändern kann, wer im Träger-Büro das Recht „Einstellungen“ hat."
            />
          ) : null}
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
            onChanged={afterLogoWrite.refresh}
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

          <SectionCard
            title="Bestätigung und Rechnung"
            concept={billingConcept}
            description="Das Schreiben, das Familien nach der Ferienanmeldung bekommen. Mit Rechnung und Vorschau."
            action={
              <ButtonLink
                href={traegerPath(
                  "/traeger/einstellungen/briefkopf/vorlagen/bestaetigung",
                )}
                variant="outline"
                size="md"
              >
                Schreiben anpassen
              </ButtonLink>
            }
          />

          <SectionCard
            title="Weitere Vorlagen"
            concept={documentTemplatesConcept}
            description="Die Texte der anderen Schreiben, zum Beispiel Rechnung, Bescheid und Zahlungserinnerung."
            action={
              <ButtonLink
                href={traegerPath("/traeger/einstellungen/briefkopf/vorlagen")}
                variant="outline"
                size="md"
              >
                Vorlagen öffnen
              </ButtonLink>
            }
          />
        </>
      ) : null}
    </TenantPage>
  );
}

"use client";

// Einstellungen des Trägers (#4652, ADR 0086): Werte, die für alle Schulen
// des Trägers gelten. Eine Schule kann abweichen, wo die Einstellung das
// erlaubt; Schul-Abweichungen ändert nur die Schule selbst.

import { useCallback, useEffect, useState } from "react";
import { Settings } from "lucide-react";
import { SettingsCategory } from "~/components/settings/settings-category";
import { displayTabLabel } from "~/components/settings/settings-filter";
import { ConceptSectionHeader } from "~/components/ui/concept-section-header";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { CardSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import type { MotoConceptKey } from "~/lib/moto-concepts";
import type { SettingsSchema } from "~/lib/settings-api";
import {
  clearTraegerSettingValue,
  fetchTraegerSettingsSchema,
  saveTraegerSettingValue,
} from "~/lib/traeger-api";

const logger = createLogger({ component: "TraegerSettingsPage" });

const TAB_CONCEPTS: Record<string, MotoConceptKey> = {
  enrollment: "enrollments",
};

function hasWritableSetting(schema: SettingsSchema): boolean {
  return schema.tabs.some((tab) =>
    tab.categories.some((category) =>
      category.items.some((item) => item.writable),
    ),
  );
}

export default function TraegerSettingsPage() {
  const [schema, setSchema] = useState<SettingsSchema | null>(null);
  const [loading, setLoading] = useState(true);
  const schemaLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = schemaLoad;

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setSchema(await fetchTraegerSettingsSchema());
      clearLoadError();
    } catch (err) {
      logger.error("traeger_settings_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setSchema(null);
      await showLoadError(err, {
        object: "die Liste der Einstellungen",
        retry: () => void load(),
      });
    } finally {
      setLoading(false);
    }
  }, [clearLoadError, showLoadError]);

  // After a save the schema is read again for the stored value, its date
  // and DependsOn. A failed refresh keeps the shown values and says so.
  const refresh = useCallback(async () => {
    try {
      setSchema(await fetchTraegerSettingsSchema());
      clearLoadError();
    } catch (err) {
      logger.warn("traeger_settings_refresh_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Liste der Einstellungen",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, load, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  // Save and reset throw the ApiError of a failed request; the field shows
  // it on the shared error path.
  const handleSave = useCallback(
    async (key: string, value: unknown) => {
      await saveTraegerSettingValue(key, value);
      logger.info("traeger_setting_saved", { key });
      void refresh();
    },
    [refresh],
  );

  const handleReset = useCallback(
    async (key: string) => {
      await clearTraegerSettingValue(key);
      logger.info("traeger_setting_reset", { key });
      void refresh();
    },
    [refresh],
  );

  const tabs = schema?.tabs ?? [];

  return (
    <div className="w-full space-y-6">
      <div>
        <h1 className="text-xl font-semibold text-gray-900 sm:text-2xl">
          Einstellungen
        </h1>
        <p className="mt-1 text-sm text-gray-500">
          Hier legen Sie Werte für alle Schulen Ihres Trägers fest. Wo es
          erlaubt ist, kann eine Schule einen eigenen Wert eintragen.
        </p>
        {schema && tabs.length > 0 && !hasWritableSetting(schema) ? (
          <p className="mt-1 text-sm text-gray-500">
            Sie können die Werte nur ansehen. Ändern kann, wer im Träger-Büro
            das Recht dazu hat.
          </p>
        ) : null}
      </div>

      <LoadErrorAlert error={schemaLoad.error} />

      {loading && !schema ? (
        <SkeletonRegion
          label="Einstellungen werden geladen"
          className="space-y-6"
        >
          <CardSkeleton rows={3} />
          <CardSkeleton rows={2} />
        </SkeletonRegion>
      ) : null}

      {!loading && schema && tabs.length === 0 ? (
        <EmptyState
          icon={<Settings className="h-12 w-12" aria-hidden="true" />}
          title="Es gibt noch keine Einstellungen für den Träger."
          description="Die Schulen legen ihre Einstellungen bis dahin selbst fest."
        />
      ) : null}

      {tabs.map((tab) => (
        <section key={tab.key} className="space-y-4">
          <ConceptSectionHeader
            title={displayTabLabel(tab)}
            concept={TAB_CONCEPTS[tab.key] ?? "settings"}
          />
          {tab.categories.map((category) => (
            <SettingsCategory
              key={category.key}
              category={category}
              onSave={handleSave}
              onReset={handleReset}
              onSchemaRefresh={() => void refresh()}
              audience="traeger"
            />
          ))}
        </section>
      ))}
    </div>
  );
}

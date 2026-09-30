"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";

import { BooleanField } from "~/components/settings/fields/boolean-field";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { SectionCard } from "~/components/ui/section-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { Skeleton } from "~/components/ui/skeleton";
import {
  HOME_BLOCKS,
  type HomeBlockDefinition,
  type HomeBlockKey,
  type HomeBlockPolicies,
  type HomeBlockPolicy,
} from "~/lib/home-blocks";
import { useHomeLayout } from "~/lib/hooks/use-home-layout";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import { createLogger } from "~/lib/logger";
import { notifySettingsChanged } from "~/lib/settings-broadcast";
import {
  setSettingValue,
  type SettingsSchema,
  type ResolvedSetting,
} from "~/lib/settings-api";
import {
  useNFCEnabled,
  useOpenCareGroupMode,
  usePresenceMode,
} from "~/lib/tenant-context";

const logger = createLogger({ component: "HomeBlocksTab" });

const POLICY_ITEMS: readonly { value: HomeBlockPolicy; label: string }[] = [
  { value: "optional", label: "Frei wählbar" },
  { value: "required", label: "Immer anzeigen" },
  { value: "disabled", label: "Aus" },
];

// Die beiden Geburtstags-Schalter der Schule (#3737) stehen direkt an der
// Geburtstagskarte. Sie sind Registry-Einstellungen im Reiter „startseite“,
// den die allgemeine Einstellungsseite ausblendet.
const BIRTHDAY_BLOCK_KEY: HomeBlockKey = "section.birthdays";
const BIRTHDAYS_ENABLED_KEY = "operations.birthday_display_enabled";
const BIRTHDAYS_STAFF_KEY = "operations.birthday_display_include_staff";
const BIRTHDAY_SETTING_KEYS = [BIRTHDAYS_ENABLED_KEY, BIRTHDAYS_STAFF_KEY];

type BirthdaySettings = Record<string, boolean>;

function schemaItem(
  schema: SettingsSchema | null | undefined,
  key: string,
): ResolvedSetting | undefined {
  for (const tab of schema?.tabs ?? []) {
    for (const category of tab.categories) {
      const item = category.items.find((candidate) => candidate.key === key);
      if (item) return item;
    }
  }
  return undefined;
}

function birthdaySettingsOf(
  schema: SettingsSchema | null | undefined,
): BirthdaySettings {
  const values: BirthdaySettings = {};
  for (const key of BIRTHDAY_SETTING_KEYS) {
    const item = schemaItem(schema, key);
    if (typeof item?.value === "boolean") values[key] = item.value;
  }
  return values;
}

/**
 * "Startseite für alle" in den Einstellungen (#2875): was die Einrichtung
 * festlegt. Der Name trägt das „für alle", weil daneben der persönliche
 * Dialog „Startseite anpassen" steht.
 *
 * Drei Zustände je Baustein. "Frei wählbar" ist der Normalfall und wird nicht
 * gespeichert — so bleibt eine spätere Änderung des Standards von einer
 * bewussten Entscheidung unterscheidbar. "Immer anzeigen" und "Aus" nehmen den
 * Baustein aus dem persönlichen Dialog heraus.
 */
export function HomeBlocksTab() {
  const { state, isLoading, savePolicies } = useHomeLayout();
  const {
    data: schema,
    isLoading: schemaLoading,
    mutate: revalidateSchema,
  } = useSettingsSchema();
  const presenceMode = usePresenceMode();
  const openCareGroupMode = useOpenCareGroupMode();
  const nfcEnabled = useNFCEnabled();

  const [draft, setDraft] = useState<HomeBlockPolicies>({});
  const storedBirthdays = useMemo(() => birthdaySettingsOf(schema), [schema]);
  const [birthdayDraft, setBirthdayDraft] = useState<BirthdaySettings>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    setDraft(state.policies);
  }, [state.policies]);

  useEffect(() => {
    setBirthdayDraft(storedBirthdays);
  }, [storedBirthdays]);

  // Bausteine, die es in dieser Schule wegen des Betriebsmodus gar nicht gibt,
  // stehen nicht zur Vorgabe: über etwas zu entscheiden, das niemand sieht,
  // stiftet nur Verwirrung.
  const blocks = useMemo(() => {
    const ctx = {
      detailed: presenceMode !== "binary",
      openCareGroupMode,
      nfcEnabled,
      // Die Geburtstagskarte hängt an einer eigenen Einstellung; die Vorgabe
      // soll sie trotzdem regeln können, deshalb hier immer verfügbar.
      birthdaysEnabled: true,
      // Dasselbe für den Betreuungsplan und die Erinnerungen: die Leitung
      // entscheidet hier für die ganze Schule, nicht für den eigenen
      // Bildschirm.
      timetableEnabled: true,
      remindersEnabled: true,
      messagingEnabled: true,
      staffMessagingEnabled: true,
    };
    return HOME_BLOCKS.filter((block) => block.available(ctx));
  }, [presenceMode, openCareGroupMode, nfcEnabled]);

  const policiesDirty = useMemo(() => {
    const keys = new Set([
      ...Object.keys(draft),
      ...Object.keys(state.policies),
    ]);
    for (const key of keys) {
      const next = draft[key] ?? "optional";
      const current = state.policies[key] ?? "optional";
      if (next !== current) return true;
    }
    return false;
  }, [draft, state.policies]);

  const changedBirthdayKeys = BIRTHDAY_SETTING_KEYS.filter(
    (key) =>
      birthdayDraft[key] !== undefined &&
      birthdayDraft[key] !== storedBirthdays[key],
  );
  const dirty = policiesDirty || changedBirthdayKeys.length > 0;

  const changeBirthdaySetting = (key: string, value: boolean) => {
    setSaved(false);
    setBirthdayDraft((prev) => ({ ...prev, [key]: value }));
  };

  const change = (key: HomeBlockKey, policy: HomeBlockPolicy) => {
    setSaved(false);
    setDraft((prev) => {
      const next = { ...prev };
      if (policy === "optional") delete next[key];
      else next[key] = policy;
      return next;
    });
  };

  const save = async () => {
    setBusy(true);
    setError(null);
    try {
      for (const key of changedBirthdayKeys) {
        const failure = await setSettingValue(key, birthdayDraft[key]);
        if (failure) throw new Error(failure);
      }
      if (changedBirthdayKeys.length > 0) {
        notifySettingsChanged();
        await revalidateSchema();
      }
      if (policiesDirty) await savePolicies(draft);
      setSaved(true);
    } catch (err) {
      logger.error("home_block_policies_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        "Das Speichern hat nicht geklappt. Bitte versuchen Sie es noch einmal.",
      );
    } finally {
      setBusy(false);
    }
  };

  if (isLoading || schemaLoading) {
    return <Skeleton className="h-64 w-full" />;
  }

  const tiles = blocks.filter((block) => block.kind === "tile");
  const sections = blocks.filter((block) => block.kind === "section");
  const birthdaysOn = birthdayDraft[BIRTHDAYS_ENABLED_KEY] !== false;
  const birthdaySwitches = (
    <BirthdaySwitches
      schema={schema}
      values={birthdayDraft}
      disabled={busy}
      onChange={changeBirthdaySetting}
    />
  );
  const extras: Partial<Record<HomeBlockKey, ReactNode>> = {
    [BIRTHDAY_BLOCK_KEY]: birthdaySwitches,
  };
  // Sind die Geburtstage für die Schule aus, gibt es keine Karte, über deren
  // Anzeige man entscheiden könnte.
  const hiddenPolicies = new Set<HomeBlockKey>(
    birthdaysOn ? [] : [BIRTHDAY_BLOCK_KEY],
  );

  return (
    <SectionCard
      title="Startseite für alle"
      actions={
        <Button
          type="button"
          variant="primary"
          size="md"
          disabled={busy || !dirty}
          onClick={() => void save()}
        >
          {busy ? "Wird gespeichert …" : "Speichern"}
        </Button>
      }
    >
      <div className="space-y-6">
        <p className="text-sm leading-6 text-gray-600">
          Legen Sie fest, was die Startseite allen zeigt. Bei „Frei wählbar“
          entscheidet jede Person selbst. „Immer anzeigen“ und „Aus“ gelten für
          alle.
        </p>

        {error ? <Alert type="error" message={error} /> : null}
        {saved && !dirty ? (
          <Alert type="success" message="Gespeichert." />
        ) : null}

        <PolicyGroup
          heading="Kennzahlen"
          blocks={tiles}
          draft={draft}
          onChange={change}
          extras={extras}
          hiddenPolicies={hiddenPolicies}
        />
        <PolicyGroup
          heading="Bereiche"
          blocks={sections}
          draft={draft}
          onChange={change}
          extras={extras}
          hiddenPolicies={hiddenPolicies}
        />
      </div>
    </SectionCard>
  );
}

function PolicyGroup({
  heading,
  blocks,
  draft,
  onChange,
  extras,
  hiddenPolicies,
}: Readonly<{
  heading: string;
  blocks: readonly HomeBlockDefinition[];
  draft: HomeBlockPolicies;
  onChange: (key: HomeBlockKey, policy: HomeBlockPolicy) => void;
  extras: Partial<Record<HomeBlockKey, ReactNode>>;
  hiddenPolicies: ReadonlySet<HomeBlockKey>;
}>) {
  if (blocks.length === 0) return null;
  return (
    <section>
      <h3 className="text-sm font-semibold text-gray-900">{heading}</h3>
      <ul className="mt-2 divide-y divide-gray-100">
        {blocks.map((block) => (
          <li key={block.key} className="space-y-3 py-3">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div className="min-w-0">
                <p className="text-sm font-medium text-gray-900">
                  {block.label}
                </p>
                <p className="text-xs text-gray-500">{block.description}</p>
              </div>
              {hiddenPolicies.has(block.key) ? null : (
                <SegmentedControl<HomeBlockPolicy>
                  items={POLICY_ITEMS}
                  value={draft[block.key] ?? "optional"}
                  onChange={(policy) => onChange(block.key, policy)}
                  ariaLabel={`Vorgabe für ${block.label}`}
                  className="shrink-0"
                />
              )}
            </div>
            {extras[block.key] ?? null}
          </li>
        ))}
      </ul>
    </section>
  );
}

/**
 * Die Schul-Einstellungen zur Geburtstagskarte (#3737). Label und
 * Beschreibung kommen aus dem Registry-Schema, damit sie nur an einer Stelle
 * gepflegt werden. Der Schalter für das Personal hängt am ersten.
 */
function BirthdaySwitches({
  schema,
  values,
  disabled,
  onChange,
}: Readonly<{
  schema: SettingsSchema | null | undefined;
  values: BirthdaySettings;
  disabled: boolean;
  onChange: (key: string, value: boolean) => void;
}>) {
  const keys =
    values[BIRTHDAYS_ENABLED_KEY] === false
      ? [BIRTHDAYS_ENABLED_KEY]
      : BIRTHDAY_SETTING_KEYS;
  const rows = keys
    .map((key) => ({ key, item: schemaItem(schema, key) }))
    .filter(
      (row): row is { key: string; item: ResolvedSetting } =>
        row.item !== undefined && typeof values[row.key] === "boolean",
    );
  if (rows.length === 0) return null;
  return (
    <div className="space-y-3 rounded-xl border border-gray-100 bg-gray-50 p-3">
      {rows.map(({ key, item }) => (
        <div key={key} className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <p className="text-sm font-medium text-gray-900">{item.label}</p>
            <p className="text-xs text-gray-500">{item.description}</p>
          </div>
          <BooleanField
            value={values[key] === true}
            onChange={(next) => onChange(key, next)}
            disabled={disabled || !item.writable}
            ariaLabel={item.label}
          />
        </div>
      ))}
    </div>
  );
}

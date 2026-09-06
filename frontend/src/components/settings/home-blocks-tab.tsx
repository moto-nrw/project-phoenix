"use client";

import { useEffect, useMemo, useState } from "react";

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
import { createLogger } from "~/lib/logger";
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
  const presenceMode = usePresenceMode();
  const openCareGroupMode = useOpenCareGroupMode();
  const nfcEnabled = useNFCEnabled();

  const [draft, setDraft] = useState<HomeBlockPolicies>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    setDraft(state.policies);
  }, [state.policies]);

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
    };
    return HOME_BLOCKS.filter((block) => block.available(ctx));
  }, [presenceMode, openCareGroupMode, nfcEnabled]);

  const dirty = useMemo(() => {
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
      await savePolicies(draft);
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

  if (isLoading) {
    return <Skeleton className="h-64 w-full" />;
  }

  const tiles = blocks.filter((block) => block.kind === "tile");
  const sections = blocks.filter((block) => block.kind === "section");

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
        />
        <PolicyGroup
          heading="Bereiche"
          blocks={sections}
          draft={draft}
          onChange={change}
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
}: Readonly<{
  heading: string;
  blocks: readonly HomeBlockDefinition[];
  draft: HomeBlockPolicies;
  onChange: (key: HomeBlockKey, policy: HomeBlockPolicy) => void;
}>) {
  if (blocks.length === 0) return null;
  return (
    <section>
      <h3 className="text-sm font-semibold text-gray-900">{heading}</h3>
      <ul className="mt-2 divide-y divide-gray-100">
        {blocks.map((block) => (
          <li
            key={block.key}
            className="flex flex-col gap-3 py-3 sm:flex-row sm:items-center sm:justify-between"
          >
            <div className="min-w-0">
              <p className="text-sm font-medium text-gray-900">{block.label}</p>
              <p className="text-xs text-gray-500">{block.description}</p>
            </div>
            <SegmentedControl<HomeBlockPolicy>
              items={POLICY_ITEMS}
              value={draft[block.key] ?? "optional"}
              onChange={(policy) => onChange(block.key, policy)}
              ariaLabel={`Vorgabe für ${block.label}`}
              className="shrink-0"
            />
          </li>
        ))}
      </ul>
    </section>
  );
}

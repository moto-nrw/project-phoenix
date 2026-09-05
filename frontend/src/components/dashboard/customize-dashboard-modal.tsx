"use client";

import { useEffect, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { Modal } from "~/components/ui/modal";
import type {
  HomeBlockDefinition,
  HomeBlockKey,
  HomeLayoutOverrides,
} from "~/lib/home-blocks";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CustomizeDashboardModal" });

/**
 * "Startseite anpassen" (#2875): alle Bausteine, über die diese Person selbst
 * entscheiden darf. Gespeichert wird nur, was vom Standard abweicht;
 * "Zurücksetzen" stellt die empfohlene Ansicht wieder her.
 *
 * Was die Einrichtung festgelegt hat, steht hier gar nicht erst — ein Schalter,
 * der nichts bewirkt, ist schlimmer als ein fehlender.
 */
export function CustomizeDashboardModal({
  isOpen,
  onClose,
  adjustable,
  visible,
  customized,
  prescribedCount,
  onSave,
  onReset,
}: Readonly<{
  isOpen: boolean;
  onClose: () => void;
  adjustable: readonly HomeBlockDefinition[];
  visible: ReadonlySet<HomeBlockKey>;
  customized: boolean;
  prescribedCount: number;
  onSave: (overrides: HomeLayoutOverrides) => Promise<void>;
  onReset: () => Promise<void>;
}>) {
  const [selected, setSelected] = useState<Set<HomeBlockKey>>(
    () => new Set(visible),
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Beim Öffnen auf den aktuellen Stand setzen, nicht auf den vom letzten Mal.
  useEffect(() => {
    if (isOpen) {
      setSelected(new Set(visible));
      setError(null);
    }
  }, [isOpen, visible]);

  const toggle = (key: HomeBlockKey, checked: boolean) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (checked) next.add(key);
      else next.delete(key);
      return next;
    });
  };

  const save = async () => {
    setBusy(true);
    setError(null);
    try {
      const overrides: HomeLayoutOverrides = {};
      for (const block of adjustable) {
        const shown = selected.has(block.key);
        if (shown !== block.defaultVisible) {
          overrides[block.key] = shown;
        }
      }
      await onSave(overrides);
      onClose();
    } catch (err) {
      logger.error("home_layout_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        "Das Speichern hat nicht geklappt. Bitte versuchen Sie es noch einmal.",
      );
    } finally {
      setBusy(false);
    }
  };

  const reset = async () => {
    setBusy(true);
    setError(null);
    try {
      await onReset();
      onClose();
    } catch (err) {
      logger.error("home_layout_reset_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        "Das Zurücksetzen hat nicht geklappt. Bitte versuchen Sie es noch einmal.",
      );
    } finally {
      setBusy(false);
    }
  };

  const tiles = adjustable.filter((block) => block.kind === "tile");
  const sections = adjustable.filter((block) => block.kind === "section");

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Startseite anpassen"
      mobileSheet
      footer={
        <div className="flex w-full flex-wrap items-center justify-between gap-2">
          <Button
            type="button"
            variant="outline"
            size="md"
            disabled={busy || !customized}
            onClick={() => void reset()}
          >
            Zurücksetzen
          </Button>
          <div className="flex gap-2">
            <Button
              type="button"
              variant="outline"
              size="md"
              disabled={busy}
              onClick={onClose}
            >
              Abbrechen
            </Button>
            <Button
              type="button"
              variant="primary"
              size="md"
              disabled={busy}
              onClick={() => void save()}
            >
              {busy ? "Wird gespeichert …" : "Speichern"}
            </Button>
          </div>
        </div>
      }
    >
      <div className="space-y-6">
        <p className="text-sm leading-6 text-gray-600">
          Wählen Sie, was Ihre Startseite zeigt. Die Auswahl gilt auf allen
          Ihren Geräten.
        </p>
        {error ? <Alert type="error" message={error} /> : null}

        {adjustable.length === 0 ? (
          <p className="text-sm leading-6 text-gray-600">
            Ihre Schule hat die Startseite fest eingestellt. Hier gibt es nichts
            zu ändern.
          </p>
        ) : (
          <>
            <BlockGroup
              heading="Kennzahlen"
              blocks={tiles}
              selected={selected}
              onToggle={toggle}
            />
            <BlockGroup
              heading="Bereiche"
              blocks={sections}
              selected={selected}
              onToggle={toggle}
            />
          </>
        )}

        {prescribedCount > 0 ? (
          // Ohne diesen Satz sieht die Person nur, dass eine Kachel fehlt, und
          // sucht den Fehler bei sich.
          <p className="text-xs leading-5 text-gray-500">
            Ihre Schule hat{" "}
            {prescribedCount === 1
              ? "eine Kachel"
              : `${prescribedCount} Kacheln`}{" "}
            fest eingestellt. Die können Sie nicht ändern.
          </p>
        ) : null}
      </div>
    </Modal>
  );
}

function BlockGroup({
  heading,
  blocks,
  selected,
  onToggle,
}: Readonly<{
  heading: string;
  blocks: readonly HomeBlockDefinition[];
  selected: ReadonlySet<HomeBlockKey>;
  onToggle: (key: HomeBlockKey, checked: boolean) => void;
}>) {
  if (blocks.length === 0) return null;
  return (
    <fieldset>
      <legend className="text-sm font-semibold text-gray-900">{heading}</legend>
      <ul className="mt-2 divide-y divide-gray-100">
        {blocks.map((block) => {
          const id = `home-block-${block.key.replace(".", "-")}`;
          return (
            <li key={block.key}>
              <label
                htmlFor={id}
                className="flex cursor-pointer items-start gap-3 py-2.5"
              >
                <Checkbox
                  id={id}
                  className="mt-0.5"
                  checked={selected.has(block.key)}
                  onChange={(event) =>
                    onToggle(block.key, event.currentTarget.checked)
                  }
                />
                <span className="min-w-0">
                  <span className="block text-sm font-medium text-gray-900">
                    {block.label}
                  </span>
                  <span className="block text-xs text-gray-500">
                    {block.description}
                  </span>
                </span>
              </label>
            </li>
          );
        })}
      </ul>
    </fieldset>
  );
}

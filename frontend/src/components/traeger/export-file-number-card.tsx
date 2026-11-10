"use client";

// Die Dateinummern des Trägers (#4693): jede Datei der Elternabrechnung
// bekommt die nächste Nummer, auch jeder wiederholte Download. Vor der
// ersten Datei lässt sich die erste Nummer einmal festlegen, damit die
// Nummern Ihres bisherigen Programms weiterlaufen.

import { useState } from "react";
import { Button } from "~/components/ui/button";
import { FormRow } from "~/components/ui/form-row";
import { Input } from "~/components/ui/input";
import { ConfirmationModal } from "~/components/ui/modal";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type ExportSettings,
  setFileNumberStart,
} from "~/lib/traeger-finance-exports-api";

const logger = createLogger({ component: "ExportFileNumberCard" });

export function ExportFileNumberCard({
  settings,
  onChanged,
}: Readonly<{
  settings: ExportSettings;
  onChanged: (settings: ExportSettings) => void;
}>) {
  const [start, setStart] = useState("");
  const [confirming, setConfirming] = useState(false);
  const [saving, setSaving] = useState(false);
  const errors = useApiErrorDisplay();
  const { success } = useToast();
  const value = Number(start);
  const valid = /^\d{1,9}$/.test(start.trim()) && value >= 1;

  const save = async () => {
    setSaving(true);
    try {
      onChanged(await setFileNumberStart(value));
      setConfirming(false);
      setStart("");
      success(`Die erste Datei bekommt die Nummer ${value}.`);
    } catch (err) {
      logger.error("export_file_number_start_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setConfirming(false);
      void errors.show(err, { object: "die erste Dateinummer" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <SectionCard
      title="Dateinummern"
      titleBadge={
        <StatusBadge
          label={`Nächste Nummer ${settings.nextFileNumber}`}
          tone="blue"
        />
      }
      description="Jede Sollstellung und jede Lastschriftdatei bekommt die nächste Nummer, auch wenn Sie dieselbe Datei noch einmal holen."
    >
      {settings.fileNumbersStarted ? (
        <p className="text-sm text-gray-600">
          Die Nummern laufen schon. Die erste Nummer lässt sich nicht mehr
          ändern.
        </p>
      ) : (
        <div className="flex flex-col gap-2">
          <FormRow className="sm:max-w-md">
            <Input
              controlSize="compact"
              label="Erste Nummer"
              name="start"
              inputMode="numeric"
              value={start}
              aria-describedby="export-file-number-hint"
              onChange={(event) => setStart(event.target.value)}
            />
            <Button
              type="button"
              variant="outline"
              size="field"
              disabled={!valid}
              onClick={() => setConfirming(true)}
            >
              Festlegen
            </Button>
          </FormRow>
          <p id="export-file-number-hint" className="text-sm text-gray-500">
            Nur einmal möglich. Nehmen Sie die Nummer nach der letzten Datei
            Ihres bisherigen Programms.
          </p>
        </div>
      )}
      <ConfirmationModal
        isOpen={confirming}
        onClose={() => setConfirming(false)}
        onConfirm={() => void save()}
        title="Erste Dateinummer festlegen?"
        confirmText="Ja, festlegen"
        isConfirmLoading={saving}
      >
        <p>
          Die nächste Datei bekommt die Nummer {valid ? value : "–"}. Danach
          lässt sich die Nummer nicht mehr ändern.
        </p>
      </ConfirmationModal>
    </SectionCard>
  );
}

"use client";

// Rechnungsnummern der Ferienbetreuung (#4668) und der Monatsbeiträge
// (#4684): das Muster, aus dem die Freigabe eines Laufs die Nummern
// vergibt, lückenlos und in Reihenfolge. Jede Leistung hat ihren eigenen
// Nummernkreis.

import { useEffect, useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type NumberRange,
  saveCareNumberRange,
  saveHolidayNumberRange,
} from "~/lib/traeger-billing-api";

const logger = createLogger({ component: "NumberRangeCard" });

/** What differs between the number range of the holidays and of the months. */
const SERVICES = {
  holiday: {
    title: "Rechnungsnummern",
    description:
      "Bei der Freigabe bekommt jede Rechnung die nächste Nummer. Es entsteht keine Lücke.",
    defaultPattern: "FE-{JJJJ}-{NNNN}",
    save: saveHolidayNumberRange,
  },
  care: {
    title: "Rechnungsnummern der Monatsbeiträge",
    description:
      "Eigener Nummernkreis für die Monatsbeiträge. Nehmen Sie ein anderes Kürzel als bei den Ferien.",
    defaultPattern: "MB-{JJJJ}-{NNNN}",
    save: saveCareNumberRange,
  },
} as const;

export function NumberRangeCard({
  numberRange,
  onChanged,
  service = "holiday",
}: Readonly<{
  numberRange: NumberRange;
  onChanged: () => Promise<unknown>;
  service?: keyof typeof SERVICES;
}>) {
  const copy = SERVICES[service];
  const [open, setOpen] = useState(false);
  const [pattern, setPattern] = useState("");
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  useEffect(() => {
    if (!open) return;
    setPattern(numberRange.pattern || copy.defaultPattern);
    clearErrors();
  }, [open, numberRange.pattern, copy.defaultPattern, clearErrors]);

  const submit = async () => {
    formErrors.clear();
    if (!pattern.trim()) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", {
        pattern: "Bitte geben Sie ein Muster an.",
      });
      return;
    }
    setSaving(true);
    try {
      await copy.save(pattern.trim());
      success("Das Muster ist gespeichert.");
      setOpen(false);
      await onChanged();
    } catch (err) {
      logger.error("number_range_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "das Muster" });
    } finally {
      setSaving(false);
    }
  };

  const set = Boolean(numberRange.pattern);
  return (
    <SectionCard
      title={copy.title}
      description={copy.description}
      actions={
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={() => setOpen(true)}
        >
          {set ? "Muster ändern" : "Muster festlegen"}
        </Button>
      }
    >
      {set ? (
        <DataGrid columns={2}>
          <DataField label="Muster">
            <span className="font-mono">{numberRange.pattern}</span>
          </DataField>
          <DataField label="Erste Nummer in diesem Jahr">
            <span className="font-mono">{numberRange.example}</span>
          </DataField>
        </DataGrid>
      ) : (
        <p className="text-sm text-gray-700">
          Noch kein Muster. Ohne Muster können Sie keinen Lauf freigeben.
        </p>
      )}
      <FormModal
        isOpen={open}
        onClose={() => setOpen(false)}
        closeDisabled={saving}
        isBackdropDismissDisabled
        title={set ? "Muster ändern" : "Muster festlegen"}
        size="md"
        error={formErrors.error}
        footer={
          <EditActions
            onCancel={() => setOpen(false)}
            onSave={() => formRef.current?.requestSubmit()}
            saving={saving}
          />
        }
      >
        <form
          ref={formRef}
          noValidate
          className="space-y-3"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <Input
            controlSize="compact"
            id={`number-range-pattern-${service}`}
            name="pattern"
            label="Muster der Rechnungsnummer"
            value={pattern}
            maxLength={40}
            error={formErrors.fieldError("pattern") ?? undefined}
            onChange={(event) => setPattern(event.target.value)}
          />
          <ul className="list-disc space-y-1 pl-5 text-sm text-gray-600">
            <li>
              <span className="font-mono">{"{NNNN}"}</span> ist die laufende
              Nummer, hier mit 4 Stellen. Erlaubt sind 1 bis 8 N.
            </li>
            <li>
              <span className="font-mono">{"{JJJJ}"}</span> oder{" "}
              <span className="font-mono">{"{JJ}"}</span> ist das Jahr. Mit Jahr
              beginnt die Nummer jedes Jahr wieder bei 1.
            </li>
            <li>
              Sonst sind Buchstaben, Ziffern und die Zeichen . / _ - erlaubt.
            </li>
          </ul>
          <p className="text-sm text-gray-600">
            Ausgestellte Rechnungen behalten ihre Nummer.
          </p>
        </form>
      </FormModal>
    </SectionCard>
  );
}

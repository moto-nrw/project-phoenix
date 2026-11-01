import { useId, useRef, useState, type SyntheticEvent } from "react";

import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { FieldLabel } from "~/components/ui/field-label";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { useApiFormError } from "~/contexts/ToastContext";
import {
  clockToMinutes,
  createSurchargeRule,
  createWageType,
  parseRate,
  SURCHARGE_KIND_LABELS,
  SURCHARGE_RATE_TYPE_LABELS,
  WAGE_TYPE_UNIT_LABELS,
  wageTypesForRate,
  type PayrollWageType,
  type SurchargeKind,
  type SurchargeRateType,
  type WageTypeUnit,
} from "~/lib/payroll-rules-api";

// Anlegen einer eigenen Lohnart und einer Zuschlagsregel auf /payroll
// (#4807). Beides speichert erst mit dem Knopf; ändern lassen sich die
// Einträge danach direkt in ihrer Zeile.

const UNIT_ITEMS = (Object.keys(WAGE_TYPE_UNIT_LABELS) as WageTypeUnit[]).map(
  (value) => ({ value, label: WAGE_TYPE_UNIT_LABELS[value] }),
);

const KIND_ITEMS = (Object.keys(SURCHARGE_KIND_LABELS) as SurchargeKind[]).map(
  (value) => ({ value, label: SURCHARGE_KIND_LABELS[value] }),
);

const RATE_TYPE_ITEMS = (
  Object.keys(SURCHARGE_RATE_TYPE_LABELS) as SurchargeRateType[]
).map((value) => ({ value, label: SURCHARGE_RATE_TYPE_LABELS[value] }));

function ModalFooter({
  formId,
  saving,
  disabled,
  onCancel,
}: {
  readonly formId: string;
  readonly saving: boolean;
  readonly disabled: boolean;
  readonly onCancel: () => void;
}) {
  return (
    <div className="flex justify-end gap-2">
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={onCancel}
        disabled={saving}
      >
        Abbrechen
      </Button>
      <Button
        type="submit"
        form={formId}
        variant="primary"
        size="md"
        isLoading={saving}
        loadingText="Wird angelegt …"
        disabled={saving || disabled}
      >
        Anlegen
      </Button>
    </div>
  );
}

export function CreateWageTypeModal({
  isOpen,
  onClose,
  onCreated,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onCreated: () => void | Promise<unknown>;
}) {
  const formId = useId();
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const [name, setName] = useState("");
  const [number, setNumber] = useState("");
  const [unit, setUnit] = useState<WageTypeUnit>("stunden");
  const [saving, setSaving] = useState(false);

  const numberValid = number.trim() === "" || /^\d{1,4}$/.test(number.trim());

  const close = () => {
    formErrors.clear();
    setName("");
    setNumber("");
    setUnit("stunden");
    onClose();
  };

  const submit = async (event: SyntheticEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (name.trim() === "" || !numberValid) return;
    setSaving(true);
    formErrors.clear();
    try {
      await createWageType({ name: name.trim(), number: number.trim(), unit });
      await onCreated();
      close();
    } catch (err) {
      void formErrors.show(err, { object: "die Lohnart" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={close}
      title="Lohnart anlegen"
      size="md"
      closeDisabled={saving}
      isBackdropDismissDisabled
      error={formErrors.error}
      footer={
        <ModalFooter
          formId={formId}
          saving={saving}
          disabled={name.trim() === "" || !numberValid}
          onCancel={close}
        />
      }
    >
      <form
        id={formId}
        ref={formRef}
        onSubmit={(event) => void submit(event)}
        className="space-y-4"
      >
        <Input
          label="Bezeichnung"
          name="name"
          value={name}
          onChange={(event) => setName(event.target.value)}
          placeholder="zum Beispiel Nachtzuschlag"
          maxLength={100}
          error={formErrors.fieldError("name")}
          disabled={saving}
          required
        />
        <Input
          label="Lohnartnummer"
          name="number"
          value={number}
          onChange={(event) => setNumber(event.target.value)}
          inputMode="numeric"
          placeholder="Nicht konfiguriert"
          error={
            numberValid ? formErrors.fieldError("number") : "1 bis 4 Ziffern."
          }
          disabled={saving}
        />
        <div className="space-y-1">
          <FieldLabel>Einheit</FieldLabel>
          <SegmentedControl
            items={UNIT_ITEMS}
            value={unit}
            onChange={setUnit}
            ariaLabel="Einheit"
            fullWidth
          />
          <p className="text-sm text-gray-600">
            So erwartet Ihr Lohnbüro die Lohnart. Zuschläge in Prozent brauchen
            Stunden, Zuschläge in Euro einen Betrag.
          </p>
        </div>
      </form>
    </FormModal>
  );
}

export function CreateSurchargeRuleModal({
  isOpen,
  wageTypes,
  onClose,
  onCreated,
}: {
  readonly isOpen: boolean;
  readonly wageTypes: readonly PayrollWageType[];
  readonly onClose: () => void;
  readonly onCreated: () => void | Promise<unknown>;
}) {
  const formId = useId();
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const [kind, setKind] = useState<SurchargeKind>("nacht");
  const [from, setFrom] = useState("20:00");
  const [until, setUntil] = useState("06:00");
  const [rateType, setRateType] = useState<SurchargeRateType>("prozent");
  const [rate, setRate] = useState("");
  const [wageTypeId, setWageTypeId] = useState("");
  const [saving, setSaving] = useState(false);

  const choices = wageTypesForRate(wageTypes, rateType);
  const selectedWageType = choices.some((w) => w.id === wageTypeId)
    ? wageTypeId
    : "";
  const rateValue = parseRate(rate);
  const fromMinute = clockToMinutes(from);
  const untilMinute = clockToMinutes(until);
  const windowValid =
    kind !== "nacht" ||
    (fromMinute !== null && untilMinute !== null && fromMinute !== untilMinute);
  const complete = rateValue !== null && selectedWageType !== "" && windowValid;

  const close = () => {
    formErrors.clear();
    setKind("nacht");
    setFrom("20:00");
    setUntil("06:00");
    setRateType("prozent");
    setRate("");
    setWageTypeId("");
    onClose();
  };

  const submit = async (event: SyntheticEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!complete || rateValue === null) return;
    setSaving(true);
    formErrors.clear();
    try {
      await createSurchargeRule({
        kind,
        windowStartMinute: kind === "nacht" ? fromMinute : null,
        windowEndMinute: kind === "nacht" ? untilMinute : null,
        rateType,
        rateHundredths: rateValue,
        wageTypeId: selectedWageType,
      });
      await onCreated();
      close();
    } catch (err) {
      void formErrors.show(err, { object: "die Zuschlagsregel" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={close}
      title="Zuschlagsregel anlegen"
      size="md"
      closeDisabled={saving}
      isBackdropDismissDisabled
      error={formErrors.error}
      footer={
        <ModalFooter
          formId={formId}
          saving={saving}
          disabled={!complete}
          onCancel={close}
        />
      }
    >
      <form
        id={formId}
        ref={formRef}
        onSubmit={(event) => void submit(event)}
        className="space-y-4"
      >
        <div className="space-y-1">
          <FieldLabel>Art</FieldLabel>
          <SegmentedControl
            items={KIND_ITEMS}
            value={kind}
            onChange={setKind}
            ariaLabel="Art"
            fullWidth
          />
        </div>
        {kind === "nacht" ? (
          <div className="grid grid-cols-2 gap-4">
            <Input
              label="Von"
              type="time"
              value={from}
              onChange={(event) => setFrom(event.target.value)}
              disabled={saving}
              required
            />
            <Input
              label="Bis"
              type="time"
              value={until}
              onChange={(event) => setUntil(event.target.value)}
              error={
                windowValid
                  ? undefined
                  : "Bitte zwei verschiedene Uhrzeiten wählen."
              }
              disabled={saving}
              required
            />
          </div>
        ) : (
          <p className="text-sm text-gray-600">
            Gilt für den ganzen Tag, von 0 bis 24 Uhr.
          </p>
        )}
        <div className="space-y-1">
          <FieldLabel>Höhe</FieldLabel>
          <SegmentedControl
            items={RATE_TYPE_ITEMS}
            value={rateType}
            onChange={setRateType}
            ariaLabel="Höhe als"
            fullWidth
          />
        </div>
        <Input
          label={rateType === "prozent" ? "Prozent" : "Euro je Stunde"}
          name="rate_hundredths"
          value={rate}
          onChange={(event) => setRate(event.target.value)}
          inputMode="decimal"
          placeholder={
            rateType === "prozent" ? "zum Beispiel 25" : "zum Beispiel 2,50"
          }
          error={
            rate.trim() === "" || rateValue !== null
              ? formErrors.fieldError("rate_hundredths")
              : "Bitte eine Zahl größer als 0 eingeben."
          }
          disabled={saving}
          required
        />
        <div className="space-y-1">
          <FieldLabel required>Lohnart</FieldLabel>
          {choices.length > 0 ? (
            <CustomSelect
              ariaLabel="Lohnart"
              value={selectedWageType}
              placeholder="Lohnart wählen"
              options={choices.map((w) => ({ value: w.id, label: w.name }))}
              onChange={setWageTypeId}
            />
          ) : (
            <p className="text-sm text-gray-600">
              {rateType === "prozent"
                ? "Es gibt noch keine eigene Lohnart mit der Einheit Stunden. Bitte legen Sie zuerst eine an."
                : "Es gibt noch keine eigene Lohnart mit der Einheit Betrag in Euro. Bitte legen Sie zuerst eine an."}
            </p>
          )}
        </div>
      </form>
    </FormModal>
  );
}

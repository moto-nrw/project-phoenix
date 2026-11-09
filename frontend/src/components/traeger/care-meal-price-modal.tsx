"use client";

// Mensa-Preis einer Schule ab einem Monat (#4686): je Essen oder als
// Pauschale je Kind und Monat. Ein Monat, den schon eine Rechnung abrechnet,
// behält seinen Preis; das prüft das Speichern.

import { useEffect, useMemo, useRef, useState } from "react";
import { CustomSelect } from "~/components/ui/custom-select";
import { EditActions } from "~/components/ui/edit-actions";
import { SectionLabel } from "~/components/ui/section-label";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type CareMealMode,
  type CareMealTariffs,
  mealTariffOn,
  saveCareMealTariff,
} from "~/lib/traeger-care-meal-api";
import { addMonths, formatMonth, monthsFromTo } from "./care-billing-format";
import { MEAL_MODE_LABELS } from "./care-meal-format";
import { centsToEuroInput, euroInputToCents } from "./holiday-period-model";

const logger = createLogger({ component: "CareMealPriceModal" });

const MONTHS_BACK = 12;
const MONTHS_AHEAD = 18;

const MODE_HINTS: Record<CareMealMode, string> = {
  per_meal:
    "Eltern zahlen jedes Essen. Es zählt jeder Tag, an dem das Kind angemeldet und da war. Abgemeldete Tage zählen nicht.",
  flat: "Eltern zahlen jeden Monat denselben Betrag. Das gilt für jedes Kind, das in dem Monat zum Essen angemeldet war.",
};

export function CareMealPriceModal({
  data,
  school,
  currentMonth,
  onClose,
  onSaved,
}: Readonly<{
  data: CareMealTariffs;
  /** The school whose Mensa price changes; null keeps the modal closed. */
  school: { id: string; name: string } | null;
  /** The current month, "YYYY-MM-01". */
  currentMonth: string;
  onClose: () => void;
  onSaved: (data: CareMealTariffs) => void;
}>) {
  const [month, setMonth] = useState(currentMonth);
  const [mode, setMode] = useState<CareMealMode>("per_meal");
  const [amount, setAmount] = useState("");
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  const months = useMemo(
    () =>
      monthsFromTo(
        addMonths(currentMonth, -MONTHS_BACK),
        addMonths(currentMonth, MONTHS_AHEAD),
      ),
    [currentMonth],
  );

  // Open with the next month the school has not billed yet and its price.
  useEffect(() => {
    if (!school) return;
    const billed = data.billedUntil[school.id];
    const start =
      billed && billed >= currentMonth ? addMonths(billed, 1) : currentMonth;
    const holding = mealTariffOn(data.tariffs, school.id, start);
    setMonth(start);
    setMode(holding?.mode ?? "per_meal");
    setAmount(holding ? centsToEuroInput(holding.amountCents) : "");
    clearErrors();
  }, [school, data, currentMonth, clearErrors]);

  const submit = async () => {
    if (!school) return;
    formErrors.clear();
    const cents = euroInputToCents(amount.trim());
    if (cents === null) {
      formErrors.invalid("Bitte prüfen Sie den Betrag.", {
        amount_cents: "Bitte einen Betrag wie 4,80 eingeben.",
      });
      return;
    }
    setSaving(true);
    try {
      onSaved(
        await saveCareMealTariff({
          schoolId: school.id,
          validFrom: month,
          mode,
          amountCents: cents,
        }),
      );
      success(`Der Mensa-Preis gilt ab ${formatMonth(month)}.`);
    } catch (err) {
      logger.error("care_meal_price_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Preisangabe für die Mensa" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <FormModal
      isOpen={school !== null}
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title={school ? `Mensa-Preis für ${school.name}` : "Mensa-Preis"}
      size="md"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          saveLabel="Preis speichern"
        />
      }
    >
      <form
        ref={formRef}
        noValidate
        className="space-y-5"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <CustomSelect
          id="care-meal-month"
          name="valid_from"
          label="Gültig ab"
          value={month}
          options={[...months].reverse().map((value) => ({
            value,
            label: formatMonth(value),
          }))}
          onChange={setMonth}
          error={formErrors.fieldError("valid_from")}
        />
        <div className="space-y-2">
          <SectionLabel as="p">Abrechnungsart</SectionLabel>
          <SegmentedControl<CareMealMode>
            ariaLabel="Abrechnungsart"
            fullWidth
            value={mode}
            onChange={setMode}
            items={[
              { value: "per_meal", label: MEAL_MODE_LABELS.per_meal },
              { value: "flat", label: MEAL_MODE_LABELS.flat },
            ]}
          />
          <p className="text-sm text-gray-600">{MODE_HINTS[mode]}</p>
        </div>
        <Input
          controlSize="touch"
          id="care-meal-amount"
          name="amount_cents"
          label={
            mode === "per_meal"
              ? "Preis je Essen in €"
              : "Betrag je Kind und Monat in €"
          }
          inputMode="decimal"
          placeholder={mode === "per_meal" ? "4,80" : "55,00"}
          required
          value={amount}
          error={formErrors.fieldError("amount_cents")}
          onChange={(event) => setAmount(event.target.value)}
        />
      </form>
    </FormModal>
  );
}

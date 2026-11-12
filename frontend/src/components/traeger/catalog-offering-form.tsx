"use client";

// Betreuungsangebot des Trägers anlegen oder ändern (#4704): die Angaben,
// die jede Schule als eigenes Angebot bekommt, dazu Preise und
// Vertragsvorlage. Preise und Vertragsvorlage schreibt das Übertragen in die
// Abrechnung und die Verträge der Schule.

import { useRef, useState } from "react";
import { CheckboxCard } from "~/components/ui/checkbox-card";
import { CustomSelect } from "~/components/ui/custom-select";
import { EditActions } from "~/components/ui/edit-actions";
import { FieldLabel } from "~/components/ui/field-label";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { FormFieldset } from "~/components/ui/form-fieldset";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { SwitchRow } from "~/components/ui/switch";
import { Textarea } from "~/components/ui/textarea";
import { WeekdayToggleGroup } from "~/components/ui/weekday-toggle-group";
import { useApiFormError } from "~/contexts/ToastContext";
import {
  SELECTION_RULE_LABELS,
  type CareSelectionRule,
  type DaysOfWeekMode,
} from "~/lib/care-offering-api";
import { createLogger } from "~/lib/logger";
import type { CareWeekday } from "~/lib/traeger-care-billing-api";
import type {
  TraegerCarePrice,
  TraegerOffering,
  TraegerOfferingDraft,
  TraegerOfferingFields,
} from "~/lib/traeger-catalog-api";
import type { ContractTerms } from "~/lib/traeger-care-contracts-api";
import { WEEKDAY_LONG, WEEKDAY_SHORT } from "./care-billing-format";
import { ContractTermsFields, DEFAULT_TERMS } from "./care-contract-conditions";
import {
  CarePriceModuleEditor,
  EMPTY_MODULE_DRAFT,
  type ModuleDraft,
  draftColumns,
  priceFieldKey,
} from "./care-price-module-editor";
import { SCHOOL_WEEKDAYS } from "./catalog-format";
import { centsToEuroInput, euroInputToCents } from "./holiday-period-model";

const logger = createLogger({ component: "CatalogOfferingForm" });

/** The key the price editor gives this form's amounts. */
const PRICE_KEY = "angebot";

const DAY_OPTIONS = SCHOOL_WEEKDAYS.map((day) => ({
  value: day,
  label: WEEKDAY_SHORT[day],
  ariaLabel: WEEKDAY_LONG[day],
}));

const DAYS_MODE_ITEMS = [
  { value: "fixed" as const, label: "Feste Tage" },
  { value: "parent_choice" as const, label: "Eltern wählen die Tage" },
];

const RULE_OPTIONS = (
  Object.entries(SELECTION_RULE_LABELS) as [CareSelectionRule, string][]
).map(([value, label]) => ({ value, label }));

/** What the form edits; numbers stay text until the save. */
export interface OfferingFormDraft {
  readonly fields: TraegerOfferingFields;
  readonly capacity: string;
  readonly priceInForm: string;
  readonly sortOrder: string;
  readonly prices: ModuleDraft;
  readonly contractTemplate: ContractTerms | null;
}

export function blankOfferingDraft(): OfferingFormDraft {
  return {
    fields: {
      name: "",
      description: null,
      daysOfWeekMode: "fixed",
      availableDays: ["mon", "tue", "wed", "thu", "fri"],
      includesHolidayCare: false,
      includesLunch: false,
      capacity: null,
      priceCents: null,
      isRequired: false,
      countsAsCare: true,
      selectionGroup: "",
      selectionRule: "optional",
      sortOrder: 0,
      pickupTimes: {},
    },
    capacity: "",
    priceInForm: "",
    sortOrder: "0",
    prices: EMPTY_MODULE_DRAFT,
    contractTemplate: null,
  };
}

/** The price table of the editor from the stored prices of one module. */
export function moduleDraftOf(
  prices: readonly TraegerCarePrice[],
): ModuleDraft {
  if (prices.length === 0) return EMPTY_MODULE_DRAFT;
  const scale = prices.some((price) => price.daysPerWeek !== null);
  const first: Record<string, string> = {};
  const further: Record<string, string> = {};
  for (const price of prices) {
    const key = scale ? String(price.daysPerWeek) : (price.weekday ?? "");
    first[key] = centsToEuroInput(price.monthlyCents);
    if (price.siblingCents !== null) {
      further[key] = centsToEuroInput(price.siblingCents);
    }
  }
  return {
    form: scale ? "days_per_week" : "weekday",
    mealExtra: prices.some((price) => price.mealExtra),
    first,
    further,
  };
}

function offeringFormDraftOf(offering: TraegerOffering): OfferingFormDraft {
  const { fields, prices, contractTemplate } = offering.draft;
  return {
    fields,
    capacity: fields.capacity === null ? "" : String(fields.capacity),
    priceInForm:
      fields.priceCents === null ? "" : centsToEuroInput(fields.priceCents),
    sortOrder: String(fields.sortOrder),
    prices: moduleDraftOf(prices),
    contractTemplate,
  };
}

/** The weekday columns of the price table: the offering's days, else Mon–Fri. */
function priceWeekdays(days: readonly CareWeekday[]): CareWeekday[] {
  const chosen = SCHOOL_WEEKDAYS.filter((day) => days.includes(day));
  return chosen.length > 0 ? chosen : [...SCHOOL_WEEKDAYS];
}

/**
 * The draft the backend reads, or the field errors that keep it from being
 * sent. Checks only what the form can see; the schools check the rest when
 * the office looks at the preview.
 */
export function offeringDraftFromForm(form: OfferingFormDraft): {
  draft: TraegerOfferingDraft | null;
  errors: Record<string, string>;
} {
  const errors: Record<string, string> = {};
  const { fields } = form;
  if (!fields.name.trim()) errors.name = "Bitte geben Sie einen Namen ein.";
  if (fields.availableDays.length === 0) {
    errors.available_days = "Bitte wählen Sie mindestens einen Wochentag.";
  }
  let capacity: number | null = null;
  if (form.capacity.trim()) {
    capacity = Number(form.capacity);
    if (!Number.isInteger(capacity) || capacity < 1) {
      errors.capacity = "Bitte eine ganze Zahl ab 1 eingeben.";
    }
  }
  let priceCents: number | null = null;
  if (form.priceInForm.trim()) {
    priceCents = euroInputToCents(form.priceInForm);
    if (priceCents === null) {
      errors.price_cents = "Bitte einen Betrag wie 32,50 eingeben.";
    }
  }
  const sortOrder = Number(form.sortOrder || 0);
  if (!Number.isInteger(sortOrder)) {
    errors.sort_order = "Bitte eine ganze Zahl eingeben.";
  }
  if (fields.countsAsCare) {
    for (const day of fields.availableDays) {
      if (!fields.pickupTimes[day]) {
        errors[`pickup_times.${day}`] = "Bitte eine Gehzeit eintragen.";
      }
    }
  }
  const prices: TraegerCarePrice[] = [];
  const weekdays = priceWeekdays(fields.availableDays);
  for (const column of draftColumns(form.prices.form, weekdays)) {
    const firstText = form.prices.first[column.key]?.trim() ?? "";
    const furtherText = form.prices.further[column.key]?.trim() ?? "";
    const firstKey = priceFieldKey(PRICE_KEY, column.key, false);
    const furtherKey = priceFieldKey(PRICE_KEY, column.key, true);
    if (!firstText) {
      if (furtherText) {
        errors[firstKey] = "Bitte zuerst den Preis für das 1. Kind eintragen.";
      }
      continue;
    }
    const cents = euroInputToCents(firstText);
    if (cents === null) {
      errors[firstKey] = "Bitte einen Betrag wie 32,50 eingeben.";
      continue;
    }
    let siblingCents: number | null = null;
    if (furtherText) {
      siblingCents = euroInputToCents(furtherText);
      if (siblingCents === null) {
        errors[furtherKey] = "Bitte einen Betrag wie 25,00 eingeben.";
        continue;
      }
      if (siblingCents > cents) {
        errors[furtherKey] = "Höchstens so hoch wie beim 1. Kind.";
        continue;
      }
    }
    prices.push({
      weekday:
        form.prices.form === "weekday" ? (column.key as CareWeekday) : null,
      daysPerWeek:
        form.prices.form === "days_per_week" ? Number(column.key) : null,
      monthlyCents: cents,
      siblingCents,
      mealExtra: form.prices.mealExtra,
    });
  }
  if (Object.keys(errors).length > 0) return { draft: null, errors };
  const pickupTimes: Partial<Record<CareWeekday, string>> = {};
  for (const day of fields.availableDays) {
    const time = fields.pickupTimes[day];
    if (time) pickupTimes[day] = time;
  }
  return {
    draft: {
      fields: { ...fields, capacity, priceCents, sortOrder, pickupTimes },
      prices,
      contractTemplate: form.contractTemplate,
    },
    errors,
  };
}

export function CatalogOfferingForm({
  offering,
  saveLabel,
  onSave,
  onSaved,
  onCancel,
}: Readonly<{
  offering?: TraegerOffering;
  saveLabel: string;
  onSave: (draft: TraegerOfferingDraft) => Promise<TraegerOffering>;
  onSaved: (offering: TraegerOffering) => void;
  onCancel: () => void;
}>) {
  const [form, setForm] = useState<OfferingFormDraft>(() =>
    offering ? offeringFormDraftOf(offering) : blankOfferingDraft(),
  );
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { fieldError } = formErrors;
  const { fields } = form;

  const setField = <K extends keyof TraegerOfferingFields>(
    key: K,
    value: TraegerOfferingFields[K],
  ) =>
    setForm((current) => ({
      ...current,
      fields: { ...current.fields, [key]: value },
    }));

  const toggleDay = (day: CareWeekday) => {
    const chosen = fields.availableDays.includes(day);
    const availableDays = SCHOOL_WEEKDAYS.filter((candidate) =>
      candidate === day ? !chosen : fields.availableDays.includes(candidate),
    );
    // Ein abgewählter Tag nimmt seine Gehzeit mit.
    const pickupTimes = { ...fields.pickupTimes };
    if (chosen) delete pickupTimes[day];
    setForm((current) => ({
      ...current,
      fields: { ...current.fields, availableDays, pickupTimes },
    }));
  };

  const setPickupTime = (day: CareWeekday, value: string) => {
    const pickupTimes = { ...fields.pickupTimes };
    if (value) pickupTimes[day] = value;
    else delete pickupTimes[day];
    setField("pickupTimes", pickupTimes);
  };

  const setTerms = <K extends keyof ContractTerms>(
    key: K,
    value: ContractTerms[K],
  ) =>
    setForm((current) => ({
      ...current,
      contractTemplate: {
        ...(current.contractTemplate ?? DEFAULT_TERMS),
        [key]: value,
      },
    }));

  const submit = async () => {
    formErrors.clear();
    const { draft, errors } = offeringDraftFromForm(form);
    if (!draft) {
      formErrors.invalid("Bitte prüfen Sie die markierten Felder.", errors);
      return;
    }
    setSaving(true);
    try {
      onSaved(await onSave(draft));
    } catch (err) {
      logger.error("traeger_catalog_offering_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "das Angebot" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <SectionCard
      title="Betreuungsangebot"
      description="Jede Schule dieser Anmeldephase bekommt das Angebot als eigenes Angebot. Die Schule kann es danach ändern oder ausschalten."
      footer={
        <EditActions
          onCancel={onCancel}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          saveLabel={saveLabel}
        />
      }
    >
      <form
        ref={formRef}
        noValidate
        className="space-y-6"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <FormErrorAlert message={formErrors.error} />
        <Input
          name="name"
          label="Name"
          required
          maxLength={200}
          value={fields.name}
          error={fieldError("name")}
          placeholder="Zum Beispiel: Frühbetreuung"
          onChange={(event) => setField("name", event.target.value)}
        />
        <Textarea
          name="description"
          label="Beschreibung (freiwillig)"
          rows={3}
          maxLength={2000}
          value={fields.description ?? ""}
          error={fieldError("description")}
          onChange={(event) => setField("description", event.target.value)}
        />

        <FormFieldset legend="Wochentage" required>
          <div className="space-y-4">
            <WeekdayToggleGroup
              ariaLabel="Wochentage"
              name="available_days"
              days={DAY_OPTIONS}
              value={fields.availableDays}
              onToggle={toggleDay}
              error={fieldError("available_days")}
            />
            <SegmentedControl
              items={DAYS_MODE_ITEMS}
              value={fields.daysOfWeekMode}
              onChange={(mode: DaysOfWeekMode) =>
                setField("daysOfWeekMode", mode)
              }
              ariaLabel="Wahl der Tage"
            />
            {fields.availableDays.length > 0 ? (
              <div>
                <FieldLabel>
                  {fields.countsAsCare
                    ? "Gehzeit je Betreuungstag"
                    : "Gehzeit je Wochentag (freiwillig)"}
                </FieldLabel>
                <div className="mt-2 grid grid-cols-2 gap-3 sm:grid-cols-5">
                  {fields.availableDays.map((day) => (
                    <Input
                      key={day}
                      name={`pickup_times.${day}`}
                      type="time"
                      controlSize="compact"
                      label={WEEKDAY_SHORT[day]}
                      aria-label={`${WEEKDAY_LONG[day]}, Gehzeit`}
                      required={fields.countsAsCare}
                      value={fields.pickupTimes[day] ?? ""}
                      error={fieldError(`pickup_times.${day}`)}
                      onChange={(event) =>
                        setPickupTime(day, event.target.value)
                      }
                    />
                  ))}
                </div>
              </div>
            ) : null}
          </div>
        </FormFieldset>

        <FormFieldset legend="Art des Angebots">
          <div className="grid gap-2 sm:grid-cols-2">
            <CheckboxCard
              checked={fields.countsAsCare}
              onChange={(checked) => setField("countsAsCare", checked)}
              label="Zählt als Betreuungstag"
              hint="Dann braucht jeder Tag eine Gehzeit."
            />
            <CheckboxCard
              checked={fields.isRequired}
              onChange={(checked) => setField("isRequired", checked)}
              label="Pflichtangebot"
              hint="Eltern müssen es wählen."
            />
            <CheckboxCard
              checked={fields.includesLunch}
              onChange={(checked) => setField("includesLunch", checked)}
              label="Mit Mittagessen"
            />
            <CheckboxCard
              checked={fields.includesHolidayCare}
              onChange={(checked) => setField("includesHolidayCare", checked)}
              label="Mit Ferienbetreuung"
            />
          </div>
        </FormFieldset>

        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            name="capacity"
            label="Plätze (freiwillig)"
            inputMode="numeric"
            value={form.capacity}
            error={fieldError("capacity")}
            hint="Leer heißt: ohne Grenze."
            onChange={(event) =>
              setForm((current) => ({
                ...current,
                capacity: event.target.value,
              }))
            }
          />
          <Input
            name="price_cents"
            label="Preis im Anmeldeformular (freiwillig)"
            inputMode="decimal"
            value={form.priceInForm}
            error={fieldError("price_cents")}
            hint="Nur zur Anzeige für Eltern. Abgerechnet wird nach den Preisen unten."
            onChange={(event) =>
              setForm((current) => ({
                ...current,
                priceInForm: event.target.value,
              }))
            }
          />
          <Input
            name="selection_group"
            label="Auswahlgruppe (freiwillig)"
            maxLength={100}
            value={fields.selectionGroup}
            error={fieldError("selection_group")}
            hint="Angebote mit derselben Gruppe gehören bei der Wahl zusammen."
            onChange={(event) => setField("selectionGroup", event.target.value)}
          />
          <CustomSelect
            id="catalog-offering-rule"
            name="selection_rule"
            label="Auswahlregel"
            value={fields.selectionRule}
            options={RULE_OPTIONS}
            error={fieldError("selection_rule")}
            onChange={(value) =>
              setField("selectionRule", value as CareSelectionRule)
            }
          />
          <Input
            name="sort_order"
            label="Reihenfolge"
            inputMode="numeric"
            value={form.sortOrder}
            error={fieldError("sort_order")}
            hint="Kleinere Zahlen stehen im Anmeldeformular weiter oben."
            onChange={(event) =>
              setForm((current) => ({
                ...current,
                sortOrder: event.target.value,
              }))
            }
          />
        </div>

        <div className="space-y-2">
          <p className="text-sm text-gray-600">
            Ein geänderter Preis gilt in den Schulen ab dem nächsten Monat.
            Ausgestellte Verträge und Rechnungen bleiben, wie sie sind.
          </p>
          <CarePriceModuleEditor
            offeringId={PRICE_KEY}
            moduleName={fields.name.trim() || "Preise"}
            endsAt=""
            bookedWeekdays={[]}
            bookedDaysPerWeek={[]}
            weekdays={priceWeekdays(fields.availableDays)}
            draft={form.prices}
            copySources={[]}
            fieldError={fieldError}
            onChange={(prices) =>
              setForm((current) => ({ ...current, prices }))
            }
            onCopyFrom={() => undefined}
          />
        </div>

        <div className="space-y-4">
          <SwitchRow
            framed
            label="Vertragsvorlage mitgeben"
            description="Die Schulen bekommen diese Vertragsbedingungen für das Angebot."
            checked={form.contractTemplate !== null}
            onCheckedChange={(checked) =>
              setForm((current) => ({
                ...current,
                contractTemplate: checked
                  ? (current.contractTemplate ?? DEFAULT_TERMS)
                  : null,
              }))
            }
          />
          {form.contractTemplate ? (
            <ContractTermsFields
              draft={form.contractTemplate}
              set={setTerms}
              fieldError={fieldError}
            />
          ) : null}
        </div>
      </form>
    </SectionCard>
  );
}

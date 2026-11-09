"use client";

// Eine Regel des Monatsbeitrags anlegen oder ändern (#4685): Name, Monate,
// Schulen, Module und Wirkung. Eine Geschwisterregel nennt dazu, ab dem
// wievielten Kind sie gilt und an welchen Schulen Geschwister zählen.

import { useEffect, useId, useMemo, useRef, useState } from "react";
import { CustomSelect } from "~/components/ui/custom-select";
import { EditActions } from "~/components/ui/edit-actions";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FieldLabel } from "~/components/ui/field-label";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { MultiSelect } from "~/components/ui/multi-select";
import { NumberStepper } from "~/components/ui/number-stepper";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { SwitchRow } from "~/components/ui/switch";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type CareRule,
  type CareRuleInput,
  type CareRuleKind,
  type CareRuleSet,
  saveCareRule,
} from "~/lib/traeger-care-reductions-api";
import { addMonths, formatMonth, monthsFromTo } from "./care-billing-format";
import { CARE_RULE_KIND_LABELS } from "./care-reduction-format";
import { centsToEuroInput, euroInputToCents } from "./holiday-period-model";

const logger = createLogger({ component: "CareRuleModal" });

type Effect = "share" | "amount" | "price" | "tariff";

const EFFECT_LABELS: Record<Effect, string> = {
  share: "Anteil weniger",
  amount: "Betrag weniger",
  price: "Fester Preis",
  tariff: "Preis laut Preisliste",
};

/** The effects a kind may have; a cap has only its amount. */
const KIND_EFFECTS: Record<CareRuleKind, readonly Effect[]> = {
  income_level: ["share", "amount", "price"],
  sibling: ["share", "amount", "price", "tariff"],
  single_parent: ["share", "amount"],
  cap: [],
};

const NAME_PLACEHOLDER: Record<CareRuleKind, string> = {
  income_level: "z. B. Stufe 2: bis 40.000 € im Jahr",
  sibling: "z. B. Geschwister Ispringen",
  single_parent: "z. B. Ermäßigung für Alleinerziehende",
  cap: "z. B. Höchstbetrag je Familie",
};

interface Draft {
  name: string;
  validFrom: string;
  validUntil: string;
  schoolIds: string[];
  moduleIds: string[];
  countSchoolIds: string[];
  fromChild: number;
  countsFamilyChildren: boolean;
  effect: Effect;
  value: string;
}

function effectOf(rule: CareRule): Effect {
  if (rule.tariffSiblingPrice) return "tariff";
  if (rule.priceCents !== null) return "price";
  if (rule.discountCents !== null) return "amount";
  return "share";
}

function valueOf(rule: CareRule): string {
  if (rule.capCents !== null) return centsToEuroInput(rule.capCents);
  if (rule.priceCents !== null) return centsToEuroInput(rule.priceCents);
  if (rule.discountCents !== null) return centsToEuroInput(rule.discountCents);
  if (rule.discountBasisPoints !== null) {
    return String(rule.discountBasisPoints / 100).replace(".", ",");
  }
  return "";
}

function draftOf(
  rule: CareRule | null,
  kind: CareRuleKind,
  currentMonth: string,
  schools: CareRuleSet["schools"],
): Draft {
  if (rule) {
    return {
      name: rule.name,
      validFrom: rule.validFrom,
      validUntil: rule.validUntil ? rule.validUntil.slice(0, 8) + "01" : "",
      schoolIds: [...rule.schoolIds],
      moduleIds: [...rule.careOfferingIds],
      countSchoolIds: [...rule.countSchoolIds],
      fromChild: rule.siblingFromChild ?? 2,
      countsFamilyChildren: rule.countsFamilyChildren,
      effect: effectOf(rule),
      value: valueOf(rule),
    };
  }
  return {
    name: "",
    validFrom: currentMonth,
    validUntil: "",
    schoolIds: schools.length === 1 && schools[0] ? [schools[0].id] : [],
    moduleIds: [],
    countSchoolIds: [],
    fromChild: 2,
    countsFamilyChildren: false,
    effect: kind === "single_parent" ? "amount" : "share",
    value: "",
  };
}

/** The last day of the month that starts on `month`. */
function monthEnd(month: string): string {
  const [year, mon] = month.split("-").map(Number);
  const last = new Date(Date.UTC(year ?? 0, mon ?? 1, 0)).getUTCDate();
  return `${month.slice(0, 8)}${String(last).padStart(2, "0")}`;
}

/** "12,5" → 1250 basis points, or null. */
function percentToBasisPoints(value: string): number | null {
  const match = /^(\d{1,3})(?:[.,](\d{1,2}))?$/.exec(value.trim());
  if (!match) return null;
  const points =
    Number(match[1]) * 100 + Number((match[2] ?? "").padEnd(2, "0"));
  return points >= 1 && points <= 10000 ? points : null;
}

export function CareRuleModal({
  data,
  open,
  currentMonth,
  onClose,
  onSaved,
}: Readonly<{
  data: CareRuleSet;
  /** The kind and the rule to change; null keeps the modal closed. */
  open: { kind: CareRuleKind; rule: CareRule | null } | null;
  currentMonth: string;
  onClose: () => void;
  onSaved: (data: CareRuleSet) => void;
}>) {
  const kind = open?.kind ?? "sibling";
  const [draft, setDraft] = useState<Draft>(() =>
    draftOf(null, kind, currentMonth, data.schools),
  );
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();
  const schoolsLabel = useId();
  const modulesLabel = useId();
  const countLabel = useId();

  useEffect(() => {
    if (!open) return;
    setDraft(draftOf(open.rule, open.kind, currentMonth, data.schools));
    clearErrors();
  }, [open, currentMonth, data.schools, clearErrors]);

  const months = useMemo(
    () =>
      monthsFromTo(addMonths(currentMonth, -12), addMonths(currentMonth, 24)),
    [currentMonth],
  );
  const moduleOptions = useMemo(() => {
    const seen = new Set<string>();
    const options = data.modules
      .filter((module) => draft.schoolIds.includes(module.schoolId))
      .filter((module) => {
        if (seen.has(module.careOfferingId)) return false;
        seen.add(module.careOfferingId);
        return true;
      })
      .map((module) => ({
        value: module.careOfferingId,
        label:
          draft.schoolIds.length > 1
            ? `${module.moduleName} (${module.schoolName})`
            : module.moduleName,
      }));
    for (const id of draft.moduleIds) {
      if (options.some((option) => option.value === id)) continue;
      const offering = data.modules.find((item) => item.careOfferingId === id);
      options.push({
        value: id,
        label: offering
          ? `${offering.moduleName} (${offering.schoolName})`
          : `Nicht mehr angebotenes Modul (${id})`,
      });
    }
    return options;
  }, [data.modules, draft.schoolIds, draft.moduleIds]);
  const schoolOptions = data.schools.map((school) => ({
    value: school.id,
    label: school.name,
  }));
  const effects = KIND_EFFECTS[kind];
  const set = (patch: Partial<Draft>) =>
    setDraft((current) => ({ ...current, ...patch }));

  const submit = async () => {
    if (!open) return;
    formErrors.clear();
    const fieldErrors: Record<string, string> = {};
    if (!draft.name.trim()) fieldErrors.name = "Bitte einen Namen eingeben.";
    if (draft.schoolIds.length === 0) {
      fieldErrors.school_ids = "Bitte mindestens eine Schule wählen.";
    }
    if (draft.validUntil && draft.validUntil < draft.validFrom) {
      fieldErrors.valid_until = "Das Ende liegt vor dem Beginn.";
    }
    let basisPoints: number | null = null;
    let cents: number | null = null;
    if (kind !== "cap" && draft.effect === "tariff") {
      // The prices come from the price list; the rule carries none.
    } else if (kind !== "cap" && draft.effect === "share") {
      basisPoints = percentToBasisPoints(draft.value);
      if (basisPoints === null) {
        fieldErrors.discount_basis_points =
          "Bitte einen Anteil wie 50 oder 12,5 eingeben.";
      }
    } else {
      cents = euroInputToCents(draft.value);
      if (cents === null) {
        fieldErrors[
          kind === "cap"
            ? "cap_cents"
            : draft.effect === "price"
              ? "price_cents"
              : "discount_cents"
        ] = "Bitte einen Betrag wie 25,00 eingeben.";
      }
    }
    if (Object.keys(fieldErrors).length > 0) {
      formErrors.invalid(
        "Bitte prüfen Sie die markierten Felder.",
        fieldErrors,
      );
      return;
    }
    const input: CareRuleInput = {
      kind,
      name: draft.name.trim(),
      validFrom: draft.validFrom,
      validUntil: draft.validUntil ? monthEnd(draft.validUntil) : null,
      schoolIds: draft.schoolIds,
      careOfferingIds: draft.moduleIds,
      countSchoolIds: kind === "sibling" ? draft.countSchoolIds : [],
      siblingFromChild: kind === "sibling" ? draft.fromChild : null,
      countsFamilyChildren: kind === "sibling" && draft.countsFamilyChildren,
      discountBasisPoints: basisPoints,
      discountCents: kind !== "cap" && draft.effect === "amount" ? cents : null,
      priceCents: kind !== "cap" && draft.effect === "price" ? cents : null,
      capCents: kind === "cap" ? cents : null,
      tariffSiblingPrice: kind === "sibling" && draft.effect === "tariff",
    };
    setSaving(true);
    try {
      onSaved(await saveCareRule(open.rule?.id ?? null, input));
      success(`Die Regel „${input.name}“ ist gespeichert.`);
    } catch (err) {
      logger.error("care_rule_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Regel" });
    } finally {
      setSaving(false);
    }
  };

  // The backend names the amount by its parameter; the form shows one field.
  const valueField =
    kind === "cap"
      ? "cap_cents"
      : draft.effect === "share"
        ? "discount_basis_points"
        : draft.effect === "price"
          ? "price_cents"
          : "discount_cents";

  const valueLabel =
    kind === "cap"
      ? "Höchstbetrag je Familie und Monat in €"
      : draft.effect === "share"
        ? "Anteil in %"
        : draft.effect === "price"
          ? "Preis je Monat in €"
          : "Betrag je Monat in €";

  return (
    <FormModal
      isOpen={open !== null}
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title={
        open?.rule
          ? `${CARE_RULE_KIND_LABELS[kind]}: ${open.rule.name}`
          : `Neue Regel: ${CARE_RULE_KIND_LABELS[kind]}`
      }
      size="lg"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          saveLabel="Regel speichern"
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
        <Input
          controlSize="touch"
          id="care-rule-name"
          name="name"
          label="Name auf der Rechnung"
          required
          placeholder={NAME_PLACEHOLDER[kind]}
          value={draft.name}
          maxLength={120}
          error={formErrors.fieldError("name")}
          onChange={(event) => set({ name: event.target.value })}
        />
        <div className="grid gap-3 sm:grid-cols-2">
          <CustomSelect
            id="care-rule-from"
            name="valid_from"
            label="Gilt ab"
            required
            value={draft.validFrom}
            options={months.map((value) => ({
              value,
              label: `1. ${formatMonth(value)}`,
            }))}
            error={formErrors.fieldError("valid_from")}
            onChange={(value) => set({ validFrom: value })}
          />
          <CustomSelect
            id="care-rule-until"
            name="valid_until"
            label="Gilt bis"
            value={draft.validUntil}
            options={[
              { value: "", label: "Ohne Ende" },
              ...months
                .filter((value) => value >= draft.validFrom)
                .map((value) => ({
                  value,
                  label: `Ende ${formatMonth(value)}`,
                })),
            ]}
            error={formErrors.fieldError("valid_until")}
            onChange={(value) => set({ validUntil: value })}
          />
        </div>
        <div>
          <FieldLabel id={schoolsLabel} required>
            Gilt an diesen Schulen
          </FieldLabel>
          <MultiSelect
            ariaLabelledBy={schoolsLabel}
            ariaDescribedBy={`${schoolsLabel}-error`}
            value={draft.schoolIds}
            options={schoolOptions}
            placeholder="Schulen wählen"
            invalid={Boolean(formErrors.fieldError("school_ids"))}
            onChange={(values) => set({ schoolIds: values })}
          />
          <FieldErrorHint
            id={`${schoolsLabel}-error`}
            message={formErrors.fieldError("school_ids")}
          />
        </div>
        <div>
          <FieldLabel id={modulesLabel}>Nur für diese Module</FieldLabel>
          <MultiSelect
            ariaLabelledBy={modulesLabel}
            value={draft.moduleIds}
            options={moduleOptions}
            placeholder="Alle Module"
            disabled={moduleOptions.length === 0}
            onChange={(values) => set({ moduleIds: values })}
          />
          <p className="mt-1 text-sm text-gray-600">
            Leer lassen, dann gilt die Regel für alle Module.
          </p>
        </div>
        {kind === "sibling" ? (
          <div className="grid gap-3 sm:grid-cols-[minmax(0,12rem)_1fr]">
            <div>
              <FieldLabel htmlFor="care-rule-child" required>
                Ab dem Kind
              </FieldLabel>
              <NumberStepper
                id="care-rule-child"
                name="sibling_from_child"
                value={String(draft.fromChild)}
                onChange={(event) => {
                  const next = Number(event.target.value);
                  if (next >= 2 && next <= 9) set({ fromChild: next });
                }}
                onStep={(delta) =>
                  set({
                    fromChild: Math.min(
                      9,
                      Math.max(2, draft.fromChild + delta),
                    ),
                  })
                }
                canDecrement={draft.fromChild > 2}
                canIncrement={draft.fromChild < 9}
                decrementLabel="Ein Kind weniger"
                incrementLabel="Ein Kind mehr"
              />
            </div>
            <div>
              <FieldLabel id={countLabel}>Geschwister zählen an</FieldLabel>
              <MultiSelect
                ariaLabelledBy={countLabel}
                value={draft.countSchoolIds}
                options={schoolOptions}
                placeholder="Schulen der Regel"
                onChange={(values) => set({ countSchoolIds: values })}
              />
              <p className="mt-1 text-sm text-gray-600">
                Leer lassen, dann zählen nur Kinder an den Schulen der Regel.
              </p>
            </div>
          </div>
        ) : null}
        {kind === "sibling" ? (
          <SwitchRow
            label="Geschwister ohne Vertrag mitzählen"
            description="moto zählt die Kinderzahl, die Sie unter „Kinderzahl“ bestätigt haben. Es zählen immer mindestens die Kinder mit Vertrag. Kinder mit Vertrag an einer anderen Schule zählen nur, wenn die Regel dort Geschwister zählt."
            checked={draft.countsFamilyChildren}
            onCheckedChange={(countsFamilyChildren) =>
              set({ countsFamilyChildren })
            }
          />
        ) : null}
        {effects.length > 0 ? (
          <div className="space-y-3">
            <FieldLabel>Wirkung</FieldLabel>
            <SegmentedControl
              ariaLabel="Wirkung der Regel"
              items={effects.map((effect) => ({
                value: effect,
                label: EFFECT_LABELS[effect],
              }))}
              value={draft.effect}
              onChange={(effect) => set({ effect, value: "" })}
            />
          </div>
        ) : null}
        {kind === "sibling" && draft.effect === "tariff" ? (
          <p className="text-sm text-gray-600">
            Jedes weitere Kind zahlt den Preis aus der Spalte „Jedes weitere
            Kind“ der Preisliste, je Modul und Tag. Wo dort nichts steht, zahlt
            es den Preis des 1. Kindes.
          </p>
        ) : (
          <Input
            controlSize="touch"
            id="care-rule-value"
            name={valueField}
            label={valueLabel}
            required
            inputMode="decimal"
            value={draft.value}
            hint={
              kind === "sibling" && draft.effect === "price"
                ? "Das Kind zahlt diesen Preis statt des Grundbeitrags."
                : undefined
            }
            error={formErrors.fieldError(valueField)}
            onChange={(event) => set({ value: event.target.value })}
          />
        )}
      </form>
    </FormModal>
  );
}

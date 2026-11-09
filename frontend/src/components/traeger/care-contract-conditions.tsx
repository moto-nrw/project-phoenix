"use client";

// Vertragsbedingungen je Angebot (#4678): Laufzeit, Kündigungsfrist,
// Sonderkündigung, Satzung und Vertragspartner. Links die Angebote der
// Schulen des Trägers, rechts die Bedingungen des gewählten Angebots. Ein
// neuer Vertrag übernimmt sie; ein bestehender behält seine.

import { SectionLabel } from "~/components/ui/section-label";
import { useMemo, useRef, useState } from "react";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { CustomSelect } from "~/components/ui/custom-select";
import { EditActions } from "~/components/ui/edit-actions";
import { EmptyState } from "~/components/ui/empty-state";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { careContractsConcept } from "~/lib/concepts/careContracts";
import { createLogger } from "~/lib/logger";
import {
  type ContractNoticeTo,
  type ContractTermKind,
  type ContractTerms,
  type OfferingTemplate,
  saveContractTemplate,
} from "~/lib/traeger-care-contracts-api";
import { formatEuro } from "./contribution-format";
import { NOTICE_TO_LABELS } from "./care-contract-format";

const logger = createLogger({ component: "CareContractConditions" });

/** The suggestion for an offering without conditions: the usual OGS contract. */
const DEFAULT_TERMS: ContractTerms = {
  termKind: "school_year",
  termMonths: null,
  noticeMonths: 1,
  noticeTo: "month_end",
  specialTermination: false,
  specialTerminationNote: "",
  statuteRequired: false,
  statuteTitle: "",
  statuteUrl: "",
  contractPartner: "",
};

const TERM_ITEMS: readonly { value: ContractTermKind; label: string }[] = [
  { value: "school_year", label: "Bis Schuljahresende" },
  { value: "open_ended", label: "Unbefristet" },
  { value: "fixed_months", label: "Feste Monate" },
];

const NOTICE_OPTIONS = (
  Object.entries(NOTICE_TO_LABELS) as [ContractNoticeTo, string][]
).map(([value, label]) => ({ value, label }));

function offeringKey(offering: OfferingTemplate): string {
  return `${offering.schoolId}:${offering.offeringId}`;
}

function termsOf(offering: OfferingTemplate | undefined): ContractTerms {
  if (!offering?.template) return DEFAULT_TERMS;
  const { id: _id, updatedAt: _updatedAt, ...terms } = offering.template;
  return terms;
}

export function CareContractConditions({
  offerings,
  focusKey,
  onSaved,
}: Readonly<{
  offerings: readonly OfferingTemplate[];
  /** The offering to open first, such as the first one without conditions. */
  focusKey?: string;
  onSaved: (offering: OfferingTemplate) => void;
}>) {
  const firstOpen = useMemo(
    () =>
      offerings.find((offering) => offering.active && !offering.template) ??
      offerings[0],
    [offerings],
  );
  const [selectedKey, setSelectedKey] = useState<string>(
    focusKey ?? (firstOpen ? offeringKey(firstOpen) : ""),
  );
  const selected = offerings.find(
    (offering) => offeringKey(offering) === selectedKey,
  );
  if (offerings.length === 0) {
    return (
      <SectionCard>
        <EmptyState
          title="Ihre Schulen haben noch kein Betreuungsangebot."
          description="Die Schulen legen ihre Angebote unter „Anmeldung“ an. Danach legen Sie hier die Vertragsbedingungen fest."
        />
      </SectionCard>
    );
  }

  const schools = [
    ...new Set(offerings.map((offering) => offering.schoolName)),
  ];
  return (
    <div className="grid gap-6 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)]">
      <SectionCard
        title="Angebote"
        description="Wählen Sie ein Angebot. Seine Bedingungen gelten für jeden neuen Vertrag."
      >
        <div className="space-y-5">
          {schools.map((school) => (
            <div key={school} className="space-y-2">
              <SectionLabel>{school}</SectionLabel>
              {offerings
                .filter((offering) => offering.schoolName === school)
                .map((offering) => {
                  const key = offeringKey(offering);
                  return (
                    <ChoiceTile
                      key={key}
                      as="button"
                      selected={key === selectedKey}
                      tone="blue"
                      className="flex w-full items-start justify-between gap-3 px-4 py-3 text-left"
                      aria-pressed={key === selectedKey}
                      onClick={() => setSelectedKey(key)}
                    >
                      <span className="min-w-0">
                        <span className="block font-medium text-gray-900">
                          {offering.offeringName}
                        </span>
                        <span className="block text-sm text-gray-600">
                          {[
                            offering.phaseName,
                            offering.priceCents === null
                              ? null
                              : `${formatEuro(offering.priceCents)} im Monat`,
                          ]
                            .filter(Boolean)
                            .join(" · ")}
                        </span>
                        {offering.template ? (
                          <span className="mt-1 block text-sm text-gray-600">
                            {offering.termText} · {offering.noticeText}
                          </span>
                        ) : null}
                      </span>
                      <StatusBadge {...conditionBadge(offering)} />
                    </ChoiceTile>
                  );
                })}
            </div>
          ))}
        </div>
      </SectionCard>

      {selected ? (
        <ConditionsForm
          key={selectedKey}
          offering={selected}
          onSaved={onSaved}
        />
      ) : null}
    </div>
  );
}

function ConditionsForm({
  offering,
  onSaved,
}: Readonly<{
  offering: OfferingTemplate;
  onSaved: (offering: OfferingTemplate) => void;
}>) {
  const selected = offering;
  const [draft, setDraft] = useState<ContractTerms>(() => termsOf(offering));
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  const set = <K extends keyof ContractTerms>(
    key: K,
    value: ContractTerms[K],
  ) => setDraft((current) => ({ ...current, [key]: value }));

  const submit = async () => {
    formErrors.clear();
    setSaving(true);
    try {
      const saved = await saveContractTemplate(
        selected.schoolId,
        selected.offeringId,
        draft,
      );
      onSaved(saved);
      success(
        `Die Vertragsbedingungen für „${saved.offeringName}“ sind gespeichert.`,
      );
    } catch (err) {
      logger.error("care_contract_conditions_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Vertragsbedingungen" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <SectionCard
      concept={careContractsConcept}
      title={selected.offeringName}
      description={`${selected.schoolName}. Gilt für neue Verträge; bestehende behalten ihre Bedingungen.`}
      footer={
        <EditActions
          onCancel={() => {
            setDraft(termsOf(selected));
            clearErrors();
          }}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          saveLabel="Bedingungen speichern"
          cancelLabel="Zurücksetzen"
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
        <fieldset className="space-y-3">
          <legend className="text-sm font-medium text-gray-900">
            Laufzeit
          </legend>
          <CustomSelect
            id="care-contract-term-kind"
            name="term_kind"
            options={TERM_ITEMS}
            value={draft.termKind}
            onChange={(value) => set("termKind", value as ContractTermKind)}
            error={formErrors.fieldError("term_kind")}
            ariaLabel="Laufzeit"
          />
          {draft.termKind === "fixed_months" ? (
            <Input
              controlSize="compact"
              name="term_months"
              label="Anzahl der Monate"
              type="number"
              min={1}
              max={48}
              inputMode="numeric"
              value={draft.termMonths ?? ""}
              error={formErrors.fieldError("term_months")}
              onChange={(event) =>
                set(
                  "termMonths",
                  event.target.value === "" ? null : Number(event.target.value),
                )
              }
            />
          ) : null}
        </fieldset>

        <fieldset className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
          <legend className="mb-3 text-sm font-medium text-gray-900">
            Kündigung
          </legend>
          <Input
            controlSize="compact"
            name="notice_months"
            label="Frist in Monaten"
            type="number"
            min={0}
            max={12}
            inputMode="numeric"
            value={draft.noticeMonths}
            error={formErrors.fieldError("notice_months")}
            onChange={(event) =>
              set("noticeMonths", Number(event.target.value || 0))
            }
          />
          <CustomSelect
            id="care-contract-notice-to"
            name="notice_to"
            label="Kündigung wirkt"
            value={draft.noticeTo}
            options={NOTICE_OPTIONS}
            error={formErrors.fieldError("notice_to")}
            onChange={(value) => set("noticeTo", value as ContractNoticeTo)}
          />
        </fieldset>

        <fieldset className="space-y-3">
          <legend className="text-sm font-medium text-gray-900">
            Sonderkündigung
          </legend>
          <label
            htmlFor="care-contract-special-termination"
            className="flex items-center gap-3 text-sm text-gray-800"
          >
            <Checkbox
              id="care-contract-special-termination"
              checked={draft.specialTermination}
              onChange={() =>
                set("specialTermination", !draft.specialTermination)
              }
            />
            Eltern dürfen in besonderen Fällen früher kündigen
          </label>
          {draft.specialTermination ? (
            <Textarea
              name="special_termination_note"
              label="Wann ist das möglich?"
              required
              rows={3}
              maxLength={1000}
              value={draft.specialTerminationNote}
              error={formErrors.fieldError("special_termination_note")}
              placeholder="Zum Beispiel bei Umzug oder Schulwechsel, zum Ende des Folgemonats."
              onChange={(event) =>
                set("specialTerminationNote", event.target.value)
              }
            />
          ) : null}
        </fieldset>

        <fieldset className="space-y-3">
          <legend className="text-sm font-medium text-gray-900">Satzung</legend>
          <label
            htmlFor="care-contract-statute"
            className="flex items-center gap-3 text-sm text-gray-800"
          >
            <Checkbox
              id="care-contract-statute"
              checked={draft.statuteRequired}
              onChange={() => set("statuteRequired", !draft.statuteRequired)}
            />
            Eltern erkennen mit dem Vertrag eine Satzung an
          </label>
          {draft.statuteRequired ? (
            <div className="grid gap-3 sm:grid-cols-2">
              <Input
                controlSize="compact"
                name="statute_title"
                label="Name der Satzung"
                required
                maxLength={200}
                value={draft.statuteTitle}
                error={formErrors.fieldError("statute_title")}
                placeholder="Satzung über die Elternbeiträge"
                onChange={(event) => set("statuteTitle", event.target.value)}
              />
              <Input
                controlSize="compact"
                name="statute_url"
                label="Link zur Satzung (freiwillig)"
                type="url"
                maxLength={500}
                value={draft.statuteUrl}
                error={formErrors.fieldError("statute_url")}
                placeholder="https://"
                onChange={(event) => set("statuteUrl", event.target.value)}
              />
            </div>
          ) : null}
        </fieldset>

        <Input
          controlSize="compact"
          name="contract_partner"
          label="Vertragspartner"
          maxLength={300}
          value={draft.contractPartner}
          error={formErrors.fieldError("contract_partner")}
          placeholder="Name des Vertragspartners"
          onChange={(event) => set("contractPartner", event.target.value)}
        />
        <p className="text-sm text-gray-600">
          Leer lassen, wenn der Name im Briefkopf steht. Sonst tragen Sie den
          Vertragspartner hier ein. Für „Bei der Annahme“ tragen Sie ihn immer
          ein.
        </p>
        {selected.acceptanceText ? (
          <Textarea
            label="Gespeicherter Text für das Anmeldeformular"
            readOnly
            rows={8}
            value={selected.acceptanceText}
            hint="Für „Bei der Annahme“ ergänzt die Schule eine eigene Zustimmung im Anmeldeformular. Sie kopiert diesen Text unverändert in „Rechtstext / Erklärung“. Sie setzt „Muss bestätigt werden“. Bestehende Rechtstexte bleiben erhalten. Nach Änderungen speichern Sie zuerst die Bedingungen. Ersetzen Sie danach nur diesen Text im Anmeldeformular."
          />
        ) : null}
        {selected.withdrawalText ? (
          <Textarea
            label="Widerrufsbelehrung für das Anmeldeformular"
            readOnly
            rows={6}
            value={selected.withdrawalText}
            hint="Eltern, die online anmelden, haben 14 Tage Widerrufsrecht. Die Schule kopiert diesen Text als eigenen Rechtstext ins Anmeldeformular und setzt „Muss bestätigt werden“. Das Vertrags-PDF enthält ihn immer."
          />
        ) : null}
        {selected.earlyStartText ? (
          <Textarea
            label="Häkchen „Betreuung vor Ende der Widerrufsfrist“"
            readOnly
            rows={4}
            value={selected.earlyStartText}
            hint="Die Schule kopiert diesen Text als weiteren Rechtstext ins Anmeldeformular, ohne „Muss bestätigt werden“. Setzen Eltern das Häkchen, steht es im Vertrag. Beginnt die Betreuung in der Frist, zahlen sie nach einem Widerruf die Tage bis dahin."
          />
        ) : null}
      </form>
    </SectionCard>
  );
}

function conditionBadge(offering: OfferingTemplate): {
  tone: "green" | "gray" | "orange";
  label: string;
} {
  if (offering.template) return { tone: "green", label: "Festgelegt" };
  if (!offering.active) return { tone: "gray", label: "Nicht aktiv" };
  return { tone: "orange", label: "Fehlt" };
}

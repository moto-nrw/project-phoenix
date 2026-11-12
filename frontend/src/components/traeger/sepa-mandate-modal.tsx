"use client";

// Ein SEPA-Mandat erfassen (#4695): Kontoinhaber, IBAN, BIC (nur für Konten
// außerhalb der EU), Datum der Unterschrift, Mandatsreferenz, Art und ob mit
// dem Mandat schon abgebucht wurde. Die IBAN geht einmal an moto und kommt
// nur maskiert zurück.

import { useEffect, useRef, useState } from "react";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { Radio } from "~/components/ui/radio";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EditActions } from "~/components/ui/edit-actions";
import { FieldLabel } from "~/components/ui/field-label";
import { FormModal } from "~/components/ui/form-modal";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { Input } from "~/components/ui/input";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { berlinTodayISO } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type MandateKind,
  type MandateSequence,
  type SepaMandate,
  createSepaMandate,
} from "~/lib/traeger-sepa-api";
import {
  MANDATE_KIND_HINTS,
  MANDATE_KIND_LABELS,
  compactCode,
  ibanLooksValid,
  referenceLooksValid,
} from "./sepa-format";

const logger = createLogger({ component: "SepaMandateModal" });

const KINDS: readonly MandateKind[] = ["recurring", "one_off"];

interface Draft {
  readonly accountHolder: string;
  readonly iban: string;
  readonly bic: string;
  readonly signedOn: string;
  readonly reference: string;
  readonly kind: MandateKind;
  readonly sequence: MandateSequence;
}

function emptyDraft(holder: string): Draft {
  return {
    accountHolder: holder,
    iban: "",
    bic: "",
    signedOn: "",
    reference: "",
    kind: "recurring",
    sequence: "first",
  };
}

/** Groups an IBAN in fours as people read it from a card. */
function groupIBAN(value: string): string {
  return compactCode(value)
    .replace(/(.{4})/g, "$1 ")
    .trim();
}

export function SepaMandateModal({
  open,
  contractId,
  holder,
  onClose,
  onSaved,
}: Readonly<{
  open: boolean;
  contractId: string;
  /** The name the form proposes as account holder. */
  holder: string;
  onClose: () => void;
  onSaved: (mandate: SepaMandate) => void;
}>) {
  const [draft, setDraft] = useState<Draft>(() => emptyDraft(holder));
  const save = useFormModalPending(open);
  const saving = save.pending;
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();
  const today = berlinTodayISO();

  useEffect(() => {
    if (!open) return;
    setDraft(emptyDraft(holder));
    clearErrors();
  }, [open, holder, clearErrors]);

  const set = (patch: Partial<Draft>) =>
    setDraft((current) => ({ ...current, ...patch }));

  const submit = async () => {
    formErrors.clear();
    const fields: Record<string, string> = {};
    if (!draft.accountHolder.trim()) {
      fields.account_holder = "Bitte den Namen auf dem Konto eingeben.";
    }
    if (!ibanLooksValid(draft.iban)) {
      fields.iban =
        "Diese IBAN stimmt nicht. Bitte prüfen Sie jedes Zeichen noch einmal.";
    }
    if (!draft.signedOn) {
      fields.signed_on = "Bitte das Datum der Unterschrift wählen.";
    } else if (draft.signedOn > today) {
      fields.signed_on = "Das Datum darf nicht in der Zukunft liegen.";
    }
    if (!referenceLooksValid(draft.reference)) {
      fields.reference =
        "Bitte nur Buchstaben ohne Umlaute, Ziffern, Leerzeichen und + ? / - : ( ) . , ' verwenden.";
    }
    if (Object.keys(fields).length > 0) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", fields);
      return;
    }
    await save.run(async (op) => {
      try {
        const saved = await createSepaMandate({
          contractId,
          reference: draft.reference,
          signedOn: draft.signedOn,
          kind: draft.kind,
          sequence: draft.sequence,
          accountHolder: draft.accountHolder,
          iban: compactCode(draft.iban),
          bic: compactCode(draft.bic),
        });
        success("Das Mandat ist gespeichert.");
        if (op.isCurrent()) onSaved(saved);
      } catch (err) {
        // The message carries no bank data: the backend names the field only.
        logger.error("sepa_mandate_save_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!op.isCurrent()) return;
        void formErrors.show(err, { object: "das Mandat" });
      }
    });
  };

  return (
    <FormModal
      isOpen={open}
      onClose={onClose}
      pending={save}
      isBackdropDismissDisabled
      title="Mandat erfassen"
      size="md"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
        />
      }
    >
      <form
        ref={formRef}
        noValidate
        className="space-y-5"
        data-analytics-block
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <p className="text-sm text-gray-600">
          Übernehmen Sie die Angaben aus dem unterschriebenen Mandat.
        </p>
        <Input
          controlSize="touch"
          id="sepa-mandate-holder"
          name="account_holder"
          label="Kontoinhaber"
          required
          maxLength={70}
          autoComplete="off"
          value={draft.accountHolder}
          error={formErrors.fieldError("account_holder") ?? undefined}
          onChange={(event) => set({ accountHolder: event.target.value })}
        />
        <div className="grid gap-4 sm:grid-cols-[2fr_1fr]">
          <Input
            controlSize="touch"
            id="sepa-mandate-iban"
            name="iban"
            label="IBAN"
            required
            autoComplete="off"
            spellCheck={false}
            inputMode="text"
            maxLength={42}
            placeholder="DE00 0000 0000 0000 0000 00"
            className="font-mono"
            value={draft.iban}
            error={formErrors.fieldError("iban") ?? undefined}
            onChange={(event) => set({ iban: groupIBAN(event.target.value) })}
          />
          <Input
            controlSize="touch"
            id="sepa-mandate-bic"
            name="bic"
            label="BIC"
            autoComplete="off"
            spellCheck={false}
            maxLength={11}
            className="font-mono"
            hint="Nur bei Konten außerhalb der EU."
            value={draft.bic}
            error={formErrors.fieldError("bic") ?? undefined}
            onChange={(event) => set({ bic: event.target.value.toUpperCase() })}
          />
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          <ISODatePicker
            id="sepa-mandate-signed"
            label="Unterschrieben am"
            required
            value={draft.signedOn}
            max={today}
            error={formErrors.fieldError("signed_on")}
            onChange={(value) => set({ signedOn: value })}
          />
          <Input
            controlSize="touch"
            id="sepa-mandate-reference"
            name="reference"
            label="Mandatsreferenz"
            autoComplete="off"
            maxLength={35}
            hint="Leer lassen: moto vergibt eine."
            value={draft.reference}
            error={formErrors.fieldError("reference") ?? undefined}
            onChange={(event) => set({ reference: event.target.value })}
          />
        </div>
        <div className="space-y-2">
          <FieldLabel id="sepa-mandate-kind-label">Art</FieldLabel>
          <div
            role="radiogroup"
            aria-labelledby="sepa-mandate-kind-label"
            className="grid gap-2 sm:grid-cols-2"
          >
            {KINDS.map((kind) => (
              <ChoiceTile
                key={kind}
                selected={draft.kind === kind}
                className="items-start gap-3 px-4 py-3"
              >
                <Radio
                  name="sepa-mandate-kind"
                  value={kind}
                  checked={draft.kind === kind}
                  onChange={() =>
                    set(
                      kind === "one_off"
                        ? { kind, sequence: "first" }
                        : { kind },
                    )
                  }
                  className="mt-0.5"
                />
                <span className="flex min-w-0 flex-col gap-0.5">
                  <span className="font-medium text-gray-900">
                    {MANDATE_KIND_LABELS[kind]}
                  </span>
                  <span className="text-sm font-normal text-gray-600">
                    {MANDATE_KIND_HINTS[kind]}
                  </span>
                </span>
              </ChoiceTile>
            ))}
          </div>
        </div>
        {draft.kind === "recurring" ? (
          <div className="space-y-2">
            <FieldLabel>Wurde mit diesem Mandat schon abgebucht?</FieldLabel>
            <SegmentedControl
              ariaLabel="Wurde mit diesem Mandat schon abgebucht?"
              items={[
                { value: "first", label: "Nein, noch nie" },
                { value: "recurring", label: "Ja, schon einmal" },
              ]}
              value={draft.sequence}
              onChange={(value) => set({ sequence: value })}
            />
            <p className="text-sm text-gray-600">
              Bei einem Mandat aus einem anderen Programm meistens „Ja“.
            </p>
          </div>
        ) : null}
      </form>
    </FormModal>
  );
}

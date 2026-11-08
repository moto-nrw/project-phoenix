"use client";

// Einen Betrag im Abrechnungslauf korrigieren (#4668): neuer Betrag eines
// Kindes und Pflicht-Begründung. Frühere Korrekturen bleiben im Verlauf.
// Dasselbe Formular stellt nach der Freigabe Storno und Gutschrift aus
// (#4690): beides ist ein eigenes Dokument mit eigener Nummer und
// Pflicht-Begründung, die Rechnung selbst ändert sich nie. Der Storno nimmt
// die ganze Rechnung zurück und hat kein Betragsfeld, die Gutschrift einen
// Betrag für ein Kind, höchstens was für das Kind noch offen ist. In der
// Vorschau ändert es auch eine aus der stornierten Rechnung übernommene
// Korrektur (#5663): Die eigene Korrektur tritt an ihre Stelle.

import { useEffect, useRef, useState } from "react";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type BillingChild,
  type BillingDocument,
  type BillingCarriedCorrection,
  type BillingRunDetail,
  cancelBillingDocument,
  changeCarriedCorrection,
  correctBillingRun,
  creditBillingDocument,
} from "~/lib/traeger-billing-api";
import {
  type CareLetterMailing,
  type CareLetters,
  cancelCareInvoice,
} from "~/lib/traeger-care-billing-api";
import { totalAfterRemovingCarriedCorrection } from "./billing-carried-correction-model";
import { formatEuro } from "./contribution-format";
import { centsToEuroInput, euroInputToCents } from "./holiday-period-model";

const logger = createLogger({ component: "BillingCorrectionModal" });

/**
 * What the Abrechnung changes: the amount of a child in a preview, or,
 * once released, the Storno of an invoice or a Gutschrift for one child.
 */
export type BillingCorrectionTarget =
  | {
      readonly kind?: "correction";
      readonly registrationId: string;
      readonly child: BillingChild;
    }
  | { readonly kind: "care-cancellation"; readonly invoice: CareLetterMailing }
  | { readonly kind: "cancellation"; readonly document: BillingDocument }
  | {
      readonly kind: "credit";
      readonly document: BillingDocument;
      readonly child: BillingChild;
      /** What the invoice and its earlier Gutschriften leave of the child. */
      readonly leftCents: number;
    }
  | {
      readonly kind: "carried";
      readonly child: BillingChild;
      readonly carried: BillingCarriedCorrection;
    };

/** The texts and the amount field of each mode. */
function modeOf(target: BillingCorrectionTarget): {
  title: string;
  saveLabel: string;
  object: string;
  intro: string;
  effect?: string;
  nextStep?: string;
  amount: {
    id: string;
    name: string;
    label: string;
    initial: string;
    placeholder: string | undefined;
  } | null;
  reasonInitial?: string;
  reasonPlaceholder: string;
} {
  switch (target.kind) {
    case "care-cancellation":
      return {
        title: `Rechnung ${target.invoice.number} stornieren?`,
        saveLabel: "Stornieren",
        object: "der Storno",
        intro: `Der Storno nimmt ${formatEuro(target.invoice.totalCents)} für ${target.invoice.childFirstName} ${target.invoice.childLastName} zurück. Er bekommt eine eigene Nummer.`,
        effect:
          target.invoice.sends > 0
            ? "Die Familie bekommt ein Storno-Schreiben. Senden Sie es danach."
            : target.invoice.printed
              ? "Die Rechnung wurde zum Drucken heruntergeladen. Geben Sie der Familie das Storno-Schreiben auf Papier."
              : "Die Familie hat die Rechnung noch nicht bekommen. Sie bekommt auch kein Storno.",
        nextStep:
          "Die neue Rechnung stellt ein neuer Monatslauf für denselben Monat aus.",
        amount: null,
        reasonPlaceholder:
          "Zum Beispiel: Falscher Empfänger. Neue Rechnung folgt.",
      };
    case "cancellation":
      return {
        title: `Rechnung ${target.document.number} stornieren?`,
        saveLabel: "Stornieren",
        object: "der Storno",
        intro: `Der Storno nimmt ${formatEuro(target.document.totalCents)} zurück. Er bekommt eine eigene Nummer und ein eigenes Schreiben. Danach kommt die Familie in den nächsten Lauf und bekommt dort eine neue Rechnung.`,
        amount: null,
        reasonPlaceholder:
          "Zum Beispiel: Anschrift war falsch. Neue Rechnung folgt.",
      };
    case "credit":
      return {
        title: `Gutschrift für ${target.child.firstName}`,
        saveLabel: "Gutschreiben",
        object: "die Gutschrift",
        intro: `Noch offen für ${target.child.firstName}: ${formatEuro(target.leftCents)}. Die Gutschrift bekommt eine eigene Nummer und ein eigenes Schreiben.`,
        amount: {
          id: "billing-credit-amount",
          name: "amount_cents",
          label: "Betrag in Euro",
          initial: "",
          placeholder: centsToEuroInput(Math.min(500, target.leftCents)),
        },
        reasonPlaceholder: "Zum Beispiel: Am ersten Ferientag krank.",
      };
    case "carried":
      return {
        title: `Korrektur für ${target.child.firstName} ändern`,
        saveLabel: "Ändern",
        object: "die Korrektur",
        intro: `Jetzt: ${formatEuro(target.child.totalCents)}, mit ${formatEuro(target.carried.amountCents)} aus ${target.carried.sourceNumber}. Ihre Änderung ersetzt diese Korrektur.`,
        amount: {
          id: "billing-correction-amount",
          name: "total_cents",
          label: "Neuer Betrag in Euro",
          initial: centsToEuroInput(target.child.totalCents),
          placeholder: undefined,
        },
        reasonInitial: target.carried.reason,
        reasonPlaceholder:
          "Zum Beispiel: Am ersten Ferientag krank, Tag wird erstattet.",
      };
    default:
      return {
        title: `Betrag für ${target.child.firstName} ${target.child.lastName} korrigieren`,
        saveLabel: "Korrigieren",
        object: "die Korrektur",
        intro: `Jetzt: ${formatEuro(target.child.totalCents)}. Die Korrektur steht als eigene Position auf der Rechnung.`,
        amount: {
          id: "billing-correction-amount",
          name: "total_cents",
          label: "Neuer Betrag in Euro",
          initial: centsToEuroInput(target.child.totalCents),
          placeholder: undefined,
        },
        reasonPlaceholder:
          "Zum Beispiel: Am ersten Ferientag krank, Tag wird erstattet.",
      };
  }
}

/** The field error of the amount, or null when it is valid. */
function amountError(
  target: BillingCorrectionTarget,
  cents: number | null,
): string | null {
  if (target.kind === "cancellation" || target.kind === "care-cancellation")
    return null;
  if (target.kind === "credit") {
    if (cents === null || cents <= 0)
      return "Bitte geben Sie einen Betrag wie 5,00 an.";
    return cents > target.leftCents
      ? `Höchstens ${formatEuro(target.leftCents)}.`
      : null;
  }
  if (cents === null) return "Bitte geben Sie einen Betrag wie 45,00 an.";
  if (target.kind === "carried") {
    return cents ===
      totalAfterRemovingCarriedCorrection(
        target.child,
        target.carried.positionId,
      )
      ? "Das ist der Betrag ohne Korrektur. Wählen Sie dafür im Menü „Entfernen“."
      : null;
  }
  return cents === target.child.totalCents
    ? "Der Betrag ist unverändert."
    : null;
}

export function BillingCorrectionModal({
  runId,
  target,
  onClose,
  onCorrected,
  onIssued,
  onCareCancelled,
}: Readonly<{
  runId: string;
  target: BillingCorrectionTarget | null;
  onClose: () => void;
  /** After a correction of the preview, with the run it returns. */
  onCorrected?: (run: BillingRunDetail) => void;
  /** After a Storno or a Gutschrift, with the document it issued. */
  onIssued?: (issued: BillingDocument) => void;
  onCareCancelled?: (letters: CareLetters) => void;
}>) {
  const [amount, setAmount] = useState("");
  const [reason, setReason] = useState("");
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  useEffect(() => {
    if (!target) return;
    const mode = modeOf(target);
    setAmount(mode.amount?.initial ?? "");
    setReason(mode.reasonInitial ?? "");
    clearErrors();
  }, [target, clearErrors]);

  const save = async (
    current: BillingCorrectionTarget,
    cents: number | null,
  ) => {
    if (current.kind === "care-cancellation") {
      const letters = await cancelCareInvoice(
        runId,
        current.invoice.invoiceId,
        reason.trim(),
      );
      const storno = letters.documents.find(
        (document) =>
          document.kind === "cancellation" &&
          document.invoiceId === current.invoice.invoiceId,
      );
      const issued = storno
        ? `Storno ${storno.number} ist ausgestellt.`
        : "Der Storno ist ausgestellt.";
      success(
        storno?.hasLetter
          ? current.invoice.sends > 0
            ? `${issued} Senden Sie das Storno-Schreiben an die Familie.`
            : `${issued} Geben Sie der Familie das Storno-Schreiben auf Papier.`
          : `${issued} Die Familie bekommt kein Schreiben.`,
      );
      onCareCancelled?.(letters);
    } else if (current.kind === "cancellation") {
      const issued = await cancelBillingDocument(
        runId,
        current.document.id,
        reason.trim(),
      );
      success(`Storno ${issued.number} ist ausgestellt.`);
      onIssued?.(issued);
    } else if (current.kind === "credit") {
      const issued = await creditBillingDocument(runId, current.document.id, {
        childId: current.child.childId,
        amountCents: cents ?? 0,
        reason: reason.trim(),
      });
      success(`Gutschrift ${issued.number} ist ausgestellt.`);
      onIssued?.(issued);
    } else if (current.kind === "carried") {
      const run = await changeCarriedCorrection(
        runId,
        current.carried.positionId,
        { totalCents: cents ?? 0, reason: reason.trim() },
      );
      success("Die Korrektur ist geändert.");
      onCorrected?.(run);
    } else {
      const run = await correctBillingRun(runId, {
        registrationId: current.registrationId,
        childId: current.child.childId,
        totalCents: cents ?? 0,
        reason: reason.trim(),
      });
      success("Der Betrag ist korrigiert.");
      onCorrected?.(run);
    }
  };

  const submit = async () => {
    if (!target) return;
    formErrors.clear();
    const mode = modeOf(target);
    const cents = mode.amount ? euroInputToCents(amount) : null;
    const fields: Record<string, string> = {};
    const invalidAmount = amountError(target, cents);
    if (mode.amount && invalidAmount) fields[mode.amount.name] = invalidAmount;
    if (!reason.trim()) fields.reason = "Bitte geben Sie eine Begründung an.";
    if (Object.keys(fields).length > 0) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", fields);
      return;
    }
    setSaving(true);
    try {
      await save(target, cents);
    } catch (err) {
      logger.error("billing_correction_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: mode.object });
    } finally {
      setSaving(false);
    }
  };

  const mode = target ? modeOf(target) : null;
  return (
    <FormModal
      isOpen={target !== null}
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title={mode?.title ?? ""}
      size="md"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          saveLabel={mode?.saveLabel ?? "Korrigieren"}
          savingLabel={
            target?.kind === "care-cancellation"
              ? "Wird storniert …"
              : undefined
          }
        />
      }
    >
      {mode ? (
        <form
          ref={formRef}
          noValidate
          className="space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <p className="text-sm text-gray-700">{mode.intro}</p>
          {mode.effect ? (
            <p className="text-sm font-medium text-gray-900">{mode.effect}</p>
          ) : null}
          {mode.nextStep ? (
            <p className="text-sm text-gray-700">{mode.nextStep}</p>
          ) : null}
          {mode.amount ? (
            <Input
              controlSize="compact"
              id={mode.amount.id}
              name={mode.amount.name}
              label={mode.amount.label}
              inputMode="decimal"
              placeholder={mode.amount.placeholder}
              value={amount}
              error={formErrors.fieldError(mode.amount.name) ?? undefined}
              onChange={(event) => setAmount(event.target.value)}
            />
          ) : null}
          <Textarea
            id="billing-correction-reason"
            name="reason"
            label="Begründung"
            required={target?.kind === "care-cancellation"}
            placeholder={mode.reasonPlaceholder}
            value={reason}
            maxLength={500}
            error={formErrors.fieldError("reason") ?? undefined}
            onChange={(event) => setReason(event.target.value)}
          />
        </form>
      ) : null}
    </FormModal>
  );
}

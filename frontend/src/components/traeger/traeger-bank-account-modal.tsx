"use client";

import { useEffect, useRef, useState } from "react";
import { Checkbox } from "~/components/ui/checkbox";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  createTraegerBankAccount,
  type TraegerBankAccount,
  type TraegerBankAccountInput,
  updateTraegerBankAccount,
} from "~/lib/traeger-api";
import { formatIBAN } from "./traeger-bank-account-format";

const logger = createLogger({ component: "TraegerBankAccountModal" });

const EMPTY: TraegerBankAccountInput = {
  label: "",
  accountHolder: "",
  iban: "",
  bic: "",
  bankName: "",
  isDefault: false,
};

/** Which bank account the modal writes: a new one or an existing one. */
export type BankAccountEdit =
  | { readonly kind: "create"; readonly first: boolean }
  | { readonly kind: "change"; readonly account: TraegerBankAccount };

/**
 * Adds or changes a bank account of the Träger (#4653). The backend checks
 * the IBAN with its check digits and keeps exactly one default.
 */
export function TraegerBankAccountModal({
  edit,
  onClose,
  onSaved,
}: Readonly<{
  edit: BankAccountEdit | null;
  onClose: () => void;
  onSaved: () => Promise<unknown>;
}>) {
  const [draft, setDraft] = useState<TraegerBankAccountInput>(EMPTY);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  useEffect(() => {
    if (!edit) return;
    setDraft(
      edit.kind === "change"
        ? { ...edit.account, iban: formatIBAN(edit.account.iban) }
        : EMPTY,
    );
    clearErrors();
  }, [edit, clearErrors]);

  const set = (field: keyof TraegerBankAccountInput) => (value: string) =>
    setDraft((current) => ({ ...current, [field]: value }));

  const submit = async () => {
    if (!edit) return;
    formErrors.clear();
    const fields: Record<string, string> = {};
    if (!draft.label.trim())
      fields.label = "Bitte geben Sie eine Bezeichnung an.";
    if (!draft.accountHolder.trim())
      fields.account_holder = "Bitte geben Sie den Kontoinhaber an.";
    if (!draft.iban.trim()) fields.iban = "Bitte geben Sie die IBAN an.";
    if (Object.keys(fields).length > 0) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", fields);
      return;
    }
    setSaving(true);
    try {
      if (edit.kind === "create") {
        await createTraegerBankAccount(draft);
        success(`Die Bankverbindung „${draft.label.trim()}“ ist gespeichert.`);
      } else {
        await updateTraegerBankAccount(edit.account.id, draft);
        success(`Die Bankverbindung „${draft.label.trim()}“ ist geändert.`);
      }
      onClose();
      await onSaved();
    } catch (err) {
      logger.error("traeger_bank_account_save_failed", {
        kind: edit.kind,
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Bankverbindung" });
    } finally {
      setSaving(false);
    }
  };

  const isDefault = edit?.kind === "change" && edit.account.isDefault;
  const isFirst = edit?.kind === "create" && edit.first;

  return (
    <FormModal
      isOpen={edit !== null}
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title={
        edit?.kind === "change"
          ? "Bankverbindung ändern"
          : "Bankverbindung hinzufügen"
      }
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
        id="traeger-bank-account-form"
        noValidate
        className="space-y-4"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <Input
          id="traeger-bank-account-label"
          label="Bezeichnung"
          name="label"
          placeholder="zum Beispiel Elternbeiträge"
          value={draft.label}
          onChange={(e) => set("label")(e.target.value)}
          error={formErrors.fieldError("label")}
          maxLength={80}
          required
        />
        <Input
          id="traeger-bank-account-holder"
          label="Kontoinhaber"
          name="account_holder"
          value={draft.accountHolder}
          onChange={(e) => set("accountHolder")(e.target.value)}
          error={formErrors.fieldError("account_holder")}
          maxLength={140}
          required
        />
        <Input
          id="traeger-bank-account-iban"
          label="IBAN"
          name="iban"
          autoComplete="off"
          spellCheck={false}
          value={draft.iban}
          onChange={(e) => set("iban")(e.target.value)}
          error={formErrors.fieldError("iban")}
          maxLength={42}
          required
        />
        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            id="traeger-bank-account-bic"
            label="BIC (freiwillig)"
            name="bic"
            autoComplete="off"
            spellCheck={false}
            value={draft.bic}
            onChange={(e) => set("bic")(e.target.value)}
            error={formErrors.fieldError("bic")}
            maxLength={14}
          />
          <Input
            id="traeger-bank-account-bank"
            label="Bank (freiwillig)"
            name="bank_name"
            value={draft.bankName}
            onChange={(e) => set("bankName")(e.target.value)}
            error={formErrors.fieldError("bank_name")}
            maxLength={120}
          />
        </div>
        {isDefault || isFirst ? (
          <p className="text-sm text-gray-600">
            {isFirst
              ? "Die erste Bankverbindung ist der Standard. Sie steht auf allen Briefen und Rechnungen."
              : "Das ist der Standard. Wählen Sie eine andere Bankverbindung als Standard, um das zu ändern."}
          </p>
        ) : (
          <label
            htmlFor="traeger-bank-account-default"
            className="flex items-start gap-3 text-sm text-gray-700"
          >
            <Checkbox
              id="traeger-bank-account-default"
              name="is_default"
              checked={draft.isDefault}
              onChange={(e) =>
                setDraft((current) => ({
                  ...current,
                  isDefault: e.target.checked,
                }))
              }
            />
            <span>
              Als Standard verwenden. Diese Bankverbindung steht dann auf allen
              Briefen und Rechnungen.
            </span>
          </label>
        )}
      </form>
    </FormModal>
  );
}

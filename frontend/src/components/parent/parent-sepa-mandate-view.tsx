"use client";

// Lastschrift-Mandat in der Eltern-App (#4696). Eltern erteilen das Mandat
// ihrer Familie in zwei Schritten: Konto angeben, dann den Text lesen und
// als Kontoinhaber bestätigen. Bestätigt wird genau der Text, den das
// Backend erzeugt hat (Prüfsumme). Ein gültiges Mandat zeigt das Konto nur
// maskiert, den bestätigten Text zum Speichern und den Widerruf. Ohne
// Schalter des Trägers bleibt nur der Hinweis auf das Papierformular.

import { FileText } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { useCallback, useEffect, useRef, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ConsentCheckbox } from "~/components/ui/consent-checkbox";
import {
  DataField,
  DataGrid,
  InfoText,
} from "~/components/ui/detail-modal-components";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { FormFieldset } from "~/components/ui/form-fieldset";
import { InsetPanel } from "~/components/ui/inset-panel";
import { Input } from "~/components/ui/input";
import { ConfirmationModal } from "~/components/ui/modal";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { ApiError, wireErrorCode } from "~/lib/api-error";
import { billingConcept } from "~/lib/concepts/billing";
import { formatDate } from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import { useRefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { createLogger } from "~/lib/logger";
import {
  fetchSepaMandate,
  grantSepaMandate,
  previewSepaMandate,
  revokeSepaMandate,
  type SepaMandate,
  type SepaMandateInput,
  type SepaMandateText,
  type SepaMandateView,
} from "~/lib/parent-sepa-mandate-api";
import { compactCode, ibanLooksValid } from "~/lib/sepa-checks";

const logger = createLogger({ component: "ParentSepaMandateView" });

const TEXT_CHANGED = "parent_billing.sepa_mandate_text_changed";

interface ContractKey {
  readonly organizationId: string;
  readonly contractId: string;
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

/** Groups an IBAN the way a bank card prints it. */
function groupIban(value: string): string {
  return compactCode(value)
    .replace(/(.{4})/g, "$1 ")
    .trim();
}

export function ParentSepaMandateView({
  organizationId,
  contractId,
}: Readonly<ContractKey>) {
  const t = useTranslations("parentSepaMandate");
  const [view, setView] = useState<SepaMandateView | null>(null);
  const loadError = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = loadError;

  // Rejects on failure: after the revocation the card shows it as a reload
  // of a saved change; only an access refusal discards the mandate (#6652).
  const fetchView = useCallback(async () => {
    try {
      setView(await fetchSepaMandate(organizationId, contractId));
      clearLoadError();
    } catch (error) {
      if (
        error instanceof ApiError &&
        (error.status === 401 ||
          error.status === 403 ||
          (error.status === 404 &&
            error.code === "parent_billing.parent_document_not_found"))
      ) {
        setView(null);
      }
      throw error;
    }
  }, [organizationId, contractId, clearLoadError]);
  const load = useCallback(async () => {
    try {
      await fetchView();
    } catch (err) {
      logger.error("parent_sepa_mandate_load_failed", {
        error: errorMessage(err),
      });
      await showLoadError(err, {
        object: t("errorObject"),
        retry: () => void load(),
      });
    }
  }, [fetchView, showLoadError, t]);

  useEffect(() => {
    void load();
  }, [load]);
  const afterRevoke = useRefreshAfterWrite(t("reloadObject"), fetchView, {
    savedSuffix: t("savedButStale"),
  });

  // Bei vorübergehenden Ladefehlern bleibt das bestätigte Mandat stehen (#6652).
  if ((loadError.error ?? afterRevoke.error) && !view) {
    return (
      <SectionCard concept={billingConcept} title={t("title")}>
        <LoadErrorAlert error={loadError.error ?? afterRevoke.error} />
      </SectionCard>
    );
  }
  if (!view) {
    return <SectionCard concept={billingConcept} title={t("title")} loading />;
  }
  if (!view.enabled) {
    return (
      <SectionCard
        concept={billingConcept}
        title={t("offTitle")}
        description={t("intro")}
      >
        <LoadErrorAlert error={loadError.error} />
        <LoadErrorAlert error={afterRevoke.error} />
        <Alert type="info" message={t("off")} />
      </SectionCard>
    );
  }
  const key = { organizationId, contractId };
  if (view.mandate && !view.mandate.revokedOn) {
    return (
      <div className="space-y-6">
        <LoadErrorAlert error={loadError.error} />
        <LoadErrorAlert error={afterRevoke.error} />
        <MandateCard
          contract={key}
          view={view}
          mandate={view.mandate}
          onRevoked={async (confirmed) => {
            setView(confirmed);
            await afterRevoke.refresh();
          }}
        />
        {view.mandate.proof ? <MandateTextCard mandate={view.mandate} /> : null}
      </div>
    );
  }
  if (view.creditorMissing || !view.canManage) {
    return (
      <SectionCard
        concept={billingConcept}
        title={t("title")}
        description={t("intro")}
      >
        <LoadErrorAlert error={loadError.error} />
        <LoadErrorAlert error={afterRevoke.error} />
        {view.creditorMissing ? (
          <Alert type="warning" message={t("creditorMissing")} />
        ) : (
          <p className="text-sm text-gray-600">{t("noRight")}</p>
        )}
      </SectionCard>
    );
  }
  return (
    <>
      <LoadErrorAlert error={loadError.error} />
      <LoadErrorAlert error={afterRevoke.error} />
      <GrantFlow contract={key} view={view} onGranted={setView} />
    </>
  );
}

/** The mandate in force: the account as a mask, who collects, and the revocation. */
function MandateCard({
  contract,
  view,
  mandate,
  onRevoked,
}: Readonly<{
  contract: ContractKey;
  view: SepaMandateView;
  mandate: SepaMandate;
  /** Applies the confirmed revocation before reloading (#6652). */
  onRevoked: (confirmed: SepaMandateView) => Promise<void>;
}>) {
  const t = useTranslations("parentSepaMandate");
  const locale = useLocale();
  const [confirming, setConfirming] = useState(false);
  const [revoking, setRevoking] = useState(false);
  const formRef = useRef<HTMLDivElement>(null);
  const revokeError = useApiFormError(formRef);
  const { success } = useToast();
  const date = (value: string) => formatDate(value, false, locale);

  const given =
    mandate.source === "parent" && mandate.proof
      ? t("givenInApp", { date: date(mandate.proof.confirmedAt) })
      : mandate.source === "import"
        ? t("givenImport")
        : t("givenOffice");

  const revoke = async () => {
    revokeError.clear();
    setRevoking(true);
    let confirmed: SepaMandateView;
    try {
      confirmed = await revokeSepaMandate(
        contract.organizationId,
        contract.contractId,
      );
    } catch (err) {
      logger.error("parent_sepa_mandate_revoke_failed", {
        error: errorMessage(err),
      });
      setConfirming(false);
      void revokeError.show(err, { object: t("errorObject") });
      setRevoking(false);
      return;
    }
    setRevoking(false);
    setConfirming(false);
    success(t("revoked"));
    await onRevoked(confirmed);
  };

  return (
    <SectionCard
      concept={billingConcept}
      title={t("activeTitle")}
      titleBadge={
        <StatusBadge
          tone={mandate.collectionEligible ? "green" : "warning"}
          label={
            mandate.collectionEligible
              ? t("activeBadge", { date: date(mandate.signedOn) })
              : t("replacementBadge")
          }
        />
      }
      description={t(
        mandate.collectionEligible
          ? "activeDescription"
          : "replacementDescription",
      )}
    >
      <div ref={formRef} className="space-y-5">
        <FormErrorAlert message={revokeError.error} />
        <DataGrid>
          <DataField label={t("holder")}>{mandate.accountHolder}</DataField>
          <DataField label={t("iban")}>
            <span className="font-mono tracking-wide">
              {mandate.ibanMasked}
            </span>
          </DataField>
          {mandate.bic ? (
            <DataField label={t("bic")} mono>
              {mandate.bic}
            </DataField>
          ) : null}
          <DataField label={t("reference")} mono>
            {mandate.reference}
          </DataField>
          <DataField label={t("creditor")}>
            {view.creditor?.name ?? "–"}
            {view.creditor?.address ? (
              <span className="mt-0.5 block text-sm font-normal text-gray-600">
                {view.creditor.address}
              </span>
            ) : null}
          </DataField>
          <DataField label={t("creditorId")} mono>
            {mandate.creditorId}
          </DataField>
          <DataField label={t("givenHow")} fullWidth>
            {given}
          </DataField>
        </DataGrid>
        <p className="text-sm text-gray-600">{t("ibanHint")}</p>
        {view.canManage ? (
          <div className="flex flex-col gap-3 border-t border-gray-100 pt-4 sm:flex-row sm:items-center sm:justify-between">
            <p className="text-sm text-gray-600">{t("replaceHint")}</p>
            <Button
              type="button"
              variant="outline_danger"
              size="md"
              className="shrink-0"
              onClick={() => setConfirming(true)}
            >
              {t("revoke")}
            </Button>
          </div>
        ) : null}
      </div>
      <ConfirmationModal
        isOpen={confirming}
        onClose={() => setConfirming(false)}
        onConfirm={() => void revoke()}
        title={t("revokeTitle")}
        confirmText={t("revokeConfirm")}
        cancelText={t("cancel")}
        confirmVariant="danger"
        isConfirmLoading={revoking}
        isDismissDisabled={revoking}
        loadingText={t("revoking")}
        closeLabel={t("close")}
        mobileSheet
      >
        <div className="space-y-2 text-sm text-gray-700">
          <p>{t("revokeBody")}</p>
          <p>{t("revokeRunning")}</p>
          <p>{t("revokePay")}</p>
        </div>
      </ConfirmationModal>
    </SectionCard>
  );
}

/** The text the parents confirmed, to keep (§ 126b BGB: a durable copy). */
function MandateTextCard({ mandate }: Readonly<{ mandate: SepaMandate }>) {
  const t = useTranslations("parentSepaMandate");
  const proof = mandate.proof;
  if (!proof) return null;
  return (
    <SectionCard
      icon={FileText}
      title={t("textTitle")}
      description={t("textDescription")}
      action={
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={() =>
            downloadBlob(
              new Blob([proof.text], { type: "text/plain;charset=utf-8" }),
              t("downloadName", { reference: mandate.reference }),
            )
          }
        >
          {t("download")}
        </Button>
      }
    >
      <InsetPanel framed padding="md">
        <InfoText>{proof.text}</InfoText>
      </InsetPanel>
      <p className="mt-3 text-xs break-all text-gray-500">
        SHA-256: <span className="font-mono">{proof.textSha256}</span>
      </p>
    </SectionCard>
  );
}

const EMPTY_INPUT: SepaMandateInput = {
  accountHolder: "",
  iban: "",
  bic: "",
  street: "",
  postalCode: "",
  city: "",
};

/** Konto angeben, dann den Text lesen und bestätigen. */
function GrantFlow({
  contract,
  view,
  onGranted,
}: Readonly<{
  contract: ContractKey;
  view: SepaMandateView;
  onGranted: (view: SepaMandateView) => void;
}>) {
  const t = useTranslations("parentSepaMandate");
  const [input, setInput] = useState<SepaMandateInput>(() => ({
    ...EMPTY_INPUT,
    ...view.prefill,
  }));
  const [text, setText] = useState<SepaMandateText | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  if (text) {
    return (
      <ConfirmStep
        contract={contract}
        input={input}
        text={text}
        notice={notice}
        onBack={() => {
          setNotice(null);
          setText(null);
        }}
        onTextChanged={(next) => {
          setNotice(t("textChanged"));
          setText(next);
        }}
        onGranted={onGranted}
      />
    );
  }
  return (
    <AccountStep
      contract={contract}
      input={input}
      reference={undefined}
      onChange={setInput}
      onPreview={(next) => {
        setNotice(null);
        setText(next);
      }}
    />
  );
}

function AccountStep({
  contract,
  input,
  reference,
  onChange,
  onPreview,
}: Readonly<{
  contract: ContractKey;
  input: SepaMandateInput;
  reference: string | undefined;
  onChange: (input: SepaMandateInput) => void;
  onPreview: (text: SepaMandateText) => void;
}>) {
  const t = useTranslations("parentSepaMandate");
  const formRef = useRef<HTMLFormElement>(null);
  const formError = useApiFormError(formRef);
  const [checking, setChecking] = useState(false);
  const set = (field: keyof SepaMandateInput) => (value: string) =>
    onChange({ ...input, [field]: value });

  const submit = async () => {
    formError.clear();
    const fields: Record<string, string> = {};
    for (const [name, value] of [
      ["account_holder", input.accountHolder],
      ["street", input.street],
      ["city", input.city],
    ] as const) {
      if (value.trim() === "") fields[name] = t("required");
    }
    if (input.iban.trim() === "") fields.iban = t("required");
    else if (!ibanLooksValid(input.iban)) fields.iban = t("ibanInvalid");
    if (!/^\d{5}$/.test(input.postalCode.trim())) {
      fields.postal_code = t("postalCodeInvalid");
    }
    if (Object.keys(fields).length > 0) {
      formError.invalid(t("checkFields"), fields);
      return;
    }
    setChecking(true);
    try {
      onPreview(
        await previewSepaMandate(
          contract.organizationId,
          contract.contractId,
          input,
          reference,
        ),
      );
    } catch (err) {
      logger.error("parent_sepa_mandate_preview_failed", {
        error: errorMessage(err),
      });
      void formError.show(err, { object: t("errorObject") });
    } finally {
      setChecking(false);
    }
  };

  return (
    <SectionCard
      concept={billingConcept}
      title={t("stepAccount")}
      description={t("intro")}
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
        <p className="text-sm text-gray-600">{t("stepAccountDescription")}</p>
        <FormErrorAlert message={formError.error} />
        <Input
          controlSize="touch"
          id="sepa-account-holder"
          name="account_holder"
          label={t("holderLabel")}
          hint={t("holderHint")}
          required
          autoComplete="name"
          maxLength={70}
          value={input.accountHolder}
          error={formError.fieldError("account_holder")}
          onChange={(event) => set("accountHolder")(event.target.value)}
        />
        <div className="grid gap-4 sm:grid-cols-[minmax(0,1fr)_12rem]">
          <Input
            controlSize="touch"
            id="sepa-iban"
            name="iban"
            label={t("ibanLabel")}
            placeholder={t("ibanPlaceholder")}
            required
            autoComplete="off"
            spellCheck={false}
            autoCapitalize="characters"
            inputMode="text"
            maxLength={42}
            className="font-mono tracking-wide"
            value={input.iban}
            error={formError.fieldError("iban")}
            onChange={(event) => set("iban")(event.target.value)}
            onBlur={() => {
              if (ibanLooksValid(input.iban))
                set("iban")(groupIban(input.iban));
            }}
            data-analytics-block
          />
          <Input
            controlSize="touch"
            id="sepa-bic"
            name="bic"
            label={t("bicLabel")}
            hint={t("bicHint")}
            autoComplete="off"
            spellCheck={false}
            autoCapitalize="characters"
            maxLength={11}
            value={input.bic}
            error={formError.fieldError("bic")}
            onChange={(event) => set("bic")(event.target.value)}
          />
        </div>
        <FormFieldset legend={t("addressLegend")} required>
          <div className="space-y-4">
            <Input
              controlSize="touch"
              id="sepa-street"
              name="street"
              label={t("streetLabel")}
              required
              autoComplete="street-address"
              maxLength={120}
              value={input.street}
              error={formError.fieldError("street")}
              onChange={(event) => set("street")(event.target.value)}
            />
            <div className="grid gap-4 sm:grid-cols-[8rem_minmax(0,1fr)]">
              <Input
                controlSize="touch"
                id="sepa-postal-code"
                name="postal_code"
                label={t("postalCodeLabel")}
                required
                autoComplete="postal-code"
                inputMode="numeric"
                maxLength={5}
                value={input.postalCode}
                error={formError.fieldError("postal_code")}
                onChange={(event) => set("postalCode")(event.target.value)}
              />
              <Input
                controlSize="touch"
                id="sepa-city"
                name="city"
                label={t("cityLabel")}
                required
                autoComplete="address-level2"
                maxLength={80}
                value={input.city}
                error={formError.fieldError("city")}
                onChange={(event) => set("city")(event.target.value)}
              />
            </div>
          </div>
        </FormFieldset>
        <Alert type="info" size="compact" message={t("privacy")} />
        <div className="flex justify-end">
          <Button
            type="submit"
            variant="primary"
            size="md"
            className="w-full sm:w-auto"
            isLoading={checking}
            loadingText={t("checking")}
          >
            {t("next")}
          </Button>
        </div>
      </form>
    </SectionCard>
  );
}

function ConfirmStep({
  contract,
  input,
  text,
  notice,
  onBack,
  onTextChanged,
  onGranted,
}: Readonly<{
  contract: ContractKey;
  input: SepaMandateInput;
  text: SepaMandateText;
  notice: string | null;
  onBack: () => void;
  onTextChanged: (text: SepaMandateText) => void;
  onGranted: (view: SepaMandateView) => void;
}>) {
  const t = useTranslations("parentSepaMandate");
  const formRef = useRef<HTMLFormElement>(null);
  const formError = useApiFormError(formRef);
  const [holder, setHolder] = useState(false);
  const [granting, setGranting] = useState(false);
  const { success } = useToast();

  const submit = async () => {
    formError.clear();
    if (!holder) {
      formError.invalid(t("checkFields"), {
        holder_confirmed: t("holderConfirmError"),
      });
      return;
    }
    setGranting(true);
    try {
      const granted = await grantSepaMandate(
        contract.organizationId,
        contract.contractId,
        input,
        text,
      );
      success(t("granted"));
      onGranted(granted);
    } catch (err) {
      let failure: unknown = err;
      if (err instanceof ApiError && wireErrorCode(err.code) === TEXT_CHANGED) {
        // The text moto renders now differs (another day, new creditor
        // data): show the new one to read and confirm again.
        try {
          setHolder(false);
          onTextChanged(
            await previewSepaMandate(
              contract.organizationId,
              contract.contractId,
              input,
            ),
          );
          return;
        } catch (previewErr) {
          failure = previewErr;
        }
      }
      logger.error("parent_sepa_mandate_grant_failed", {
        error: errorMessage(failure),
      });
      void formError.show(failure, { object: t("errorObject") });
    } finally {
      setGranting(false);
    }
  };

  return (
    <SectionCard
      concept={billingConcept}
      title={t("stepConfirm")}
      description={t("stepConfirmDescription")}
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
        {notice ? <Alert type="warning" message={notice} /> : null}
        <FormErrorAlert message={formError.error} />
        <InsetPanel framed padding="md">
          <InfoText>{text.text}</InfoText>
        </InsetPanel>
        <ConsentCheckbox
          id="sepa-holder-confirmed"
          name="holder_confirmed"
          label={t("holderConfirm")}
          description={t("onlyOwnAccount")}
          checked={holder}
          required
          error={formError.fieldError("holder_confirmed")}
          onChange={setHolder}
        />
        <div className="flex flex-col-reverse gap-3 sm:flex-row sm:justify-between">
          <Button
            type="button"
            variant="outline"
            size="md"
            disabled={granting}
            onClick={onBack}
          >
            {t("back")}
          </Button>
          <Button
            type="submit"
            variant="success"
            size="md"
            isLoading={granting}
            loadingText={t("granting")}
          >
            {t("grant")}
          </Button>
        </div>
      </form>
    </SectionCard>
  );
}

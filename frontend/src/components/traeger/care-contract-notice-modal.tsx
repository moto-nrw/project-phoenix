"use client";

// Kündigung oder Widerruf eintragen (#4682): das Formular, mit dem das
// Träger-Büro eine Erklärung einträgt, die per Mail oder Brief kam. Die Karte
// „Vertrag beenden“ lädt es erst beim Öffnen.

import { useEffect, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { DatePicker } from "~/components/ui/date-picker";
import { FieldLabel } from "~/components/ui/field-label";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { FormModal } from "~/components/ui/form-modal";
import { Radio } from "~/components/ui/radio";
import { SegmentedControl } from "~/components/ui/segmented-control";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import type {
  ContractNoticeOptions,
  NoticeKind,
  TerminationKind,
} from "~/lib/care-contract-notices";
import {
  fetchContractNoticeOptions,
  recordContractNotice,
} from "~/lib/care-contract-notices-api";
import {
  berlinTodayISO,
  formatDate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CareContractNoticeModal" });

/**
 * Records a declaration the office received by mail or letter. The
 * contract ends with it and the parents get the confirmation at once, so
 * the button names what happens.
 */
export function RecordNoticeModal({
  isOpen,
  onClose,
  contractId,
  options,
  concludedAt,
  onRecorded,
}: Readonly<{
  isOpen: boolean;
  onClose: () => void;
  contractId: string;
  options: ContractNoticeOptions;
  concludedAt: string;
  onRecorded: (next: ContractNoticeOptions) => void;
}>) {
  const today = berlinTodayISO();
  const [kind, setKind] = useState<NoticeKind>("kuendigung");
  const [terminationKind, setTerminationKind] =
    useState<TerminationKind>("ordentlich");
  const [receivedOn, setReceivedOn] = useState<string>(today);
  const [dispatchedOn, setDispatchedOn] = useState("");
  const [saving, setSaving] = useState(false);
  const savingRef = useRef(false);
  const form = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(form);
  const { success } = useToast();
  const [datedOptions, setDatedOptions] = useState<{
    receivedOn: string;
    options: ContractNoticeOptions;
  } | null>(null);
  const [retry, setRetry] = useState(0);
  const {
    error: optionsError,
    show: showOptionsError,
    clear: clearOptionsError,
  } = useApiLoadError();
  const choices =
    datedOptions?.receivedOn === receivedOn ? datedOptions.options : null;
  const withdrawalOpen = choices?.withdrawalUntil != null;

  useEffect(() => {
    if (!isOpen) return;
    let current = true;
    setDatedOptions(null);
    clearOptionsError();
    void fetchContractNoticeOptions(contractId, receivedOn)
      .then((next) => {
        if (!current) return;
        setDatedOptions({ receivedOn, options: next });
        if (!next.withdrawalUntil) setKind("kuendigung");
        if (!next.specialEndsOn) setTerminationKind("ordentlich");
      })
      .catch(async (err: unknown) => {
        if (!current) return;
        logger.error("traeger_care_contract_notice_options_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        await showOptionsError(err, {
          object: "die Kündigung",
          retry: () => setRetry((value) => value + 1),
        });
      });
    return () => {
      current = false;
    };
  }, [
    isOpen,
    contractId,
    receivedOn,
    retry,
    clearOptionsError,
    showOptionsError,
  ]);

  const submit = async () => {
    if (
      savingRef.current ||
      !choices ||
      optionsError ||
      (kind === "widerruf" && !dispatchedOn)
    )
      return;
    savingRef.current = true;
    formErrors.clear();
    setSaving(true);
    try {
      const next = await recordContractNotice(contractId, {
        kind,
        terminationKind,
        receivedOn,
        ...(kind === "widerruf"
          ? { dispatchedOn }
          : {
              // The end date the office saw: the backend refuses a changed one.
              expectedEndsOn:
                (terminationKind === "sonderkuendigung"
                  ? choices?.specialEndsOn
                  : choices?.ordinaryEndsOn) ?? undefined,
            }),
      });
      success(
        kind === "widerruf"
          ? "Der Widerruf ist eingetragen. Die Eltern haben eine Bestätigung bekommen."
          : "Die Kündigung ist eingetragen. Die Eltern haben eine Bestätigung bekommen.",
      );
      onRecorded(next);
    } catch (err) {
      logger.error("traeger_care_contract_notice_record_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Erklärung" });
    } finally {
      savingRef.current = false;
      setSaving(false);
    }
  };

  const verb = kind === "widerruf" ? "Widerruf" : "Kündigung";

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title="Kündigung oder Widerruf eintragen"
      size="md"
      closeDisabled={saving}
      isBackdropDismissDisabled
      error={formErrors.error}
      footer={
        <div className="flex w-full flex-wrap justify-end gap-3">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            disabled={saving}
          >
            Abbrechen
          </Button>
          <Button
            type="submit"
            form="care-contract-notice-form"
            variant="primary"
            size="md"
            disabled={
              saving ||
              !choices ||
              Boolean(optionsError) ||
              (kind === "widerruf" && !dispatchedOn)
            }
            isLoading={saving}
            loadingText="Wird eingetragen …"
          >
            {verb} eintragen
          </Button>
        </div>
      }
    >
      <form
        id="care-contract-notice-form"
        ref={form}
        noValidate
        className="space-y-5"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <p className="text-sm text-gray-600">
          {options.childName} · {options.modules} · {options.schoolName}
        </p>
        <div className="space-y-1.5">
          <FieldLabel htmlFor="care-contract-notice-received" required>
            Eingegangen am
          </FieldLabel>
          <DatePicker
            id="care-contract-notice-received"
            name="received_on"
            value={parseISODate(receivedOn)}
            onChange={(date) => {
              setKind("kuendigung");
              setTerminationKind("ordentlich");
              setReceivedOn(date ? toISODate(date) : today);
              setDispatchedOn("");
            }}
            maxDate={parseISODate(today)}
            minDate={parseISODate(berlinTodayISO(new Date(concludedAt)))}
            hideClearButton
            required
            invalid={Boolean(formErrors.fieldError("received_on"))}
          />
        </div>
        {optionsError ? <LoadErrorAlert error={optionsError} /> : null}
        {choices ? (
          <>
            {withdrawalOpen ? (
              <SegmentedControl<NoticeKind>
                ariaLabel="Was haben die Eltern erklärt?"
                fullWidth
                value={kind}
                onChange={setKind}
                items={[
                  { value: "kuendigung", label: "Kündigung" },
                  { value: "widerruf", label: "Widerruf" },
                ]}
              />
            ) : (
              <p className="text-sm text-gray-600">
                Ein Widerruf ist nicht mehr möglich. Die Eltern können kündigen.
              </p>
            )}
            {kind === "kuendigung" ? (
              <fieldset className="space-y-2">
                <legend className="mb-2 text-sm font-medium text-gray-900">
                  Zu welchem Termin?
                </legend>
                <ChoiceTile
                  selected={terminationKind === "ordentlich"}
                  className="p-3"
                >
                  <Radio
                    name="termination_kind"
                    value="ordentlich"
                    checked={terminationKind === "ordentlich"}
                    onChange={() => setTerminationKind("ordentlich")}
                  />
                  <span className="min-w-0">
                    <span className="block text-gray-900">
                      Nächster Termin
                      {choices.ordinaryEndsOn
                        ? `: ${formatDate(choices.ordinaryEndsOn)}`
                        : ""}
                    </span>
                    <span className="block font-normal text-gray-500">
                      {options.terms}
                    </span>
                  </span>
                </ChoiceTile>
                {choices.specialEndsOn ? (
                  <ChoiceTile
                    selected={terminationKind === "sonderkuendigung"}
                    className="p-3"
                  >
                    <Radio
                      name="termination_kind"
                      value="sonderkuendigung"
                      checked={terminationKind === "sonderkuendigung"}
                      onChange={() => setTerminationKind("sonderkuendigung")}
                    />
                    <span className="min-w-0">
                      <span className="block text-gray-900">
                        Sonderkündigung zum Halbjahr:{" "}
                        {formatDate(choices.specialEndsOn)}
                      </span>
                      <span className="block font-normal text-gray-500">
                        Nur, wenn ein Grund aus dem Vertrag vorliegt.
                      </span>
                    </span>
                  </ChoiceTile>
                ) : null}
                <p className="text-sm text-gray-500">
                  Die Termine gelten für den gewählten Eingangstag.
                </p>
              </fieldset>
            ) : (
              <div className="space-y-4">
                <div className="space-y-1.5">
                  <FieldLabel
                    htmlFor="care-contract-notice-dispatched"
                    required
                  >
                    Abgesendet am
                  </FieldLabel>
                  <DatePicker
                    id="care-contract-notice-dispatched"
                    name="dispatched_on"
                    value={
                      dispatchedOn ? parseISODate(dispatchedOn) : undefined
                    }
                    onChange={(date) =>
                      setDispatchedOn(date ? toISODate(date) : "")
                    }
                    minDate={parseISODate(
                      berlinTodayISO(new Date(concludedAt)),
                    )}
                    maxDate={parseISODate(
                      choices.withdrawalUntil &&
                        choices.withdrawalUntil < receivedOn
                        ? choices.withdrawalUntil
                        : receivedOn,
                    )}
                    required
                    invalid={Boolean(formErrors.fieldError("dispatched_on"))}
                  />
                </div>
                <Alert
                  type="info"
                  message={`Der Vertrag wird vom ersten Tag an aufgehoben. Rechtzeitig abgesendet: bis ${formatDate(choices.withdrawalUntil ?? "")}.`}
                />
              </div>
            )}
          </>
        ) : !optionsError ? (
          <p className="text-sm text-gray-500">Termine werden geladen …</p>
        ) : null}

        <Alert
          type="warning"
          message="Die Eltern bekommen sofort eine Bestätigung per E-Mail. Das lässt sich nicht rückgängig machen."
        />
      </form>
    </FormModal>
  );
}

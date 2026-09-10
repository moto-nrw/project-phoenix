"use client";

import { useEffect, useMemo, useState } from "react";
import { Eye, Landmark, Loader2 } from "lucide-react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { useFormError } from "~/components/ui/form-error";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { useToast } from "~/contexts/ToastContext";
import type { GuardianWithRelationship } from "@/lib/guardian-helpers";
import { getGuardianFullName } from "@/lib/guardian-helpers";
import {
  fetchGuardianPayment,
  revealGuardianPayment,
  setStudentPayer,
  updateGuardianPayment,
} from "~/lib/guardian-payment-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentPaymentCard" });

const NO_PAYER = "none";

interface StudentPaymentCardProps {
  readonly studentId: string;
  readonly guardians: readonly GuardianWithRelationship[];
  readonly readOnly?: boolean;
  readonly onChanged?: () => void;
}

interface PaymentDraft {
  readonly payerId: string | null;
  readonly iban: string;
  readonly holder: string;
}

/**
 * Bank account charged for one child (#2608).
 *
 * The IBAN belongs to the selected person, not to this child — the card says so
 * out loud, because editing it here also changes what siblings are charged from.
 * The unmasked IBAN is never fetched with the card: "Anzeigen" is a separate,
 * server-audited request.
 *
 * One edit state for the whole card (Bauart 2, Regel 4, #3112): „Bearbeiten“
 * opens payer, IBAN and account holder together, „Speichern“ writes them. The
 * payer used to save the moment the dropdown changed, next to an IBAN form
 * that waited for its own Speichern — two save models in one card, and the
 * direct debit hangs on the payer.
 */
export function StudentPaymentCard({
  studentId,
  guardians,
  readOnly = false,
  onChanged,
}: StudentPaymentCardProps) {
  const toast = useToast();

  const payer = useMemo(
    () => guardians.find((g) => g.isPayer) ?? null,
    [guardians],
  );

  const [ibanMasked, setIbanMasked] = useState<string | null>(null);
  const [accountHolder, setAccountHolder] = useState<string | null>(null);
  const [revealedIban, setRevealedIban] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [isRevealing, setIsRevealing] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [draft, setDraft] = useState<PaymentDraft | null>(null);
  // Stand beim Öffnen des Bearbeiten-Zustands; Speichern ist nur bei einer
  // Abweichung davon möglich.
  const [baseline, setBaseline] = useState<PaymentDraft | null>(null);
  // Fehler des Bearbeiten-Zustands (Öffnen oder Speichern) stehen als Alert
  // oben im Bearbeiten-Bereich, nicht als Toast (Bauart 2, Regel 5).
  const [editError, setEditError] = useFormError();

  const payerId = payer?.id ?? null;

  useEffect(() => {
    if (draft !== null && baseline?.payerId !== payerId) {
      setDraft(null);
      setBaseline(null);
      setEditError(null);
    }
  }, [baseline?.payerId, draft, payerId, setEditError]);

  // Reload trigger. The effect below deliberately depends on payerId and this
  // counter only: a card that refetches on every render — and resets the
  // revealed IBAN while doing so — is not a failure worth risking.
  const [reloadToken, setReloadToken] = useState(0);

  useEffect(() => {
    if (!payerId) {
      setIbanMasked(null);
      setAccountHolder(null);
      setRevealedIban(null);
      setLoadError(null);
      return;
    }
    let cancelled = false;
    setIsLoading(true);
    setLoadError(null);
    fetchGuardianPayment(payerId)
      .then((data) => {
        if (cancelled) return;
        setIbanMasked(data.ibanMasked);
        setAccountHolder(data.accountHolder);
        setRevealedIban(null);
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        logger.error("load_guardian_payment_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        setLoadError(
          error instanceof Error
            ? error.message
            : "Die Bankverbindung konnte nicht geladen werden. Bitte noch einmal versuchen.",
        );
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [payerId, reloadToken]);

  const handleReveal = async () => {
    if (!payerId) return;
    setIsRevealing(true);
    setEditError(null);
    try {
      const data = await revealGuardianPayment(payerId);
      setRevealedIban(data.iban);
    } catch (error) {
      setEditError(
        error instanceof Error
          ? error.message
          : "Die IBAN konnte nicht angezeigt werden. Bitte noch einmal versuchen.",
      );
    } finally {
      setIsRevealing(false);
    }
  };

  const handleStartEdit = async () => {
    setEditError(null);
    if (!payerId) {
      const empty = { payerId: null, iban: "", holder: "" };
      setBaseline(empty);
      setDraft(empty);
      return;
    }
    setIsRevealing(true);
    try {
      // Edit starts from the real value: a form prefilled with a masked string
      // would silently overwrite the stored IBAN with dots on save.
      const data = await revealGuardianPayment(payerId);
      const opened = {
        payerId,
        iban: data.iban ?? "",
        holder: data.accountHolder ?? "",
      };
      setBaseline(opened);
      setDraft(opened);
    } catch (error) {
      setEditError(
        error instanceof Error
          ? error.message
          : "Die Bankverbindung konnte nicht geöffnet werden. Bitte noch einmal versuchen.",
      );
    } finally {
      setIsRevealing(false);
    }
  };

  const cancelEdit = () => {
    setDraft(null);
    setEditError(null);
  };

  const payerChanged = draft !== null && draft.payerId !== payerId;
  const bankChanged =
    draft !== null &&
    baseline !== null &&
    !payerChanged &&
    payerId !== null &&
    (draft.iban.trim() !== baseline.iban.trim() ||
      draft.holder.trim() !== baseline.holder.trim());

  const handleSave = async () => {
    if (!draft) return;
    setIsSaving(true);
    setEditError(null);
    try {
      if (payerChanged) {
        await setStudentPayer(studentId, draft.payerId);
        setDraft(null);
        toast.success("Zahlungskonto gespeichert.");
        onChanged?.();
        return;
      }
      if (payerId) {
        await updateGuardianPayment(payerId, {
          iban: draft.iban.trim() === "" ? null : draft.iban.trim(),
          accountHolder:
            draft.holder.trim() === "" ? null : draft.holder.trim(),
        });
        setDraft(null);
        setReloadToken((token) => token + 1);
        toast.success("Bankverbindung gespeichert.");
      }
    } catch (error) {
      setEditError(
        error instanceof Error
          ? error.message
          : "Das Zahlungskonto konnte nicht gespeichert werden. Bitte noch einmal versuchen.",
      );
    } finally {
      setIsSaving(false);
    }
  };

  const options = [
    { value: NO_PAYER, label: "Niemand ausgewählt" },
    ...guardians.map((g) => ({
      value: g.id,
      label: getGuardianFullName(g),
    })),
  ];

  const draftPayer = draft?.payerId
    ? (guardians.find((g) => g.id === draft.payerId) ?? null)
    : null;
  const editing = draft !== null;

  return (
    <SectionCard
      title="Zahlungskonto"
      description="Von diesem Konto zieht die Schule den Beitrag für dieses Kind ein."
      icon={Landmark}
      headingLevel={3}
      action={
        !readOnly && guardians.length > 0 && !editing ? (
          <Button
            type="button"
            size="md"
            variant="outline"
            className="bg-white"
            onClick={() => void handleStartEdit()}
            disabled={isRevealing || isLoading}
          >
            Bearbeiten
          </Button>
        ) : undefined
      }
    >
      <div className="space-y-4">
        <FormErrorAlert message={editError} />
        {loadError && !editing && <Alert type="error" message={loadError} />}

        {editing ? (
          <div className="max-w-sm space-y-3">
            <div>
              <label
                htmlFor="payment-payer"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                Wer zahlt für dieses Kind?
              </label>
              <CustomSelect
                id="payment-payer"
                value={draft.payerId ?? NO_PAYER}
                options={options}
                onChange={(value) =>
                  setDraft((current) =>
                    current
                      ? {
                          ...current,
                          payerId: value === NO_PAYER ? null : value,
                        }
                      : current,
                  )
                }
                disabled={isSaving}
                testId="payment-payer-select"
              />
              <p className="mt-1 text-xs text-gray-500">
                Zur Auswahl stehen die Erziehungsberechtigten dieses Kindes.
              </p>
            </div>

            {payerChanged ? (
              draftPayer ? (
                <p className="text-sm text-gray-600">
                  Die Bankverbindung von {getGuardianFullName(draftPayer)}{" "}
                  können Sie nach dem Speichern eintragen oder ändern.
                </p>
              ) : (
                <p className="text-sm text-gray-600">
                  Ohne Zahlungskonto steht das Kind ohne Bankverbindung in der
                  Liste.
                </p>
              )
            ) : payer ? (
              <>
                <Input
                  id="payment-iban"
                  label="IBAN"
                  value={draft.iban}
                  onChange={(e) =>
                    setDraft((current) =>
                      current ? { ...current, iban: e.target.value } : current,
                    )
                  }
                  placeholder="DE00 0000 0000 0000 0000 00"
                  autoComplete="off"
                  spellCheck={false}
                  disabled={isSaving}
                />
                <Input
                  id="payment-account-holder"
                  label="Anderer Kontoinhaber"
                  value={draft.holder}
                  onChange={(e) =>
                    setDraft((current) =>
                      current
                        ? { ...current, holder: e.target.value }
                        : current,
                    )
                  }
                  placeholder={getGuardianFullName(payer)}
                  autoComplete="off"
                  disabled={isSaving}
                />
                <p className="text-xs text-gray-500">
                  Nur ausfüllen, wenn das Konto auf einen anderen Namen läuft.
                  Die IBAN gehört zur Person, nicht zum Kind. Sie gilt auch für
                  Geschwister mit derselben Person.
                </p>
              </>
            ) : null}

            <EditActions
              onCancel={cancelEdit}
              onSave={() => void handleSave()}
              saving={isSaving}
              disabled={!payerChanged && !bankChanged}
            />
          </div>
        ) : (
          <>
            <DataGrid>
              <DataField label="Wer zahlt für dieses Kind?">
                {payer ? getGuardianFullName(payer) : "Niemand ausgewählt"}
              </DataField>
              {payer && (
                <DataField label="Kontoinhaber">
                  {isLoading ? (
                    <span className="inline-flex items-center gap-2 text-gray-600">
                      <Loader2 className="h-4 w-4 animate-spin" aria-hidden />
                      Wird geladen…
                    </span>
                  ) : (
                    (accountHolder ?? getGuardianFullName(payer))
                  )}
                </DataField>
              )}
              {payer && (
                <DataField label="IBAN" mono={Boolean(ibanMasked)} fullWidth>
                  {isLoading ? (
                    <span className="font-sans text-gray-600">
                      Wird geladen…
                    </span>
                  ) : ibanMasked ? (
                    (revealedIban ?? ibanMasked)
                  ) : (
                    <span className="font-sans font-normal text-gray-600">
                      Noch keine IBAN gespeichert.
                    </span>
                  )}
                </DataField>
              )}
            </DataGrid>

            {payer && !isLoading && ibanMasked && !revealedIban && (
              <div>
                <Button
                  type="button"
                  size="md"
                  variant="outline"
                  onClick={() => void handleReveal()}
                  disabled={isRevealing}
                >
                  <Eye className="mr-1.5 h-4 w-4" aria-hidden />
                  {isRevealing ? "Wird geladen…" : "Anzeigen"}
                </Button>
              </div>
            )}

            {payer && (
              <p className="text-xs text-gray-500">
                Die IBAN gehört zur Person, nicht zum Kind. Sie gilt auch für
                Geschwister mit derselben Person. Wer die IBAN anzeigt, wird
                protokolliert.
              </p>
            )}

            {!payer && guardians.length > 0 && (
              <p className="text-sm text-gray-600">
                Für dieses Kind ist noch niemand als Zahlungskonto eingetragen.
                Ohne Eintrag steht das Kind ohne Bankverbindung in der Liste.
              </p>
            )}
            {guardians.length === 0 && (
              <p className="text-sm text-gray-600">
                Für dieses Kind ist noch keine erziehungsberechtigte Person
                eingetragen. Erst danach können Sie ein Zahlungskonto auswählen.
              </p>
            )}
          </>
        )}
      </div>
    </SectionCard>
  );
}

"use client";

import { useEffect, useMemo, useState } from "react";
import { Eye, Landmark, Loader2 } from "lucide-react";

import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { Input } from "~/components/ui/input";
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

/**
 * Bank account charged for one child (#2608).
 *
 * The IBAN belongs to the selected person, not to this child — the card says so
 * out loud, because editing it here also changes what siblings are charged from.
 * The unmasked IBAN is never fetched with the card: "Anzeigen" is a separate,
 * server-audited request.
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
  const [isRevealing, setIsRevealing] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [ibanDraft, setIbanDraft] = useState("");
  const [holderDraft, setHolderDraft] = useState("");

  const payerId = payer?.id ?? null;

  // Reload trigger. The effect below deliberately depends on payerId and this
  // counter only: `toast` comes from a context whose value is memoized today,
  // but a card that refetches on every render — and resets the revealed IBAN
  // while doing so — is not a failure worth risking on that.
  const [reloadToken, setReloadToken] = useState(0);

  useEffect(() => {
    if (!payerId) {
      setIbanMasked(null);
      setAccountHolder(null);
      setRevealedIban(null);
      return;
    }
    let cancelled = false;
    setIsLoading(true);
    setIsEditing(false);
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
        toast.error(
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
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [payerId, reloadToken]);

  const handleSelectPayer = async (value: string) => {
    const nextId = value === NO_PAYER ? null : value;
    try {
      await setStudentPayer(studentId, nextId);
      setIsEditing(false);
      onChanged?.();
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : "Das Zahlungskonto konnte nicht gespeichert werden. Bitte noch einmal versuchen.",
      );
    }
  };

  const handleReveal = async () => {
    if (!payerId) return;
    setIsRevealing(true);
    try {
      const data = await revealGuardianPayment(payerId);
      setRevealedIban(data.iban);
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : "Die IBAN konnte nicht angezeigt werden. Bitte noch einmal versuchen.",
      );
    } finally {
      setIsRevealing(false);
    }
  };

  const handleStartEdit = async () => {
    if (!payerId) return;
    setIsRevealing(true);
    try {
      // Edit starts from the real value: a form prefilled with a masked string
      // would silently overwrite the stored IBAN with dots on save.
      const data = await revealGuardianPayment(payerId);
      setIbanDraft(data.iban ?? "");
      setHolderDraft(data.accountHolder ?? "");
      setIsEditing(true);
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : "Die Bankverbindung konnte nicht geöffnet werden. Bitte noch einmal versuchen.",
      );
    } finally {
      setIsRevealing(false);
    }
  };

  const handleSave = async () => {
    if (!payerId) return;
    setIsSaving(true);
    try {
      await updateGuardianPayment(payerId, {
        iban: ibanDraft.trim() === "" ? null : ibanDraft.trim(),
        accountHolder: holderDraft.trim() === "" ? null : holderDraft.trim(),
      });
      setIsEditing(false);
      setReloadToken((token) => token + 1);
      toast.success("Bankverbindung gespeichert.");
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : "Die Bankverbindung konnte nicht gespeichert werden. Bitte noch einmal versuchen.",
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

  return (
    <section className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
      <div className="mb-4 flex items-center gap-3">
        <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl bg-gray-100">
          <Landmark className="h-4 w-4 text-gray-600" aria-hidden />
        </div>
        <div>
          <h3 className="text-base font-semibold text-gray-900">
            Zahlungskonto
          </h3>
          <p className="text-sm text-gray-600">
            Von diesem Konto zieht die Schule den Beitrag für dieses Kind ein.
          </p>
        </div>
      </div>

      <div className="space-y-4">
        <div>
          <label
            htmlFor="payment-payer"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Wer zahlt für dieses Kind?
          </label>
          <CustomSelect
            id="payment-payer"
            value={payerId ?? NO_PAYER}
            options={options}
            onChange={(value) => void handleSelectPayer(value)}
            disabled={readOnly || guardians.length === 0}
            testId="payment-payer-select"
            className="max-w-sm"
          />
          <p className="mt-1 text-xs text-gray-500">
            Zur Auswahl stehen die Erziehungsberechtigten dieses Kindes.
          </p>
        </div>

        {payer && (
          <div className="rounded-xl bg-gray-50 p-4">
            {isLoading ? (
              <p className="flex items-center gap-2 text-sm text-gray-600">
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden />
                Bankverbindung wird geladen…
              </p>
            ) : isEditing ? (
              <div className="max-w-sm space-y-3">
                <Input
                  id="payment-iban"
                  label="IBAN"
                  value={ibanDraft}
                  onChange={(e) => setIbanDraft(e.target.value)}
                  placeholder="DE00 0000 0000 0000 0000 00"
                  autoComplete="off"
                  spellCheck={false}
                />
                <Input
                  id="payment-account-holder"
                  label="Anderer Kontoinhaber"
                  value={holderDraft}
                  onChange={(e) => setHolderDraft(e.target.value)}
                  placeholder={getGuardianFullName(payer)}
                  autoComplete="off"
                />
                <p className="text-xs text-gray-500">
                  Nur ausfüllen, wenn das Konto auf einen anderen Namen läuft.
                </p>
                <div className="flex flex-wrap gap-2">
                  <Button
                    type="button"
                    size="md"
                    onClick={() => void handleSave()}
                    disabled={isSaving}
                  >
                    {isSaving ? "Wird gespeichert…" : "Speichern"}
                  </Button>
                  <Button
                    type="button"
                    size="md"
                    variant="outline"
                    onClick={() => setIsEditing(false)}
                    disabled={isSaving}
                  >
                    Abbrechen
                  </Button>
                </div>
              </div>
            ) : (
              <div className="space-y-3">
                <div>
                  <p className="text-xs font-medium tracking-wide text-gray-500 uppercase">
                    Kontoinhaber
                  </p>
                  <p className="text-sm text-gray-900">
                    {accountHolder ?? getGuardianFullName(payer)}
                  </p>
                </div>
                <div>
                  <p className="text-xs font-medium tracking-wide text-gray-500 uppercase">
                    IBAN
                  </p>
                  {ibanMasked ? (
                    <p className="font-mono text-sm text-gray-900">
                      {revealedIban ?? ibanMasked}
                    </p>
                  ) : (
                    <p className="text-sm text-gray-600">
                      Noch keine IBAN gespeichert.
                    </p>
                  )}
                </div>
                <div className="flex flex-wrap gap-2">
                  {ibanMasked && !revealedIban && (
                    <Button
                      type="button"
                      size="md"
                      variant="outline"
                      onClick={() => void handleReveal()}
                      disabled={isRevealing}
                      className="grow sm:grow-0"
                    >
                      <Eye className="mr-1.5 h-4 w-4" aria-hidden />
                      {isRevealing ? "Wird geladen…" : "Anzeigen"}
                    </Button>
                  )}
                  {!readOnly && (
                    <Button
                      type="button"
                      size="md"
                      variant="outline"
                      onClick={() => void handleStartEdit()}
                      disabled={isRevealing}
                      className="grow sm:grow-0"
                    >
                      {ibanMasked ? "Bearbeiten" : "IBAN eintragen"}
                    </Button>
                  )}
                </div>
                <p className="text-xs text-gray-500">
                  Die IBAN gehört zur Person, nicht zum Kind. Sie gilt auch für
                  Geschwister mit derselben Person. Wer die IBAN anzeigt, wird
                  protokolliert.
                </p>
              </div>
            )}
          </div>
        )}

        {!payer && guardians.length > 0 && (
          <p className="text-sm text-gray-600">
            Für dieses Kind ist noch niemand als Zahlungskonto eingetragen. Ohne
            Eintrag steht das Kind ohne Bankverbindung in der Liste.
          </p>
        )}
        {guardians.length === 0 && (
          <p className="text-sm text-gray-600">
            Für dieses Kind ist noch keine erziehungsberechtigte Person
            eingetragen. Erst danach können Sie ein Zahlungskonto auswählen.
          </p>
        )}
      </div>
    </section>
  );
}

"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { ConfirmationModal } from "~/components/ui/modal";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  useApiErrorDisplay,
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { formatBerlinDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  endMealCatererSupply,
  getMealCaterer,
  inviteMealCaterer,
  revokeMealCatererInvitation,
  type MealCaterer as MealCatererState,
  type MealCatererInvitation,
} from "~/lib/meal-caterer-api";

const logger = createLogger({ component: "MealCaterer" });

const EMPTY_FORM = { email: "", catererName: "", deliveryAddress: "" };

/**
 * Der Caterer der Schule (ADR 0074): wer liefert, eine Einladung per E-Mail
 * und das Ende der Belieferung. Der Link der Einladung geht nur an den
 * Caterer; die Schule sieht ihn nie.
 */
export function MealCaterer() {
  const [state, setState] = useState<MealCatererState | null>(null);
  const [loading, setLoading] = useState(true);
  const [form, setForm] = useState(EMPTY_FORM);
  const [sending, setSending] = useState(false);
  const [revokeTarget, setRevokeTarget] =
    useState<MealCatererInvitation | null>(null);
  const [endOpen, setEndOpen] = useState(false);
  const [working, setWorking] = useState(false);
  const loadErrors = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = loadErrors;
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { show: showActionError } = useApiErrorDisplay();
  const toast = useToast();
  const reloadRef = useRef<() => void>(() => undefined);

  const load = useCallback(async () => {
    setLoading(true);
    clearLoadError();
    try {
      setState(await getMealCaterer());
    } catch (cause) {
      logger.error("meal_caterer_load_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      void showLoadError(cause, {
        object: "der Caterer",
        retry: () => reloadRef.current(),
      });
    } finally {
      setLoading(false);
    }
  }, [clearLoadError, showLoadError]);
  useEffect(() => {
    reloadRef.current = () => void load();
  });
  useEffect(() => {
    void load();
  }, [load]);

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    setSending(true);
    formErrors.clear();
    try {
      await inviteMealCaterer(form);
      toast.success(`Die Einladung ist unterwegs an ${form.email.trim()}.`);
      setForm(EMPTY_FORM);
      await load();
    } catch (cause) {
      void formErrors.show(cause, { object: "die Einladung" });
    } finally {
      setSending(false);
    }
  }

  async function confirmRevoke() {
    if (!revokeTarget) return;
    setWorking(true);
    try {
      await revokeMealCatererInvitation(revokeTarget.id);
      setRevokeTarget(null);
      await load();
    } catch (cause) {
      void showActionError(cause, { object: "die Einladung" });
    } finally {
      setWorking(false);
    }
  }

  async function confirmEnd() {
    setWorking(true);
    try {
      await endMealCatererSupply();
      setEndOpen(false);
      toast.success("Die Belieferung ist beendet.");
      await load();
    } catch (cause) {
      void showActionError(cause, { object: "das Ende der Belieferung" });
    } finally {
      setWorking(false);
    }
  }

  const openInvitations =
    state?.invitations.filter((invitation) => invitation.status === "open") ??
    [];
  const supply = state?.supply ?? null;

  return (
    <SectionCard
      title="Caterer"
      description="Ihr Caterer sieht die Bestellungen der Schule in seinem eigenen Zugang bei moto. Namen der Kinder sieht er nur, wenn Sie das in den Einstellungen einschalten."
      actions={
        supply ? (
          <Button
            type="button"
            variant="outline_danger"
            size="md"
            onClick={() => setEndOpen(true)}
          >
            Belieferung beenden
          </Button>
        ) : undefined
      }
    >
      {loading ? (
        <Skeleton className="h-24 w-full" />
      ) : loadErrors.error ? (
        <LoadErrorAlert error={loadErrors.error} />
      ) : supply ? (
        <DataGrid>
          <DataField label="Caterer">{supply.catererName || "–"}</DataField>
          <DataField label="Beliefert seit">
            {formatBerlinDate(supply.startedAt)}
          </DataField>
          <DataField label="Lieferadresse" fullWidth>
            {supply.deliveryAddress || "–"}
          </DataField>
        </DataGrid>
      ) : (
        <div className="space-y-5">
          <form
            ref={formRef}
            onSubmit={(event) => void handleSubmit(event)}
            noValidate
            className="space-y-4"
          >
            {formErrors.error ? (
              <FormErrorAlert message={formErrors.error} />
            ) : null}
            <p className="text-sm text-gray-700">
              Noch kein Caterer. Laden Sie Ihren Caterer per E-Mail ein. Er legt
              dann selbst sein Passwort fest.
            </p>
            <div className="grid gap-4 sm:grid-cols-2">
              <Input
                label="Name des Caterers"
                name="caterer_name"
                value={form.catererName}
                onChange={(event) =>
                  setForm({ ...form, catererName: event.target.value })
                }
                error={formErrors.fieldError("caterer_name")}
                disabled={sending}
                required
              />
              <Input
                label="E-Mail-Adresse des Caterers"
                name="email"
                type="email"
                value={form.email}
                onChange={(event) =>
                  setForm({ ...form, email: event.target.value })
                }
                error={formErrors.fieldError("email")}
                disabled={sending}
                required
              />
            </div>
            <Input
              label="Lieferadresse (optional)"
              name="delivery_address"
              value={form.deliveryAddress}
              onChange={(event) =>
                setForm({ ...form, deliveryAddress: event.target.value })
              }
              error={formErrors.fieldError("delivery_address")}
              disabled={sending}
            />
            <Button
              type="submit"
              variant="primary"
              size="md"
              isLoading={sending}
              loadingText="Wird gesendet …"
              disabled={
                sending ||
                form.email.trim() === "" ||
                form.catererName.trim() === ""
              }
            >
              Einladung senden
            </Button>
          </form>

          {openInvitations.length > 0 ? (
            <div className="space-y-2">
              <h3 className="text-sm font-semibold text-gray-900">
                Offene Einladungen
              </h3>
              <ul className="divide-y divide-gray-100">
                {openInvitations.map((invitation) => (
                  <li
                    key={invitation.id}
                    className="flex flex-wrap items-center justify-between gap-2 py-2"
                  >
                    <div className="text-sm">
                      <span className="font-medium text-gray-900">
                        {invitation.catererName}
                      </span>{" "}
                      <span className="text-gray-600">{invitation.email}</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <StatusBadge
                        label={`Gültig bis ${formatBerlinDate(invitation.expiresAt)}`}
                        tone="gray"
                        compact
                      />
                      <Button
                        type="button"
                        variant="ghost"
                        size="compact"
                        onClick={() => setRevokeTarget(invitation)}
                      >
                        Zurücknehmen
                      </Button>
                    </div>
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
        </div>
      )}

      <ConfirmationModal
        isOpen={revokeTarget !== null}
        onClose={() => setRevokeTarget(null)}
        onConfirm={() => void confirmRevoke()}
        title="Einladung zurücknehmen"
        confirmText="Zurücknehmen"
        cancelText="Abbrechen"
        confirmVariant="danger"
        isConfirmLoading={working}
      >
        <p className="text-sm text-gray-600">
          Der Link in der E-Mail an {revokeTarget?.email} gilt dann nicht mehr.
        </p>
      </ConfirmationModal>

      <ConfirmationModal
        isOpen={endOpen}
        onClose={() => setEndOpen(false)}
        onConfirm={() => void confirmEnd()}
        title="Belieferung beenden"
        confirmText="Beenden"
        cancelText="Abbrechen"
        confirmVariant="danger"
        isConfirmLoading={working}
      >
        <p className="text-sm text-gray-600">
          {supply?.catererName || "Der Caterer"} sieht die Bestellungen der
          Schule danach nicht mehr. Für einen neuen Caterer senden Sie danach
          eine neue Einladung.
        </p>
      </ConfirmationModal>
    </SectionCard>
  );
}

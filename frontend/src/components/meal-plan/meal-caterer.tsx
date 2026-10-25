"use client";

import { useCallback, useEffect, useId, useRef, useState } from "react";
import { Plus } from "lucide-react";

import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { ConfirmationModal } from "~/components/ui/modal";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
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
 * Der Caterer der Schule (ADR 0077): wer liefert, eine Einladung per E-Mail
 * und das Ende der Belieferung. Der Link der Einladung geht nur an den
 * Caterer; die Schule sieht ihn nie.
 */
interface MealCatererProps {
  /** Einladen, Zurücknehmen und Beenden dürfen nur Admins der Schule. */
  canManageSupply: boolean;
}

export function MealCaterer({ canManageSupply }: MealCatererProps) {
  const [state, setState] = useState<MealCatererState | null>(null);
  const [loading, setLoading] = useState(true);
  const [form, setForm] = useState(EMPTY_FORM);
  const [inviteOpen, setInviteOpen] = useState(false);
  const [sending, setSending] = useState(false);
  const [revokeTarget, setRevokeTarget] =
    useState<MealCatererInvitation | null>(null);
  const [endOpen, setEndOpen] = useState(false);
  const [working, setWorking] = useState(false);
  const loadErrors = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = loadErrors;
  const formRef = useRef<HTMLFormElement>(null);
  const formId = useId();
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

  function openInvite() {
    setForm(EMPTY_FORM);
    formErrors.clear();
    setInviteOpen(true);
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (sending || !form.email.trim() || !form.catererName.trim()) return;
    setSending(true);
    formErrors.clear();
    try {
      await inviteMealCaterer(form);
      toast.success(`Die Einladung ist unterwegs an ${form.email.trim()}.`);
      setInviteOpen(false);
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
      toast.success("Die Einladung ist widerrufen.");
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

  const invitationColumns: DataTableColumn<MealCatererInvitation>[] = [
    {
      key: "caterer",
      header: "Caterer",
      stacked: "title",
      render: (invitation) => invitation.catererName,
    },
    {
      key: "email",
      header: "E-Mail-Adresse",
      stacked: "meta",
      render: (invitation) => invitation.email,
    },
    {
      key: "expires",
      header: "Gültig bis",
      stacked: "meta",
      render: (invitation) => formatBerlinDate(invitation.expiresAt),
    },
    {
      key: "actions",
      header: "",
      align: "right",
      render: (invitation) => (
        <OverflowMenu
          ariaLabel={`Aktionen für die Einladung an ${invitation.email}`}
          items={[
            {
              label: "Widerrufen",
              destructive: true,
              onClick: () => setRevokeTarget(invitation),
            },
          ]}
        />
      ),
    },
  ];

  // Kopfaktion je Zustand: ohne Caterer das Einladen (Bauart 2 Regel 4),
  // mit Caterer das seltene Beenden im Menü des Kartenkopfs.
  let headerActions: React.ReactNode;
  if (canManageSupply && !loading && !loadErrors.error) {
    headerActions = supply ? (
      <OverflowMenu
        ariaLabel="Weitere Aktionen"
        items={[
          {
            label: "Belieferung beenden",
            destructive: true,
            onClick: () => setEndOpen(true),
          },
        ]}
      />
    ) : (
      <Button
        type="button"
        variant="primary"
        size="md"
        className="gap-2"
        onClick={openInvite}
      >
        <Plus className="h-4 w-4" aria-hidden="true" />
        Caterer einladen
      </Button>
    );
  }

  return (
    <SectionCard
      title="Caterer"
      description="Ihr Caterer sieht die Bestellungen der Schule in seinem eigenen Zugang bei moto. Namen der Kinder sieht er nur, wenn Sie das in den Einstellungen einschalten."
      actions={headerActions}
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
      ) : !canManageSupply ? (
        <p className="text-sm text-gray-700">
          Noch kein Caterer. Einen Caterer einladen können nur Admins Ihrer
          Schule.
        </p>
      ) : (
        <div className="space-y-4">
          <p className="text-sm text-gray-700">
            Noch kein Caterer. Laden Sie Ihren Caterer per E-Mail ein.
          </p>

          {openInvitations.length > 0 ? (
            <DataTable
              heading={{
                title: "Offene Einladungen",
                count: openInvitations.length,
              }}
              columns={invitationColumns}
              rows={openInvitations}
              getRowKey={(invitation) => invitation.id}
              rowHasInteractiveControls
              stackedOnMobile
            />
          ) : null}
        </div>
      )}

      <FormModal
        isOpen={inviteOpen}
        onClose={() => setInviteOpen(false)}
        title="Caterer einladen"
        size="md"
        closeDisabled={sending}
        isBackdropDismissDisabled
        error={formErrors.error}
        footer={
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => setInviteOpen(false)}
              disabled={sending}
            >
              Abbrechen
            </Button>
            <Button
              type="submit"
              form={formId}
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
          </div>
        }
      >
        <form
          id={formId}
          ref={formRef}
          onSubmit={handleSubmit}
          className="space-y-4"
        >
          <p className="text-sm text-gray-600">
            Ihr Caterer bekommt eine E-Mail mit einem Link. Damit legt er sein
            Passwort fest.
          </p>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
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
            <div className="sm:col-span-2">
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
            </div>
          </div>
        </form>
      </FormModal>

      <ConfirmationModal
        isOpen={revokeTarget !== null}
        onClose={() => setRevokeTarget(null)}
        onConfirm={() => void confirmRevoke()}
        title="Einladung widerrufen?"
        confirmText="Widerrufen"
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

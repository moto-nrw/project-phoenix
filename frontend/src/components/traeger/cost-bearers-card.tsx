"use client";

// Kostenträger des Trägers (#5627): Stellen, die den Ferienbeitrag einer
// Familie übernehmen, zum Beispiel das Jugendamt bei Leistungsbezug. Das
// Träger-Büro wählt einen aktiven Kostenträger beim Bestätigen einer
// Anmeldung. Ein Kostenträger, den ein Beitrag oder eine Rechnung nennt,
// lässt sich nur ausschalten, nicht löschen.

import { Building2, Plus } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EditActions } from "~/components/ui/edit-actions";
import { EmptyState } from "~/components/ui/empty-state";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { Textarea } from "~/components/ui/textarea";
import {
  useApiErrorDisplay,
  useApiFormError,
  useToast,
} from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type CostBearer,
  type CostBearerInput,
  deleteCostBearer,
  saveCostBearer,
} from "~/lib/traeger-contributions-api";

const logger = createLogger({ component: "CostBearersCard" });

/** The cost bearer the form writes: a new one or an existing one. */
type CostBearerEdit = { readonly bearer: CostBearer | null };

const EMPTY: CostBearerInput = {
  name: "",
  address: "",
  fileReference: "",
  active: true,
};

function CostBearerModal({
  edit,
  onClose,
  onSaved,
}: Readonly<{
  edit: CostBearerEdit | null;
  onClose: () => void;
  onSaved: () => Promise<unknown>;
}>) {
  const [draft, setDraft] = useState<CostBearerInput>(EMPTY);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  useEffect(() => {
    if (!edit) return;
    setDraft(edit.bearer ?? EMPTY);
    clearErrors();
  }, [edit, clearErrors]);

  const submit = async () => {
    if (!edit) return;
    formErrors.clear();
    if (!draft.name.trim()) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", {
        name: "Bitte geben Sie den Namen ein.",
      });
      return;
    }
    setSaving(true);
    try {
      await saveCostBearer(edit.bearer?.id ?? null, draft);
      success("Der Kostenträger ist gespeichert.");
      onClose();
      await onSaved();
    } catch (err) {
      logger.error("cost_bearer_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "der Kostenträger" });
    } finally {
      setSaving(false);
    }
  };

  const set = (patch: Partial<CostBearerInput>) =>
    setDraft((current) => ({ ...current, ...patch }));

  return (
    <FormModal
      isOpen={edit !== null}
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title={edit?.bearer ? "Kostenträger ändern" : "Kostenträger hinzufügen"}
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
        className="space-y-4"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <Input
          id="cost-bearer-name"
          name="name"
          label="Name"
          required
          placeholder="Fachdienst Jugend des Landkreises"
          maxLength={200}
          value={draft.name}
          error={formErrors.fieldError("name") ?? undefined}
          onChange={(event) => set({ name: event.target.value })}
        />
        <Textarea
          id="cost-bearer-address"
          name="address"
          label="Anschrift (freiwillig)"
          maxLength={500}
          value={draft.address}
          error={formErrors.fieldError("address") ?? undefined}
          onChange={(event) => set({ address: event.target.value })}
        />
        <Input
          id="cost-bearer-file-reference"
          name="file_reference"
          label="Ihr Aktenzeichen beim Kostenträger (freiwillig)"
          maxLength={100}
          value={draft.fileReference}
          error={formErrors.fieldError("file_reference") ?? undefined}
          onChange={(event) => set({ fileReference: event.target.value })}
        />
        <p className="text-sm text-gray-600">
          Name, Anschrift und Aktenzeichen stehen auf der Liste für die
          Erstattung.
        </p>
      </form>
    </FormModal>
  );
}

export function CostBearersCard({
  bearers,
  onChanged,
}: Readonly<{
  bearers: readonly CostBearer[];
  onChanged: () => Promise<unknown>;
}>) {
  const [edit, setEdit] = useState<CostBearerEdit | null>(null);
  const [deleting, setDeleting] = useState<CostBearer | null>(null);
  const [busy, setBusy] = useState(false);
  const deleteErrors = useApiFormError();
  const { show: showActionError } = useApiErrorDisplay();
  const { success } = useToast();

  const switchActive = async (bearer: CostBearer) => {
    try {
      await saveCostBearer(bearer.id, { ...bearer, active: !bearer.active });
      success(
        bearer.active
          ? "Der Kostenträger ist ausgeschaltet."
          : "Der Kostenträger ist eingeschaltet.",
      );
      await onChanged();
    } catch (err) {
      logger.error("cost_bearer_switch_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showActionError(err, { object: "der Kostenträger" });
    }
  };

  const remove = async () => {
    if (!deleting) return;
    setBusy(true);
    deleteErrors.clear();
    try {
      await deleteCostBearer(deleting.id);
      success("Der Kostenträger ist gelöscht.");
      setDeleting(null);
      await onChanged();
    } catch (err) {
      logger.error("cost_bearer_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void deleteErrors.show(err, { object: "das Löschen des Kostenträgers" });
    } finally {
      setBusy(false);
    }
  };

  const columns: DataTableColumn<CostBearer>[] = [
    {
      key: "name",
      header: "Name",
      render: (bearer) => (
        <span className="inline-flex flex-wrap items-center gap-2">
          <span className="font-medium wrap-anywhere text-gray-900">
            {bearer.name}
          </span>
          {bearer.active ? null : (
            <StatusBadge tone="gray" label="Ausgeschaltet" />
          )}
        </span>
      ),
      stacked: "title",
    },
    {
      key: "address",
      header: "Anschrift",
      render: (bearer) => (
        <span className="wrap-anywhere whitespace-pre-line">
          {bearer.address || "–"}
        </span>
      ),
      stacked: "field",
    },
    {
      key: "fileReference",
      header: "Aktenzeichen",
      render: (bearer) => (
        <span className="wrap-anywhere">{bearer.fileReference || "–"}</span>
      ),
      stacked: "field",
    },
    {
      key: "actions",
      header: "",
      align: "right",
      stacked: "meta",
      stackedLabel: false,
      render: (bearer) => (
        <OverflowMenu
          ariaLabel={`Aktionen für ${bearer.name}`}
          triggerSize="sm"
          items={[
            { label: "Ändern", onClick: () => setEdit({ bearer }) },
            {
              label: bearer.active ? "Ausschalten" : "Einschalten",
              onClick: () => void switchActive(bearer),
            },
            ...(bearer.inUse
              ? []
              : [
                  {
                    label: "Löschen",
                    destructive: true,
                    onClick: () => setDeleting(bearer),
                  },
                ]),
          ]}
        />
      ),
    },
  ];

  return (
    <SectionCard
      title="Kostenträger"
      description="Stellen, die den Beitrag für eine Familie bezahlen, zum Beispiel das Jugendamt. Sie wählen den Kostenträger beim Bestätigen der Anmeldung. Wurde er schon gewählt, können Sie ihn nur ausschalten."
      actions={
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={() => setEdit({ bearer: null })}
        >
          <Plus className="mr-2 h-4 w-4" aria-hidden="true" />
          Kostenträger hinzufügen
        </Button>
      }
    >
      <DataTable
        columns={columns}
        rows={[...bearers]}
        getRowKey={(bearer) => bearer.id}
        rowHasInteractiveControls
        stackedOnMobile
        emptyState={
          <EmptyState
            icon={<Building2 className="h-12 w-12" aria-hidden="true" />}
            title="Kein Kostenträger. Beim Bestätigen bezahlt die Familie oder ist befreit."
          />
        }
      />
      <CostBearerModal
        edit={edit}
        onClose={() => setEdit(null)}
        onSaved={onChanged}
      />
      <ConfirmDeleteModal
        isOpen={deleting !== null}
        onClose={() => {
          setDeleting(null);
          deleteErrors.clear();
        }}
        onConfirm={() => void remove()}
        title="Kostenträger löschen?"
        description="Er steht danach nicht mehr zur Wahl."
        confirmLabel="Löschen"
        loadingLabel="Wird gelöscht …"
        gate={{ mode: "twoStep", firstStepLabel: "Ja, löschen" }}
        loading={busy}
        error={deleteErrors.error}
      />
    </SectionCard>
  );
}

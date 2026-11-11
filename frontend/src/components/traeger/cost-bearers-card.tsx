"use client";

// Kostenträger des Trägers (#5627): Stellen, die den Ferienbeitrag einer
// Familie übernehmen, zum Beispiel das Jugendamt bei Leistungsbezug. Das
// Träger-Büro wählt einen aktiven Kostenträger beim Bestätigen einer
// Anmeldung. Ein Kostenträger, den ein Beitrag oder eine Rechnung nennt,
// lässt sich nur ausschalten, nicht löschen. Neu angelegt wird im Dialog;
// „Ändern“ macht die Zeile selbst zum Formular (Bauart 2, kein Overlay).

import { Building2, Pencil, Plus, Trash2 } from "lucide-react";
import { type RefObject, useEffect, useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { EditActions } from "~/components/ui/edit-actions";
import { EmptyState } from "~/components/ui/empty-state";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
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

const EMPTY: CostBearerInput = {
  name: "",
  address: "",
  fileReference: "",
  active: true,
};

type FormErrors = ReturnType<typeof useApiFormError>;

/**
 * Saves a cost bearer from a draft: a new one for a null id. The name is
 * required; the backend checks the rest. Returns whether it was saved.
 */
function useCostBearerSave(
  formRef: RefObject<HTMLFormElement | null>,
): readonly [
  (id: string | null, draft: CostBearerInput) => Promise<boolean>,
  boolean,
  FormErrors,
] {
  const [saving, setSaving] = useState(false);
  const formErrors = useApiFormError(formRef);
  const { success } = useToast();
  const save = async (id: string | null, draft: CostBearerInput) => {
    formErrors.clear();
    if (!draft.name.trim()) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", {
        name: "Bitte geben Sie den Namen ein.",
      });
      return false;
    }
    setSaving(true);
    try {
      await saveCostBearer(id, draft);
      success("Der Kostenträger ist gespeichert.");
      return true;
    } catch (err) {
      logger.error("cost_bearer_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "der Kostenträger" });
      return false;
    } finally {
      setSaving(false);
    }
  };
  return [save, saving, formErrors] as const;
}

/** The fields of a cost bearer, in the dialog and in the row. */
function CostBearerFields({
  idPrefix,
  draft,
  formErrors,
  onChange,
}: Readonly<{
  idPrefix: string;
  draft: CostBearerInput;
  formErrors: FormErrors;
  onChange: (patch: Partial<CostBearerInput>) => void;
}>) {
  return (
    <>
      <Input
        controlSize="touch"
        id={`${idPrefix}-name`}
        name="name"
        label="Name"
        required
        placeholder="Fachdienst Jugend des Landkreises"
        maxLength={200}
        value={draft.name}
        error={formErrors.fieldError("name") ?? undefined}
        onChange={(event) => onChange({ name: event.target.value })}
      />
      <Textarea
        id={`${idPrefix}-address`}
        name="address"
        label="Anschrift (freiwillig)"
        maxLength={500}
        value={draft.address}
        error={formErrors.fieldError("address") ?? undefined}
        onChange={(event) => onChange({ address: event.target.value })}
      />
      <Input
        controlSize="touch"
        id={`${idPrefix}-file-reference`}
        name="file_reference"
        label="Ihr Aktenzeichen beim Kostenträger (freiwillig)"
        maxLength={100}
        value={draft.fileReference}
        error={formErrors.fieldError("file_reference") ?? undefined}
        onChange={(event) => onChange({ fileReference: event.target.value })}
      />
    </>
  );
}

/** Adds a new cost bearer. Existing ones change in their row. */
function CostBearerCreateModal({
  isOpen,
  onClose,
  onSaved,
}: Readonly<{
  isOpen: boolean;
  onClose: () => void;
  onSaved: () => Promise<unknown>;
}>) {
  const [draft, setDraft] = useState<CostBearerInput>(EMPTY);
  const formRef = useRef<HTMLFormElement>(null);
  const [save, saving, formErrors] = useCostBearerSave(formRef);
  const { clear: clearErrors } = formErrors;

  useEffect(() => {
    if (!isOpen) return;
    setDraft(EMPTY);
    clearErrors();
  }, [isOpen, clearErrors]);

  const submit = async () => {
    if (!(await save(null, draft))) return;
    onClose();
    await onSaved();
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title="Kostenträger hinzufügen"
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
        <CostBearerFields
          idPrefix="cost-bearer"
          draft={draft}
          formErrors={formErrors}
          onChange={(patch) =>
            setDraft((current) => ({ ...current, ...patch }))
          }
        />
        <p className="text-sm text-gray-600">
          Name, Anschrift und Aktenzeichen stehen auf der Liste für die
          Erstattung.
        </p>
      </form>
    </FormModal>
  );
}

/** One cost bearer in its row while it is changed. */
function CostBearerEditRow({
  bearer,
  onDone,
  onSaved,
}: Readonly<{
  bearer: CostBearer;
  onDone: () => void;
  onSaved: () => Promise<unknown>;
}>) {
  const [draft, setDraft] = useState<CostBearerInput>(bearer);
  const formRef = useRef<HTMLFormElement>(null);
  const [save, saving, formErrors] = useCostBearerSave(formRef);
  const idPrefix = `cost-bearer-${bearer.id}`;

  useEffect(() => {
    document.getElementById(`${idPrefix}-name`)?.focus();
  }, [idPrefix]);

  const submit = async () => {
    if (!(await save(bearer.id, draft))) return;
    onDone();
    await onSaved();
  };

  return (
    <li className="py-3 first:pt-0 last:pb-0 sm:col-span-2">
      <form
        ref={formRef}
        noValidate
        aria-label={`${bearer.name} ändern`}
        className="space-y-4"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <FormErrorAlert message={formErrors.error} />
        <CostBearerFields
          idPrefix={idPrefix}
          draft={draft}
          formErrors={formErrors}
          onChange={(patch) =>
            setDraft((current) => ({ ...current, ...patch }))
          }
        />
        <EditActions onCancel={onDone} saving={saving} saveType="submit" />
      </form>
    </li>
  );
}

export function CostBearersCard({
  bearers,
  onChanged,
}: Readonly<{
  bearers: readonly CostBearer[];
  onChanged: () => Promise<unknown>;
}>) {
  const [creating, setCreating] = useState(false);
  // Only one row is a form at a time.
  const [editingId, setEditingId] = useState<string | null>(null);
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

  return (
    <SectionCard
      title="Kostenträger"
      description="Stellen, die den Beitrag für eine Familie bezahlen, zum Beispiel das Jugendamt. Sie wählen den Kostenträger beim Bestätigen der Anmeldung. Wurde er schon gewählt, können Sie ihn nur ausschalten."
      actions={
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={() => setCreating(true)}
        >
          <Plus className="mr-2 h-4 w-4" aria-hidden="true" />
          Kostenträger hinzufügen
        </Button>
      }
    >
      {bearers.length === 0 ? (
        <EmptyState
          icon={<Building2 className="h-12 w-12" aria-hidden="true" />}
          title="Kein Kostenträger. Beim Bestätigen bezahlt die Familie oder ist befreit."
        />
      ) : (
        <ResponsiveActionList>
          {bearers.map((bearer) =>
            editingId === bearer.id ? (
              <CostBearerEditRow
                key={bearer.id}
                bearer={bearer}
                onDone={() => setEditingId(null)}
                onSaved={onChanged}
              />
            ) : (
              <li
                key={bearer.id}
                className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-start"
              >
                <div className="min-w-0 space-y-0.5 text-sm text-gray-700">
                  <p className="flex flex-wrap items-center gap-2">
                    <span className="font-semibold wrap-anywhere text-gray-900">
                      {bearer.name}
                    </span>
                    {bearer.active ? null : (
                      <StatusBadge tone="gray" label="Ausgeschaltet" />
                    )}
                  </p>
                  {bearer.address ? (
                    <p className="wrap-anywhere whitespace-pre-line">
                      {bearer.address}
                    </p>
                  ) : null}
                  {bearer.fileReference ? (
                    <p className="wrap-anywhere">
                      Aktenzeichen: {bearer.fileReference}
                    </p>
                  ) : null}
                </div>
                <OverflowMenu
                  ariaLabel={`Aktionen für ${bearer.name}`}
                  items={[
                    {
                      icon: <Pencil className="size-4" aria-hidden />,
                      label: "Ändern",
                      onClick: () => setEditingId(bearer.id),
                    },
                    {
                      label: bearer.active ? "Ausschalten" : "Einschalten",
                      onClick: () => void switchActive(bearer),
                    },
                    ...(bearer.inUse
                      ? []
                      : [
                          {
                            label: "Löschen",
                            icon: <Trash2 className="size-4" aria-hidden />,
                            destructive: true,
                            onClick: () => setDeleting(bearer),
                          },
                        ]),
                  ]}
                />
              </li>
            ),
          )}
        </ResponsiveActionList>
      )}
      <CostBearerCreateModal
        isOpen={creating}
        onClose={() => setCreating(false)}
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

"use client";

// Eine Personalakte des Trägers (#4705, ADR 0089): die Person, die
// Personalnummer des Trägers, die Einsatzorte, die Bank- und Steuerdaten und
// der Verlauf. Die Personalnummer und die Bankdaten ändert nur die
// Personalverwaltung; die Schulen sehen sie nur noch als „vom Träger“.

import { EyeIcon, PencilSimpleIcon, TrashIcon } from "@phosphor-icons/react";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import {
  STAFF_FIELD_LABELS,
  schoolCount,
  staffChangeText,
} from "~/components/traeger/staff/staff-record-format";
import { Button } from "~/components/ui/button";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { EmptyState } from "~/components/ui/empty-state";
import { FramedList } from "~/components/ui/framed-list";
import { Input } from "~/components/ui/input";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import {
  useApiErrorDisplay,
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { changeHistoryConcept } from "~/lib/concepts/changeHistory";
import { payrollConcept } from "~/lib/concepts/payroll";
import { schoolsConcept } from "~/lib/concepts/schools";
import { staffConcept } from "~/lib/concepts/staff";
import { formatBerlinDate, formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  dissolveStaffRecord,
  fetchStaffRecord,
  revealStaffFinancial,
  staffName,
  updateStaffFinancial,
  updateStaffPersonnelNumber,
  type StaffFinancial,
  type StaffRecordDetail,
} from "~/lib/traeger-staff-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerStaffRecordPage" });

const errorMessage = (err: unknown) =>
  err instanceof Error ? err.message : String(err);

const FINANCIAL_FIELDS = [
  ["iban", "IBAN", "DE…"],
  ["taxId", "Steuer-ID", "11 Ziffern"],
  ["socialSecurityNumber", "Sozialversicherungsnummer", "12 Zeichen"],
] as const satisfies readonly (readonly [
  keyof StaffFinancial,
  string,
  string,
])[];

function PersonnelNumberCard({
  record,
  onSaved,
}: Readonly<{
  record: StaffRecordDetail;
  onSaved: (record: StaffRecordDetail) => void;
}>) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");
  const [saving, setSaving] = useState(false);
  const errors = useApiFormError();
  const { success } = useToast();

  const save = async () => {
    const value = draft.trim();
    if (value !== "" && !/^\d{1,9}$/.test(value)) {
      errors.invalid("Bitte prüfen Sie die Personalnummer.", {
        personnel_number: "Die Personalnummer hat 1 bis 9 Ziffern.",
      });
      return;
    }
    setSaving(true);
    try {
      const saved = await updateStaffPersonnelNumber(record.id, value || null);
      setEditing(false);
      success("Die Personalnummer ist gespeichert. Alle Schulen sehen sie.");
      onSaved(saved);
    } catch (err) {
      logger.error("traeger_staff_personnel_number_failed", {
        error: errorMessage(err),
      });
      void errors.show(err, { object: "die Personalnummer" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <SectionCard
      concept={payrollConcept}
      title="Personalnummer"
      description="Die Nummer aus Ihrem Lohnsystem. Sie gilt an allen Schulen dieser Person und ist bei Ihrem Träger nur einmal vergeben."
      action={
        editing ? undefined : (
          <Button
            type="button"
            variant="outline"
            className="gap-2"
            tooltip="Öffnet das Feld, um die Personalnummer einzutragen oder zu ändern."
            onClick={() => {
              setDraft(record.personnelNumber ?? "");
              errors.clear();
              setEditing(true);
            }}
          >
            <PencilSimpleIcon className="h-4 w-4" aria-hidden="true" />
            Ändern
          </Button>
        )
      }
      footer={
        editing ? (
          <EditActions
            onCancel={() => setEditing(false)}
            onSave={() => void save()}
            saving={saving}
          />
        ) : undefined
      }
    >
      {editing ? (
        <Input
          id="traeger-staff-personnel-number"
          label="Personalnummer"
          hint="Leer lassen, wenn es noch keine Nummer gibt."
          inputMode="numeric"
          autoComplete="off"
          value={draft}
          error={errors.fieldError("personnel_number") ?? errors.error?.message}
          onChange={(event) => setDraft(event.target.value)}
        />
      ) : (
        <p className="font-mono text-2xl font-semibold tracking-tight text-gray-900 tabular-nums">
          {record.personnelNumber ?? "–"}
        </p>
      )}
    </SectionCard>
  );
}

function FinancialCard({
  record,
  onSaved,
}: Readonly<{ record: StaffRecordDetail; onSaved: () => void }>) {
  const [revealed, setRevealed] = useState<StaffFinancial | null>(null);
  const [revealing, setRevealing] = useState(false);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState<StaffFinancial>(record.financial);
  const [saving, setSaving] = useState(false);
  const errors = useApiFormError();
  const actionErrors = useApiErrorDisplay();
  const { success } = useToast();
  const shown = revealed ?? record.financial;

  const reveal = async () => {
    setRevealing(true);
    try {
      setRevealed(await revealStaffFinancial(record.id));
    } catch (err) {
      logger.error("traeger_staff_financial_reveal_failed", {
        error: errorMessage(err),
      });
      void actionErrors.show(err, { object: "die Bank- und Steuerdaten" });
    } finally {
      setRevealing(false);
    }
  };

  const startEdit = async () => {
    errors.clear();
    let values = revealed;
    if (!values && record.hasFinancial) {
      try {
        values = await revealStaffFinancial(record.id);
        setRevealed(values);
      } catch (err) {
        logger.error("traeger_staff_financial_reveal_failed", {
          error: errorMessage(err),
        });
        void actionErrors.show(err, { object: "die Bank- und Steuerdaten" });
        return;
      }
    }
    setDraft(values ?? { iban: null, taxId: null, socialSecurityNumber: null });
    setEditing(true);
  };

  const save = async () => {
    setSaving(true);
    try {
      const clean = (value: string | null) =>
        value && value.trim() !== "" ? value.trim() : null;
      await updateStaffFinancial(record.id, {
        iban: clean(draft.iban),
        taxId: clean(draft.taxId),
        socialSecurityNumber: clean(draft.socialSecurityNumber),
      });
      setEditing(false);
      setRevealed(null);
      success("Die Bank- und Steuerdaten sind gespeichert.");
      onSaved();
    } catch (err) {
      logger.error("traeger_staff_financial_save_failed", {
        error: errorMessage(err),
      });
      void errors.show(err, { object: "die Bank- und Steuerdaten" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <SectionCard
      concept={payrollConcept}
      title="Bank und Steuer"
      description="Nur die Personalverwaltung sieht und ändert diese Werte. Jedes Anzeigen steht im Zugriffsprotokoll der Schulen."
      action={
        editing ? undefined : (
          <span className="flex flex-wrap gap-2">
            {record.hasFinancial && !revealed ? (
              <Button
                type="button"
                variant="outline"
                className="gap-2"
                isLoading={revealing}
                loadingText="Wird geöffnet …"
                tooltip="Zeigt die vollständigen Werte. Das steht im Zugriffsprotokoll der Schulen."
                onClick={() => void reveal()}
              >
                <EyeIcon className="h-4 w-4" aria-hidden="true" />
                Anzeigen
              </Button>
            ) : null}
            <Button
              type="button"
              variant="outline"
              className="gap-2"
              tooltip="Öffnet die Felder für IBAN, Steuer-ID und Sozialversicherungsnummer."
              onClick={() => void startEdit()}
            >
              <PencilSimpleIcon className="h-4 w-4" aria-hidden="true" />
              Ändern
            </Button>
          </span>
        )
      }
      footer={
        editing ? (
          <EditActions
            onCancel={() => setEditing(false)}
            onSave={() => void save()}
            saving={saving}
          />
        ) : undefined
      }
    >
      {editing ? (
        <div className="grid gap-4 sm:grid-cols-3">
          {FINANCIAL_FIELDS.map(([key, label, placeholder]) => (
            <Input
              key={key}
              id={`traeger-staff-${key}`}
              label={label}
              placeholder={placeholder}
              autoComplete="off"
              value={draft[key] ?? ""}
              error={key === "iban" ? errors.error?.message : undefined}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  [key]: event.target.value,
                }))
              }
            />
          ))}
        </div>
      ) : (
        <DataGrid columns={4}>
          {FINANCIAL_FIELDS.map(([key, label]) => (
            <DataField key={key} label={label} mono>
              {shown[key] ?? "Nicht hinterlegt"}
            </DataField>
          ))}
        </DataGrid>
      )}
    </SectionCard>
  );
}

export default function TraegerStaffRecordPage() {
  const params = useParams<{ id: string }>();
  const recordId = params?.id ?? "";
  const router = useRouter();
  const { success } = useToast();
  const [record, setRecord] = useState<StaffRecordDetail | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [dissolving, setDissolving] = useState(false);
  const dissolveErrors = useApiFormError();
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;

  const load = useCallback(async () => {
    try {
      setRecord(await fetchStaffRecord(recordId));
      clearLoadError();
    } catch (err) {
      logger.error("traeger_staff_record_load_failed", {
        error: errorMessage(err),
      });
      await showLoadError(err, {
        object: "die Personalakte",
        retry: () => void load(),
      });
    }
  }, [recordId, clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const dissolve = async () => {
    if (!record) return;
    setDissolving(true);
    try {
      await dissolveStaffRecord(record.id);
      success(`Die Akte von ${staffName(record)} ist aufgelöst.`);
      router.push(traegerPath("/traeger/personal"));
    } catch (err) {
      logger.error("traeger_staff_record_dissolve_failed", {
        error: errorMessage(err),
      });
      void dissolveErrors.show(err, { object: "die Personalakte" });
    } finally {
      setDissolving(false);
    }
  };

  const activeSchools =
    record?.employments.filter((employment) => employment.active).length ?? 0;

  return (
    <TenantPage
      title={record ? staffName(record) : "Personalakte"}
      leading={<ConceptIconTile concept={staffConcept} size="page" />}
      back
      backHref={traegerPath("/traeger/personal")}
      backLabel="Zurück zu den Personalakten"
      stats={
        record ? (
          <TenantPageStats
            items={[
              {
                value: activeSchools,
                label: activeSchools === 1 ? "Einsatzort" : "Einsatzorte",
              },
              {
                value: record.personnelNumber ?? "–",
                label: "Personalnummer",
              },
            ]}
          />
        ) : null
      }
      statsLoading={!record && !pageLoad.error}
      error={pageLoad.error}
      loading={!record && !pageLoad.error}
      actions={
        record ? (
          <OverflowMenu
            ariaLabel="Weitere Aktionen zur Personalakte"
            items={[
              {
                label: "Akte auflösen",
                description:
                  "moto fragt vorher nach. Die Schulen bekommen ihre Bankdaten zurück.",
                icon: <TrashIcon className="size-4" aria-hidden="true" />,
                destructive: true,
                onClick: () => {
                  dissolveErrors.clear();
                  setConfirming(true);
                },
              },
            ]}
          />
        ) : undefined
      }
      overlays={
        record ? (
          <ConfirmDeleteModal
            isOpen={confirming}
            onClose={() => {
              if (dissolving) return;
              setConfirming(false);
            }}
            onConfirm={() => void dissolve()}
            title="Akte auflösen?"
            description={
              <div className="space-y-3">
                <p>
                  Die Akte von{" "}
                  <span className="font-medium text-gray-900">
                    {staffName(record)}
                  </span>{" "}
                  wird aufgelöst. Danach führt jede Schule die Person wieder
                  selbst.
                </p>
                <p>
                  Jede Schule mit laufender Beschäftigung bekommt die Bank- und
                  Steuerdaten zurück. Die Personalnummer bleibt an den Schulen
                  stehen. Der Verlauf der Akte wird gelöscht.
                </p>
              </div>
            }
            gate={{ mode: "twoStep" }}
            confirmLabel="Akte auflösen"
            loadingLabel="Wird aufgelöst …"
            loading={dissolving}
            error={dissolveErrors.error}
          />
        ) : null
      }
    >
      {record ? (
        <>
          <SectionCard
            concept={staffConcept}
            title="Person"
            description="Name und Geburtstag haben Sie beim Verknüpfen gewählt. Die Schulen pflegen ihre eigenen Angaben weiter selbst."
          >
            <DataGrid columns={4}>
              <DataField label={STAFF_FIELD_LABELS.first_name}>
                {record.firstName}
              </DataField>
              <DataField label={STAFF_FIELD_LABELS.last_name}>
                {record.lastName}
              </DataField>
              <DataField label={STAFF_FIELD_LABELS.birthday}>
                {record.birthday ? formatDate(record.birthday) : "–"}
              </DataField>
              <DataField label="Schulen">
                {schoolCount(record.employments.length)}
              </DataField>
            </DataGrid>
          </SectionCard>

          <div className="grid gap-6 lg:grid-cols-2">
            <PersonnelNumberCard record={record} onSaved={setRecord} />
            <SectionCard
              concept={schoolsConcept}
              title="Einsatzorte"
              description="An diesen Schulen arbeitet die Person. Jede Schule plant und erfasst ihre Zeiten wie bisher."
            >
              <FramedList>
                {record.employments.map((employment) => (
                  <li
                    key={`${employment.schoolId}-${employment.staffId}`}
                    className="flex items-center justify-between gap-3 px-4 py-3"
                  >
                    <span className="min-w-0 truncate text-sm font-medium text-gray-900">
                      {employment.schoolName}
                    </span>
                    <StatusBadge
                      label={employment.active ? "Beschäftigt" : "Beendet"}
                      tone={employment.active ? "green" : "gray"}
                    />
                  </li>
                ))}
              </FramedList>
            </SectionCard>
          </div>

          <FinancialCard record={record} onSaved={() => void load()} />

          <SectionCard
            concept={changeHistoryConcept}
            title="Verlauf"
            description="Was beim Verknüpfen nicht übernommen wurde und was sich seitdem geändert hat. Bank- und Steuerwerte stehen nur maskiert hier."
          >
            {record.changes.length > 0 ? (
              <FramedList>
                {record.changes.map((change, index) => (
                  <li
                    key={`${change.occurredAt}-${index}`}
                    className="flex flex-col gap-0.5 px-4 py-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4"
                  >
                    <span className="min-w-0 text-sm wrap-anywhere text-gray-900">
                      {staffChangeText(change)}
                    </span>
                    <span className="shrink-0 text-sm text-gray-500 tabular-nums">
                      {formatBerlinDate(change.occurredAt)}
                    </span>
                  </li>
                ))}
              </FramedList>
            ) : (
              <EmptyState title="Noch kein Eintrag." variant="compact" />
            )}
          </SectionCard>
        </>
      ) : null}
    </TenantPage>
  );
}

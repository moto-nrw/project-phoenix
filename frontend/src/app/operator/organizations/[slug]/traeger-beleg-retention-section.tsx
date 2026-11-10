"use client";

import { useCallback, useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { FormModal } from "~/components/ui/form-modal";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { Input } from "~/components/ui/input";
import { ConsentCheckbox } from "~/components/ui/consent-checkbox";
import { ConfirmationModal } from "~/components/ui/modal";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { Textarea } from "~/components/ui/textarea";
import {
  useApiErrorDisplay,
  useApiFormError,
  useToast,
} from "~/contexts/ToastContext";
import { berlinTodayISO, formatDate } from "~/lib/date-helpers";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import {
  BASES_BY_TYPE,
  BASIS_LABELS,
  BELEG_TYPES,
  CLARIFICATION_REASON_LABELS,
  TRIGGERS_BY_TYPE,
  TRIGGER_LABELS,
  belegRetentionService,
  belegTypeLabel,
  minimumPeriod,
  periodUnit,
  type BelegClarification,
  type BelegDeletion,
  type BelegSummary,
  type BelegType,
  type RetentionInstruction,
  type RetentionSuspension,
} from "~/lib/operator/beleg-retention-api";
import { useSWRAuth } from "~/lib/swr";
import { operatorProvisioningKeys } from "~/lib/swr/portal-keys";
import { ClarificationReviewModal } from "./traeger-beleg-clarification-review-modal";

const logger = createLogger({ component: "TraegerBelegRetentionSection" });

const TYPE_OPTIONS = BELEG_TYPES.map((type) => ({
  value: type,
  label: belegTypeLabel(type),
}));

const errorMessage = (error: unknown) =>
  error instanceof Error ? error.message : String(error);

/**
 * Belegaufbewahrung eines Trägers (#4720, ADR 0184): das moto-Team erfasst
 * die Weisung des Trägers je Belegart, sieht die Klärungsliste mit
 * Prüfdatum und Eskalation, setzt die Löschung aus und sieht, was gelöscht
 * wurde. Ohne Weisung löscht moto nichts.
 */
export function TraegerBelegRetentionSection({
  organizationId,
}: {
  readonly organizationId: string;
}) {
  const { data, error, isLoading, mutate } = useSWRAuth(
    operatorProvisioningKeys.orgBelegRetention(organizationId),
    () => belegRetentionService.getOverview(organizationId),
    { keepPreviousData: false },
  );
  const loadError = useSwrLoadError(
    error,
    "die Belegaufbewahrung",
    () => void mutate(),
  );
  const [instructionOpen, setInstructionOpen] = useState(false);
  const [suspensionOpen, setSuspensionOpen] = useState(false);
  // Each opening is a new form: a Weisung never inherits the confirmation
  // or the fields of the previous one.
  const [formSession, setFormSession] = useState(0);
  const [reviewing, setReviewing] = useState<BelegClarification | null>(null);
  const reload = useCallback(() => mutate(), [mutate]);

  if (loadError) return <LoadErrorAlert error={loadError} />;
  const loading = isLoading || data === undefined;
  const today = berlinTodayISO();

  return (
    <>
      <SectionCard
        title="Klärungsliste"
        description="Belege ohne Weisung oder ohne berechenbares Datum. moto löscht sie nicht. Klären Sie sie bis zum Prüfdatum mit dem Träger und erfassen Sie jede Prüfung."
      >
        <DataTable
          columns={clarificationColumns(today, setReviewing)}
          rows={[...(data?.clarifications ?? [])]}
          getRowKey={(row) => `${row.belegType}-${row.reason}`}
          isLoading={loading}
          rowHasInteractiveControls
          stackedOnMobile
          emptyState={
            <EmptyState
              variant="compact"
              title="Nichts zu klären"
              description={
                data?.evaluatedOn
                  ? `Stand ${formatDate(data.evaluatedOn)}.`
                  : "Die erste Bewertung läuft mit dem nächsten stündlichen Lauf."
              }
            />
          }
        />
      </SectionCard>

      <SectionCard
        title="Weisungen des Trägers"
        description="Je Belegart die dokumentierte Weisung. Erst mit einer Weisung löscht moto nach Fristende."
        action={
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => {
              setFormSession((session) => session + 1);
              setInstructionOpen(true);
            }}
          >
            Weisung erfassen
          </Button>
        }
      >
        <DataTable
          columns={instructionColumns}
          rows={[...(data?.instructions ?? [])]}
          getRowKey={(row) => row.id}
          isLoading={loading}
          stackedOnMobile
          emptyState={
            <EmptyState
              variant="compact"
              title="Keine Weisung"
              description="Ohne Weisung bleiben alle Belege erhalten."
            />
          }
        />
      </SectionCard>

      <SuspensionsCard
        organizationId={organizationId}
        suspensions={data?.suspensions ?? []}
        loading={loading}
        onAdd={() => {
          setFormSession((session) => session + 1);
          setSuspensionOpen(true);
        }}
        onChanged={reload}
      />

      <SectionCard
        title="Stand je Belegart"
        description="Wie viele Belege eine Frist haben, noch laufen, offen oder ausgesetzt sind."
      >
        <DataTable
          columns={summaryColumns}
          rows={[...(data?.summary ?? [])]}
          getRowKey={(row) => row.belegType}
          isLoading={loading}
          stackedOnMobile
          emptyState={
            <EmptyState variant="compact" title="Noch keine Belege bewertet" />
          }
        />
      </SectionCard>

      <SectionCard
        title="Löschprotokoll"
        description="Was moto nach Fristende gelöscht hat, ohne Inhalt."
      >
        <DataTable
          columns={deletionColumns}
          rows={[...(data?.deletions ?? [])]}
          getRowKey={(row) => `${row.belegType}-${row.deletedOn}`}
          isLoading={loading}
          stackedOnMobile
          emptyState={
            <EmptyState variant="compact" title="Noch nichts gelöscht" />
          }
        />
      </SectionCard>

      <InstructionModal
        key={`instruction-${formSession}`}
        isOpen={instructionOpen}
        organizationId={organizationId}
        onClose={() => setInstructionOpen(false)}
        onSaved={reload}
      />
      <SuspensionModal
        key={`suspension-${formSession}`}
        isOpen={suspensionOpen}
        organizationId={organizationId}
        onClose={() => setSuspensionOpen(false)}
        onSaved={reload}
      />
      <ClarificationReviewModal
        entry={reviewing}
        organizationId={organizationId}
        onClose={() => setReviewing(null)}
        onSaved={reload}
      />
    </>
  );
}

function clarificationColumns(
  today: string,
  onReview: (entry: BelegClarification) => void,
): DataTableColumn<BelegClarification>[] {
  return [
    {
      key: "type",
      header: "Belegart",
      render: (row) => belegTypeLabel(row.belegType),
      stacked: "title",
    },
    {
      key: "reason",
      header: "Grund",
      render: (row) => CLARIFICATION_REASON_LABELS[row.reason] ?? row.reason,
    },
    { key: "count", header: "Belege", render: (row) => row.count },
    {
      key: "listed",
      header: "Aufgenommen",
      render: (row) => formatDate(row.listedOn),
    },
    {
      key: "review",
      header: "Prüfen bis",
      render: (row) =>
        row.overdue > 0 || row.reviewOn < today ? (
          <StatusBadge
            tone="red"
            label={`${formatDate(row.reviewOn)} · überfällig, Eskalation`}
          />
        ) : (
          formatDate(row.reviewOn)
        ),
    },
    {
      key: "responsible",
      header: "Zuständig",
      render: (row) => row.responsible,
    },
    {
      key: "last-review",
      header: "Letzte Prüfung",
      render: (row) => <LastReview entry={row} />,
    },
    {
      key: "actions",
      header: "Aktionen",
      headerSrOnly: true,
      align: "right",
      stacked: "meta",
      render: (row) => (
        <OverflowMenu
          ariaLabel={`Aktionen für ${belegTypeLabel(row.belegType)}`}
          triggerSize="sm"
          items={[
            {
              label: "Prüfung erfassen",
              description: "Ergebnis, Eskalation und nächstes Prüfdatum.",
              onClick: () => onReview(row),
            },
          ]}
        />
      ),
    },
  ];
}

function LastReview({ entry }: { readonly entry: BelegClarification }) {
  const review = entry.lastReview;
  if (!review) return "Noch keine";
  return (
    <div className="space-y-1">
      <p>
        {formatDate(review.reviewedOn)}: {review.note}
      </p>
      {review.escalated && (
        <StatusBadge
          tone="orange"
          label={`An Florian eskaliert: ${review.escalationReason}`}
        />
      )}
    </div>
  );
}

const instructionColumns: DataTableColumn<RetentionInstruction>[] = [
  {
    key: "type",
    header: "Belegart",
    render: (row) => belegTypeLabel(row.belegType),
    stacked: "title",
  },
  {
    key: "period",
    header: "Frist",
    render: (row) =>
      `${row.period} ${row.period === 1 ? "Jahr" : periodUnit(row.belegType)} ab ${TRIGGER_LABELS[row.triggerEvent] ?? row.triggerEvent}`,
  },
  {
    key: "basis",
    header: "Grundlage",
    render: (row) => (
      <div className="space-y-1">
        <p>
          {BASIS_LABELS[row.basis] ?? row.basis}: {row.legalBasis}
        </p>
        {row.basisJustification && (
          <p className="text-gray-600">Begründung: {row.basisJustification}</p>
        )}
        {row.statutesNotApplicableConfirmed && (
          <p className="text-gray-600">
            Schriftlich bestätigt: AO, HGB und UStG gelten nicht
            {row.confirmedAt
              ? ` (erfasst am ${formatDate(row.confirmedAt)})`
              : ""}
          </p>
        )}
      </div>
    ),
  },
  {
    key: "by",
    header: "Weisung von",
    render: (row) => `${row.instructedBy}, ${formatDate(row.instructedOn)}`,
  },
  {
    key: "reference",
    header: "Nachweis",
    render: (row) => row.instructionReference,
  },
];

const summaryColumns: DataTableColumn<BelegSummary>[] = [
  {
    key: "type",
    header: "Belegart",
    render: (row) => belegTypeLabel(row.belegType),
    stacked: "title",
  },
  { key: "scheduled", header: "Mit Frist", render: (row) => row.scheduled },
  {
    key: "next",
    header: "Nächste Löschung",
    render: (row) => (row.nextEnd ? formatDate(row.nextEnd) : "–"),
  },
  { key: "running", header: "Läuft noch", render: (row) => row.running },
  { key: "open", header: "Zu klären", render: (row) => row.clarification },
  { key: "suspended", header: "Ausgesetzt", render: (row) => row.suspended },
];

const deletionColumns: DataTableColumn<BelegDeletion>[] = [
  {
    key: "day",
    header: "Gelöscht am",
    render: (row) => formatDate(row.deletedOn),
    stacked: "title",
  },
  {
    key: "type",
    header: "Belegart",
    render: (row) => belegTypeLabel(row.belegType),
  },
  { key: "count", header: "Anzahl", render: (row) => row.count },
];

function SuspensionsCard({
  organizationId,
  suspensions,
  loading,
  onAdd,
  onChanged,
}: {
  readonly organizationId: string;
  readonly suspensions: readonly RetentionSuspension[];
  readonly loading: boolean;
  readonly onAdd: () => void;
  readonly onChanged: () => Promise<unknown>;
}) {
  const [lifting, setLifting] = useState<RetentionSuspension | null>(null);
  const [busy, setBusy] = useState(false);
  const { show: showError } = useApiErrorDisplay();
  const { success } = useToast();

  const lift = useCallback(async () => {
    if (!lifting) return;
    setBusy(true);
    try {
      await belegRetentionService.liftSuspension(organizationId, lifting.id);
      setLifting(null);
      success("Die Aussetzung ist aufgehoben.");
      await onChanged();
    } catch (liftError) {
      logger.warn("beleg_suspension_lift_failed", {
        error: errorMessage(liftError),
      });
      void showError(liftError, { object: "die Aufhebung der Aussetzung" });
    } finally {
      setBusy(false);
    }
  }, [lifting, onChanged, organizationId, showError, success]);

  const columns: DataTableColumn<RetentionSuspension>[] = [
    {
      key: "type",
      header: "Belegart",
      render: (row) => belegTypeLabel(row.belegType),
      stacked: "title",
    },
    { key: "scope", header: "Umfang", render: (row) => row.scope },
    { key: "reason", header: "Grund", render: (row) => row.reason },
    {
      key: "responsible",
      header: "Zuständig",
      render: (row) => row.responsible,
    },
    {
      key: "review",
      header: "Nächste Prüfung",
      render: (row) => formatDate(row.reviewOn),
    },
    {
      key: "actions",
      header: "Aktionen",
      headerSrOnly: true,
      align: "right",
      stacked: "meta",
      render: (row) => (
        <OverflowMenu
          ariaLabel={`Aktionen für die Aussetzung ${belegTypeLabel(row.belegType)}`}
          triggerSize="sm"
          items={[
            {
              label: "Aussetzung aufheben",
              description: "Die Frist gilt danach wieder.",
              onClick: () => setLifting(row),
            },
          ]}
        />
      ),
    },
  ];

  return (
    <>
      <SectionCard
        title="Aussetzungen"
        description="Hält die Löschung an, etwa bei einer Betriebsprüfung oder einem Rechtsstreit."
        action={
          <Button type="button" variant="outline" size="md" onClick={onAdd}>
            Löschung aussetzen
          </Button>
        }
      >
        <DataTable
          columns={columns}
          rows={[...suspensions]}
          getRowKey={(row) => row.id}
          isLoading={loading}
          rowHasInteractiveControls
          stackedOnMobile
          emptyState={<EmptyState variant="compact" title="Keine Aussetzung" />}
        />
      </SectionCard>
      <ConfirmationModal
        isOpen={lifting !== null}
        onClose={() => setLifting(null)}
        onConfirm={() => void lift()}
        title="Aussetzung aufheben?"
        confirmText="Aufheben"
        isConfirmLoading={busy}
        loadingText="Wird aufgehoben..."
      >
        <p className="text-sm text-gray-700">
          Danach löscht moto die Belege wieder nach ihrer Frist.
        </p>
      </ConfirmationModal>
    </>
  );
}

function InstructionModal({
  isOpen,
  organizationId,
  onClose,
  onSaved,
}: {
  readonly isOpen: boolean;
  readonly organizationId: string;
  readonly onClose: () => void;
  readonly onSaved: () => Promise<unknown>;
}) {
  const [type, setType] = useState<BelegType>("holiday_document");
  const [trigger, setTrigger] = useState("issued");
  const [period, setPeriod] = useState("8");
  const [basis, setBasis] = useState("booking_voucher");
  const [legalBasis, setLegalBasis] = useState("");
  const [justification, setJustification] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const [instructedBy, setInstructedBy] = useState("");
  const [instructedOn, setInstructedOn] = useState("");
  const [reference, setReference] = useState("");
  const save = useFormModalPending(isOpen);
  const saving = save.pending;
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { success } = useToast();
  const triggers = TRIGGERS_BY_TYPE[type];
  const withoutStatutes = basis === "not_applicable";
  const minimum = minimumPeriod(type, basis);

  const changeType = (value: string) => {
    const next = value as BelegType;
    const nextBasis = BASES_BY_TYPE[next][0] ?? "booking_voucher";
    setType(next);
    setTrigger(TRIGGERS_BY_TYPE[next][0] ?? "issued");
    setBasis(nextBasis);
    setConfirmed(false);
    setPeriod(String(minimumPeriod(next, nextBasis)));
  };
  const changeBasis = (value: string) => {
    setBasis(value);
    setConfirmed(false);
    if (Number(period) < minimumPeriod(type, value)) {
      setPeriod(String(minimumPeriod(type, value)));
    }
  };
  const complete =
    legalBasis.trim() !== "" &&
    (!withoutStatutes || (justification.trim() !== "" && confirmed)) &&
    instructedBy.trim() !== "" &&
    instructedOn !== "" &&
    reference.trim() !== "" &&
    Number(period) >= minimum;

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!complete) return;
    await save.run(async (op) => {
      formErrors.clear();
      try {
        await belegRetentionService.recordInstruction(organizationId, {
          beleg_type: type,
          trigger_event: trigger,
          period: Number(period),
          basis,
          legal_basis: legalBasis.trim(),
          basis_justification: withoutStatutes ? justification.trim() : "",
          instructed_by: instructedBy.trim(),
          instructed_on: instructedOn,
          instruction_reference: reference.trim(),
          statutes_not_applicable_confirmed: withoutStatutes && confirmed,
        });
        success("Die Weisung ist erfasst.");
        if (op.isCurrent()) onClose();
        await onSaved();
      } catch (saveError) {
        logger.warn("beleg_instruction_record_failed", {
          error: errorMessage(saveError),
        });
        if (!op.isCurrent()) return;
        void formErrors.show(saveError, { object: "die Weisung" });
      }
    });
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      pending={save}
      title="Weisung erfassen"
      error={formErrors.error}
      isBackdropDismissDisabled
      footer={
        <Button
          type="submit"
          form="beleg-instruction-form"
          variant="primary"
          size="md"
          disabled={!complete || saving}
          isLoading={saving}
          loadingText="Wird erfasst..."
        >
          Weisung erfassen
        </Button>
      }
    >
      <form
        id="beleg-instruction-form"
        ref={formRef}
        onSubmit={(event) => void submit(event)}
        className="space-y-4"
      >
        <CustomSelect
          id="beleg-instruction-type"
          label="Belegart"
          value={type}
          options={TYPE_OPTIONS}
          onChange={changeType}
          required
        />
        <div className="grid gap-4 sm:grid-cols-2">
          <CustomSelect
            id="beleg-instruction-trigger"
            label="Frist beginnt mit"
            value={trigger}
            options={triggers.map((value) => ({
              value,
              label: TRIGGER_LABELS[value] ?? value,
            }))}
            onChange={setTrigger}
            required
          />
          <Input
            id="beleg-instruction-period"
            label={`Frist in ${periodUnit(type)}n`}
            type="number"
            min={minimum}
            value={period}
            onChange={(event) => setPeriod(event.target.value)}
            hint={
              type === "sepa_mandate"
                ? "Mindestens 14 Monate nach der letzten Lastschrift."
                : `Mindestens ${minimum} ${minimum === 1 ? "Jahr" : "Jahre"} nach Ende des Jahres, in dem die Frist beginnt.`
            }
            required
          />
        </div>
        <CustomSelect
          id="beleg-instruction-kind"
          label="Einordnung"
          value={basis}
          options={BASES_BY_TYPE[type].map((value) => ({
            value,
            label: BASIS_LABELS[value] ?? value,
          }))}
          onChange={changeBasis}
          required
        />
        <Input
          id="beleg-instruction-basis"
          label="Fundstelle"
          value={legalBasis}
          onChange={(event) => setLegalBasis(event.target.value)}
          placeholder={
            withoutStatutes
              ? "zum Beispiel Satzung des Trägers"
              : "zum Beispiel § 147 Abs. 1 Nr. 4 AO"
          }
          hint={
            withoutStatutes ? "Hier stehen weder AO, HGB noch UStG." : undefined
          }
          maxLength={500}
          required
        />
        {withoutStatutes && (
          <Textarea
            id="beleg-instruction-justification"
            label="Warum die Vorschriften nicht gelten"
            value={justification}
            onChange={(event) => setJustification(event.target.value)}
            hint="Schriftliche Begründung des Trägers."
            maxLength={1000}
            required
          />
        )}
        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            id="beleg-instruction-by"
            label="Weisungsberechtigte Person"
            value={instructedBy}
            onChange={(event) => setInstructedBy(event.target.value)}
            placeholder="Name und Funktion beim Träger"
            maxLength={500}
            required
          />
          <ISODatePicker
            id="beleg-instruction-on"
            label="Weisung vom"
            value={instructedOn}
            onChange={setInstructedOn}
            max={berlinTodayISO()}
            required
          />
        </div>
        <Input
          id="beleg-instruction-reference"
          label="Nachweis der schriftlichen Weisung"
          value={reference}
          onChange={(event) => setReference(event.target.value)}
          placeholder="zum Beispiel Mail der Kämmerei vom 09.11.2026"
          hint="Dokument oder Mail des Trägers, damit man die Weisung wiederfindet."
          maxLength={500}
          required
        />
        {withoutStatutes && (
          <ConsentCheckbox
            name="statutes_not_applicable_confirmed"
            id="beleg-instruction-confirmed"
            checked={confirmed}
            onChange={setConfirmed}
            label="Der Träger hat schriftlich bestätigt: § 147 AO, § 257 HGB und § 14b UStG gelten für diese Belege nicht."
            description="Nur mit dieser Bestätigung ist eine kürzere Frist möglich."
            required
          />
        )}
      </form>
    </FormModal>
  );
}

function SuspensionModal({
  isOpen,
  organizationId,
  onClose,
  onSaved,
}: {
  readonly isOpen: boolean;
  readonly organizationId: string;
  readonly onClose: () => void;
  readonly onSaved: () => Promise<unknown>;
}) {
  const [type, setType] = useState<string>("holiday_document");
  const [reason, setReason] = useState("");
  const [scope, setScope] = useState("");
  const [responsible, setResponsible] = useState("");
  const [reviewOn, setReviewOn] = useState("");
  const save = useFormModalPending(isOpen);
  const saving = save.pending;
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { success } = useToast();
  const complete =
    reason.trim() !== "" &&
    scope.trim() !== "" &&
    responsible.trim() !== "" &&
    reviewOn !== "";

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!complete) return;
    await save.run(async (op) => {
      formErrors.clear();
      try {
        await belegRetentionService.recordSuspension(organizationId, {
          beleg_type: type,
          reason: reason.trim(),
          scope: scope.trim(),
          responsible: responsible.trim(),
          review_on: reviewOn,
        });
        success("Die Löschung ist ausgesetzt.");
        if (op.isCurrent()) onClose();
        await onSaved();
      } catch (saveError) {
        logger.warn("beleg_suspension_record_failed", {
          error: errorMessage(saveError),
        });
        if (!op.isCurrent()) return;
        void formErrors.show(saveError, { object: "die Aussetzung" });
      }
    });
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      pending={save}
      title="Löschung aussetzen"
      error={formErrors.error}
      isBackdropDismissDisabled
      footer={
        <Button
          type="submit"
          form="beleg-suspension-form"
          variant="primary"
          size="md"
          disabled={!complete || saving}
          isLoading={saving}
          loadingText="Wird ausgesetzt..."
        >
          Aussetzen
        </Button>
      }
    >
      <form
        id="beleg-suspension-form"
        ref={formRef}
        onSubmit={(event) => void submit(event)}
        className="space-y-4"
      >
        <CustomSelect
          id="beleg-suspension-type"
          label="Belegart"
          value={type}
          options={TYPE_OPTIONS}
          onChange={setType}
          required
        />
        <Input
          id="beleg-suspension-scope"
          label="Umfang"
          value={scope}
          onChange={(event) => setScope(event.target.value)}
          placeholder="zum Beispiel alle Belege aus 2026"
          maxLength={500}
          required
        />
        <Input
          id="beleg-suspension-reason"
          label="Grund"
          value={reason}
          onChange={(event) => setReason(event.target.value)}
          placeholder="zum Beispiel Betriebsprüfung"
          maxLength={500}
          required
        />
        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            id="beleg-suspension-responsible"
            label="Zuständig"
            value={responsible}
            onChange={(event) => setResponsible(event.target.value)}
            maxLength={500}
            required
          />
          <ISODatePicker
            id="beleg-suspension-review"
            label="Nächste Prüfung"
            value={reviewOn}
            onChange={setReviewOn}
            min={berlinTodayISO()}
            required
          />
        </div>
      </form>
    </FormModal>
  );
}

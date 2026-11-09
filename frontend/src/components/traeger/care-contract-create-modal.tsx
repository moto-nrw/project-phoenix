"use client";

// Verträge aus angenommenen Anmeldungen erstellen (#4678). Das Modal zeigt
// je Schule die Kinder mit gebuchter Betreuung, die noch keinen Vertrag
// haben, mit ihren Modulen je Wochentag. Ein Vertrag umfasst alle Module des
// Kindes. Fehlen für ein Modul die Vertragsbedingungen oder weichen sie
// zwischen den Modulen ab, entsteht kein Entwurf; die Zeile sagt das und
// nennt den nächsten Schritt.

import { SectionLabel } from "~/components/ui/section-label";
import { useEffect, useMemo, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { EditActions } from "~/components/ui/edit-actions";
import { EmptyState } from "~/components/ui/empty-state";
import { FormModal } from "~/components/ui/form-modal";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiFormError } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type ContractCandidate,
  type CreateResult,
  createCareContracts,
} from "~/lib/traeger-care-contracts-api";
import {
  TEMPLATE_ISSUE_LABELS,
  TEMPLATE_ISSUE_REASONS,
  childName,
  formatServiceDays,
} from "./care-contract-format";

const logger = createLogger({ component: "CareContractCreateModal" });

function candidateKey(candidate: ContractCandidate): string {
  return `${candidate.schoolId}:${candidate.requestChildId}`;
}

interface SchoolGroup {
  readonly schoolName: string;
  readonly candidates: readonly ContractCandidate[];
}

function bySchool(candidates: readonly ContractCandidate[]): SchoolGroup[] {
  const groups = new Map<string, ContractCandidate[]>();
  for (const candidate of candidates) {
    const group = groups.get(candidate.schoolName) ?? [];
    group.push(candidate);
    groups.set(candidate.schoolName, group);
  }
  return [...groups].map(([schoolName, rows]) => ({
    schoolName,
    candidates: rows,
  }));
}

/** The children's names as one list: „Jona Kern und Lea Kern“. */
function nameList(children: readonly ContractCandidate[]): string {
  const names = children.map(childName);
  return names.length === 1
    ? names[0]!
    : `${names.slice(0, -1).join(", ")} und ${names.at(-1)}`;
}

/**
 * Which children wait for the school before their contract comes at the
 * acceptance (#4680), and what the school does there. The address of an
 * application cannot be changed; the school frees it at the child.
 */
function heldMailMessage(
  children: readonly ContractCandidate[],
  issue: "mail_blocked" | "mail_stopped",
): string {
  const one = children.length === 1;
  const subject = one
    ? `Der Vertrag für ${nameList(children)} kommt`
    : `Die Verträge für ${nameList(children)} kommen`;
  if (issue === "mail_stopped") {
    return `${subject} erst, wenn die Schule wieder Mails an die Eltern sendet. Die Schule hat die Mails gestoppt. Sie wählt beim Kind unter „Erziehungsberechtigte“ „Mails wieder senden“.`;
  }
  return `${subject} erst, wenn die Schule die Sperre der Mail-Adresse aufhebt. Mails an ${one ? "diese Adresse" : "diese Adressen"} kamen nicht an. Die Schule prüft die Adresse beim Kind unter „Erziehungsberechtigte“ und wählt „Sperre aufheben“.`;
}

function conditionsMessage(missing: number, differing: number): string {
  const children = (count: number) =>
    count === 1 ? "1 Kind" : `${count} Kinder`;
  const parts: string[] = [];
  if (missing > 0) {
    parts.push(
      `Für ${children(missing)} fehlen die Vertragsbedingungen eines Moduls.`,
    );
  }
  if (differing > 0) {
    parts.push(
      `Bei ${children(differing)} haben die gebuchten Module verschiedene Bedingungen. Ein Vertrag braucht für alle Module dieselben.`,
    );
  }
  return `${parts.join(" ")} Legen Sie die Bedingungen zuerst fest.`;
}

export function CareContractCreateModal({
  isOpen,
  candidates,
  onClose,
  onCreated,
  onOpenConditions,
}: Readonly<{
  isOpen: boolean;
  candidates: readonly ContractCandidate[];
  onClose: () => void;
  onCreated: (result: CreateResult) => void;
  onOpenConditions: () => void;
}>) {
  const ready = useMemo(
    () => candidates.filter((candidate) => candidate.templateIssue === null),
    [candidates],
  );
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;

  useEffect(() => {
    if (!isOpen) return;
    setSelected(new Set(ready.map(candidateKey)));
    clearErrors();
  }, [isOpen, ready, clearErrors]);

  const missing = candidates.filter(
    (candidate) => candidate.templateIssue === "template_missing",
  ).length;
  const differing = candidates.filter(
    (candidate) => candidate.templateIssue === "templates_differ",
  ).length;
  const blockedMail = candidates.filter(
    (candidate) => candidate.templateIssue === "mail_blocked",
  );
  const stoppedMail = candidates.filter(
    (candidate) => candidate.templateIssue === "mail_stopped",
  );
  const toggle = (key: string) =>
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  const allSelected = ready.length > 0 && selected.size === ready.length;

  const submit = async () => {
    formErrors.clear();
    const keys = ready.filter((candidate) =>
      selected.has(candidateKey(candidate)),
    );
    if (keys.length === 0) {
      formErrors.invalid("Bitte wählen Sie mindestens ein Kind aus.", {});
      return;
    }
    setSaving(true);
    try {
      onCreated(await createCareContracts(keys));
    } catch (err) {
      logger.error("care_contract_create_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "das Erstellen der Entwürfe" });
    } finally {
      setSaving(false);
    }
  };

  const count = selected.size;
  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title="Verträge erstellen"
      size="lg"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          disabled={count === 0}
          saveLabel={
            count === 1 ? "1 Entwurf erstellen" : `${count} Entwürfe erstellen`
          }
          savingLabel="Wird erstellt …"
        />
      }
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
        <p className="text-sm text-gray-600">
          Diese Kinder sind angenommen und haben Betreuung gebucht. moto
          übernimmt Kind, Eltern, Beginn und die Module je Wochentag aus der
          Anmeldung. Ein Vertrag umfasst alle Module eines Kindes. Sie prüfen
          jeden Entwurf, bevor Sie ihn senden. Für „Bei der Annahme“ sind hier
          keine Entwürfe nötig.
        </p>
        {candidates.length === 0 ? (
          <EmptyState
            title="Jede angenommene Anmeldung hat einen Vertrag."
            description="Neue Anmeldungen erscheinen hier, sobald die Schule sie annimmt."
          />
        ) : null}
        {missing + differing > 0 ? (
          <Alert
            type="warning"
            message={conditionsMessage(missing, differing)}
            actionLayout="responsive"
            action={
              <Button
                type="button"
                variant="ghost_tone"
                size="compact"
                onClick={onOpenConditions}
              >
                Bedingungen festlegen
              </Button>
            }
          />
        ) : null}
        {blockedMail.length > 0 ? (
          <Alert
            type="warning"
            message={heldMailMessage(blockedMail, "mail_blocked")}
          />
        ) : null}
        {stoppedMail.length > 0 ? (
          <Alert
            type="warning"
            message={heldMailMessage(stoppedMail, "mail_stopped")}
          />
        ) : null}
        {ready.length > 1 ? (
          <label className="flex items-center gap-3 text-sm font-medium text-gray-900">
            <Checkbox
              checked={allSelected}
              onChange={() =>
                setSelected(
                  allSelected ? new Set() : new Set(ready.map(candidateKey)),
                )
              }
            />
            Alle {ready.length} auswählen
          </label>
        ) : null}
        {bySchool(candidates).map((group) => (
          <fieldset key={group.schoolName} className="space-y-2">
            <SectionLabel as="legend" className="mb-2">
              {group.schoolName}
            </SectionLabel>
            {group.candidates.map((candidate) => {
              const key = candidateKey(candidate);
              const isSelected = selected.has(key);
              return (
                <ChoiceTile
                  key={key}
                  selected={isSelected && candidate.templateIssue === null}
                  disabled={candidate.templateIssue !== null}
                  tone="blue"
                  className="flex items-start gap-3 px-4 py-3"
                >
                  <Checkbox
                    checked={isSelected && candidate.templateIssue === null}
                    disabled={candidate.templateIssue !== null}
                    onChange={() => toggle(key)}
                    aria-label={`${childName(candidate)} auswählen`}
                  />
                  <span className="min-w-0 flex-1">
                    <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
                      <span className="font-medium text-gray-900">
                        {childName(candidate)}
                      </span>
                      {candidate.templateIssue === null ? null : (
                        <StatusBadge
                          tone={
                            candidate.templateIssue === "conclusion_automatic"
                              ? "green"
                              : "orange"
                          }
                          label={TEMPLATE_ISSUE_LABELS[candidate.templateIssue]}
                        />
                      )}
                    </span>
                    {candidate.templateIssue !== null &&
                    TEMPLATE_ISSUE_REASONS[candidate.templateIssue] ? (
                      <span className="mt-0.5 block text-sm text-gray-700">
                        {TEMPLATE_ISSUE_REASONS[candidate.templateIssue]}
                      </span>
                    ) : null}
                    <span className="mt-0.5 block text-sm text-gray-600">
                      {candidate.modulesText || "–"} · ab{" "}
                      {formatDate(candidate.effectiveFrom)}
                    </span>
                    <span className="block text-sm text-gray-600">
                      {formatServiceDays(candidate.services)}
                    </span>
                    <span className="block text-sm wrap-anywhere text-gray-500">
                      {candidate.guardianName}
                      {candidate.guardianEmail
                        ? ` · ${candidate.guardianEmail}`
                        : ""}
                    </span>
                  </span>
                </ChoiceTile>
              );
            })}
          </fieldset>
        ))}
      </form>
    </FormModal>
  );
}

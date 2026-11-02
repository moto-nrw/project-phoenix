"use client";

import { Plus, Receipt } from "lucide-react";
import { useState } from "react";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type ContributionRule,
  type ContributionRuleKind,
  deleteContributionRule,
} from "~/lib/traeger-contributions-api";
import { ruleValidity, siblingRuleText } from "./contribution-format";
import {
  type ContributionRuleEdit,
  ContributionRuleModal,
} from "./contribution-rule-modal";

const logger = createLogger({ component: "ContributionRulesCard" });

const KIND_TEXT: Record<
  ContributionRuleKind,
  {
    readonly title: string;
    readonly description: string;
    readonly add: string;
    readonly empty: string;
  }
> = {
  sibling: {
    title: "Geschwister",
    description:
      "Geschwister in derselben Ferienanmeldung zahlen weniger. Das Kind mit dem höchsten Preis zahlt voll.",
    add: "Geschwisterregel hinzufügen",
    empty: "Keine Geschwisterregel. Alle Kinder zahlen den vollen Preis.",
  },
  benefits_exemption: {
    title: "Befreiung bei Leistungsbezug",
    description:
      "Familien mit Leistungsbezug zahlen nichts. Ob die Befreiung gilt, entscheiden Sie beim Bestätigen der Anmeldung.",
    add: "Befreiung hinzufügen",
    empty: "Keine Befreiung. Ohne Regel zahlen alle Familien.",
  },
};

/**
 * The rules of one kind (#4664), each with its validity. Rules of one kind
 * never overlap; a change applies to registrations confirmed after it.
 */
export function ContributionRulesCard({
  kind,
  rules,
  onChanged,
}: Readonly<{
  kind: ContributionRuleKind;
  rules: readonly ContributionRule[];
  onChanged: () => Promise<unknown>;
}>) {
  const [edit, setEdit] = useState<ContributionRuleEdit | null>(null);
  const [deleting, setDeleting] = useState<ContributionRule | null>(null);
  const [busy, setBusy] = useState(false);
  const deleteErrors = useApiFormError();
  const { success } = useToast();
  const text = KIND_TEXT[kind];

  const remove = async () => {
    if (!deleting) return;
    setBusy(true);
    deleteErrors.clear();
    try {
      await deleteContributionRule(deleting.id);
      success("Die Regel ist gelöscht.");
      setDeleting(null);
      await onChanged();
    } catch (err) {
      logger.error("contribution_rule_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void deleteErrors.show(err, { object: "das Löschen der Regel" });
    } finally {
      setBusy(false);
    }
  };

  const columns: DataTableColumn<ContributionRule>[] = [
    {
      key: "validity",
      header: "Gilt",
      render: (rule) => (
        <span className="font-medium text-gray-900 tabular-nums">
          {ruleValidity(rule)}
        </span>
      ),
      stacked: "title",
    },
    {
      key: "rule",
      header: kind === "sibling" ? "Ermäßigung" : "Vermerk",
      render: (rule) => (
        <span className="wrap-anywhere">
          {kind === "sibling" ? siblingRuleText(rule) : rule.note}
        </span>
      ),
      stacked: "field",
    },
    {
      key: "actions",
      header: "",
      render: (rule) => (
        <OverflowMenu
          ariaLabel={`Aktionen für die Regel ${ruleValidity(rule)}`}
          triggerSize="sm"
          items={[
            {
              label: "Ändern",
              onClick: () => setEdit({ kind: "change", rule }),
            },
            {
              label: "Löschen",
              destructive: true,
              onClick: () => setDeleting(rule),
            },
          ]}
        />
      ),
      align: "right",
      stacked: "meta",
    },
  ];

  return (
    <SectionCard
      title={text.title}
      description={text.description}
      actions={
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={() => setEdit({ kind: "create", ruleKind: kind })}
        >
          <Plus className="mr-2 h-4 w-4" aria-hidden="true" />
          {text.add}
        </Button>
      }
    >
      <DataTable
        columns={columns}
        rows={[...rules]}
        getRowKey={(rule) => rule.id}
        rowHasInteractiveControls
        stackedOnMobile
        emptyState={
          <EmptyState
            icon={<Receipt className="h-12 w-12" aria-hidden="true" />}
            title={text.empty}
          />
        }
      />
      <ContributionRuleModal
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
        title="Regel löschen?"
        description="Bereits bestätigte Anmeldungen behalten ihren Beitrag. Neue Bestätigungen rechnen ohne diese Regel."
        confirmLabel="Löschen"
        loadingLabel="Wird gelöscht …"
        gate={{ mode: "twoStep", firstStepLabel: "Ja, löschen" }}
        loading={busy}
        error={deleteErrors.error}
      />
    </SectionCard>
  );
}

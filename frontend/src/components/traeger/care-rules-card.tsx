"use client";

// Die Regeln einer Art des Monatsbeitrags (#4685), etwa alle
// Geschwisterregeln. Jede Zeile zeigt Name, Wirkung, Monate und Schulen; die
// Aktionen stehen im Menü der Zeile.

import { BadgePercent, Crown, HeartHandshake, User } from "lucide-react";
import { Button } from "~/components/ui/button";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import type {
  CareRule,
  CareRuleKind,
  CareRuleSet,
} from "~/lib/traeger-care-reductions-api";
import { formatMonth } from "./care-billing-format";
import {
  CARE_RULE_KIND_LABELS,
  CARE_RULE_KIND_PURPOSE,
  ruleEffect,
  validityLabel,
} from "./care-reduction-format";

const KIND_ICONS = {
  income_level: BadgePercent,
  // One head means a child (concept icons): a sibling rule is about children.
  sibling: User,
  single_parent: HeartHandshake,
  cap: Crown,
} as const;

const CREATE_LABELS: Record<CareRuleKind, string> = {
  income_level: "Stufe anlegen",
  sibling: "Regel anlegen",
  single_parent: "Regel anlegen",
  cap: "Höchstbetrag anlegen",
};

function names(ids: readonly string[], data: CareRuleSet): string {
  return ids
    .map((id) => data.schools.find((school) => school.id === id)?.name ?? "–")
    .join(", ");
}

function moduleNames(rule: CareRule, data: CareRuleSet): string {
  const seen = new Set<string>();
  for (const entry of data.modules) {
    if (rule.careOfferingIds.includes(entry.careOfferingId)) {
      seen.add(entry.moduleName);
    }
  }
  return [...seen].join(", ");
}

/** Whether the rule holds in the month, starts later or has ended. */
function stateOf(
  rule: CareRule,
  month: string,
): { label: string; tone: "green" | "blue" | "gray" } {
  if (rule.validFrom > month) {
    return { label: `Ab ${formatMonth(rule.validFrom)}`, tone: "blue" };
  }
  if (rule.validUntil && rule.validUntil < month) {
    return { label: "Beendet", tone: "gray" };
  }
  return { label: "Gilt jetzt", tone: "green" };
}

function RuleDetails({
  rule,
  data,
}: Readonly<{ rule: CareRule; data: CareRuleSet }>) {
  const modules = moduleNames(rule, data);
  const lines = [
    [
      names(rule.schoolIds, data),
      validityLabel(rule.validFrom, rule.validUntil, formatMonth),
      rule.careOfferingIds.length === 0
        ? "alle Module"
        : `nur ${modules || `${rule.careOfferingIds.length} nicht mehr angebotene Module`}`,
    ].join(" · "),
  ];
  if (rule.kind === "sibling") {
    lines.push(
      `Ab dem ${rule.siblingFromChild ?? 2}. Kind · ${
        rule.countSchoolIds.length > 0
          ? `Geschwister zählen auch an: ${names(rule.countSchoolIds, data)}`
          : "Geschwister zählen nur an diesen Schulen"
      }${rule.countsFamilyChildren ? " · Geschwister ohne Vertrag zählen mit" : ""}`,
    );
  }
  return (
    <>
      {lines.map((line) => (
        <p key={line} className="mt-0.5 text-sm text-gray-600">
          {line}
        </p>
      ))}
    </>
  );
}

export function CareRulesCard({
  kind,
  data,
  currentMonth,
  onCreate,
  onEdit,
  onDelete,
}: Readonly<{
  kind: CareRuleKind;
  data: CareRuleSet;
  currentMonth: string;
  onCreate: () => void;
  onEdit: (rule: CareRule) => void;
  onDelete: (rule: CareRule) => void;
}>) {
  const rules = data.rules.filter((rule) => rule.kind === kind);
  const label = CARE_RULE_KIND_LABELS[kind];
  return (
    <SectionCard
      title={label}
      description={CARE_RULE_KIND_PURPOSE[kind]}
      icon={KIND_ICONS[kind]}
      titleBadge={
        rules.length > 0 ? (
          <StatusBadge tone="gray" label={String(rules.length)} />
        ) : null
      }
      action={
        <Button type="button" variant="outline" size="md" onClick={onCreate}>
          {CREATE_LABELS[kind]}
        </Button>
      }
    >
      {rules.length === 0 ? (
        <p className="text-sm text-gray-600">
          Noch keine Regel. Ohne Regel gilt der Preis aus der Preisliste.
        </p>
      ) : (
        <ResponsiveActionList>
          {rules.map((rule) => {
            const state = stateOf(rule, currentMonth);
            return (
              <li
                key={rule.id}
                className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center"
              >
                <div className="min-w-0">
                  <p className="flex flex-wrap items-center gap-2 text-sm font-semibold text-gray-900">
                    <span>{rule.name}</span>
                    <span className="font-normal text-gray-700">
                      {ruleEffect(rule)}
                    </span>
                    <StatusBadge tone={state.tone} label={state.label} />
                  </p>
                  <RuleDetails rule={rule} data={data} />
                </div>
                <div className="flex justify-end">
                  <OverflowMenu
                    ariaLabel={`Aktionen: ${rule.name}`}
                    items={[
                      { label: "Ändern", onClick: () => onEdit(rule) },
                      {
                        label: "Löschen",
                        destructive: true,
                        onClick: () => onDelete(rule),
                      },
                    ]}
                  />
                </div>
              </li>
            );
          })}
        </ResponsiveActionList>
      )}
    </SectionCard>
  );
}

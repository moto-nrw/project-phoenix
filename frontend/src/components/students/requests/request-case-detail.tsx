"use client";

/**
 * Alle Anfragen EINES Kindes an einer Stelle (#2267): erst die Wünsche, die
 * sich widersprechen und zusammen entschieden werden müssen, dann die
 * einzelnen Anfragen, dann die offenen Abmeldungen.
 */

import { useEffect, useRef } from "react";

import { Button } from "~/components/ui/button";
import { FamilyProtectionControl } from "~/components/students/family-protection-control";
import type { CareWithdrawalCompletion } from "~/lib/care-exit-api";
import {
  caseTypeLabels,
  conflictedItemKeys,
  itemKey,
  openRequestCount,
  pastRequestCount,
  type OpenCase,
} from "./case-model";
import { ConflictDecisionGroup } from "./conflict-decision-group";
import { RequestDecisionRow } from "./request-decision-row";
import { OpenWithdrawalCard } from "./withdrawal-cards";

export function RequestCaseDetail({
  childCase,
  canManageFamilyProtection,
  reasonRequired,
  selected,
  narrow,
  onBackToList,
  onSelectionChange,
  onDecided,
  onProtectionChanged,
  onReload,
  onNotice,
  finishWithdrawal,
  removeWithdrawal,
}: Readonly<{
  childCase: OpenCase;
  canManageFamilyProtection: boolean;
  /**
   * Verlangt die Schule beim Freigeben eine Begründung
   * (operations.parent_request_reason_policy)? Ablehnen verlangt sie immer,
   * ein Widerspruchs-Ergebnis ebenfalls — das regeln die Karten selbst.
   */
  reasonRequired: boolean;
  selected: ReadonlySet<string>;
  /** Schmale Ansicht: die Liste ist ersetzt, es braucht einen Weg zurück. */
  narrow: boolean;
  onBackToList: () => void;
  onSelectionChange: (key: string, checked: boolean) => void;
  onDecided: (key: string, notice: string) => void;
  onProtectionChanged: (studentID: string, enabled: boolean) => void;
  onReload: () => void;
  onNotice: (notice: string) => void;
  finishWithdrawal: (row: CareWithdrawalCompletion) => void;
  removeWithdrawal: (row: CareWithdrawalCompletion) => void;
}>) {
  const paneRef = useRef<HTMLDivElement>(null);
  // In der schmalen Ansicht ersetzt der Detailbereich die Liste. Ohne diesen
  // Fokus stünde die Tastatur danach wieder am Seitenanfang.
  useEffect(() => {
    if (narrow) paneRef.current?.focus();
  }, [narrow]);

  const conflicted = conflictedItemKeys(childCase.conflicts);
  const requestCount = openRequestCount(childCase);
  const expired = pastRequestCount(childCase);
  const typeLabels = caseTypeLabels(childCase);
  const total = childCase.items.length + childCase.withdrawals.length;

  return (
    <div
      ref={paneRef}
      tabIndex={-1}
      className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm focus:outline-none"
    >
      {narrow && (
        <div className="px-4 pt-3 sm:px-5">
          <Button
            type="button"
            variant="ghost"
            size="md"
            className="max-sm:min-h-11"
            onClick={onBackToList}
          >
            Zur Liste
          </Button>
        </div>
      )}
      <header className="flex flex-wrap items-start justify-between gap-3 border-b border-gray-100 bg-gray-50/70 px-4 py-3 sm:px-5">
        <div className="min-w-0">
          <h3 className="truncate font-semibold text-gray-900">
            {childCase.studentName}
          </h3>
          <p className="text-sm font-medium text-gray-600">
            {requestCount > 0 || expired === 0 ? (
              <span>
                {requestCount === 1
                  ? "1 offene Anfrage"
                  : `${requestCount} offene Anfragen`}
              </span>
            ) : null}
            {expired > 0
              ? `${requestCount > 0 ? " · " : ""}${expired === 1 ? "1 abgelaufene Anfrage" : `${expired} abgelaufene Anfragen`}`
              : ""}
            {typeLabels.length > 0 ? ` · ${typeLabels.join(", ")}` : ""}
          </p>
        </div>
        {childCase.studentID && childCase.familyProtected !== undefined ? (
          <FamilyProtectionControl
            studentId={childCase.studentID}
            canManage={canManageFamilyProtection}
            initialEnabled={childCase.familyProtected}
            compact
            onChanged={(enabled) =>
              onProtectionChanged(childCase.studentID!, enabled)
            }
          />
        ) : null}
      </header>
      {childCase.conflicts.length > 0 && (
        <div className="space-y-3 border-b border-gray-100 bg-gray-50/40 p-4 sm:p-5">
          {childCase.conflicts.map((group) => (
            <ConflictDecisionGroup
              key={group.key}
              group={group}
              onResolved={(notice: string) => {
                onNotice(notice);
                onReload();
              }}
              onStale={onReload}
            />
          ))}
        </div>
      )}
      <div>
        {childCase.items.map((request, index) => (
          <RequestDecisionRow
            key={itemKey(request)}
            request={request}
            selected={selected.has(itemKey(request))}
            position={index + 1}
            total={total}
            inConflict={conflicted.has(itemKey(request))}
            approveReasonRequired={reasonRequired}
            onSelectionChange={onSelectionChange}
            onDecided={onDecided}
            onStale={onReload}
          />
        ))}
        {childCase.withdrawals.map((row, index) => (
          <OpenWithdrawalCard
            key={`care_withdrawal:${row.id}`}
            row={row}
            grouped
            position={childCase.items.length + index + 1}
            total={total}
            finish={finishWithdrawal}
            remove={removeWithdrawal}
          />
        ))}
      </div>
    </div>
  );
}

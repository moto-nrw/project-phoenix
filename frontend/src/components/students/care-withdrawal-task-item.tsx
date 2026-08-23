"use client";

import { useState } from "react";

import { CareExitModal } from "~/components/students/care-exit-modal";
import { RequestReviewCard } from "~/components/students/request-review-card";
import { Button } from "~/components/ui/button";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type { CareWithdrawalCompletion } from "~/lib/care-exit-api";

export function CareWithdrawalTaskItem({
  row,
  onFinished,
}: Readonly<{
  row: CareWithdrawalCompletion;
  onFinished: () => void;
}>) {
  const [open, setOpen] = useState(false);
  const name = `${row.firstName} ${row.lastName}`.trim();
  const overdue = row.urgency === "overdue";

  return (
    <>
      <RequestReviewCard
        type="care_withdrawal"
        typeLabel="Abmeldung"
        childName={name}
        summary={`Keine Betreuungstage ab ${formatDate(row.firstBookinglessDay)}`}
        badge={
          <StatusBadge
            tone={overdue ? "red" : "orange"}
            label={overdue ? "Überfällig" : "Geplant"}
          />
        }
        history={{
          kind: "readonly",
          label: overdue ? "Überfällig" : "Geplant",
          tone: overdue ? "red" : "orange",
        }}
        action={
          <Button
            type="button"
            variant="ghost"
            size="compact"
            onClick={() => setOpen(true)}
          >
            Betreuung beenden
          </Button>
        }
      >
        <p className="text-sm text-gray-600">
          Für dieses Kind ist kein Betreuungstag mehr gebucht. Beenden Sie jetzt
          die Betreuung.
        </p>
      </RequestReviewCard>
      <CareExitModal
        isOpen={open}
        studentIds={[row.studentId]}
        completionId={row.id}
        firstBookinglessDay={row.firstBookinglessDay}
        onClose={() => setOpen(false)}
        onFinished={() => {
          setOpen(false);
          onFinished();
        }}
      />
    </>
  );
}

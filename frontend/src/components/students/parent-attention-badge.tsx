"use client";

import dynamic from "next/dynamic";
import { useState } from "react";

import { Button } from "~/components/ui/button";
import { NotificationBadge } from "~/components/ui/notification-badge";
import type { ParentAttention } from "~/lib/parent-attention";
import { useStudentParentAttention } from "~/lib/hooks/use-parent-attention";

// Der Dialog lädt erst beim ersten Öffnen: die Kacheln einer Gruppe tragen
// sonst die Anfragenliste mit, ohne dass jemand sie öffnet.
const ParentAttentionDialog = dynamic(
  () =>
    import("~/components/students/parent-attention-dialog").then(
      (module) => module.ParentAttentionDialog,
    ),
  { ssr: false },
);

/** Vorlesetext der Zahl: was neu ist, für welches Kind. */
export function parentAttentionLabel(
  attention: ParentAttention,
  studentName: string,
): string {
  const parts: string[] = [];
  if (attention.messages > 0) {
    parts.push(
      attention.messages === 1
        ? "1 neue Nachricht"
        : `${attention.messages} neue Nachrichten`,
    );
  }
  if (attention.requests > 0) {
    parts.push(
      attention.requests === 1
        ? "1 offene Anfrage"
        : `${attention.requests} offene Anfragen`,
    );
  }
  return `${parts.join(" und ")} der Eltern von ${studentName} öffnen`;
}

/**
 * Rote Zahl oben rechts an der Kinderkachel (#6486): neue Nachrichten und
 * offene Anfragen der Eltern. Ein Tipp öffnet sie, ohne die Kachel zu
 * verlassen. Ohne Neues zeigt sie nichts.
 */
export function ParentAttentionBadge({
  studentId,
  studentName,
  className,
}: Readonly<{
  studentId: string;
  studentName: string;
  className?: string;
}>) {
  const attention = useStudentParentAttention(studentId);
  // Die Zahlen beim Öffnen: der Dialog behält seine Teile, auch wenn die Zahl
  // während der Arbeit darin sinkt.
  const [openedWith, setOpenedWith] = useState<ParentAttention | null>(null);

  return (
    <>
      {attention ? (
        <Button
          type="button"
          variant="count_badge"
          size="icon"
          className={className}
          aria-label={parentAttentionLabel(attention, studentName)}
          tooltip="Öffnet neue Nachrichten und offene Anfragen der Eltern."
          data-parent-attention={attention.total}
          onClick={() => setOpenedWith(attention)}
        >
          <NotificationBadge
            count={attention.total}
            tone="danger"
            ariaLabel=""
          />
        </Button>
      ) : null}
      {openedWith ? (
        <ParentAttentionDialog
          studentId={studentId}
          studentName={studentName}
          attention={openedWith}
          isOpen
          onClose={() => setOpenedWith(null)}
        />
      ) : null}
    </>
  );
}

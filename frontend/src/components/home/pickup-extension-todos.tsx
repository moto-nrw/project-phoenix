"use client";

import { useState } from "react";

import {
  PickupExtensionDialog,
  pickupExtensionWhen,
} from "~/components/timetable/pickup-extension-dialog";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { usePickupExtensions } from "~/lib/hooks/use-pickup-extensions";
import type { PickupExtension } from "~/lib/pickup-extension-api";

function todoMessage(tasks: readonly PickupExtension[]): string {
  const [first] = tasks;
  if (!first) return "";
  if (tasks.length === 1) {
    return `${first.studentName} wird ${pickupExtensionWhen(first)} erst um ${first.pickupTime} Uhr abgeholt und ist in dieser Zeit in keinem Termin.`;
  }
  const names = [...new Set(tasks.map((task) => task.studentName))];
  return `${tasks.length} spätere Abholzeiten ohne Termin: ${names.join(", ")}.`;
}

/**
 * Aufgabe auf der Startseite (#3261): Kinder, die später abgeholt werden als
 * bisher und für die zusätzliche Zeit in keinem Termin stehen. Steht fest
 * über dem Brett wie die Hinweise zum Phasenende, damit sie niemand durch
 * ein eigenes Layout verliert. Ohne offene Aufgabe ist sie nicht da.
 */
export function PickupExtensionTodos({
  enabled,
}: Readonly<{
  /** Recht am Betreuungsplan und Betreuungsplan eingeschaltet. */
  enabled: boolean;
}>) {
  const { tasks, error, refresh } = usePickupExtensions(enabled);
  // Die Liste im Fenster bleibt fest, solange es offen ist: das Neuladen
  // nach jeder Entscheidung darf die Reihenfolge nicht verschieben.
  const [open, setOpen] = useState<readonly PickupExtension[]>([]);

  if (!enabled || (tasks.length === 0 && open.length === 0 && !error))
    return null;

  return (
    <>
      {error ? (
        <Alert
          type="error"
          message="Die Aufgaben konnten nicht geladen werden. Bitte laden Sie die Seite neu."
        />
      ) : tasks.length > 0 ? (
        <Alert
          type="warning"
          announce="polite"
          title="Längere Betreuung eintragen"
          message={todoMessage(tasks)}
          action={
            <Button
              type="button"
              variant="surface"
              size="md"
              onClick={() => setOpen(tasks)}
            >
              {tasks.length === 1 ? "Termin wählen" : "Termine wählen"}
            </Button>
          }
        />
      ) : null}
      <PickupExtensionDialog
        tasks={open}
        isOpen={open.length > 0}
        onClose={() => setOpen([])}
        onChanged={() => void refresh()}
        onStale={() => {
          void refresh().finally(() => setOpen([]));
        }}
      />
    </>
  );
}

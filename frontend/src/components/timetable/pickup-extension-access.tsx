"use client";

import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
} from "react";
import type { ReactNode } from "react";

import { PickupExtensionDialog } from "~/components/timetable/pickup-extension-dialog";
import { createLogger } from "~/lib/logger";
import {
  fetchPickupExtensions,
  type PickupExtension,
} from "~/lib/pickup-extension-api";

const logger = createLogger({ component: "PickupExtensionAccessProvider" });

interface PickupExtensionPrompt {
  /** Darf die Person Kinder Terminen zuordnen (schedules:manage)? */
  readonly enabled: boolean;
  /** Öffnet die Auswahl für die übergebenen offenen Fragen. */
  readonly prompt: (tasks: readonly PickupExtension[]) => void;
}

const PickupExtensionPromptContext = createContext<PickupExtensionPrompt>({
  enabled: false,
  prompt: () => undefined,
});

/**
 * Hält die Terminauswahl nach dem Freigeben einer späteren Abholzeit (#3261)
 * auf Seitenebene. Die freigegebene Anfrage verschwindet sofort aus der
 * Liste; läge das Fenster in ihrer Karte, ginge es mit ihr zu.
 */
export function PickupExtensionAccessProvider({
  value,
  children,
}: Readonly<{ value: boolean; children: ReactNode }>) {
  const [open, setOpen] = useState<readonly PickupExtension[]>([]);
  const reloadOpenTask = useCallback((task: PickupExtension) => {
    void fetchPickupExtensions(task.studentId)
      .then(setOpen)
      .catch((err: unknown) => {
        logger.warn("pickup_extension_reload_failed", {
          error: err instanceof Error ? err.message : String(err),
          task_id: task.id,
        });
        setOpen([]);
      });
  }, []);
  const context = useMemo(() => ({ enabled: value, prompt: setOpen }), [value]);
  return (
    <PickupExtensionPromptContext.Provider value={context}>
      {children}
      <PickupExtensionDialog
        key={open.map((task) => task.id).join(":")}
        tasks={open}
        isOpen={open.length > 0}
        onClose={() => setOpen([])}
        onStale={reloadOpenTask}
      />
    </PickupExtensionPromptContext.Provider>
  );
}

export function usePickupExtensionPrompt(): PickupExtensionPrompt {
  return useContext(PickupExtensionPromptContext);
}

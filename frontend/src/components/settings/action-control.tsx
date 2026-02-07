"use client";

import { useState, useCallback } from "react";
import type { ResolvedSetting } from "~/lib/settings-helpers";
import { clientExecuteAction } from "~/lib/settings-api";
import { Button } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";

interface ActionControlProps {
  setting: ResolvedSetting;
}

type ActionStatus = "idle" | "executing" | "success" | "error";

export function ActionControl({ setting }: ActionControlProps) {
  const [isConfirmOpen, setIsConfirmOpen] = useState(false);
  const [status, setStatus] = useState<ActionStatus>("idle");
  const [resultMessage, setResultMessage] = useState<string | null>(null);

  const { definition } = setting;
  const isDangerous = definition.actionIsDangerous ?? false;
  const requiresConfirmation = definition.actionRequiresConfirmation ?? true;

  const handleExecute = useCallback(async () => {
    setIsConfirmOpen(false);
    setStatus("executing");
    setResultMessage(null);

    try {
      const result = await clientExecuteAction(setting.key);
      if (result.success) {
        setStatus("success");
        setResultMessage(
          result.message ?? definition.actionSuccessMessage ?? "Erfolgreich",
        );
      } else {
        setStatus("error");
        setResultMessage(
          result.message ?? definition.actionErrorMessage ?? "Fehler",
        );
      }
    } catch {
      setStatus("error");
      setResultMessage(
        definition.actionErrorMessage ?? "Fehler bei der Ausführung",
      );
    }

    // Clear status after delay
    setTimeout(() => {
      setStatus("idle");
      setResultMessage(null);
    }, 3000);
  }, [
    setting.key,
    definition.actionSuccessMessage,
    definition.actionErrorMessage,
  ]);

  const handleClick = useCallback(() => {
    if (requiresConfirmation) {
      setIsConfirmOpen(true);
    } else {
      void handleExecute();
    }
  }, [requiresConfirmation, handleExecute]);

  const confirmButtonClass = isDangerous
    ? "bg-red-600 hover:bg-red-700"
    : "bg-blue-500 hover:bg-blue-600";

  return (
    <div className="flex items-center gap-3">
      <Button
        type="button"
        variant={isDangerous ? "outline_danger" : "outline"}
        size="sm"
        onClick={handleClick}
        disabled={!setting.canEdit}
        isLoading={status === "executing"}
        loadingText="Wird ausgeführt..."
      >
        {definition.label ?? setting.key}
      </Button>

      {status === "success" && resultMessage && (
        <span className="text-sm text-green-600">{resultMessage}</span>
      )}
      {status === "error" && resultMessage && (
        <span className="text-sm text-red-600">{resultMessage}</span>
      )}

      {requiresConfirmation && (
        <ConfirmationModal
          isOpen={isConfirmOpen}
          onClose={() => setIsConfirmOpen(false)}
          onConfirm={handleExecute}
          title={definition.actionConfirmationTitle ?? "Aktion ausführen?"}
          confirmText={definition.actionConfirmationButton ?? "Ausführen"}
          cancelText="Abbrechen"
          confirmButtonClass={confirmButtonClass}
          isConfirmLoading={status === "executing"}
        >
          <p className="text-sm text-gray-700">
            {definition.actionConfirmationMessage ??
              "Möchten Sie diese Aktion ausführen?"}
          </p>
        </ConfirmationModal>
      )}
    </div>
  );
}

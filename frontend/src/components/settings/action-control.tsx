"use client";

import { useState, useCallback } from "react";
import type { ResolvedSetting } from "~/lib/settings-helpers";
import { clientExecuteAction } from "~/lib/settings-api";
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
        setResultMessage(result.message ?? definition.actionSuccessMessage ?? "Erfolgreich");
      } else {
        setStatus("error");
        setResultMessage(result.message ?? definition.actionErrorMessage ?? "Fehler");
      }
    } catch {
      setStatus("error");
      setResultMessage(definition.actionErrorMessage ?? "Fehler bei der Ausführung");
    }

    // Clear status after delay
    setTimeout(() => {
      setStatus("idle");
      setResultMessage(null);
    }, 3000);
  }, [setting.key, definition.actionSuccessMessage, definition.actionErrorMessage]);

  const handleClick = useCallback(() => {
    if (requiresConfirmation) {
      setIsConfirmOpen(true);
    } else {
      void handleExecute();
    }
  }, [requiresConfirmation, handleExecute]);

  const buttonVariant = isDangerous
    ? "border-red-300 text-red-700 hover:border-red-400 hover:bg-red-50"
    : "border-gray-300 text-gray-700 hover:border-gray-400 hover:bg-gray-50";

  const confirmButtonClass = isDangerous
    ? "bg-red-600 hover:bg-red-700"
    : "bg-blue-500 hover:bg-blue-600";

  return (
    <div className="flex items-center gap-3">
      <button
        type="button"
        onClick={handleClick}
        disabled={!setting.canEdit || status === "executing"}
        className={`rounded-lg border px-4 py-2 text-sm font-medium transition-all duration-200 hover:shadow-md disabled:cursor-not-allowed disabled:opacity-50 ${buttonVariant}`}
      >
        {status === "executing" ? (
          <span className="flex items-center gap-2">
            <svg
              className="h-4 w-4 animate-spin"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                className="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                strokeWidth="4"
              />
              <path
                className="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              />
            </svg>
            Wird ausgeführt...
          </span>
        ) : (
          definition.label ?? setting.key
        )}
      </button>

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

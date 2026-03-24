"use client";

import { useState, useCallback } from "react";
import { Modal } from "~/components/ui/modal";
import { TeacherForm } from "./teacher-form";
import type { Teacher } from "@/lib/teacher-api";

const EMPTY_INITIAL_DATA: Partial<Teacher> = {};

interface TeacherCreateModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onCreate: (
    data: Partial<Teacher> & { password?: string; linkExisting?: boolean },
  ) => Promise<{ status?: string; email?: string } | void>;
  readonly loading?: boolean;
}

export function TeacherCreateModal({
  isOpen,
  onClose,
  onCreate,
  loading = false,
}: TeacherCreateModalProps) {
  const [linkConfirmation, setLinkConfirmation] = useState<{
    email: string;
    data: Partial<Teacher> & { password?: string };
  } | null>(null);
  const [isLinking, setIsLinking] = useState(false);

  const handleClose = useCallback(() => {
    setLinkConfirmation(null);
    setIsLinking(false);
    onClose();
  }, [onClose]);

  const handleCreate = useCallback(
    async (data: Partial<Teacher> & { password?: string }) => {
      const result = await onCreate(data);
      // Check if the result signals an existing account
      if (
        result &&
        typeof result === "object" &&
        "status" in result &&
        result.status === "account_exists" &&
        "email" in result &&
        typeof result.email === "string"
      ) {
        setLinkConfirmation({ email: result.email, data });
      }
    },
    [onCreate],
  );

  const handleConfirmLink = useCallback(async () => {
    if (!linkConfirmation) return;
    setIsLinking(true);
    try {
      await onCreate({ ...linkConfirmation.data, linkExisting: true });
      setLinkConfirmation(null);
    } finally {
      setIsLinking(false);
    }
  }, [linkConfirmation, onCreate]);

  // Link confirmation view
  if (linkConfirmation) {
    return (
      <Modal isOpen={isOpen} onClose={handleClose} title="Konto verknüpfen">
        <div className="space-y-4">
          <p className="text-sm text-gray-700">
            Ein Konto mit der E-Mail{" "}
            <span className="font-semibold text-gray-900">
              {linkConfirmation.email}
            </span>{" "}
            existiert bereits.
          </p>
          <p className="text-sm text-gray-600">
            Möchten Sie dieses Konto mit dieser Einrichtung verknüpfen? Das
            bestehende Passwort bleibt unverändert.
          </p>

          <div className="flex gap-3 border-t border-gray-100 pt-4">
            <button
              type="button"
              onClick={handleClose}
              className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
            >
              Abbrechen
            </button>
            <button
              type="button"
              onClick={() => {
                handleConfirmLink().catch(() => undefined);
              }}
              disabled={isLinking}
              className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700 disabled:opacity-50"
            >
              {isLinking ? "Wird verknüpft..." : "Verknüpfen"}
            </button>
          </div>
        </div>
      </Modal>
    );
  }

  // Normal create form
  return (
    <Modal isOpen={isOpen} onClose={handleClose} title="Neues Personal anlegen">
      <TeacherForm
        initialData={EMPTY_INITIAL_DATA}
        onSubmitAction={handleCreate}
        onCancelAction={handleClose}
        isLoading={loading}
        formTitle=""
        wrapInCard={false}
        submitLabel="Erstellen"
      />
    </Modal>
  );
}

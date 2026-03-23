"use client";

import { Modal } from "~/components/ui/modal";
import { TeacherForm } from "./teacher-form";
import type { Teacher } from "@/lib/teacher-api";

const EMPTY_INITIAL_DATA: Partial<Teacher> = {};

interface TeacherCreateModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onCreate: (
    data: Partial<Teacher> & { password?: string },
  ) => Promise<void>;
  readonly loading?: boolean;
}

export function TeacherCreateModal({
  isOpen,
  onClose,
  onCreate,
  loading = false,
}: TeacherCreateModalProps) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Neues Personal anlegen">
      <TeacherForm
        initialData={EMPTY_INITIAL_DATA}
        onSubmitAction={onCreate}
        onCancelAction={onClose}
        isLoading={loading}
        formTitle=""
        wrapInCard={false}
        submitLabel="Erstellen"
      />
    </Modal>
  );
}

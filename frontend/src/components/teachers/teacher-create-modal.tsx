"use client";

import { Modal } from "~/components/ui/modal";
import { TeacherForm } from "./teacher-form";
import type { Teacher } from "@/lib/teacher-api";

interface TeacherCreateModalProps {
  isOpen: boolean;
  onClose: () => void;
  onCreate: (data: Partial<Teacher> & { password?: string }) => Promise<void>;
  loading?: boolean;
}

export function TeacherCreateModal({
  isOpen,
  onClose,
  onCreate,
  loading = false,
}: TeacherCreateModalProps) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Neuen Betreuer erstellen">
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <div className="flex flex-col items-center gap-4">
            <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-[#F78C10]"></div>
            <p className="text-gray-600">Daten werden geladen...</p>
          </div>
        </div>
      ) : (
        <TeacherForm
          initialData={{}}
          onSubmitAction={onCreate}
          onCancelAction={onClose}
          isLoading={loading}
          formTitle=""
          wrapInCard={false}
          submitLabel="Erstellen"
        />
      )}
    </Modal>
  );
}

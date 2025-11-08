"use client";

import { Modal } from "~/components/ui/modal";
import { TeacherForm } from "./teacher-form";
import type { Teacher } from "@/lib/teacher-api";

interface TeacherEditModalProps {
  isOpen: boolean;
  onClose: () => void;
  teacher: Teacher | null;
  onSave: (data: Partial<Teacher> & { password?: string }) => Promise<void>;
  loading?: boolean;
}

export function TeacherEditModal({
  isOpen,
  onClose,
  teacher,
  onSave,
  loading = false,
}: TeacherEditModalProps) {
  if (!teacher) return null;

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Lehrkraft bearbeiten">
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <div className="flex flex-col items-center gap-4">
            <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-[#F78C10]"></div>
            <p className="text-gray-600">Daten werden geladen...</p>
          </div>
        </div>
      ) : (
        <TeacherForm
          initialData={teacher}
          onSubmitAction={onSave}
          onCancelAction={onClose}
          isLoading={loading}
          formTitle=""
          wrapInCard={false}
          submitLabel="Speichern"
        />
      )}
    </Modal>
  );
}

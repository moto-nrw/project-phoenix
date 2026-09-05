"use client";

import {
  SlideOver,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { TeacherForm } from "./teacher-form";
import type { Teacher } from "@/lib/teacher-api";

interface TeacherEditModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly teacher: Teacher | null;
  readonly onSave: (
    data: Partial<Teacher> & { password?: string },
  ) => Promise<void>;
  readonly loading?: boolean;
  readonly existingPositions?: readonly string[];
  // Vorname, Nachname und NFC-Karte liegen am Personen-Datensatz (#2906).
  readonly canEditPersonFields?: boolean;
}

const EMPTY_POSITIONS: readonly string[] = [];

/**
 * Die Stammdaten einer Person sind mehr als sechs Felder. Sie stehen deshalb
 * im Panel neben der Liste und nicht in einem zentrierten Fenster über ihr.
 */
export function TeacherEditModal({
  isOpen,
  onClose,
  teacher,
  onSave,
  loading = false,
  existingPositions = EMPTY_POSITIONS,
  canEditPersonFields = true,
}: TeacherEditModalProps) {
  if (!teacher) return null;

  return (
    <SlideOver
      open={isOpen}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <SlideOverContent widthClass="sm:w-[640px]">
        <SlideOverHeader className="flex-row items-start justify-between gap-3">
          <div className="min-w-0">
            <SlideOverTitle>Personal bearbeiten</SlideOverTitle>
          </div>
          <SlideOverCloseButton />
        </SlideOverHeader>
        <div className="flex-1 overflow-y-auto px-5 py-4">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <div className="flex flex-col items-center gap-4">
                <div className="border-t-moto-orange h-12 w-12 animate-spin rounded-full border-2 border-gray-200"></div>
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
              existingPositions={existingPositions}
              canEditPersonFields={canEditPersonFields}
            />
          )}
        </div>
      </SlideOverContent>
    </SlideOver>
  );
}

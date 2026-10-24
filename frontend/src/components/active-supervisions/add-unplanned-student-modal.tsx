"use client";

import { useId, useState } from "react";
import {
  StudentChoiceList,
  matchStudentChoices,
  useStudentChoices,
} from "~/components/students/student-choice-list";
import { Button } from "~/components/ui/button";
import type { FormErrorInput } from "~/components/ui/form-error";
import { FormModal } from "~/components/ui/form-modal";

export interface AddUnplannedStudentModalProps {
  readonly isOpen: boolean;
  readonly instanceId: string;
  /** Kinder, die gerade schon in diesem Block sind; sie stehen nicht zur Wahl. */
  readonly inBlockStudentIds: ReadonlySet<string>;
  readonly isAddingStudent: boolean;
  readonly search: string;
  readonly error: FormErrorInput;
  readonly onAdd: (studentId: string) => Promise<boolean>;
  readonly onClose: () => void;
  readonly onSearchChange: (value: string) => void;
}

// Das Nachtragen eines Kindes ist eine Kopf-Aktion mit Dialog, kein
// Formular im Listenkörper (#3112, Bauart 1 Regel 3): die Liste bleibt eine
// Liste, und „Hinzufügen“ steht unten im Dialog, wo eine Aktion hingehört.
export function AddUnplannedStudentModal({
  isOpen,
  instanceId,
  inBlockStudentIds,
  isAddingStudent,
  search,
  error,
  onAdd,
  onClose,
  onSearchChange,
}: AddUnplannedStudentModalProps) {
  const formId = useId();
  // Each block is its own scope: after a block switch the old list must not
  // stay selectable for even one render.
  const { load, error: loadError } = useStudentChoices(isOpen, instanceId);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  // Ein Termin-Wechsel bei offenem Dialog verwirft die Auswahl: eine alte
  // Auswahl darf nie in den neuen Termin geschrieben werden.
  const [selectionInstanceId, setSelectionInstanceId] = useState(instanceId);
  if (selectionInstanceId !== instanceId) {
    setSelectionInstanceId(instanceId);
    setSelectedId(null);
  }

  const { target } = matchStudentChoices(
    load,
    search,
    selectedId,
    inBlockStudentIds,
  );

  const handleClose = () => {
    setSelectedId(null);
    onClose();
  };
  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (target && !isAddingStudent && (await onAdd(target.id))) {
      handleClose();
    }
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={handleClose}
      title="Kind ungeplant hinzufügen"
      size="md"
      error={error}
      closeDisabled={isAddingStudent}
      footer={
        <div className="flex flex-wrap justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={handleClose}
            disabled={isAddingStudent}
          >
            Abbrechen
          </Button>
          <Button
            type="submit"
            form={formId}
            disabled={isAddingStudent || !target}
            variant="success"
            size="md"
          >
            {isAddingStudent ? "Wird hinzugefügt…" : "Hinzufügen"}
          </Button>
        </div>
      }
    >
      <form id={formId} onSubmit={handleSubmit} className="space-y-3">
        <p className="text-sm text-gray-600">
          Das Kind wird sofort als anwesend in dieser Aktivität eingetragen.
        </p>
        <StudentChoiceList
          load={load}
          loadError={loadError}
          search={search}
          onSearchChange={onSearchChange}
          searchLabel="Kind ungeplant suchen"
          radioName="unplanned-student"
          selectedId={selectedId}
          onSelectedIdChange={setSelectedId}
          unavailableIds={inBlockStudentIds}
          unavailableLabel="schon dabei"
          disabled={isAddingStudent}
        />
      </form>
    </FormModal>
  );
}

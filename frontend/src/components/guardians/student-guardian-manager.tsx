"use client";

import { useState, useEffect } from "react";
import { UserPlus, Loader2 } from "lucide-react";
import GuardianList from "./guardian-list";
import GuardianFormModal, { type RelationshipFormData } from "./guardian-form-modal";
import type {
  GuardianWithRelationship,
  GuardianFormData,
} from "@/lib/guardian-helpers";
import {
  fetchStudentGuardians,
  createGuardian,
  updateGuardian,
  linkGuardianToStudent,
  updateStudentGuardianRelationship,
  removeGuardianFromStudent,
} from "@/lib/guardian-api";

interface StudentGuardianManagerProps {
  studentId: string;
  readOnly?: boolean;
  onUpdate?: () => void;
}

export default function StudentGuardianManager({
  studentId,
  readOnly = false,
  onUpdate,
}: StudentGuardianManagerProps) {
  const [guardians, setGuardians] = useState<GuardianWithRelationship[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingGuardian, setEditingGuardian] = useState<GuardianWithRelationship | undefined>();
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Load guardians
  const loadGuardians = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await fetchStudentGuardians(studentId);
      setGuardians(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Fehler beim Laden der Erziehungsberechtigten");
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    loadGuardians();
  }, [studentId]);

  // Handle create guardian
  const handleCreateGuardian = async (
    guardianData: GuardianFormData,
    relationshipData: RelationshipFormData
  ) => {
    setIsSubmitting(true);
    try {
      // Create guardian profile
      const newGuardian = await createGuardian(guardianData);

      // Link to student
      await linkGuardianToStudent(studentId, {
        guardianProfileId: newGuardian.id,
        ...relationshipData,
      });

      // Reload guardians
      await loadGuardians();
      onUpdate?.();
    } finally {
      setIsSubmitting(false);
    }
  };

  // Handle edit guardian
  const handleEditGuardian = async (
    guardianData: GuardianFormData,
    relationshipData: RelationshipFormData
  ) => {
    if (!editingGuardian) return;

    setIsSubmitting(true);
    try {
      // Update guardian profile
      await updateGuardian(editingGuardian.id, guardianData);

      // Update relationship
      await updateStudentGuardianRelationship(
        editingGuardian.relationshipId,
        relationshipData
      );

      // Reload guardians
      await loadGuardians();
      onUpdate?.();
      setEditingGuardian(undefined);
    } finally {
      setIsSubmitting(false);
    }
  };

  // Handle delete guardian
  const handleDeleteGuardian = async (guardianId: string) => {
    if (
      !confirm(
        "Möchten Sie diese/n Erziehungsberechtigte/n wirklich von diesem Schüler entfernen?"
      )
    ) {
      return;
    }

    try {
      await removeGuardianFromStudent(studentId, guardianId);
      await loadGuardians();
      onUpdate?.();
    } catch (err) {
      alert(
        err instanceof Error
          ? err.message
          : "Fehler beim Entfernen der/des Erziehungsberechtigten"
      );
    }
  };

  // Open modal for creating
  const handleOpenCreateModal = () => {
    setEditingGuardian(undefined);
    setIsModalOpen(true);
  };

  // Open modal for editing
  const handleOpenEditModal = (guardian: GuardianWithRelationship) => {
    setEditingGuardian(guardian);
    setIsModalOpen(true);
  };

  // Close modal
  const handleCloseModal = () => {
    setIsModalOpen(false);
    setEditingGuardian(undefined);
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="h-8 w-8 animate-spin text-purple-600" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg">
        {error}
      </div>
    );
  }

  // Debug: Log permission state
  console.log('[GuardianManager] readOnly:', readOnly, 'guardians:', guardians.length);

  return (
    <div className="space-y-4">
      {/* Header with Add Button */}
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <h3 className="text-lg font-semibold text-gray-900">
          Erziehungsberechtigte ({guardians.length})
        </h3>
        <div className="flex items-center gap-3">
          {readOnly && (
            <span className="inline-flex items-center gap-1 rounded-md bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600">
              <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
              </svg>
              Nur Ansicht
            </span>
          )}
          {!readOnly && (
            <button
              onClick={handleOpenCreateModal}
              className="inline-flex items-center gap-2 px-4 py-2 bg-gradient-to-r from-purple-500 to-blue-500 text-white rounded-lg hover:from-purple-600 hover:to-blue-600 transition-colors"
            >
              <UserPlus className="h-4 w-4" />
              Hinzufügen
            </button>
          )}
        </div>
      </div>

      {/* Guardian List */}
      <GuardianList
        guardians={guardians}
        onEdit={readOnly ? undefined : handleOpenEditModal}
        onDelete={readOnly ? undefined : handleDeleteGuardian}
        readOnly={readOnly}
        showRelationship={true}
      />

      {/* Form Modal */}
      <GuardianFormModal
        isOpen={isModalOpen}
        onClose={handleCloseModal}
        onSubmit={editingGuardian ? handleEditGuardian : handleCreateGuardian}
        initialData={editingGuardian}
        mode={editingGuardian ? "edit" : "create"}
      />
    </div>
  );
}

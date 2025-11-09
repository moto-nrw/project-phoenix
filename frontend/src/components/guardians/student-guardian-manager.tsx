"use client";

import { useState, useEffect } from "react";
import { UserPlus, Loader2 } from "lucide-react";
import GuardianList from "./guardian-list";
import GuardianFormModal, { RelationshipFormData } from "./guardian-form-modal";
import {
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

  return (
    <div className="space-y-4">
      {/* Header with Add Button */}
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-semibold text-gray-900">
          Erziehungsberechtigte ({guardians.length})
        </h3>
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

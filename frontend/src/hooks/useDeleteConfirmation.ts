"use client";

import { useState, useCallback } from "react";

/**
 * Hook to manage delete confirmation modal flow for database pages.
 *
 * This hook handles the common pattern of:
 * 1. Closing detail modal when delete is clicked
 * 2. Opening confirmation modal
 * 3. Reopening detail modal on cancel
 * 4. Closing confirmation modal and triggering delete on confirm
 *
 * @param setShowDetailModal - Function to control detail modal visibility
 * @returns Object containing confirmation modal state and handlers
 *
 * @example
 * ```tsx
 * const {
 *   showConfirmModal,
 *   handleDeleteClick,
 *   handleDeleteCancel,
 *   confirmDelete,
 * } = useDeleteConfirmation(setShowDetailModal);
 *
 * // In ConfirmationModal:
 * <ConfirmationModal
 *   isOpen={showConfirmModal}
 *   onClose={handleDeleteCancel}
 *   onConfirm={() => confirmDelete(() => void handleDelete())}
 * />
 * ```
 */
export function useDeleteConfirmation(
  setShowDetailModal: (show: boolean) => void,
): {
  showConfirmModal: boolean;
  handleDeleteClick: () => void;
  handleDeleteCancel: () => void;
  confirmDelete: (onDelete: () => void) => void;
} {
  const [showConfirmModal, setShowConfirmModal] = useState(false);

  const handleDeleteClick = useCallback(() => {
    setShowDetailModal(false);
    setShowConfirmModal(true);
  }, [setShowDetailModal]);

  const handleDeleteCancel = useCallback(() => {
    setShowConfirmModal(false);
    setShowDetailModal(true);
  }, [setShowDetailModal]);

  const confirmDelete = useCallback((onDelete: () => void) => {
    setShowConfirmModal(false);
    onDelete();
  }, []);

  return {
    showConfirmModal,
    handleDeleteClick,
    handleDeleteCancel,
    confirmDelete,
  };
}

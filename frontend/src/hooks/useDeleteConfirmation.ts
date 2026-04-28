"use client";

import { useState, useCallback } from "react";

/**
 * Hook to manage delete confirmation modal flow for database pages.
 *
 * Pattern handled:
 * 1. Optionally close a parent detail modal when delete is clicked
 * 2. Open confirmation modal
 * 3. Optionally reopen the parent detail modal on cancel
 * 4. Close confirmation modal and run the delete on confirm
 *
 * Master-detail pages keep the detail panel mounted, so the parent toggle is
 * optional — pass nothing for those callers.
 *
 * @example
 * ```tsx
 * const {
 *   showConfirmModal,
 *   handleDeleteClick,
 *   handleDeleteCancel,
 *   confirmDelete,
 * } = useDeleteConfirmation();
 *
 * <ConfirmationModal
 *   isOpen={showConfirmModal}
 *   onClose={handleDeleteCancel}
 *   onConfirm={() => confirmDelete(() => void handleDelete())}
 * />
 * ```
 */
export function useDeleteConfirmation(
  setShowDetailModal?: (show: boolean) => void,
): {
  showConfirmModal: boolean;
  handleDeleteClick: () => void;
  handleDeleteCancel: () => void;
  confirmDelete: (onDelete: () => void) => void;
} {
  const [showConfirmModal, setShowConfirmModal] = useState(false);

  const handleDeleteClick = useCallback(() => {
    setShowDetailModal?.(false);
    setShowConfirmModal(true);
  }, [setShowDetailModal]);

  const handleDeleteCancel = useCallback(() => {
    setShowConfirmModal(false);
    setShowDetailModal?.(true);
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

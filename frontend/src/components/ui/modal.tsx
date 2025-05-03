"use client";

import React, { useEffect } from "react";

interface ModalProps {
  isOpen: boolean;
  onClose: () => void;
  title: string;
  children: React.ReactNode;
  footer?: React.ReactNode;
}

export function Modal({
  isOpen,
  onClose,
  title,
  children,
  footer,
}: ModalProps) {
  // Close on escape key press
  useEffect(() => {
    const handleEscKey = (event: KeyboardEvent) => {
      if (event.key === "Escape" && isOpen) {
        onClose();
      }
    };

    if (isOpen) {
      document.addEventListener("keydown", handleEscKey);
      // Disable scrolling on body when modal is open
      document.body.style.overflow = "hidden";
    }

    return () => {
      document.removeEventListener("keydown", handleEscKey);
      // Re-enable scrolling when modal is closed
      document.body.style.overflow = "";
    };
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  // Close when clicking on the backdrop (not the modal itself)
  const handleBackdropClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (e.target === e.currentTarget) {
      onClose();
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center backdrop-blur-sm"
      onClick={handleBackdropClick}
    >
      <div
        className="w-full max-w-md rounded-lg border border-gray-100 bg-white/95 p-6 shadow-xl backdrop-blur-sm backdrop-filter"
        onClick={(e) => e.stopPropagation()}
      >
        {title && (
          <h3 className="mb-4 text-lg font-semibold text-gray-900">{title}</h3>
        )}

        <div className="mb-6 text-gray-700">{children}</div>

        {footer && <div className="flex justify-end gap-3">{footer}</div>}
      </div>
    </div>
  );
}

// A specialized confirmation modal with yes/no buttons
interface ConfirmationModalProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  children: React.ReactNode;
  confirmText?: string;
  cancelText?: string;
  isConfirmLoading?: boolean;
  confirmButtonClass?: string;
}

export function ConfirmationModal({
  isOpen,
  onClose,
  onConfirm,
  title,
  children,
  confirmText = "Bestätigen",
  cancelText = "Abbrechen",
  isConfirmLoading = false,
  confirmButtonClass = "bg-blue-500 hover:bg-blue-600",
}: ConfirmationModalProps) {
  const modalFooter = (
    <>
      <button
        onClick={onClose}
        className="rounded-lg bg-gray-200 px-4 py-2 text-gray-800 transition-colors hover:bg-gray-300"
      >
        {cancelText}
      </button>

      <button
        onClick={onConfirm}
        disabled={isConfirmLoading}
        className={`px-4 py-2 ${confirmButtonClass} rounded-lg text-white transition-colors`}
      >
        {isConfirmLoading ? "Wird geladen..." : confirmText}
      </button>
    </>
  );

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={title} footer={modalFooter}>
      {children}
    </Modal>
  );
}

// A specialized deletion confirmation modal with delete/cancel buttons
interface DeleteModalProps {
  isOpen: boolean;
  onClose: () => void;
  onDelete: () => void;
  title: string;
  children: React.ReactNode;
  deleteText?: string;
  cancelText?: string;
  isDeleting?: boolean;
}

export function DeleteModal({
  isOpen,
  onClose,
  onDelete,
  title,
  children,
  deleteText = "Löschen",
  cancelText = "Abbrechen",
  isDeleting = false,
}: DeleteModalProps) {
  return (
    <ConfirmationModal
      isOpen={isOpen}
      onClose={onClose}
      onConfirm={onDelete}
      title={title}
      confirmText={deleteText}
      cancelText={cancelText}
      isConfirmLoading={isDeleting}
      confirmButtonClass="bg-red-500 hover:bg-red-600"
    >
      {children}
    </ConfirmationModal>
  );
}

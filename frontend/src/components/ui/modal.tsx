"use client";

import React, { useEffect, useCallback } from "react";
import { createPortal } from "react-dom";

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
  const [isAnimating, setIsAnimating] = React.useState(false);
  const [isExiting, setIsExiting] = React.useState(false);

  // Enhanced close handler with exit animation
  const handleClose = useCallback(() => {
    setIsExiting(true);
    setIsAnimating(false);
    
    // Delay actual close to allow exit animation
    setTimeout(() => {
      onClose();
    }, 250);
  }, [onClose]);

  // Close on escape key press
  useEffect(() => {
    const handleEscKey = (event: KeyboardEvent) => {
      if (event.key === "Escape" && isOpen) {
        handleClose();
      }
    };

    if (isOpen) {
      document.addEventListener("keydown", handleEscKey);
      // Disable scrolling on body when modal is open
      document.body.style.overflow = "hidden";
      
      // Trigger sophisticated entrance animation with slight delay for smooth effect
      setTimeout(() => {
        setIsAnimating(true);
      }, 10);
    }

    return () => {
      document.removeEventListener("keydown", handleEscKey);
      // Re-enable scrolling when modal is closed
      document.body.style.overflow = "";
      if (!isOpen) {
        setIsAnimating(false);
        setIsExiting(false);
      }
    };
  }, [isOpen, handleClose]);

  if (!isOpen) return null;

  // Close when clicking on the backdrop (not the modal itself)
  const handleBackdropClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (e.target === e.currentTarget) {
      handleClose();
    }
  };

  const modalContent = (
    <div
      className={`fixed inset-0 z-[9999] flex items-center justify-center transition-all duration-400 ease-out ${
        isAnimating && !isExiting 
          ? 'bg-black/40 backdrop-blur-md' 
          : 'bg-black/0 backdrop-blur-none'
      }`}
      onClick={handleBackdropClick}
      style={{ 
        position: 'fixed', 
        top: 0, 
        left: 0, 
        right: 0, 
        bottom: 0,
        animation: isAnimating && !isExiting ? 'backdropEnter 400ms ease-out' : undefined
      }}
    >
      <div
        className={`relative w-[calc(100%-2rem)] max-w-lg mx-4 rounded-2xl shadow-2xl border border-gray-200/50 overflow-hidden transform ${
          isAnimating && !isExiting
            ? 'animate-modalEnter' 
            : isExiting
            ? 'animate-modalExit'
            : 'scale-75 opacity-0 translate-y-8 -rotate-1'
        }`}
        onClick={(e) => e.stopPropagation()}
        style={{
          background: 'linear-gradient(135deg, rgba(255,255,255,0.95) 0%, rgba(248,250,252,0.98) 100%)',
          backdropFilter: 'blur(20px)',
          boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.25), 0 8px 16px -8px rgba(80, 128, 216, 0.15)',
          animationFillMode: 'both'
        }}
      >
        {/* Header with close button */}
        <div className="flex items-center justify-between p-6 border-b border-gray-100">
          {title && (
            <h3 className="text-xl font-semibold text-gray-900 pr-4">{title}</h3>
          )}
          <button
            onClick={handleClose}
            className="group relative flex-shrink-0 p-2 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded-xl transition-all duration-200 hover:scale-105 active:scale-95"
            aria-label="Modal schließen"
          >
            {/* Animated X icon */}
            <svg 
              className="w-5 h-5 transition-transform duration-200 group-hover:rotate-90" 
              fill="none" 
              viewBox="0 0 24 24" 
              stroke="currentColor"
              strokeWidth={2}
            >
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
            
            {/* Subtle hover glow */}
            <div 
              className="absolute inset-0 rounded-xl opacity-0 group-hover:opacity-100 transition-opacity duration-200"
              style={{
                boxShadow: '0 0 12px rgba(80,128,216,0.3)'
              }}
            />
          </button>
        </div>

        {/* Content area with custom scrollbar and reveal animation */}
        <div className="max-h-[70vh] overflow-y-auto scrollbar-thin scrollbar-thumb-gray-300 scrollbar-track-gray-100">
          <div className={`p-6 text-gray-700 leading-relaxed ${
            isAnimating && !isExiting ? 'animate-contentReveal' : 'opacity-0'
          }`}>
            {children}
          </div>
        </div>

        {/* Footer if provided */}
        {footer && (
          <div className="flex justify-end gap-3 p-6 border-t border-gray-100 bg-gray-50/50">
            {footer}
          </div>
        )}
      </div>
    </div>
  );

  // Render to body to avoid any positioning issues
  if (typeof document !== 'undefined') {
    return createPortal(modalContent, document.body);
  }

  return modalContent;
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

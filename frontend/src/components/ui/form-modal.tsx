"use client";

import { useEffect, useCallback, useState } from "react";
import type { ReactNode } from "react";
import { createPortal } from "react-dom";
import { useModal } from "../dashboard/modal-context";

interface FormModalProps {
  isOpen: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  footer?: ReactNode;
  size?: "sm" | "md" | "lg" | "xl";
}

export function FormModal({
  isOpen,
  onClose,
  title,
  children,
  footer,
  size = "lg"
}: FormModalProps) {
  const [isAnimating, setIsAnimating] = useState(false);
  const [isExiting, setIsExiting] = useState(false);
  const { openModal, closeModal } = useModal();

  // Map size to max-width classes
  const sizeClasses = {
    sm: "max-w-md",
    md: "max-w-lg", 
    lg: "max-w-2xl",
    xl: "max-w-4xl"
  };

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
      document.body.style.overflow = "hidden";
      openModal();
      window.dispatchEvent(new CustomEvent('mobile-modal-open'));
      
      setTimeout(() => {
        setIsAnimating(true);
      }, 10);
    } else {
      closeModal();
      window.dispatchEvent(new CustomEvent('mobile-modal-close'));
    }

    return () => {
      document.removeEventListener("keydown", handleEscKey);
      document.body.style.overflow = "";
      if (!isOpen) {
        setIsAnimating(false);
        setIsExiting(false);
      }
    };
  }, [isOpen, handleClose, openModal, closeModal]);

  if (!isOpen) return null;

  // Close when clicking on the backdrop
  const handleBackdropClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (e.target === e.currentTarget) {
      handleClose();
    }
  };

  const modalContent = (
    <div
      className={`fixed inset-0 z-[9999] flex items-end md:items-center justify-center md:p-6 transition-all duration-200 ${
        isAnimating && !isExiting 
          ? 'bg-black/40' 
          : 'bg-black/0'
      }`}
      onClick={handleBackdropClick}
    >
      <div
        className={`relative w-full ${sizeClasses[size]} h-full md:h-auto max-h-[90vh] md:max-h-[85vh] rounded-t-2xl md:rounded-xl shadow-lg md:shadow-xl border border-gray-200 bg-white overflow-hidden transform transition-all duration-200 ${
          isAnimating && !isExiting
            ? 'translate-y-0 md:scale-100 md:opacity-100' 
            : isExiting
            ? 'translate-y-full md:translate-y-0 md:scale-95 md:opacity-0'
            : 'translate-y-full md:translate-y-0 md:scale-95 md:opacity-0'
        }`}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header with close button */}
        <div className="flex items-center justify-between p-4 md:p-6 border-b border-gray-100">
          {title && (
            <h3 className="text-lg md:text-xl font-semibold text-gray-900 pr-4">{title}</h3>
          )}
          <button
            onClick={handleClose}
            className="group relative flex-shrink-0 h-10 w-10 flex items-center justify-center text-gray-400 hover:text-gray-600 hover:bg-gray-50 rounded-lg transition-all duration-200 hover:scale-105 active:scale-[0.98]"
            aria-label="Modal schließen"
          >
            <svg 
              className="w-5 h-5 transition-transform duration-200 group-hover:rotate-90" 
              fill="none" 
              viewBox="0 0 24 24" 
              stroke="currentColor"
              strokeWidth={2}
            >
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Content area */}
        <div className="max-h-[60vh] md:max-h-[70vh] overflow-y-auto">
          <div className={`p-4 md:p-6 text-gray-700 ${
            isAnimating && !isExiting ? 'opacity-100' : 'opacity-0'
          } transition-opacity duration-200`}>
            {children}
          </div>
        </div>

        {/* Footer if provided */}
        {footer && (
          <div className="flex justify-end gap-3 p-4 md:p-6 border-t border-gray-100 bg-gray-50">
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

interface CreateFormModalProps {
  isOpen: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  size?: "sm" | "md" | "lg" | "xl";
}

export function CreateFormModal({
  isOpen,
  onClose,
  title,
  children,
  size = "lg"
}: CreateFormModalProps) {
  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={title}
      size={size}
    >
      {children}
    </FormModal>
  );
}

interface DetailFormModalProps {
  isOpen: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  size?: "sm" | "md" | "lg" | "xl";
  loading?: boolean;
  error?: string | null;
  onRetry?: () => void;
}

export function DetailFormModal({
  isOpen,
  onClose,
  title,
  children,
  size = "xl",
  loading = false,
  error = null,
  onRetry
}: DetailFormModalProps) {
  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={title}
      size={size}
    >
      {loading ? (
        <div className="flex flex-col items-center justify-center py-8 md:py-12">
          <div className="flex flex-col items-center gap-4">
            <div className="h-10 w-10 md:h-12 md:w-12 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
            <p className="text-sm md:text-base text-gray-600">Daten werden geladen...</p>
          </div>
        </div>
      ) : error ? (
        <div className="rounded-lg bg-red-50 p-4 md:p-6 text-red-800 shadow-sm">
          <h3 className="mb-2 text-base md:text-lg font-semibold">Fehler</h3>
          <p className="mb-4 text-sm md:text-base">{error}</p>
          {onRetry && (
            <button
              onClick={onRetry}
              className="min-h-[44px] rounded-lg border border-red-300 bg-white px-4 py-2 text-sm font-medium text-red-600 shadow-sm transition-all duration-200 hover:bg-red-50 active:scale-[0.98]"
            >
              Erneut versuchen
            </button>
          )}
        </div>
      ) : (
        children
      )}
    </FormModal>
  );
}
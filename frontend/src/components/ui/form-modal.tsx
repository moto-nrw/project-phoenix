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
      className={`fixed inset-0 z-[9999] flex items-center justify-center transition-all duration-400 ease-out ${
        isAnimating && !isExiting 
          ? 'bg-black/40' 
          : 'bg-black/0'
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
        className={`relative w-[calc(100%-2rem)] ${sizeClasses[size]} mx-4 rounded-2xl shadow-2xl border border-gray-200/50 overflow-hidden transform ${
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
            <svg 
              className="w-5 h-5 transition-transform duration-200 group-hover:rotate-90" 
              fill="none" 
              viewBox="0 0 24 24" 
              stroke="currentColor"
              strokeWidth={2}
            >
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
            
            <div 
              className="absolute inset-0 rounded-xl opacity-0 group-hover:opacity-100 transition-opacity duration-200"
              style={{
                boxShadow: '0 0 12px rgba(80,128,216,0.3)'
              }}
            />
          </button>
        </div>

        {/* Content area */}
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
        <div className="flex flex-col items-center justify-center py-12">
          <div className="flex flex-col items-center gap-4">
            <div className="h-10 w-10 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
            <p className="text-sm text-gray-600">Daten werden geladen...</p>
          </div>
        </div>
      ) : error ? (
        <div className="rounded-lg bg-red-50 p-4 text-red-800">
          <h3 className="mb-2 font-semibold">Fehler</h3>
          <p className="mb-4">{error}</p>
          {onRetry && (
            <button
              onClick={onRetry}
              className="rounded-lg bg-red-100 px-4 py-2 text-red-800 transition-colors hover:bg-red-200"
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
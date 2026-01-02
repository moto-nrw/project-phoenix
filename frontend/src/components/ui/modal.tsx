"use client";

import React, { useEffect, useCallback } from "react";
import { createPortal } from "react-dom";
import { useModal } from "../dashboard/modal-context";
import { useScrollLock } from "~/hooks/useScrollLock";
import {
  stopPropagation,
  backdropAriaProps,
  dialogAriaProps,
  getModalAnimationClass,
} from "./modal-utils";

interface ModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly title: string;
  readonly children: React.ReactNode;
  readonly footer?: React.ReactNode;
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
  const { openModal, closeModal } = useModal();

  // Use scroll lock hook
  useScrollLock(isOpen);

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
      // Trigger blur effect on layout
      openModal();
      // Dispatch custom event for ResponsiveLayout (help modal)
      window.dispatchEvent(new CustomEvent("mobile-modal-open"));

      // Trigger sophisticated entrance animation with slight delay for smooth effect
      setTimeout(() => {
        setIsAnimating(true);
      }, 10);
    } else {
      // Remove blur effect on layout
      closeModal();
      // Dispatch custom event for ResponsiveLayout
      window.dispatchEvent(new CustomEvent("mobile-modal-close"));
    }

    return () => {
      document.removeEventListener("keydown", handleEscKey);
      if (!isOpen) {
        setIsAnimating(false);
        setIsExiting(false);
      }
    };
  }, [isOpen, handleClose, openModal, closeModal]);

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
        isAnimating && !isExiting ? "bg-black/40" : "bg-black/0"
      }`}
      onClick={handleBackdropClick}
      {...backdropAriaProps}
      style={{
        position: "fixed",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        animation:
          isAnimating && !isExiting
            ? "backdropEnter 400ms ease-out"
            : undefined,
      }}
    >
      <div
        className={`relative mx-4 w-[calc(100%-2rem)] max-w-lg transform overflow-hidden rounded-2xl border border-gray-200/50 shadow-2xl ${getModalAnimationClass(isAnimating, isExiting)}`}
        {...stopPropagation}
        {...dialogAriaProps}
        style={{
          background:
            "linear-gradient(135deg, rgba(255,255,255,0.95) 0%, rgba(248,250,252,0.98) 100%)",
          backdropFilter: "blur(20px)",
          boxShadow:
            "0 25px 50px -12px rgba(0, 0, 0, 0.25), 0 8px 16px -8px rgba(80, 128, 216, 0.15)",
          animationFillMode: "both",
        }}
      >
        {/* Header with close button - only show border if title exists */}
        {title ? (
          <div className="flex items-center justify-between border-b border-gray-100 p-6">
            <h3 className="pr-4 text-xl font-semibold text-gray-900">
              {title}
            </h3>
            <button
              onClick={handleClose}
              className="group relative flex-shrink-0 rounded-xl p-2 text-gray-400 transition-all duration-200 hover:scale-105 hover:bg-gray-100 hover:text-gray-600 active:scale-95"
              aria-label="Modal schließen"
            >
              {/* Animated X icon */}
              <svg
                className="h-5 w-5 transition-transform duration-200 group-hover:rotate-90"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={2}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>

              {/* Subtle hover glow */}
              <div
                className="absolute inset-0 rounded-xl opacity-0 transition-opacity duration-200 group-hover:opacity-100"
                style={{
                  boxShadow: "0 0 12px rgba(80,128,216,0.3)",
                }}
              />
            </button>
          </div>
        ) : (
          /* X button positioned absolutely in top-right when no title */
          <button
            onClick={handleClose}
            className="group absolute top-4 right-4 z-10 rounded-xl p-2 text-gray-400 transition-all duration-200 hover:scale-105 hover:bg-gray-100 hover:text-gray-600 active:scale-95"
            aria-label="Modal schließen"
          >
            {/* Animated X icon */}
            <svg
              className="h-5 w-5 transition-transform duration-200 group-hover:rotate-90"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>

            {/* Subtle hover glow */}
            <div
              className="absolute inset-0 rounded-xl opacity-0 transition-opacity duration-200 group-hover:opacity-100"
              style={{
                boxShadow: "0 0 12px rgba(80,128,216,0.3)",
              }}
            />
          </button>
        )}

        {/* Content area with hidden scrollbar and reveal animation */}
        <div
          className="scrollbar-hidden max-h-[calc(100vh-8rem)] overflow-y-auto md:max-h-[70vh]"
          data-modal-content="true"
        >
          <div
            className={`p-4 leading-relaxed text-gray-700 md:p-6 ${
              isAnimating && !isExiting ? "animate-contentReveal" : "opacity-0"
            }`}
          >
            {children}
          </div>
        </div>

        {/* Footer if provided */}
        {footer && (
          <div className="flex justify-end gap-3 border-t border-gray-100 bg-gray-50/50 p-6">
            {footer}
          </div>
        )}
      </div>
    </div>
  );

  // Render to body to avoid any positioning issues
  if (typeof document !== "undefined") {
    return createPortal(modalContent, document.body);
  }

  return modalContent;
}

// A specialized confirmation modal with yes/no buttons
interface ConfirmationModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onConfirm: () => void;
  readonly title: string;
  readonly children: React.ReactNode;
  readonly confirmText?: string;
  readonly cancelText?: string;
  readonly isConfirmLoading?: boolean;
  readonly confirmButtonClass?: string;
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
        type="button"
        onClick={onClose}
        className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:scale-105 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100"
      >
        {cancelText}
      </button>

      <button
        type="button"
        onClick={onConfirm}
        disabled={isConfirmLoading}
        className={`flex-1 rounded-lg px-4 py-2 ${confirmButtonClass} text-sm font-medium text-white transition-all duration-200 hover:scale-105 hover:shadow-lg active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:scale-100`}
      >
        {isConfirmLoading ? (
          <span className="flex items-center justify-center gap-2">
            <svg
              className="h-4 w-4 animate-spin text-white"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                className="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                strokeWidth="4"
              ></circle>
              <path
                className="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
            Wird geladen...
          </span>
        ) : (
          confirmText
        )}
      </button>
    </>
  );

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={title} footer={modalFooter}>
      {children}
    </Modal>
  );
}

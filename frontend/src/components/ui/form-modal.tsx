"use client";

import { useEffect, useCallback, useState, useRef } from "react";
import type { ReactNode } from "react";
import { createPortal } from "react-dom";
import { useModal } from "../dashboard/modal-context";
import { useScrollLock } from "~/hooks/useScrollLock";
import { dialogAriaProps } from "./modal-utils";

interface FormModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly title: string;
  readonly children: ReactNode;
  readonly footer?: ReactNode;
  readonly size?: "sm" | "md" | "lg" | "xl";
  // Where to position the modal on mobile viewports
  // 'bottom' mimics a bottom sheet; 'center' behaves like a classic modal
  readonly mobilePosition?: "bottom" | "center";
}

export function FormModal({
  isOpen,
  onClose,
  title,
  children,
  footer,
  size = "lg",
  mobilePosition = "bottom",
}: FormModalProps) {
  const [isAnimating, setIsAnimating] = useState(false);
  const [isExiting, setIsExiting] = useState(false);
  const { openModal, closeModal } = useModal();

  // Store functions in refs to avoid effect re-runs
  const openModalRef = useRef(openModal);
  const closeModalRef = useRef(closeModal);
  openModalRef.current = openModal;
  closeModalRef.current = closeModal;

  // Use scroll lock hook (handles overflow:hidden and event blocking)
  useScrollLock(isOpen);

  // Map size to max-width classes
  const sizeClasses = {
    sm: "max-w-md",
    md: "max-w-lg",
    lg: "max-w-2xl",
    xl: "max-w-4xl",
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

  // Handle modal context state for blur overlay
  useEffect(() => {
    if (isOpen) {
      openModalRef.current();
      return () => {
        closeModalRef.current();
      };
    }
  }, [isOpen]);

  // Close on escape key press and handle animations
  useEffect(() => {
    const handleEscKey = (event: KeyboardEvent) => {
      if (event.key === "Escape" && isOpen) {
        handleClose();
      }
    };

    if (isOpen) {
      document.addEventListener("keydown", handleEscKey);
      globalThis.dispatchEvent(new CustomEvent("mobile-modal-open"));

      setTimeout(() => {
        setIsAnimating(true);
      }, 10);
    } else {
      globalThis.dispatchEvent(new CustomEvent("mobile-modal-close"));
    }

    return () => {
      document.removeEventListener("keydown", handleEscKey);
      if (!isOpen) {
        setIsAnimating(false);
        setIsExiting(false);
      }
    };
  }, [isOpen, handleClose]);

  if (!isOpen) return null;

  const radiusClass =
    mobilePosition === "bottom"
      ? "rounded-t-2xl md:rounded-2xl"
      : "rounded-2xl";
  const modalContent = (
    <div
      className={`fixed inset-0 z-[9999] flex ${mobilePosition === "bottom" ? "items-end" : "items-center"} justify-center md:items-center md:p-6`}
      style={{ position: "fixed", top: 0, left: 0, right: 0, bottom: 0 }}
    >
      {/* Backdrop button - native button for accessibility (keyboard + click support) */}
      <button
        type="button"
        onClick={handleClose}
        aria-label="Hintergrund - Klicken zum Schließen"
        className={`absolute inset-0 cursor-default border-none bg-transparent p-0 transition-all duration-400 ease-out ${
          isAnimating && !isExiting ? "bg-black/40" : "bg-black/0"
        }`}
        style={{
          animation:
            isAnimating && !isExiting
              ? "backdropEnter 400ms ease-out"
              : undefined,
        }}
      />
      {/* Dialog container */}
      <div
        className={`relative w-full ${sizeClasses[size]} ${mobilePosition === "bottom" ? "h-full" : "h-auto"} max-h-[90vh] md:h-auto md:max-h-[85vh] ${radiusClass} ${mobilePosition === "center" ? "mx-4" : ""} transform overflow-hidden border border-gray-200/50 shadow-2xl ${(() => {
          if (isAnimating && !isExiting) return "animate-modalEnter";
          if (isExiting) return "animate-modalExit";
          return "translate-y-8 scale-75 -rotate-1 opacity-0";
        })()}`}
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
        {/* Header with close button */}
        <div className="flex items-center justify-between border-b border-gray-100 p-4 md:p-6">
          {title && (
            <h3 className="pr-4 text-lg font-semibold text-gray-900 md:text-xl">
              {title}
            </h3>
          )}
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

        {/* Content area with custom scrollbar and reveal animation */}
        <div
          className={`${footer ? "max-h-[calc(90vh-240px)] md:max-h-[calc(85vh-240px)]" : "max-h-[60vh] md:max-h-[70vh]"} scrollbar-thin scrollbar-thumb-gray-300 scrollbar-track-gray-100 overflow-y-auto`}
        >
          <div
            className={`p-4 leading-relaxed text-gray-700 md:p-6 ${
              isAnimating && !isExiting ? "animate-contentReveal" : "opacity-0"
            }`}
          >
            {children}
          </div>
        </div>

        {/* Footer if provided - now sticky at bottom */}
        {footer && (
          <div className="sticky bottom-0 flex justify-end gap-3 border-t border-gray-100 bg-gray-50/95 p-4 backdrop-blur-sm md:p-6">
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

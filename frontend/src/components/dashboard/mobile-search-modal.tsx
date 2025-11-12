// components/dashboard/mobile-search-modal.tsx
"use client";

import React, { useEffect, useRef } from "react";
import { createPortal } from "react-dom";
import { GlobalSearch } from "./global-search";
import { useModal } from "./modal-context";

interface MobileSearchModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export function MobileSearchModal({ isOpen, onClose }: MobileSearchModalProps) {
  const modalRef = useRef<HTMLDivElement>(null);
  const { openModal, closeModal } = useModal();

  // Handle escape key and backdrop click
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        onClose();
      }
    };

    const handleClickOutside = (e: MouseEvent) => {
      if (modalRef.current && !modalRef.current.contains(e.target as Node)) {
        onClose();
      }
    };

    if (isOpen) {
      document.addEventListener("keydown", handleKeyDown);
      document.addEventListener("mousedown", handleClickOutside);
      // Prevent body scroll when modal is open
      document.body.style.overflow = "hidden";
      // Trigger blur effect on layout
      openModal();
      // Dispatch custom event for ResponsiveLayout
      window.dispatchEvent(new CustomEvent("mobile-modal-open"));
    } else {
      // Remove blur effect on layout
      closeModal();
      // Dispatch custom event for ResponsiveLayout
      window.dispatchEvent(new CustomEvent("mobile-modal-close"));
    }

    return () => {
      document.removeEventListener("keydown", handleKeyDown);
      document.removeEventListener("mousedown", handleClickOutside);
      document.body.style.overflow = "unset";
    };
  }, [isOpen, onClose, openModal, closeModal]);

  // Auto-focus search input when opened
  useEffect(() => {
    if (isOpen) {
      // Small delay to ensure modal is rendered
      setTimeout(() => {
        const searchInput = modalRef.current?.querySelector("input");
        searchInput?.focus();
      }, 100);
    }
  }, [isOpen]);

  if (!isOpen) return null;

  const modalContent = (
    <div className="fixed inset-0 z-[9999] lg:hidden">
      {/* Backdrop without blur (blur is handled by ResponsiveLayout) */}
      <div className="fixed inset-0 bg-black/30 transition-all duration-300" />

      {/* Modal */}
      <div className="fixed inset-x-0 top-0 z-[9999]">
        <div
          ref={modalRef}
          className="mb-safe mx-4 mt-4 overflow-hidden rounded-2xl bg-white shadow-2xl"
          style={{
            maxHeight:
              "calc(100vh - 2rem - env(safe-area-inset-top) - env(safe-area-inset-bottom))",
          }}
        >
          {/* Header */}
          <div className="flex items-center justify-between border-b border-gray-100 bg-gray-50/50 p-4">
            <div className="flex items-center space-x-3">
              <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-[#5080d8]/10 to-[#83cd2d]/10">
                <svg
                  className="h-4 w-4 text-[#5080d8]"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                  />
                </svg>
              </div>
              <h2 className="text-lg font-semibold text-gray-900">Suche</h2>
            </div>

            <button
              onClick={onClose}
              className="rounded-lg p-2 transition-colors duration-200 hover:bg-gray-100 active:bg-gray-200"
              aria-label="Suche schließen"
            >
              <svg
                className="h-5 w-5 text-gray-500"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>

          {/* Search Content */}
          <div className="p-4">
            <GlobalSearch
              className="w-full"
              placeholder="Schüler, Räume, Aktivitäten suchen..."
              onResultSelect={onClose}
            />
          </div>

          {/* Quick Actions */}
          <div className="px-4 pb-4">
            <h3 className="mb-3 text-sm font-medium text-gray-700">
              Schnellzugriff
            </h3>
            <div className="grid grid-cols-2 gap-3">
              {[
                {
                  href: "/students/search",
                  label: "Alle Schüler",
                  icon: "user",
                },
                { href: "/rooms", label: "Räume", icon: "building" },
                {
                  href: "/activities",
                  label: "Aktivitäten",
                  icon: "clipboard",
                },
                { href: "/ogs_groups", label: "OGS Gruppen", icon: "users" },
              ].map((action) => (
                <button
                  key={action.href}
                  onClick={() => {
                    onClose();
                    window.location.href = action.href;
                  }}
                  className="flex items-center space-x-3 rounded-lg border border-gray-200 p-3 transition-all duration-200 hover:border-[#5080d8]/30 hover:bg-[#5080d8]/5 active:bg-[#5080d8]/10"
                >
                  <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-gray-100">
                    {action.icon === "user" && (
                      <svg
                        className="h-4 w-4 text-gray-600"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
                        />
                      </svg>
                    )}
                    {action.icon === "building" && (
                      <svg
                        className="h-4 w-4 text-gray-600"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
                        />
                      </svg>
                    )}
                    {action.icon === "clipboard" && (
                      <svg
                        className="h-4 w-4 text-gray-600"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
                        />
                      </svg>
                    )}
                    {action.icon === "users" && (
                      <svg
                        className="h-4 w-4 text-gray-600"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
                        />
                      </svg>
                    )}
                  </div>
                  <span className="text-sm font-medium text-gray-900">
                    {action.label}
                  </span>
                </button>
              ))}
            </div>
          </div>

          {/* Safe area padding */}
          <div className="h-safe-area-inset-bottom" />
        </div>
      </div>
    </div>
  );

  // Render to body to avoid being affected by ResponsiveLayout blur
  if (typeof document !== "undefined") {
    return createPortal(modalContent, document.body);
  }

  return modalContent;
}

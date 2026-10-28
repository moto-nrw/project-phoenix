"use client";

import React, { useState } from "react";
import { useShellAuth } from "~/lib/shell-auth-context";
import { DELIBERATE_LOGOUT_KEY } from "~/lib/session-cache";
import { Modal } from "./modal";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "LogoutModal" });

interface LogoutModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
}

const LogOutIcon = ({ className }: { className?: string }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    width="48"
    height="48"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
  >
    <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
    <polyline points="16 17 21 12 16 7" />
    <line x1="21" x2="9" y1="12" y2="12" />
  </svg>
);

export function LogoutModal({ isOpen, onClose }: LogoutModalProps) {
  const { logout } = useShellAuth();
  const [isLoggingOut, setIsLoggingOut] = useState(false);

  const handleConfirmLogout = async () => {
    setIsLoggingOut(true);

    try {
      await logout();
    } catch (error) {
      logger.error("failed to sign out", {
        error: error instanceof Error ? error.message : String(error),
      });
      try {
        sessionStorage.removeItem(DELIBERATE_LOGOUT_KEY);
      } catch {
        // sessionStorage unavailable
      }
      setIsLoggingOut(false);
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={isLoggingOut ? () => undefined : onClose}
      title=""
      footer={undefined}
    >
      <div className="py-4 text-center">
        <LogOutIcon className="mx-auto mb-4 h-12 w-12 text-gray-700" />
        <h1 className="mt-4 text-3xl font-bold tracking-tight text-gray-800 sm:text-4xl">
          Abmelden
        </h1>
        <p className="mt-4 text-gray-600">
          Möchten Sie sich wirklich von Ihrem Konto abmelden?
        </p>
        <div className="mt-6">
          <button
            type="button"
            onClick={handleConfirmLogout}
            disabled={isLoggingOut}
            className="inline-flex items-center rounded-xl bg-gray-900 px-6 py-2.5 text-sm font-semibold text-white shadow-lg transition-all duration-200 hover:scale-105 hover:bg-gray-800 hover:shadow-xl focus:outline-none active:scale-95 disabled:cursor-not-allowed disabled:bg-gray-400 disabled:shadow-none disabled:hover:scale-100"
          >
            <span>{isLoggingOut ? "Abmeldung läuft..." : "Abmelden"}</span>
          </button>
        </div>
      </div>
    </Modal>
  );
}

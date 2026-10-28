"use client";

import React, { useState } from "react";
import { useShellAuth } from "~/lib/shell-auth-context";
import { DELIBERATE_LOGOUT_KEY } from "~/lib/session-cache";
import { ConfirmationModal } from "./modal";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
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
  const [isLoggingOut, setIsLoggingOut] = useState(false);

  const { logout } = useShellAuth();
  const { show: showLogoutError } = useApiErrorDisplay();

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
      // The session stays signed in until the server confirms the logout
      // (#5194). The modal closes so the error toast and its retry are not
      // covered by the backdrop.
      onClose();
      void showLogoutError(error, {
        object: "die Abmeldung",
        retry: () => void handleConfirmLogout(),
      });
    }
  };

  return (
    <ConfirmationModal
      isOpen={isOpen}
      onClose={onClose}
      onConfirm={() => void handleConfirmLogout()}
      title="Abmelden"
      confirmText="Abmelden"
      loadingText="Abmeldung läuft..."
      isConfirmLoading={isLoggingOut}
      isDismissDisabled={isLoggingOut}
      mobileSheet
    >
      <LogOutIcon className="mx-auto mb-4 h-12 w-12 text-gray-700" />
      <p className="text-center">
        Möchten Sie sich wirklich von Ihrem Konto abmelden?
      </p>
    </ConfirmationModal>
  );
}

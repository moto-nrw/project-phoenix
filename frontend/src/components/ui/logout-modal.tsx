"use client";

import React, { useState } from "react";
import { useShellAuth } from "~/lib/shell-auth-context";
import { Modal } from "./modal";

interface LogoutModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
}

// Confetti animation constants for better maintainability
const CONFETTI_CONFIG = {
  PIECE_COUNT: 100,
  COLORS: ["#FF3130", "#F78C10", "#83CD2D", "#5080D8"],
  SIZE: {
    MIN_WIDTH: 5,
    MAX_WIDTH: 10,
    MIN_HEIGHT: 5,
    MAX_HEIGHT: 5,
  },
  ANIMATION: {
    MIN_DURATION: 2000,
    MAX_DURATION: 2000,
    SPREAD_DISTANCE: 400,
    MAX_ROTATION: 720,
    STAGGER_DELAY: 300,
  },
} as const;

// Logout Icon als React Component
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

  const launchConfetti = () => {
    const confettiContainer = document.createElement("div");
    confettiContainer.style.position = "fixed";
    confettiContainer.style.width = "100%";
    confettiContainer.style.height = "100%";
    confettiContainer.style.top = "0";
    confettiContainer.style.left = "0";
    confettiContainer.style.pointerEvents = "none";
    confettiContainer.style.zIndex = "10000";
    document.body.appendChild(confettiContainer);

    const centerX = window.innerWidth / 2;
    const centerY = window.innerHeight / 2;

    for (let i = 0; i < CONFETTI_CONFIG.PIECE_COUNT; i++) {
      setTimeout(() => {
        const confetti = document.createElement("div");
        const color =
          CONFETTI_CONFIG.COLORS[
            Math.floor(Math.random() * CONFETTI_CONFIG.COLORS.length)
          ]!;

        // Calculate random sizes using config
        const width =
          Math.random() * CONFETTI_CONFIG.SIZE.MAX_WIDTH +
          CONFETTI_CONFIG.SIZE.MIN_WIDTH;
        const height =
          Math.random() * CONFETTI_CONFIG.SIZE.MAX_HEIGHT +
          CONFETTI_CONFIG.SIZE.MIN_HEIGHT;

        confetti.style.position = "absolute";
        confetti.style.width = `${width}px`;
        confetti.style.height = `${height}px`;
        confetti.style.backgroundColor = color;
        confetti.style.borderRadius = Math.random() > 0.5 ? "50%" : "0";
        confetti.style.opacity = "0.8";
        confetti.style.left = `${centerX}px`;
        confetti.style.top = `${centerY}px`;

        confettiContainer.appendChild(confetti);

        // Calculate random spread and rotation
        const spreadX =
          Math.random() * CONFETTI_CONFIG.ANIMATION.SPREAD_DISTANCE -
          CONFETTI_CONFIG.ANIMATION.SPREAD_DISTANCE / 2;
        const spreadY =
          Math.random() * CONFETTI_CONFIG.ANIMATION.SPREAD_DISTANCE -
          CONFETTI_CONFIG.ANIMATION.SPREAD_DISTANCE / 2;
        const rotation = Math.random() * CONFETTI_CONFIG.ANIMATION.MAX_ROTATION;
        const duration =
          Math.random() * CONFETTI_CONFIG.ANIMATION.MAX_DURATION +
          CONFETTI_CONFIG.ANIMATION.MIN_DURATION;

        const animation = confetti.animate(
          [
            {
              transform: "translate(-50%, -50%) rotate(0deg)",
              opacity: 0.8,
            },
            {
              transform: `translate(${spreadX}px, ${spreadY}px) rotate(${rotation}deg)`,
              opacity: 0,
            },
          ],
          {
            duration: duration,
            easing: "cubic-bezier(0.2, 0.8, 0.2, 1)",
          },
        );

        animation.onfinish = () => {
          confetti.remove();
          if (confettiContainer.children.length === 0) {
            confettiContainer.remove();
          }
        };
      }, Math.random() * CONFETTI_CONFIG.ANIMATION.STAGGER_DELAY);
    }
  };

  const handleConfirmLogout = async () => {
    setIsLoggingOut(true);
    launchConfetti();

    try {
      // Allow NextAuth to perform the CSRF handshake before navigating away.
      await logout();
    } catch (error) {
      console.error("Failed to sign out:", error);
      setIsLoggingOut(false);
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={isLoggingOut ? () => undefined : onClose}
      title="" // Leerer String zeigt nur X-Button ohne Titel
      footer={undefined} // Kein Footer
    >
      <div className="py-4 text-center">
        {isLoggingOut ? (
          <>
            {/* Minimal loading state - just icon and text */}
            <LogOutIcon className="mx-auto mb-4 h-12 w-12 text-gray-400 opacity-50" />
            <h2 className="mb-2 animate-pulse text-2xl font-bold text-gray-800">
              Abmelden...
            </h2>
            <p className="text-gray-600">
              Sie werden zur Anmeldeseite weitergeleitet
            </p>
          </>
        ) : (
          <>
            <LogOutIcon className="mx-auto mb-4 h-12 w-12 text-gray-700" />
            <h1 className="mt-4 text-3xl font-bold tracking-tight text-gray-800 sm:text-4xl">
              Abmelden
            </h1>
            <p className="mt-4 text-gray-600">
              Möchten Sie sich wirklich von Ihrem Konto abmelden?
            </p>
            <div className="mt-6">
              <button
                onClick={handleConfirmLogout}
                className="inline-flex items-center rounded-xl bg-gray-900 px-6 py-2.5 text-sm font-semibold text-white shadow-lg transition-all duration-200 hover:scale-105 hover:bg-gray-800 hover:shadow-xl focus:ring-2 focus:ring-gray-900 focus:ring-offset-2 focus:outline-none active:scale-95"
              >
                <span>Abmelden</span>
              </button>
            </div>
          </>
        )}
      </div>
    </Modal>
  );
}

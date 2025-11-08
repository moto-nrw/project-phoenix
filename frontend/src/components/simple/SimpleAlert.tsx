"use client";

import React, { useEffect, useState, useRef, useContext } from "react";
import { AlertContext } from "~/contexts/AlertContext";

interface SimpleAlertProps {
  type: "success" | "error" | "info" | "warning";
  message: string;
  onClose?: () => void;
  autoClose?: boolean;
  duration?: number;
}

const alertStyles = {
  success: {
    bg: "bg-[#83CD2D]/10",
    border: "border-[#83CD2D]/20",
    text: "text-[#5A8B1F]",
    icon: "M5 13l4 4L19 7",
  },
  error: {
    bg: "bg-[#FF3130]/10",
    border: "border-[#FF3130]/20",
    text: "text-[#CC2626]",
    icon: "M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z",
  },
  info: {
    bg: "bg-[#5080D8]/10",
    border: "border-[#5080D8]/20",
    text: "text-[#4070C8]",
    icon: "M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z",
  },
  warning: {
    bg: "bg-[#F78C10]/10",
    border: "border-[#F78C10]/20",
    text: "text-[#C56F0D]",
    icon: "M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z",
  },
};

export function SimpleAlert({
  type,
  message,
  onClose,
  autoClose = false,
  duration = 3000,
}: SimpleAlertProps) {
  // Use context safely - it's optional for backward compatibility
  const alertContext = useContext(AlertContext);

  const styles = alertStyles[type];
  const [isVisible, setIsVisible] = useState(false);
  const [isExiting, setIsExiting] = useState(false);

  // Notify context when alert is shown/hidden (if context is available)
  useEffect(() => {
    if (alertContext) {
      alertContext.showAlert(type, message);
      return () => {
        alertContext.hideAlert();
      };
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [type, message]);

  // Use ref to store the latest onClose callback without triggering effect re-runs
  const onCloseRef = useRef(onClose);
  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);

  useEffect(() => {
    // Trigger entrance animation
    const showTimer = setTimeout(() => {
      setIsVisible(true);
    }, 10);

    let autoCloseTimer: NodeJS.Timeout | undefined;
    let exitTimer: NodeJS.Timeout | undefined;

    if (autoClose) {
      autoCloseTimer = setTimeout(() => {
        // Start exit animation
        setIsExiting(true);
        // Actually close after animation
        exitTimer = setTimeout(() => {
          if (onCloseRef.current) {
            onCloseRef.current();
          }
        }, 300);
      }, duration);
    }

    return () => {
      clearTimeout(showTimer);
      if (autoCloseTimer) clearTimeout(autoCloseTimer);
      if (exitTimer) clearTimeout(exitTimer);
    };
  }, [autoClose, duration]); // Remove onClose from dependencies to prevent timer reset

  return (
    <div
      className={`fixed right-4 bottom-6 left-4 z-[9998] md:right-6 md:left-auto md:max-w-sm ${styles.bg} ${styles.border} rounded-2xl border p-4 shadow-lg backdrop-blur-sm transition-all duration-300 ease-out ${isVisible && !isExiting ? "translate-y-0 opacity-100" : "translate-y-4 opacity-0"} `}
      style={{
        transform:
          isVisible && !isExiting ? "translateY(0)" : "translateY(16px)",
        opacity: isVisible && !isExiting ? 1 : 0,
      }}
    >
      <div className="flex items-start gap-3">
        <div className={`flex-shrink-0 ${styles.text}`}>
          <svg
            className="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d={styles.icon}
            />
          </svg>
        </div>
        <p className={`flex-1 text-sm font-medium ${styles.text}`}>{message}</p>
        {onClose && (
          <button
            onClick={onClose}
            className={`flex-shrink-0 ${styles.text} transition-opacity hover:opacity-70`}
          >
            <svg
              className="h-4 w-4"
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
          </button>
        )}
      </div>
      {autoClose && (
        <div className="absolute right-0 bottom-0 left-2 h-1 overflow-hidden rounded-b-2xl bg-gray-200/20">
          <div
            className="h-full bg-current opacity-30"
            style={{
              animation: `shrink ${duration}ms linear forwards`,
              transformOrigin: "left",
              width: "100%",
            }}
          />
        </div>
      )}
    </div>
  );
}

// Add this CSS to your global styles or as a style tag
export const alertAnimationStyles = (
  <style>{`
    @keyframes shrink {
      from {
        transform: scaleX(1);
      }
      to {
        transform: scaleX(0);
      }
    }
  `}</style>
);

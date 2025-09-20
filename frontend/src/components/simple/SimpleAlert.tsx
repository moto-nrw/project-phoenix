"use client";

import React, { useEffect } from "react";

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
    icon: "M6 18L18 6M6 6l12 12",
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
  const styles = alertStyles[type];

  useEffect(() => {
    if (autoClose && onClose) {
      const timer = setTimeout(onClose, duration);
      return () => clearTimeout(timer);
    }
  }, [autoClose, duration, onClose]);

  return (
    <div
      className={`
        fixed bottom-24 lg:bottom-6 right-6 z-50 max-w-sm
        ${styles.bg} ${styles.border} 
        rounded-2xl border p-4 shadow-lg backdrop-blur-sm
        animate-in slide-in-from-bottom-5 duration-300
      `}
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
        <p className={`flex-1 text-sm font-medium ${styles.text}`}>
          {message}
        </p>
        {onClose && (
          <button
            onClick={onClose}
            className={`flex-shrink-0 ${styles.text} hover:opacity-70 transition-opacity`}
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
        <div className="absolute bottom-0 left-0 right-0 h-1 overflow-hidden rounded-b-2xl">
          <div
            className={`h-full bg-current opacity-20 animate-[shrink_${duration}ms_linear_forwards]`}
            style={{
              animation: `shrink ${duration}ms linear forwards`,
            }}
          />
        </div>
      )}
    </div>
  );
}

// Add this CSS to your global styles
export const alertAnimation = `
  @keyframes shrink {
    from {
      width: 100%;
    }
    to {
      width: 0%;
    }
  }
`;
"use client";

import React, { useEffect, useState } from "react";

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
  // Dispatch event to notify FAB to move up on mobile
  useEffect(() => {
    if (typeof window !== 'undefined') {
      window.dispatchEvent(new CustomEvent('alert-show'));
      return () => {
        window.dispatchEvent(new CustomEvent('alert-hide'));
      };
    }
  }, []);
  const styles = alertStyles[type];
  const [isVisible, setIsVisible] = useState(false);
  const [isExiting, setIsExiting] = useState(false);

  useEffect(() => {
    // Trigger entrance animation
    const showTimer = setTimeout(() => {
      setIsVisible(true);
    }, 10);

    if (autoClose && onClose) {
      const timer = setTimeout(() => {
        // Start exit animation
        setIsExiting(true);
        // Actually close after animation
        setTimeout(onClose, 300);
      }, duration);
      return () => {
        clearTimeout(timer);
        clearTimeout(showTimer);
      };
    }

    return () => clearTimeout(showTimer);
  }, [autoClose, duration, onClose]);

  return (
    <div
      className={`
        fixed bottom-24 lg:bottom-6 left-4 right-4 md:left-auto md:right-6 z-[9998] md:max-w-sm
        ${styles.bg} ${styles.border} 
        rounded-2xl border p-4 shadow-lg backdrop-blur-sm
        transition-all duration-300 ease-out
        ${isVisible && !isExiting ? 'translate-y-0 opacity-100' : 'translate-y-4 opacity-0'}
      `}
      style={{
        transform: isVisible && !isExiting ? 'translateY(0)' : 'translateY(16px)',
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
        <div className="absolute bottom-0 left-0 right-0 h-1 bg-gray-200/20 overflow-hidden rounded-b-2xl">
          <div
            className="h-full bg-current opacity-30"
            style={{
              animation: `shrink ${duration}ms linear forwards`,
              transformOrigin: 'left',
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
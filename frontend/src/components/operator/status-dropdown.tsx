"use client";

import { useState, useRef, useEffect, useCallback } from "react";
import { createPortal } from "react-dom";
import {
  OPERATOR_STATUS_LABELS,
  OPERATOR_STATUS_STYLES,
  OPERATOR_STATUS_DOT_COLORS,
} from "~/lib/operator/suggestions-helpers";
import type { OperatorSuggestionStatus } from "~/lib/operator/suggestions-helpers";

interface StatusDropdownProps {
  readonly value: OperatorSuggestionStatus;
  readonly onChange: (value: OperatorSuggestionStatus) => void;
  readonly disabled?: boolean;
  readonly size?: "sm" | "md";
  readonly onOpenChange?: (open: boolean) => void;
}

interface DropdownPosition {
  top: number;
  left: number;
  alignRight: boolean;
}

export function StatusDropdown({
  value,
  onChange,
  disabled = false,
  size = "sm",
  onOpenChange,
}: StatusDropdownProps) {
  const [isOpen, setIsOpenRaw] = useState(false);
  const [mounted, setMounted] = useState(false);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const [position, setPosition] = useState<DropdownPosition>({
    top: 0,
    left: 0,
    alignRight: false,
  });

  const setIsOpen = useCallback(
    (open: boolean) => {
      setIsOpenRaw(open);
      onOpenChange?.(open);
    },
    [onOpenChange],
  );

  // Client-side only for portal
  useEffect(() => {
    setMounted(true);
  }, []);

  // Calculate position when opening
  useEffect(() => {
    if (isOpen && buttonRef.current) {
      const rect = buttonRef.current.getBoundingClientRect();
      const dropdownWidth = 192;
      const windowWidth = window.innerWidth;

      const alignRight = rect.left + dropdownWidth > windowWidth - 16;

      // Use viewport coordinates directly (no scroll offset needed for position: fixed)
      setPosition({
        top: rect.bottom + 4,
        left: alignRight ? rect.right : rect.left,
        alignRight,
      });
    }
  }, [isOpen]);

  // Close on click outside
  useEffect(() => {
    if (!isOpen) return;

    function handleClickOutside(event: MouseEvent) {
      const target = event.target as Node;
      const clickedButton = buttonRef.current?.contains(target);
      const clickedMenu = menuRef.current?.contains(target);

      if (!clickedButton && !clickedMenu) {
        setIsOpen(false);
      }
    }

    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [isOpen, setIsOpen]);

  // Close on scroll
  useEffect(() => {
    if (!isOpen) return;

    function handleScroll() {
      setIsOpen(false);
    }

    window.addEventListener("scroll", handleScroll, true);
    return () => window.removeEventListener("scroll", handleScroll, true);
  }, [isOpen, setIsOpen]);

  const sizeClasses =
    size === "sm" ? "px-2.5 py-0.5 text-xs" : "px-3 py-1 text-sm";

  const dropdownMenu = isOpen && mounted && (
    <div
      ref={menuRef}
      role="listbox"
      className="fixed z-[9999] w-48 rounded-xl border border-gray-200 bg-white py-1 shadow-lg"
      style={{
        top: position.top,
        left: position.alignRight ? "auto" : position.left,
        right: position.alignRight ? window.innerWidth - position.left : "auto",
      }}
      onClick={(e) => e.stopPropagation()}
      onKeyDown={(e) => {
        if (e.key === "Escape") {
          setIsOpen(false);
        }
      }}
    >
      {Object.entries(OPERATOR_STATUS_LABELS).map(([key, label]) => {
        const isSelected = key === value;
        return (
          <button
            key={key}
            type="button"
            onClick={() => {
              onChange(key as OperatorSuggestionStatus);
              setIsOpen(false);
            }}
            className={`flex w-full items-center gap-2 px-4 py-2 text-left text-sm transition-colors hover:bg-gray-50 ${isSelected ? "bg-gray-50 font-medium text-gray-900" : "text-gray-700"}`}
          >
            <span
              className={`inline-block h-2 w-2 flex-shrink-0 rounded-full ${OPERATOR_STATUS_DOT_COLORS[key as OperatorSuggestionStatus]}`}
            />
            <span className="flex-1">{label}</span>
            {isSelected && (
              <svg
                className="h-4 w-4 flex-shrink-0 text-gray-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M5 13l4 4L19 7"
                />
              </svg>
            )}
          </button>
        );
      })}
    </div>
  );

  return (
    <>
      <button
        ref={buttonRef}
        type="button"
        onClick={(e) => {
          e.stopPropagation();
          if (!disabled) setIsOpen(!isOpen);
        }}
        disabled={disabled}
        className={`flex items-center gap-1 rounded-full font-medium outline-none ${sizeClasses} ${OPERATOR_STATUS_STYLES[value]} transition-all disabled:opacity-50 ${isOpen ? "ring-2 ring-blue-500 ring-offset-1" : ""}`}
      >
        <span>{OPERATOR_STATUS_LABELS[value]}</span>
        <svg
          className={`h-3.5 w-3.5 flex-shrink-0 opacity-60 transition-transform ${isOpen ? "rotate-180" : ""}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </button>

      {mounted && createPortal(dropdownMenu, document.body)}
    </>
  );
}

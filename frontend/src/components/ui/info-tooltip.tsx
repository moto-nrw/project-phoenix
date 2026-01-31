"use client";

import { useState, useRef, useEffect } from "react";
import { HelpCircle } from "lucide-react";

interface InfoTooltipProps {
  /** The content to display in the tooltip */
  content: string;
  /** Optional custom icon size */
  iconSize?: number;
  /** Optional additional className for the icon */
  iconClassName?: string;
}

/**
 * Info tooltip component that shows a "?" icon and displays
 * content on hover/focus.
 */
export function InfoTooltip({
  content,
  iconSize = 16,
  iconClassName = "",
}: InfoTooltipProps) {
  const [isVisible, setIsVisible] = useState(false);
  const [position, setPosition] = useState<"top" | "bottom">("top");
  const triggerRef = useRef<HTMLButtonElement>(null);
  const tooltipRef = useRef<HTMLDivElement>(null);

  // Determine tooltip position based on available space
  useEffect(() => {
    if (isVisible && triggerRef.current) {
      const rect = triggerRef.current.getBoundingClientRect();
      const spaceAbove = rect.top;
      const spaceBelow = window.innerHeight - rect.bottom;

      // Show below if not enough space above (less than 100px)
      setPosition(
        spaceAbove < 100 && spaceBelow > spaceAbove ? "bottom" : "top",
      );
    }
  }, [isVisible]);

  return (
    <div className="relative inline-flex items-center">
      <button
        ref={triggerRef}
        type="button"
        className={`rounded-full p-0.5 text-gray-400 transition-colors hover:text-gray-600 focus:ring-2 focus:ring-blue-500 focus:ring-offset-1 focus:outline-none ${iconClassName}`}
        onMouseEnter={() => setIsVisible(true)}
        onMouseLeave={() => setIsVisible(false)}
        onFocus={() => setIsVisible(true)}
        onBlur={() => setIsVisible(false)}
        aria-label="Mehr Informationen"
        aria-describedby={isVisible ? "info-tooltip" : undefined}
      >
        <HelpCircle size={iconSize} />
      </button>

      {isVisible && (
        <div
          ref={tooltipRef}
          id="info-tooltip"
          role="tooltip"
          className={`absolute z-50 w-64 rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700 shadow-lg ${
            position === "top"
              ? "bottom-full left-1/2 mb-2 -translate-x-1/2"
              : "top-full left-1/2 mt-2 -translate-x-1/2"
          }`}
        >
          {content}
          {/* Arrow */}
          <div
            className={`absolute left-1/2 h-2 w-2 -translate-x-1/2 rotate-45 border-gray-200 bg-white ${
              position === "top"
                ? "top-full -mt-1 border-r border-b"
                : "bottom-full -mb-1 border-t border-l"
            }`}
          />
        </div>
      )}
    </div>
  );
}

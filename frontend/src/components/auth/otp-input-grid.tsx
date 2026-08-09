"use client";

import {
  type ClipboardEvent,
  type KeyboardEvent,
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
} from "react";

export interface OTPInputGridHandle {
  /** Clear all digits and refocus the first input. */
  reset(): void;
  /** Move focus to the first digit input. */
  focus(): void;
}

interface OTPInputGridProps {
  readonly length: number;
  readonly disabled?: boolean;
  /** Fires exactly once per fill — guarded internally to prevent double-submit. */
  readonly onComplete: (code: string) => void;
  readonly ariaLabel?: string;
  readonly autoFocus?: boolean;
}

/**
 * 6-digit (or configurable length) one-time-code input grid. Owns all
 * input state, key/paste UX, focus management, and the "fire onComplete
 * once when all digits are filled" guard. Parents control submit flow
 * via {@link OTPInputGridHandle} (reset/focus) and the onComplete
 * callback.
 *
 * Shared by MFAChallengeForm and MFAEnrollmentScreen; introduced to
 * collapse 82 lines of duplicated UX logic flagged by SonarCloud.
 */
export const OTPInputGrid = forwardRef<OTPInputGridHandle, OTPInputGridProps>(
  function OTPInputGrid(
    {
      length,
      disabled = false,
      onComplete,
      ariaLabel = "Bestätigungscode",
      autoFocus = true,
    },
    ref,
  ) {
    const [digits, setDigits] = useState<string[]>(() =>
      Array.from({ length }, () => ""),
    );
    const slots = useMemo(
      () => Array.from({ length }, (_, position) => `otp-slot-${position}`),
      [length],
    );
    const inputRefs = useRef<(HTMLInputElement | null)[]>([]);
    const submittedRef = useRef(false);

    useImperativeHandle(
      ref,
      () => ({
        reset: () => {
          setDigits(Array.from({ length }, () => ""));
          submittedRef.current = false;
          inputRefs.current[0]?.focus();
        },
        focus: () => {
          inputRefs.current[0]?.focus();
        },
      }),
      [length],
    );

    useEffect(() => {
      if (autoFocus) inputRefs.current[0]?.focus();
    }, [autoFocus]);

    const handleDigitChange = useCallback(
      (index: number, value: string) => {
        const sanitized = value.replace(/\D/g, "");
        if (sanitized.length === 0) {
          setDigits((prev) => {
            const next = [...prev];
            next[index] = "";
            return next;
          });
          return;
        }
        const next = [...digits];
        const chars = sanitized.slice(0, length - index).split("");
        for (let i = 0; i < chars.length; i++) {
          next[index + i] = chars[i] ?? "";
        }
        setDigits(next);

        const focusTarget = Math.min(index + chars.length, length - 1);
        requestAnimationFrame(() => inputRefs.current[focusTarget]?.focus());
        if (next.every((digit) => digit !== "") && !submittedRef.current) {
          submittedRef.current = true;
          onComplete(next.join(""));
        }
      },
      [digits, length, onComplete],
    );

    const handleKeyDown = useCallback(
      (index: number, e: KeyboardEvent<HTMLInputElement>) => {
        if (e.key === "Backspace") {
          if (digits[index]) {
            setDigits((prev) => {
              const next = [...prev];
              next[index] = "";
              return next;
            });
            return;
          }
          if (index > 0) {
            e.preventDefault();
            inputRefs.current[index - 1]?.focus();
            setDigits((prev) => {
              const next = [...prev];
              next[index - 1] = "";
              return next;
            });
          }
        } else if (e.key === "ArrowLeft" && index > 0) {
          e.preventDefault();
          inputRefs.current[index - 1]?.focus();
        } else if (e.key === "ArrowRight" && index < length - 1) {
          e.preventDefault();
          inputRefs.current[index + 1]?.focus();
        }
      },
      [digits, length],
    );

    const handlePaste = useCallback(
      (e: ClipboardEvent<HTMLInputElement>) => {
        const pasted = e.clipboardData.getData("text").replace(/\D/g, "");
        if (pasted.length === 0) return;
        e.preventDefault();
        handleDigitChange(0, pasted);
      },
      [handleDigitChange],
    );

    return (
      <div
        role="group"
        className="flex justify-center gap-2"
        aria-label={ariaLabel}
      >
        {slots.map((slotId, index) => (
          <input
            key={slotId}
            ref={(el) => {
              inputRefs.current[index] = el;
            }}
            type="text"
            inputMode="numeric"
            autoComplete="one-time-code"
            pattern="[0-9]"
            maxLength={length}
            value={digits[index] ?? ""}
            onChange={(e) => handleDigitChange(index, e.target.value)}
            onKeyDown={(e) => handleKeyDown(index, e)}
            onPaste={handlePaste}
            disabled={disabled}
            aria-label={`Stelle ${index + 1}`}
            className="focus-visible:ring-moto-blue h-14 w-12 rounded-lg border-0 bg-white text-center text-2xl font-semibold text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all ring-inset focus:outline-none focus-visible:ring-2 disabled:bg-gray-50 disabled:text-gray-400"
          />
        ))}
      </div>
    );
  },
);

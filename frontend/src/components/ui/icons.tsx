/**
 * Reusable SVG icon components
 * Extracted to eliminate code duplication across components
 */

interface IconProps {
  readonly className?: string;
}

/**
 * Checkmark icon - for success states
 */
export function CheckIcon({ className = "h-5 w-5" }: IconProps) {
  return (
    <svg
      className={className}
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
  );
}

/**
 * Spinner icon - for loading states
 * Note: animate-spin is always applied; className adds sizing/color
 */
export function SpinnerIcon({ className = "h-4 w-4" }: IconProps) {
  return (
    <svg
      className={`animate-spin motion-reduce:animate-none ${className}`}
      aria-hidden="true"
      fill="none"
      viewBox="0 0 24 24"
    >
      <circle
        className="opacity-25"
        cx="12"
        cy="12"
        r="10"
        stroke="currentColor"
        strokeWidth="4"
      />
      <path
        className="opacity-75"
        fill="currentColor"
        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
      />
    </svg>
  );
}

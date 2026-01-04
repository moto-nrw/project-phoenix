"use client";

interface ModalLoadingStateProps {
  /** The accent color for the spinner. Defaults to gray. */
  readonly accentColor?: "orange" | "indigo" | "blue" | "green" | "gray";
  /** Custom loading message. Defaults to "Daten werden geladen..." */
  readonly message?: string;
}

const accentColorClasses = {
  orange: "border-t-[#F78C10]",
  indigo: "border-t-indigo-500",
  blue: "border-t-[#5080D8]",
  green: "border-t-green-500",
  gray: "border-t-gray-500",
} as const;

/**
 * Loading state component for use inside modals.
 * Shows a centered spinner with customizable accent color.
 */
export function ModalLoadingState({
  accentColor = "gray",
  message = "Daten werden geladen...",
}: ModalLoadingStateProps) {
  return (
    <div className="flex items-center justify-center py-12">
      <div className="flex flex-col items-center gap-4">
        <div
          className={`h-12 w-12 animate-spin rounded-full border-2 border-gray-200 ${accentColorClasses[accentColor]}`}
        />
        <p className="text-gray-600">{message}</p>
      </div>
    </div>
  );
}

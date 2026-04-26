"use client";

interface DatabaseCreateActionProps {
  label: string;
  ariaLabel: string;
  onClick: () => void;
  showDesktop: boolean;
}

export function DatabaseCreateAction({
  label,
  ariaLabel,
  onClick,
  showDesktop,
}: DatabaseCreateActionProps) {
  return (
    <>
      {showDesktop ? (
        <button
          type="button"
          onClick={onClick}
          className="flex h-10 items-center gap-2 rounded-lg bg-[#83CD2D] px-4 text-sm font-semibold text-white hover:bg-[#76B929]"
          aria-label={ariaLabel}
        >
          + {label}
        </button>
      ) : null}
      <button
        type="button"
        onClick={onClick}
        className="fixed right-4 bottom-24 z-40 flex h-14 w-14 items-center justify-center rounded-full bg-[#83CD2D] text-white shadow-lg hover:bg-[#76B929] md:hidden"
        aria-label={ariaLabel}
      >
        <svg
          className="h-6 w-6"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2.5}
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M12 4.5v15m7.5-7.5h-15"
          />
        </svg>
      </button>
    </>
  );
}

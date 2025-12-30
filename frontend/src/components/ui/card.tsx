"use client";

interface CardProps {
  children: React.ReactNode;
  className?: string;
  onClick?: () => void;
}

const baseCardStyles =
  "w-full max-w-md space-y-6 rounded-xl border-0 bg-white/95 p-8 shadow-xl transition-all duration-300 hover:translate-y-[-2px] hover:shadow-2xl";

export function Card({
  children,
  className = "",
  onClick,
}: Readonly<CardProps>) {
  // Use semantic button element when clickable for proper accessibility
  if (onClick) {
    return (
      <button
        type="button"
        className={`${baseCardStyles} cursor-pointer text-left ${className}`}
        onClick={onClick}
      >
        {children}
      </button>
    );
  }

  return <div className={`${baseCardStyles} ${className}`}>{children}</div>;
}

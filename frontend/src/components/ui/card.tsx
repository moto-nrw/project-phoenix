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

interface CardHeaderProps {
  title: string;
  description?: string;
}

export function CardHeader({ title, description }: Readonly<CardHeaderProps>) {
  return (
    <div className="text-center">
      <h1 className="text-3xl font-bold text-teal-600">{title}</h1>
      {description && (
        <p className="mt-2 text-sm text-gray-600">{description}</p>
      )}
    </div>
  );
}

interface CardContentProps {
  children: React.ReactNode;
}

export function CardContent({ children }: Readonly<CardContentProps>) {
  return <div className="mt-8 space-y-6">{children}</div>;
}

interface CardFooterProps {
  children: React.ReactNode;
}

export function CardFooter({ children }: Readonly<CardFooterProps>) {
  return <div className="mt-4 text-center text-sm">{children}</div>;
}

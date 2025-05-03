"use client";

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  isLoading?: boolean;
  loadingText?: string;
  variant?: "primary" | "secondary" | "outline";
}

export function Button({
  children,
  isLoading,
  loadingText,
  variant = "primary",
  className = "",
  ...props
}: ButtonProps) {
  // Base styles
  const baseStyles =
    "flex w-full justify-center rounded-lg px-4 py-3 text-sm font-medium shadow-md focus:outline-none focus:ring-2 focus:ring-offset-2 disabled:opacity-50 transition-all duration-200";

  // Variant-specific styles
  const variantStyles = {
    primary:
      "bg-gradient-to-r from-teal-500 to-blue-500 text-white hover:from-teal-600 hover:to-blue-600 hover:scale-[1.02] hover:shadow-lg focus:ring-teal-500",
    secondary:
      "bg-gray-200 text-gray-800 hover:bg-gray-300 hover:scale-[1.02] hover:shadow-md focus:ring-gray-500",
    outline:
      "bg-transparent text-teal-600 ring-1 ring-teal-500 hover:bg-teal-50 hover:scale-[1.02] hover:shadow-sm focus:ring-teal-500",
  };

  return (
    <button
      type="submit"
      disabled={isLoading}
      className={`${baseStyles} ${variantStyles[variant]} ${className}`}
      {...props}
    >
      {isLoading ? (loadingText ?? "Loading...") : children}
    </button>
  );
}

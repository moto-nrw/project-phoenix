"use client";

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  isLoading?: boolean;
  loadingText?: string;
  variant?:
    | "primary"
    | "secondary"
    | "outline"
    | "outline_danger"
    | "danger"
    | "success";
  size?: "sm" | "base" | "lg" | "xl";
}

export function Button({
  children,
  isLoading,
  loadingText,
  variant = "primary",
  size = "base",
  className = "",
  ...props
}: Readonly<ButtonProps>) {
  // Text sizes basierend auf size prop
  const textSizes = {
    sm: "text-sm", // 14px
    base: "text-base", // 16px
    lg: "text-lg", // 18px
    xl: "text-xl", // 20px
  };

  // Base styles ohne text-sm
  const baseStyles =
    "inline-flex items-center justify-center rounded-lg px-5 py-3 font-medium shadow-md focus:outline-none focus:ring-2 focus:ring-offset-2 disabled:opacity-50 transition-all duration-200";

  // Variant-specific styles (using current app standards: bg-gray-900 for primary)
  const variantStyles = {
    primary:
      "bg-gray-900 text-white hover:bg-gray-700 hover:shadow-lg focus:ring-gray-900",
    secondary:
      "bg-gray-200 text-gray-800 hover:bg-gray-300 hover:shadow-md focus:ring-gray-500",
    outline:
      "bg-transparent text-gray-700 ring-1 ring-gray-300 hover:bg-gray-50 hover:ring-gray-400 focus:ring-gray-500",
    outline_danger:
      "bg-red-50 text-red-600 ring-1 ring-red-300 hover:bg-red-100 hover:ring-red-400 focus:ring-red-500",
    danger:
      "bg-red-600 text-white hover:bg-red-700 hover:shadow-lg focus:ring-red-500",
    success:
      "bg-green-600 text-white hover:bg-green-700 hover:shadow-lg active:scale-95 focus:ring-green-500",
  };

  return (
    <button
      type="submit"
      disabled={isLoading}
      className={`${baseStyles} ${variantStyles[variant]} ${textSizes[size]} ${className}`}
      {...props}
    >
      {isLoading ? (loadingText ?? "Loading...") : children}
    </button>
  );
}

"use client";

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  isLoading?: boolean;
  loadingText?: string;
  variant?:
    | "primary"
    | "secondary"
    | "outline"
    | "outline_danger"
    | "outline_purple"
    | "danger"
    | "success"
    | "ghost";
  size?: "sm" | "md" | "base" | "lg" | "xl" | "compact" | "icon";
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
    md: "text-sm", // 14px — modal/form-action height (Flo standard)
    base: "text-base", // 16px
    lg: "text-lg", // 18px
    xl: "text-xl", // 20px
    compact: "text-xs", // 12px — dense toolbar/chrome
    icon: "text-xs", // icon-only square
  };

  // Geometry (radius + padding) per size. sm/base/lg/xl keep the original
  // page-level geometry so every existing caller renders unchanged. md is the
  // modal-footer / dense-form-action height (px-4 py-2 text-sm ≈ 36px) that
  // matches the ConfirmationModal action buttons — use it for slide-over /
  // modal footers and in-form actions instead of the oversized page sizes.
  // compact and icon are the dense chrome variants for toolbars, dropdown
  // triggers, and icon actions.
  const sizeStyles = {
    sm: "rounded-lg px-5 py-3",
    md: "rounded-lg px-4 py-2",
    base: "rounded-lg px-5 py-3",
    lg: "rounded-lg px-5 py-3",
    xl: "rounded-lg px-5 py-3",
    compact: "h-8 gap-1.5 rounded-md px-2.5 disabled:cursor-not-allowed",
    icon: "h-8 w-8 rounded-md disabled:cursor-not-allowed",
  };

  // Base styles — geometry and shadow are applied per size/variant so the
  // ghost variant can stay flat while every other variant keeps shadow-md.
  const baseStyles =
    "inline-flex items-center justify-center font-medium focus:outline-none disabled:opacity-50 transition-all duration-200";

  // Variant-specific styles (using current app standards: bg-gray-900 for primary)
  const variantStyles = {
    primary:
      "bg-gray-900 text-white shadow-md hover:bg-gray-700 hover:shadow-lg",
    secondary:
      "bg-gray-200 text-gray-800 shadow-md hover:bg-gray-300 hover:shadow-md",
    outline:
      "bg-transparent text-gray-700 ring-1 ring-gray-300 shadow-md hover:bg-gray-50 hover:ring-gray-400",
    outline_danger:
      "bg-red-50 text-red-600 ring-1 ring-red-300 shadow-md hover:bg-red-100 hover:ring-red-400",
    // Purple = the "Rückfrage"/query accent (LOCATION_COLORS.EXCUSED #7C3AED).
    outline_purple:
      "bg-transparent text-[#7C3AED] ring-1 ring-[#7C3AED]/40 shadow-md hover:bg-[#7C3AED]/5 hover:ring-[#7C3AED]/60",
    danger: "bg-red-600 text-white shadow-md hover:bg-red-700 hover:shadow-lg",
    success:
      "bg-green-600 text-white shadow-md hover:bg-green-700 hover:shadow-lg active:scale-95",
    // Flat, no shadow — for dense toolbar/menu/icon chrome. Pair with size
    // "compact" or "icon" and pass type="button" outside forms.
    ghost: "bg-transparent text-gray-600 hover:bg-gray-100 hover:text-gray-900",
  };

  return (
    <button
      type="submit"
      disabled={isLoading}
      className={`${baseStyles} ${sizeStyles[size]} ${variantStyles[variant]} ${textSizes[size]} ${className}`}
      {...props}
    >
      {isLoading ? (loadingText ?? "Loading...") : children}
    </button>
  );
}

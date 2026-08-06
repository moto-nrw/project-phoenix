"use client";

import Link from "next/link";

type ButtonVariant =
  | "primary"
  | "secondary"
  | "outline"
  | "outline_danger"
  | "danger"
  | "success"
  | "ghost";
type ButtonSize = "sm" | "md" | "base" | "lg" | "xl" | "compact" | "icon";

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  isLoading?: boolean;
  loadingText?: string;
  variant?: ButtonVariant;
  size?: ButtonSize;
}

interface ButtonLinkProps extends Omit<
  React.ComponentProps<typeof Link>,
  "className"
> {
  readonly variant?: ButtonVariant;
  readonly size?: ButtonSize;
  readonly className?: string;
}

function buttonClassName({
  variant,
  size,
  className,
}: Readonly<{
  variant: ButtonVariant;
  size: ButtonSize;
  className: string;
}>) {
  const textSizes: Record<ButtonSize, string> = {
    sm: "text-sm",
    md: "text-sm",
    base: "text-base",
    lg: "text-lg",
    xl: "text-xl",
    compact: "text-xs",
    icon: "text-xs",
  };
  const sizeStyles: Record<ButtonSize, string> = {
    sm: "rounded-lg px-5 py-3",
    md: "rounded-lg px-4 py-2",
    base: "rounded-lg px-5 py-3",
    lg: "rounded-lg px-5 py-3",
    xl: "rounded-lg px-5 py-3",
    compact: "h-8 gap-1.5 rounded-md px-2.5 disabled:cursor-not-allowed",
    icon: "h-8 w-8 rounded-md disabled:cursor-not-allowed",
  };
  const variantStyles: Record<ButtonVariant, string> = {
    primary:
      "bg-gray-900 text-white shadow-md hover:bg-gray-700 hover:shadow-lg",
    secondary:
      "bg-gray-200 text-gray-800 shadow-md hover:bg-gray-300 hover:shadow-md",
    outline:
      "bg-transparent text-gray-700 ring-1 ring-gray-300 shadow-md hover:bg-gray-50 hover:ring-gray-400",
    outline_danger:
      "bg-moto-red-soft text-moto-red-strong ring-moto-red/30 shadow-md ring-1 hover:bg-moto-red/20 hover:ring-moto-red/50",
    danger:
      "bg-moto-red text-gray-950 shadow-md hover:bg-moto-red-strong hover:text-white hover:shadow-lg",
    success:
      "bg-moto-green text-gray-950 shadow-md hover:bg-moto-green-hover hover:shadow-lg active:scale-95",
    ghost: "bg-transparent text-gray-600 hover:bg-gray-100 hover:text-gray-900",
  };
  const baseStyles =
    "inline-flex items-center justify-center font-medium focus:outline-none disabled:opacity-50 transition-all duration-200";

  return `${baseStyles} ${sizeStyles[size]} ${variantStyles[variant]} ${textSizes[size]} ${className}`;
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
  return (
    <button
      type="submit"
      disabled={isLoading}
      className={buttonClassName({ variant, size, className })}
      {...props}
    >
      {isLoading ? (loadingText ?? "Loading...") : children}
    </button>
  );
}

/** A navigation link styled by the same shared button variants as actions. */
export function ButtonLink({
  variant = "primary",
  size = "base",
  className = "",
  ...props
}: Readonly<ButtonLinkProps>) {
  return (
    <Link
      className={buttonClassName({ variant, size, className })}
      {...props}
    />
  );
}

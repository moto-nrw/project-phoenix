import { type ReactNode } from "react";

export type BadgeVariant =
  | "gray"
  | "blue"
  | "green"
  | "purple"
  | "red"
  | "yellow"
  | "indigo";
export type BadgeSize = "sm" | "md";

export interface BadgeProps {
  variant?: BadgeVariant;
  size?: BadgeSize;
  children: ReactNode;
  className?: string;
  icon?: ReactNode;
}

const variantStyles: Record<BadgeVariant, string> = {
  gray: "bg-gray-100 text-gray-800",
  blue: "bg-blue-100 text-blue-800",
  green: "bg-green-100 text-green-800",
  purple: "bg-purple-100 text-purple-800",
  red: "bg-red-100 text-red-800",
  yellow: "bg-yellow-100 text-yellow-800",
  indigo: "bg-indigo-100 text-indigo-800",
};

const sizeStyles: Record<BadgeSize, string> = {
  sm: "px-1.5 md:px-2 py-0.5 text-xs",
  md: "px-2 md:px-2.5 py-1 text-sm",
};

export function Badge({
  variant = "gray",
  size = "sm",
  children,
  className = "",
  icon,
}: BadgeProps) {
  return (
    <span
      className={`inline-flex items-center rounded font-medium ${variantStyles[variant]} ${sizeStyles[size]} ${className}`}
    >
      {icon && <span className="mr-0.5 md:mr-1">{icon}</span>}
      {children}
    </span>
  );
}

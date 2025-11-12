import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

// Test comment for Husky hook
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}

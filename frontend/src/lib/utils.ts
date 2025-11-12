import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

// Final test - Hook should run now!
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}

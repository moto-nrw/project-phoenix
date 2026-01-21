/**
 * Mock SWR module for testing
 * This file is used when the actual swr package is not installed
 */
import { vi } from "vitest";

// Default SWR return value
const defaultSWRReturn = {
  data: undefined,
  error: undefined,
  isLoading: true,
  isValidating: false,
  mutate: vi.fn(),
};

// Mock useSWR hook
const useSWR = vi.fn(() => defaultSWRReturn);

// Mock useSWRConfig hook
export const useSWRConfig = vi.fn(() => ({
  mutate: vi.fn(),
  cache: new Map(),
}));

// Mock useSWRMutation hook
export const useSWRMutation = vi.fn(() => ({
  trigger: vi.fn(),
  data: undefined,
  error: undefined,
  isMutating: false,
  reset: vi.fn(),
}));

// Mock useSWRImmutable hook
export const useSWRImmutable = vi.fn(() => defaultSWRReturn);

// Mock unstable_serialize
export const unstable_serialize = vi.fn((key) => JSON.stringify(key));

// Export useSWR as default
export default useSWR;

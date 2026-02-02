import { vi } from "vitest";

/**
 * Returns a mock Next.js router object with all methods as vi.fn().
 * Overrides merge shallowly.
 */
export function mockRouter(
  overrides?: Partial<{
    push: ReturnType<typeof vi.fn>;
    replace: ReturnType<typeof vi.fn>;
    back: ReturnType<typeof vi.fn>;
    forward: ReturnType<typeof vi.fn>;
    refresh: ReturnType<typeof vi.fn>;
    prefetch: ReturnType<typeof vi.fn>;
  }>,
) {
  return {
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    forward: vi.fn(),
    refresh: vi.fn(),
    prefetch: vi.fn(),
    ...overrides,
  };
}

/**
 * Returns a mock URLSearchParams.
 * Pass a record of key-value pairs to pre-populate.
 */
export function mockSearchParams(params?: Record<string, string>) {
  return new URLSearchParams(params);
}

/**
 * Returns a vi.fn() that returns the given pathname.
 */
export function mockPathname(path = "/") {
  return vi.fn(() => path);
}

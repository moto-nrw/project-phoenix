import { describe, it, expect, vi } from "vitest";

// Mock auth-client before importing
vi.mock("~/lib/auth-client", () => ({
  authClient: {
    useSession: vi.fn(() => ({ data: null, isPending: true })),
  },
  useSession: vi.fn(() => ({ data: null, isPending: true })),
}));

// Mock swr
vi.mock("swr", () => ({
  default: vi.fn(),
  mutate: vi.fn(),
}));

// Import from index to test re-exports
import {
  useSWRAuth,
  useImmutableSWR,
  useSWRWithId,
  swrConfig,
  immutableConfig,
  mutate,
} from "./index";

describe("SWR index exports", () => {
  it("exports useSWRAuth hook", () => {
    expect(useSWRAuth).toBeDefined();
    expect(typeof useSWRAuth).toBe("function");
  });

  it("exports useImmutableSWR hook", () => {
    expect(useImmutableSWR).toBeDefined();
    expect(typeof useImmutableSWR).toBe("function");
  });

  it("exports useSWRWithId hook", () => {
    expect(useSWRWithId).toBeDefined();
    expect(typeof useSWRWithId).toBe("function");
  });

  it("exports swrConfig", () => {
    expect(swrConfig).toBeDefined();
    expect(typeof swrConfig).toBe("object");
  });

  it("exports immutableConfig", () => {
    expect(immutableConfig).toBeDefined();
    expect(typeof immutableConfig).toBe("object");
  });

  it("re-exports mutate from swr", () => {
    expect(mutate).toBeDefined();
    expect(typeof mutate).toBe("function");
  });
});

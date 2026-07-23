import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mockUseSWRAuth = vi.fn();

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (...args: unknown[]) => mockUseSWRAuth(...args),
}));

import { fetchSettingsSchema } from "~/lib/settings-api";
import { useSettingsSchema } from "./use-settings-schema";

describe("useSettingsSchema", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseSWRAuth.mockReturnValue({ data: undefined, isLoading: true });
  });

  it("uses the tenant-aware SWR wrapper for the shared schema key", () => {
    const options = { revalidateOnFocus: false };
    renderHook(() => useSettingsSchema(true, options));

    expect(mockUseSWRAuth).toHaveBeenCalledWith(
      "settings-schema",
      fetchSettingsSchema,
      options,
    );
  });

  it("disables the request with a null key", () => {
    renderHook(() => useSettingsSchema(false));

    expect(mockUseSWRAuth).toHaveBeenCalledWith(
      null,
      fetchSettingsSchema,
      undefined,
    );
  });
});

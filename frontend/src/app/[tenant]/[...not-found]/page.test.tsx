import { render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

const replace = vi.hoisted(() => vi.fn());

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ replace }),
}));

import UnknownTenantRoute from "./page";

describe("unknown tenant route", () => {
  it("returns to the tenant entry page", () => {
    render(<UnknownTenantRoute />);

    expect(replace).toHaveBeenCalledWith("/");
  });
});

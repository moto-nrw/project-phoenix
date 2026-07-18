import { render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  redirect: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  redirect: mocks.redirect,
  useSearchParams: () => new URLSearchParams("week=1&view=week"),
}));

vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (href: string) => `/test-tenant${href}`,
}));

import { PlanungRedirect } from "./planung-redirect";

describe("PlanungRedirect", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("preserves the tenant prefix when redirecting a legacy route", () => {
    render(<PlanungRedirect target="betreuungsplan" />);

    expect(mocks.redirect).toHaveBeenCalledWith(
      expect.stringMatching(
        /^\/test-tenant\/betreuungsplan\?d=\d{4}-\d{2}-\d{2}&view=woche$/,
      ),
    );
  });
});

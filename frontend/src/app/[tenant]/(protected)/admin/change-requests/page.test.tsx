import { render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  redirect: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  redirect: mocks.redirect,
}));

vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (href: string) => `/test-tenant${href}`,
}));

import AdminChangeRequestsRedirect from "./page";

// Die Freigabeansicht ist in das Anfragen-Modul umgezogen (#2429); die
// Alt-Route muss gespeicherte Links tenant-korrekt dorthin weiterleiten.
describe("AdminChangeRequestsRedirect", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("redirects the legacy route to /anfragen with the tenant prefix", () => {
    render(<AdminChangeRequestsRedirect />);

    expect(mocks.redirect).toHaveBeenCalledWith("/test-tenant/anfragen");
  });
});

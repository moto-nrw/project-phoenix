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

import AdminEnrollmentChangeRequestsRedirect from "./page";

// Anmeldungsänderungen leben seit #2435 im Anfragen-Modul; der zweite
// gleichnamige Sidebar-Eintrag ist weg, gespeicherte Links müssen trotzdem
// tenant-korrekt dort ankommen.
describe("AdminEnrollmentChangeRequestsRedirect", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("leitet die Alt-Route mit Tenant-Präfix auf /anfragen", () => {
    render(<AdminEnrollmentChangeRequestsRedirect />);

    expect(mocks.redirect).toHaveBeenCalledWith("/test-tenant/anfragen");
  });
});

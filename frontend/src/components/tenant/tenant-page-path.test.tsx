import { render, screen } from "@testing-library/react";
import type { ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";

import { TenantPage } from "~/components/ui/tenant-page";
import { TenantPagePathProvider } from "./tenant-page-path";

// The test setup resolves app paths for the test school; this test checks
// that the tenant area's provider does it.
vi.unmock("~/components/ui/page-path");

vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: ComponentProps<"a">) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

describe("TenantPagePathProvider", () => {
  it("führt den Rückweg des Gerüsts in die Schule", () => {
    render(
      <TenantPagePathProvider>
        <TenantPage title="Raum 1" back backHref="/rooms" />
      </TenantPagePathProvider>,
    );

    expect(screen.getByRole("link", { name: "Zurück" })).toHaveAttribute(
      "href",
      "/test-tenant/rooms",
    );
  });
});

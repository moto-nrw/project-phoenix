import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { HELP_TOPICS } from "~/lib/help-topics";

vi.mock("next/navigation", () => ({
  usePathname: () => "/testschule/students/search",
  useSearchParams: () => new URLSearchParams("status=anwesend"),
}));

vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: () => ({
    user: { roles: ["admin"] },
  }),
}));

vi.mock("~/lib/tenant-context", () => ({
  useNFCEnabled: () => true,
  useOpenCareGroupMode: () => true,
  usePresenceMode: () => "binary",
  useTenantRoutingModeSafe: () => "path",
  useTenantSlugSafe: () => "testschule",
}));

import { buildContextHelpHref, ContextHelpLink } from "./context-help-link";

describe("ContextHelpLink", () => {
  it("adds the page context and the exact return path to the help URL", () => {
    render(<ContextHelpLink topic={HELP_TOPICS.studentSearch} />);

    const link = screen.getByRole("link", { name: "Hilfe zu dieser Seite" });
    const href = link.getAttribute("href");
    expect(href).not.toBeNull();
    if (!href) throw new Error("expected contextual help link to have href");

    const target = new URL(href, "https://moto.invalid");
    expect(target.pathname).toBe("/help/prototype/kindersuche");
    expect(Object.fromEntries(target.searchParams)).toEqual({
      role: "lead",
      nfc_enabled: "true",
      presence_mode: "binary",
      group_mode: "open_care",
      return_to: "/testschule/students/search?status=anwesend",
    });
    expect(link).not.toHaveAttribute("target");
    expect(screen.getByRole("tooltip")).toHaveTextContent(
      "Hilfe zu dieser Seite",
    );
  });

  it("encodes the topic and all supported variants", () => {
    expect(
      buildContextHelpHref({
        topic: HELP_TOPICS.studentSearch,
        role: "caregiver",
        nfcEnabled: false,
        presenceMode: "detailed",
        groupMode: "fixed_groups",
        returnTo: "/students/search",
      }),
    ).toBe(
      "/help/prototype/kindersuche?role=caregiver&nfc_enabled=false&presence_mode=detailed&group_mode=fixed_groups&return_to=%2Fstudents%2Fsearch",
    );
  });
});

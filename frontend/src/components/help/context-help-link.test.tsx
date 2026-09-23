import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { HELP_TOPICS } from "~/lib/help-topics";

vi.mock("next/navigation", () => ({
  usePathname: () => "/testschule/students/search",
  useSearchParams: () => new URLSearchParams("status=anwesend"),
}));

// Rolle und Portal wechseln je Test: `vi.mock` wird hochgezogen, deshalb
// liegen die veraenderlichen Werte in `vi.hoisted`.
const auth = vi.hoisted(() => ({
  roles: ["admin"] as string[],
  mode: "teacher" as string,
}));

vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: () => ({
    user: { roles: auth.roles },
    mode: auth.mode,
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
    expect(target.pathname).toBe("/help/kindersuche");
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

  it("stays away when the page has no article for this role", () => {
    // `/settings` und `/tagesinformationen` duerfen Betreuungskraefte oeffnen,
    // ihre Artikel sind aber fuer die Leitung geschrieben. Ein Fragezeichen
    // wuerde in eine fremde Seitenleiste fuehren.
    auth.roles = ["user"];
    try {
      const { container } = render(
        <ContextHelpLink topic={HELP_TOPICS.settings} />,
      );
      expect(container).toBeEmptyDOMElement();
    } finally {
      auth.roles = ["admin"];
    }
  });

  it("still offers the lead article to a lead", () => {
    render(<ContextHelpLink topic={HELP_TOPICS.settings} />);

    expect(
      screen.getByRole("link", { name: "Hilfe zu dieser Seite" }),
    ).toHaveAttribute(
      "href",
      expect.stringContaining("/help/einstellungen-ueberblick"),
    );
  });

  // Im Elternportal hat niemand die Rolle `admin`. Ohne den Portal-Modus
  // fiele der Knopf auf `caregiver` zurueck und schickte Eltern in die
  // Betreuungs-Hilfe.
  it("opens the parent article from the parents portal", () => {
    auth.mode = "parent";
    auth.roles = ["guardian"];
    try {
      render(<ContextHelpLink topic={HELP_TOPICS.parentChildOverview} />);

      const link = screen.getByRole("link", {
        name: "Hilfe zu dieser Seite",
      });
      const href = link.getAttribute("href") ?? "";
      const target = new URL(href, "https://moto.invalid");
      expect(target.pathname).toBe(
        "/help/mein-kind-und-den-heutigen-tag-ansehen",
      );
      expect(target.searchParams.get("role")).toBe("parent");
    } finally {
      auth.mode = "teacher";
      auth.roles = ["admin"];
    }
  });

  it("opens a teacher article from moto schule", () => {
    auth.mode = "school";
    auth.roles = ["lehrkraft"];
    try {
      render(<ContextHelpLink topic={HELP_TOPICS.teacherClassDay} />);

      const link = screen.getByRole("link", {
        name: "Hilfe zu dieser Seite",
      });
      const href = link.getAttribute("href") ?? "";
      const target = new URL(href, "https://moto.invalid");
      expect(target.pathname).toBe("/help/meinen-klassentag-ansehen");
      expect(target.searchParams.get("role")).toBe("teacher");
    } finally {
      auth.mode = "teacher";
      auth.roles = ["admin"];
    }
  });

  // Ein Betreuungsartikel hat fuer Eltern keine Fassung. Lieber kein
  // Fragezeichen als eines in die falsche Seitenleiste.
  it("stays away from a caregiver article in the parents portal", () => {
    auth.mode = "parent";
    auth.roles = ["guardian"];
    try {
      const { container } = render(
        <ContextHelpLink topic={HELP_TOPICS.studentSearch} />,
      );
      expect(container).toBeEmptyDOMElement();
    } finally {
      auth.mode = "teacher";
      auth.roles = ["admin"];
    }
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
      "/help/kindersuche?role=caregiver&nfc_enabled=false&presence_mode=detailed&group_mode=fixed_groups&return_to=%2Fstudents%2Fsearch",
    );
  });
});

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ModulePortalProvider } from "~/lib/module-portal-context";
import { ModuleNotBookedAlert } from "./module-not-booked-state";

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: null, status: "unauthenticated" }),
}));

describe("ModuleNotBookedAlert", () => {
  it("explains a refused page to the OGS team", () => {
    render(<ModuleNotBookedAlert module="zeiterfassung" />);
    expect(
      screen.getByText(/Diese Seite gehört zum Modul Zeiterfassung./),
    ).toBeInTheDocument();
  });

  // #4895: parents cannot book a module and must not be sent to the moto
  // team or "their Leitung".
  it("names the school in the parents portal", () => {
    render(
      <ModulePortalProvider portal="parent">
        <ModuleNotBookedAlert module="eltern_app" />
      </ModulePortalProvider>,
    );
    expect(
      screen.getByText(/Ihre Schule nutzt das Elternportal zurzeit nicht./),
    ).toBeInTheDocument();
  });

  it("names the OGS in the school portal", () => {
    render(
      <ModulePortalProvider portal="school">
        <ModuleNotBookedAlert module="unterricht_vormittag" />
      </ModulePortalProvider>,
    );
    expect(
      screen.getByText(/Ihre Schule nutzt moto schule zurzeit nicht./),
    ).toBeInTheDocument();
  });
});

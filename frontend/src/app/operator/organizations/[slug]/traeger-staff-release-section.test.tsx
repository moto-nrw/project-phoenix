import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";

const { service } = vi.hoisted(() => ({
  service: { getState: vi.fn(), grant: vi.fn(), withdraw: vi.fn() },
}));

vi.mock("~/lib/operator/traeger-staff-release-api", () => ({
  traegerStaffReleaseService: service,
}));

import { TraegerStaffReleaseSection } from "./traeger-staff-release-section";

function renderSection() {
  return render(
    <ToastProvider>
      <TraegerStaffReleaseSection organizationId="7" />
    </ToastProvider>,
  );
}

describe("TraegerStaffReleaseSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("records the statement the Träger confirmed with the release", async () => {
    service.getState
      .mockResolvedValueOnce({ release: null, records: 0 })
      .mockResolvedValue({
        release: { representation: "none", releasedAt: "2026-11-14T10:00:00Z" },
        records: 0,
      });
    service.grant.mockResolvedValue(undefined);
    renderSection();

    expect(await screen.findByText("Nicht freigeschaltet")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Freischalten" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Personal beim Träger freischalten",
    });
    expect(
      within(dialog).getByText(
        /§ 87 Abs. 1 Nr. 2 und Nr. 6 BetrVG, MAVO, MVG.EKD/,
      ),
    ).toBeInTheDocument();

    fireEvent.click(
      within(dialog).getByRole("button", { name: "Freischalten" }),
    );
    expect(
      await within(dialog).findByText(
        "Bitte wählen Sie die Erklärung des Trägers.",
      ),
    ).toBeInTheDocument();
    expect(service.grant).not.toHaveBeenCalled();

    fireEvent.click(
      within(dialog).getByRole("radio", { name: /Keine Vertretung/ }),
    );
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Freischalten" }),
    );
    await vi.waitFor(() =>
      expect(service.grant).toHaveBeenCalledWith("7", "none"),
    );
    expect(await screen.findByText("Freigeschaltet")).toBeInTheDocument();
    expect(screen.getByText("Keine Vertretung")).toBeInTheDocument();
  });

  it("keeps the withdrawal closed while the Träger keeps records", async () => {
    service.getState.mockResolvedValue({
      release: {
        representation: "agreement",
        releasedAt: "2026-11-14T10:00:00Z",
      },
      records: 3,
    });
    renderSection();
    const withdraw = await screen.findByRole("button", {
      name: /Freigabe zurücknehmen/,
    });
    expect(withdraw).toHaveAttribute("aria-disabled", "true");
    expect(
      screen.getByText("Vereinbarung mit der Vertretung liegt vor"),
    ).toBeInTheDocument();
  });
});

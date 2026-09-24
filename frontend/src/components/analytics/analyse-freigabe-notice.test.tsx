import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AnalyseFreigabeNotice } from "./analyse-freigabe-notice";

const mocks = vi.hoisted(() => ({ useTenant: vi.fn() }));
vi.mock("~/lib/tenant-context", () => ({ useTenant: mocks.useTenant }));

describe("AnalyseFreigabeNotice", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("tells staff about the recording while the Freigabe is on", () => {
    mocks.useTenant.mockReturnValue({
      tenant: { tenantId: 1, analyticsFreigabe: true },
    });

    render(<AnalyseFreigabeNotice />);

    expect(screen.getByText("Nutzungsanalyse")).toBeInTheDocument();
    expect(screen.getByText(/Ihre Schule hat zugestimmt/)).toBeInTheDocument();
    // Read-only: the moto team switches it, so the card offers no control.
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it.each([
    [{ tenantId: 1, analyticsFreigabe: false }],
    [{ tenantId: 1 }],
    [null],
  ])("stays hidden without the Freigabe (%o)", (tenant) => {
    mocks.useTenant.mockReturnValue({ tenant });

    const { container } = render(<AnalyseFreigabeNotice />);

    expect(container).toBeEmptyDOMElement();
  });
});

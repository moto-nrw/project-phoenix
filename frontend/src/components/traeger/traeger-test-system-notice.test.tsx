import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockFetchOrganization } = vi.hoisted(() => ({
  mockFetchOrganization: vi.fn(),
}));

vi.mock("~/lib/traeger-api", () => ({
  fetchTraegerOrganization: mockFetchOrganization,
}));

import { TraegerTestSystemNotice } from "./traeger-test-system-notice";

describe("TraegerTestSystemNotice", () => {
  beforeEach(() => {
    mockFetchOrganization.mockReset();
  });

  it("marks a test Träger as a test system with invented children", async () => {
    mockFetchOrganization.mockResolvedValue({
      id: "4",
      name: "Test-Träger",
      testSystem: true,
    });

    render(<TraegerTestSystemNotice />);

    expect(await screen.findByText("Testsystem")).toBeInTheDocument();
    expect(screen.getByText(/Alle Kinder sind erfunden/)).toBeInTheDocument();
  });

  it("shows nothing for a real Träger", async () => {
    mockFetchOrganization.mockResolvedValue({
      id: "5",
      name: "Echter Träger",
      testSystem: false,
    });

    const { container } = render(<TraegerTestSystemNotice />);

    await waitFor(() => expect(mockFetchOrganization).toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
  });
});

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ParentApiError, getRequestSharingOptions } from "~/lib/parent-api";
import { catalogText } from "~/test/error-catalog-text";
import { RequestSharingSelector } from "./request-sharing-control";
import { SharingOptionsProvider } from "./sharing-options-context";

vi.mock("~/lib/parent-api", async (importOriginal) => ({
  ParentApiError: (await importOriginal<typeof import("~/lib/parent-api")>())
    .ParentApiError,
  getRequestSharingOptions: vi.fn(),
  getRequestSharing: vi.fn(),
  setRequestSharing: vi.fn(),
}));

const getOptions = vi.mocked(getRequestSharingOptions);

describe("SharingOptionsProvider", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getOptions.mockResolvedValue({
      family_protected: false,
      recipients: [
        {
          guardian_profile_id: "7",
          first_name: "Mara",
          last_name: "Muster",
          selected: false,
        },
      ],
    });
  });

  // Zwei Auswahlfelder auf einer Seite dürfen die Empfängerliste nur einmal
  // holen (#2267).
  it("loads the recipients once for two selectors on the same child", async () => {
    render(
      <SharingOptionsProvider>
        <RequestSharingSelector
          studentId="42"
          selected={[]}
          onChange={vi.fn()}
        />
        <RequestSharingSelector
          studentId="42"
          selected={[]}
          onChange={vi.fn()}
        />
      </SharingOptionsProvider>,
    );

    await waitFor(() =>
      expect(
        screen.getAllByRole("checkbox", { name: "Mara Muster" }),
      ).toHaveLength(2),
    );
    expect(getOptions).toHaveBeenCalledTimes(1);
  });

  it("loads once per child", async () => {
    render(
      <SharingOptionsProvider>
        <RequestSharingSelector
          studentId="42"
          selected={[]}
          onChange={vi.fn()}
        />
        <RequestSharingSelector
          studentId="43"
          selected={[]}
          onChange={vi.fn()}
        />
      </SharingOptionsProvider>,
    );

    await waitFor(() => expect(getOptions).toHaveBeenCalledTimes(2));
    expect(getOptions).toHaveBeenCalledWith("42");
    expect(getOptions).toHaveBeenCalledWith("43");
  });

  // Ohne Provider bleibt das Auswahlfeld allein lauffähig.
  it("falls back to a direct fetch without a provider", async () => {
    render(
      <RequestSharingSelector
        studentId="42"
        selected={[]}
        onChange={vi.fn()}
      />,
    );

    expect(
      await screen.findByRole("checkbox", { name: "Mara Muster" }),
    ).toBeInTheDocument();
    expect(getOptions).toHaveBeenCalledTimes(1);
  });

  // #2518: a failed load shows the shared message in place; Wiederholen
  // fetches again because the provider dropped the failed request.
  it("retries a failed load through the provider", async () => {
    getOptions.mockRejectedValueOnce(
      new ParentApiError("diag", 503, "general.unavailable"),
    );
    render(
      <SharingOptionsProvider>
        <RequestSharingSelector
          studentId="42"
          selected={[]}
          onChange={vi.fn()}
        />
      </SharingOptionsProvider>,
    );

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Empfänger"),
        { exact: false },
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(
      await screen.findByRole("checkbox", { name: "Mara Muster" }),
    ).toBeInTheDocument();
    expect(getOptions).toHaveBeenCalledTimes(2);
  });
});

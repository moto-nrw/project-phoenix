import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ModalProvider } from "~/components/dashboard/modal-context";
import {
  getRequestSharing,
  getRequestSharingOptions,
  setRequestSharing,
} from "~/lib/parent-api";
import {
  RequestSharingControl,
  RequestSharingSelector,
} from "./request-sharing-control";

vi.mock("~/lib/parent-api", () => ({
  getRequestSharing: vi.fn(),
  getRequestSharingOptions: vi.fn(),
  setRequestSharing: vi.fn(),
}));

const getSharing = vi.mocked(getRequestSharing);
const setSharing = vi.mocked(setRequestSharing);
const getOptions = vi.mocked(getRequestSharingOptions);

function renderControl(isSelf = true) {
  return render(
    <ModalProvider>
      <RequestSharingControl
        studentId="42"
        requestType="excused"
        requestId="9"
        isSelf={isSelf}
      />
    </ModalProvider>,
  );
}

describe("RequestSharingControl", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getSharing.mockResolvedValue({
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
    setSharing.mockResolvedValue({
      family_protected: false,
      recipients: [],
    });
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

  it("is only available to the guardian who submitted the request", () => {
    renderControl(false);
    expect(screen.queryByText("Sichtbarkeit ändern")).not.toBeInTheDocument();
  });

  it("saves only the explicitly selected guardian", async () => {
    renderControl();
    fireEvent.click(await screen.findByText("Sichtbarkeit ändern"));
    const checkbox = await screen.findByRole("checkbox", {
      name: "Mara Muster",
    });
    fireEvent.click(checkbox);
    fireEvent.click(screen.getByText("Freigabe speichern"));

    await waitFor(() =>
      expect(setSharing).toHaveBeenCalledWith("42", "excused", "9", ["7"]),
    );
  });

  it("offers no recipient choices while family protection is active", async () => {
    getSharing.mockResolvedValue({
      family_protected: true,
      recipients: [],
    });
    renderControl();
    expect(await screen.findByText(/Familienschutz aktiv/)).toBeInTheDocument();
    expect(screen.queryByText("Sichtbarkeit ändern")).not.toBeInTheDocument();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
  });

  it("shows named recipients before a request is submitted", async () => {
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
  });
});

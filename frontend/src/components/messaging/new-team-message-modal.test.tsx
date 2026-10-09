import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type { StaffMessagesApi } from "~/lib/staff-messages-api";
import { catalogText } from "~/test/error-catalog-text";

import { NewTeamMessageModal } from "./new-team-message-modal";

vi.mock("~/components/ui/modal", () => ({
  Modal: ({ isOpen, children }: { isOpen: boolean; children: ReactNode }) =>
    isOpen ? <div>{children}</div> : null,
}));

function stubApi() {
  return {
    fetchInbox: vi.fn<StaffMessagesApi["fetchInbox"]>(),
    fetchUnreadCount: vi.fn<StaffMessagesApi["fetchUnreadCount"]>(),
    fetchRecipients: vi.fn<StaffMessagesApi["fetchRecipients"]>(),
    fetchThread: vi.fn<StaffMessagesApi["fetchThread"]>(),
    openThread: vi.fn<StaffMessagesApi["openThread"]>(),
    postMessage: vi.fn<StaffMessagesApi["postMessage"]>(),
  };
}

const anna = {
  account_id: "7",
  name: "Anna Beispiel",
  role_kind: "staff" as const,
};

describe("NewTeamMessageModal", () => {
  let api: ReturnType<typeof stubApi>;

  beforeEach(() => {
    api = stubApi();
  });

  function renderModal(onOpened = vi.fn()) {
    render(
      <NewTeamMessageModal
        api={api}
        portal="tenant"
        hint="Sie erreichen hier Ihr Team."
        onClose={vi.fn()}
        onOpened={onOpened}
      />,
    );
    return { onOpened };
  }

  // #2517: Katalogtext statt Serversatz, nie „Es gibt niemanden …“ für eine
  // Liste, die nicht geladen wurde.
  it("shows a failed list load in the dialog and reloads on retry", async () => {
    api.fetchRecipients
      .mockRejectedValueOnce(
        new ApiError("recipients kaputt", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce([anna]);
    renderModal();

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Personen"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/kaputt/)).not.toBeInTheDocument();
    expect(
      screen.queryByText("Es gibt niemanden, dem Sie schreiben können."),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(await screen.findByText("Anna Beispiel")).toBeInTheDocument();
    expect(api.fetchRecipients).toHaveBeenCalledTimes(2);
  });

  it("keeps a failed open in the dialog and retries the same person", async () => {
    api.fetchRecipients.mockResolvedValue([anna]);
    api.openThread
      .mockRejectedValueOnce(
        new ApiError("open kaputt", 500, { code: "general.server" }),
      )
      .mockResolvedValueOnce({
        thread_id: "t9",
        counterpart_account_id: "7",
        counterpart_name: "Anna Beispiel",
        counterpart_role_kind: "staff",
        messages: [],
      });
    const { onOpened } = renderModal();

    fireEvent.click(await screen.findByText("Anna Beispiel"));

    expect(
      await screen.findByText(
        catalogText("general.server", "die Unterhaltung"),
      ),
    ).toBeInTheDocument();
    expect(onOpened).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    await waitFor(() => expect(onOpened).toHaveBeenCalledWith("t9"));
    expect(api.openThread).toHaveBeenLastCalledWith("7");
  });
});

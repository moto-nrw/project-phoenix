import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { RequestFeedDialog } from "./request-feed-dialog";
import {
  createRequestFeed,
  getRequestFeedStatus,
  rotateRequestFeed,
} from "~/lib/request-feed-api";

const { toastSuccess, toastError, copy } = vi.hoisted(() => ({
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
  copy: vi.fn(),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: toastSuccess, error: toastError }),
}));

vi.mock("~/lib/use-clipboard-copy", () => ({
  useClipboardCopy: () => ({ copied: false, copy }),
}));

vi.mock("~/lib/request-feed-api", () => ({
  getRequestFeedStatus: vi.fn(),
  createRequestFeed: vi.fn(),
  rotateRequestFeed: vi.fn(),
}));

vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    title,
    children,
    footer,
  }: {
    isOpen: boolean;
    title: string;
    children: React.ReactNode;
    footer: React.ReactNode;
  }) =>
    isOpen ? (
      <div role="dialog" aria-label={title}>
        {children}
        {footer}
      </div>
    ) : null,
}));

const status = vi.mocked(getRequestFeedStatus);
const create = vi.mocked(createRequestFeed);
const rotate = vi.mocked(rotateRequestFeed);

describe("RequestFeedDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    copy.mockResolvedValue(true);
  });

  it("zeigt einen neu erstellten Link genau in diesem Dialog", async () => {
    status.mockResolvedValue({ active: false });
    create.mockResolvedValue({
      url: "https://schule.test/api/request-feed/secret",
    });

    render(<RequestFeedDialog isOpen onClose={vi.fn()} />);

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "RSS-Link erstellen" }),
      ).toBeEnabled(),
    );
    fireEvent.click(screen.getByRole("button", { name: "RSS-Link erstellen" }));

    const input = await screen.findByLabelText("Ihr RSS-Link");
    expect(input).toHaveValue("https://schule.test/api/request-feed/secret");
    expect(
      screen.getByText(/persönliche Daten stehen nicht im Feed/),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Kopieren" }));
    await waitFor(() =>
      expect(copy).toHaveBeenCalledWith(
        "https://schule.test/api/request-feed/secret",
      ),
    );
  });

  it("ersetzt einen bestehenden Link erst nach einer Bestätigung", async () => {
    status.mockResolvedValue({ active: true });
    rotate.mockResolvedValue({
      url: "https://schule.test/api/request-feed/new",
    });

    render(<RequestFeedDialog isOpen onClose={vi.fn()} />);

    expect(
      await screen.findByText("Der RSS-Link ist bereits eingerichtet."),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Neuen Link erstellen" }),
    );
    expect(
      screen.getByText(/bisherige Link funktioniert danach nicht mehr/),
    ).toBeInTheDocument();
    expect(rotate).not.toHaveBeenCalled();

    fireEvent.click(
      screen.getByRole("button", { name: "Alten Link ersetzen" }),
    );
    await waitFor(() => expect(rotate).toHaveBeenCalledOnce());
    expect(await screen.findByLabelText("Ihr RSS-Link")).toHaveValue(
      "https://schule.test/api/request-feed/new",
    );
  });
});

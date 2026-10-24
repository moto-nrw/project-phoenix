import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { AttachmentList } from "./attachment-list";

const mocks = vi.hoisted(() => ({
  fetch: vi.fn(),
  download: vi.fn(),
  show: vi.fn(),
  showMessage: vi.fn(),
}));
vi.mock("~/lib/api-error", async (original) => ({
  ...(await original<object>()),
  transportFetch: mocks.fetch,
}));
vi.mock("~/lib/file-download", () => ({ downloadBlob: mocks.download }));
vi.mock("~/contexts/ToastContext", () => ({
  useApiErrorDisplay: () => ({
    show: mocks.show,
    showMessage: mocks.showMessage,
  }),
}));

const attachments = [
  {
    label: "Informationen zum Schulstart",
    document_url: "/api/public/enrollment-form-legal-documents/42_start.pdf",
  },
  {
    label: "Abholen und Kontakt",
    document_url: "/uploads/enrollment-form-legal-documents/42_pickup.pdf",
  },
];

beforeEach(() => {
  vi.clearAllMocks();
});
afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

function pdf(contents: string) {
  return new Response(new Blob([contents], { type: "application/pdf" }), {
    headers: { "content-type": "application/pdf" },
  });
}

describe("anonymous enrollment PDF actions", () => {
  it("downloads both distinct PDFs under their readable names through the public route", async () => {
    mocks.fetch
      .mockResolvedValueOnce(pdf("%PDF school start"))
      .mockResolvedValueOnce(pdf("%PDF pickup"));
    render(<AttachmentList attachments={attachments} />);
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", {
        name: "Informationen zum Schulstart herunterladen",
      }),
    );
    await waitFor(() => expect(mocks.download).toHaveBeenCalledTimes(1));
    fireEvent.click(
      screen.getByRole("button", { name: "Abholen und Kontakt herunterladen" }),
    );
    await waitFor(() => expect(mocks.download).toHaveBeenCalledTimes(2));
    expect(mocks.fetch.mock.calls).toEqual([
      [attachments[0]!.document_url, { cache: "no-store" }],
      [
        "/api/public/enrollment-form-legal-documents/42_pickup.pdf",
        { cache: "no-store" },
      ],
    ]);
    expect(mocks.download.mock.calls[0]![1]).toBe(
      "Informationen zum Schulstart.pdf",
    );
    expect(mocks.download.mock.calls[1]![1]).toBe("Abholen und Kontakt.pdf");
    expect(await (mocks.download.mock.calls[0]![0] as Blob).text()).toBe(
      "%PDF school start",
    );
    expect(await (mocks.download.mock.calls[1]![0] as Blob).text()).toBe(
      "%PDF pickup",
    );
  });

  it("opens both PDFs in tabs created synchronously before the fetch", async () => {
    const tabs = attachments.map(() => ({
      opener: {},
      document: { title: "" },
      location: { href: "about:blank" },
      close: vi.fn(),
    }));
    const open = vi
      .spyOn(window, "open")
      .mockImplementationOnce(() => tabs[0] as unknown as Window)
      .mockImplementationOnce(() => tabs[1] as unknown as Window);
    vi.stubGlobal(
      "URL",
      class extends URL {
        static createObjectURL = vi
          .fn()
          .mockReturnValueOnce("blob:start")
          .mockReturnValueOnce("blob:pickup");
        static revokeObjectURL = vi.fn();
      },
    );
    mocks.fetch.mockImplementation(() => {
      expect(open).toHaveBeenCalled();
      return Promise.resolve(pdf("%PDF"));
    });
    render(<AttachmentList attachments={attachments} />);
    fireEvent.click(
      screen.getByRole("button", {
        name: "Informationen zum Schulstart öffnen",
      }),
    );
    await waitFor(() => expect(tabs[0]!.location.href).toBe("blob:start"));
    fireEvent.click(
      screen.getByRole("button", { name: "Abholen und Kontakt öffnen" }),
    );
    await waitFor(() => expect(tabs[1]!.location.href).toBe("blob:pickup"));
    expect(tabs.map((tab) => tab.document.title)).toEqual(
      attachments.map((a) => a.label),
    );
    expect(tabs.map((tab) => tab.opener)).toEqual([null, null]);
    expect(mocks.download).not.toHaveBeenCalled();
  });

  it.each(["network", "missing", "invalid content"])(
    "reports %s failures, closes the empty tab and permits retry",
    async (failure) => {
      const tab = { opener: {}, document: { title: "" }, close: vi.fn() };
      vi.spyOn(window, "open").mockReturnValue(tab as unknown as Window);
      if (failure === "network")
        mocks.fetch.mockRejectedValueOnce(new TypeError("offline"));
      else
        mocks.fetch.mockResolvedValueOnce(
          failure === "missing"
            ? new Response("gone", { status: 404 })
            : new Response("HTML", {
                headers: { "content-type": "text/html" },
              }),
        );
      mocks.fetch.mockResolvedValueOnce(pdf("%PDF retry"));
      render(<AttachmentList attachments={attachments} />);
      fireEvent.click(
        screen.getByRole("button", {
          name: "Informationen zum Schulstart öffnen",
        }),
      );
      await waitFor(() => expect(mocks.show).toHaveBeenCalledTimes(1));
      expect(tab.close).toHaveBeenCalledOnce();
      expect(mocks.download).not.toHaveBeenCalled();
      const retry = screen.getByRole("button", {
        name: "Informationen zum Schulstart herunterladen",
      });
      await waitFor(() => expect(retry).toBeEnabled());
      fireEvent.click(retry);
      await waitFor(() => expect(mocks.download).toHaveBeenCalledOnce());
    },
  );

  it("reports blocked popups without starting a download", async () => {
    vi.spyOn(window, "open").mockReturnValue(null);
    render(<AttachmentList attachments={attachments} />);
    fireEvent.click(
      screen.getByRole("button", { name: "Abholen und Kontakt öffnen" }),
    );
    expect(mocks.showMessage).toHaveBeenCalledOnce();
    expect(mocks.fetch).not.toHaveBeenCalled();
  });

  it("rejects an external document URL instead of fetching it", async () => {
    render(
      <AttachmentList
        attachments={[
          { label: "External", document_url: "https://example.com/file.pdf" },
        ]}
      />,
    );
    fireEvent.click(
      screen.getByRole("button", { name: "External herunterladen" }),
    );
    await waitFor(() => expect(mocks.show).toHaveBeenCalledOnce());
    expect(mocks.fetch).not.toHaveBeenCalled();
  });
});

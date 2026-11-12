import { render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { InvoiceExportHistoryCard } from "./invoice-export-history-card";

afterEach(() => vi.unstubAllGlobals());

function step(accountId: number | null, byYou: boolean, accountName: string) {
  return {
    kind: "mail_requested",
    at: "2026-09-09T10:30:00Z",
    account_id: accountId,
    account_name: accountName,
    by_you: byYou,
    note: "invoice",
  };
}

// #4693: every step names its account by name, never by e-mail address.
it("names who did each step of the invoice", async () => {
  const fetchMock = vi.fn<typeof fetch>();
  vi.stubGlobal("fetch", fetchMock);
  fetchMock.mockResolvedValue(
    new Response(
      JSON.stringify({
        data: {
          number: "FE-0002",
          entries: [
            step(1, true, "Ich Selbst"),
            step(2, false, "Eva Müller"),
            step(3, false, ""),
            step(null, false, ""),
          ],
        },
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ),
  );

  render(
    <ToastProvider>
      <InvoiceExportHistoryCard kind="holiday-documents" invoiceId="5" />
    </ToastProvider>,
  );

  expect(await screen.findByText(/· von Eva Müller$/)).toBeInTheDocument();
  expect(screen.getByText(/· von Ihnen$/)).toBeInTheDocument();
  expect(screen.getByText(/· von einem Konto ohne Namen$/)).toBeInTheDocument();
  expect(screen.getByText(/· von einem gelöschten Konto$/)).toBeInTheDocument();
  expect(fetchMock).toHaveBeenCalledWith(
    "/api/traeger/finance-exports/invoice-history/holiday-documents/5",
    expect.anything(),
  );
});

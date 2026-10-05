import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import StudentChangeHistoryPage from "./page";

vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "1" }),
  useSearchParams: () => ({
    get: (key: string) => (key === "from" ? "/students/search" : null),
  }),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useStudentHistoryBreadcrumb: vi.fn(),
}));

vi.mock("~/components/ui/back-button", () => ({
  BackButton: ({ referrer }: { referrer: string }) => (
    <a data-testid="back-button" href={referrer}>
      Zurück
    </a>
  ),
}));

const student = {
  id: "1",
  first_name: "Emma",
  second_name: "Müller",
  name: "Emma Müller",
  school_class: "3b",
};

function stubFetch(history: () => Response) {
  const fetchMock = vi.fn(async (url: string) =>
    url.endsWith("/change-history")
      ? history()
      : Response.json({ data: student }),
  );
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

describe("StudentChangeHistoryPage", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("shows a failed load in place with retry and request ID", async () => {
    let calls = 0;
    const fetchMock = stubFetch(() => {
      calls += 1;
      return calls === 1
        ? Response.json(
            { status: "error", code: "general.server", instance: "req-ch" },
            { status: 500 },
          )
        : Response.json({ data: [] });
    });

    render(<StudentChangeHistoryPage />);

    expect(
      await screen.findByText(
        "Die Liste der Änderungen konnte nicht bearbeitet werden. Bitte versuchen Sie es später erneut.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Vorgangskennung kopieren" }),
    ).toHaveTextContent("req-ch");

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(
      await screen.findByText(/Noch keine Änderungen erfasst\./),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(
        fetchMock.mock.calls.filter(([url]) => url.endsWith("/change-history")),
      ).toHaveLength(2),
    );
  });

  it("names the permission class for a 403", async () => {
    stubFetch(() =>
      Response.json(
        { status: "error", code: "general.permission" },
        { status: 403 },
      ),
    );

    render(<StudentChangeHistoryPage />);

    expect(
      await screen.findByText(
        "Für die Liste der Änderungen fehlt Ihnen die Berechtigung. Bitte fragen Sie die Schule.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByTestId("back-button")).toHaveAttribute(
      "href",
      "/students/search",
    );
  });

  it("says the child was not found for a 404", async () => {
    stubFetch(() => new Response(null, { status: 404 }));

    render(<StudentChangeHistoryPage />);

    expect(await screen.findByText("Kind nicht gefunden.")).toBeInTheDocument();
  });
});

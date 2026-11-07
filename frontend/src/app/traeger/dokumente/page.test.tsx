import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

const { retrySchools } = vi.hoisted(() => ({ retrySchools: vi.fn() }));

vi.mock("next/navigation", async (importOriginal) => ({
  ...(await importOriginal<typeof import("next/navigation")>()),
  useRouter: () => ({ push: vi.fn() }),
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { scope: "org", permissions: ["traeger_overview:read"] } },
  }),
}));
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));

vi.mock("~/components/traeger/use-traeger-documents", () => ({
  useTraegerDocuments: () => ({
    data: [
      {
        id: "4",
        title: "Hausordnung",
        fileName: "hausordnung.pdf",
        contentType: "application/pdf",
        sizeBytes: 2048,
        createdAt: "2026-09-02T10:00:00Z",
        reads: 0,
        releases: [
          {
            id: "1",
            audience: "schools",
            schoolId: "7",
            groupId: null,
            groupName: "",
            releasedAt: "2026-09-02T10:00:00Z",
          },
        ],
      },
    ],
    isLoading: false,
    loadError: null,
    mutate: vi.fn(),
  }),
}));

vi.mock("~/components/traeger/use-traeger-schools", async () => {
  const { useSwrLoadError } = await import("~/lib/hooks/use-swr-load-error");
  return {
    useTraegerSchools: () => {
      const failure = new ApiError("down", 503, {
        code: "general.unavailable",
      });
      const error = useSwrLoadError(
        failure,
        "die Liste der Schulen",
        retrySchools,
      );
      return {
        schools: null,
        error,
        fetchError: failure,
        mutate: retrySchools,
      };
    },
  };
});

const { default: TraegerDocumentsPage } = await import("./page");

// The release column names the schools; without them releases to different
// schools read alike (#4692 review). The list shows the failed load with a
// retry instead.
it("shows a failed load of the schools with a retry", async () => {
  render(
    <ToastProvider>
      <TraegerDocumentsPage />
    </ToastProvider>,
  );

  expect(
    await screen.findByText(
      catalogText("general.unavailable", "die Liste der Schulen"),
    ),
  ).toBeInTheDocument();
  expect(screen.queryByText("eine Schule")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
  expect(retrySchools).toHaveBeenCalled();
});

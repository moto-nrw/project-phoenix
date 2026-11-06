import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

const { retrySchools, schoolsState } = vi.hoisted(() => ({
  retrySchools: vi.fn(),
  schoolsState: { failed: true },
}));

vi.mock("next/navigation", async (importOriginal) => ({
  ...(await importOriginal<typeof import("next/navigation")>()),
  useParams: () => ({ id: "4" }),
  useRouter: () => ({ push: vi.fn() }),
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: {
        scope: "org",
        permissions: ["traeger_overview:read", "traeger_enrollment:manage"],
      },
    },
  }),
}));
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));

const document = {
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
    {
      id: "2",
      audience: "schools",
      schoolId: "8",
      groupId: null,
      groupName: "",
      releasedAt: "2026-09-02T10:00:00Z",
    },
  ],
};

vi.mock("~/components/traeger/use-traeger-documents", () => ({
  useTraegerDocuments: () => ({
    data: [document],
    isLoading: false,
    loadError: null,
    mutate: vi.fn(),
  }),
  useTraegerDocumentAudiences: () => ({
    data: [],
    isLoading: false,
    loadError: null,
  }),
}));

vi.mock("~/components/traeger/use-traeger-schools", async () => {
  const { useSwrLoadError } = await import("~/lib/hooks/use-swr-load-error");
  return {
    useTraegerSchools: () => {
      const failure = schoolsState.failed
        ? new ApiError("down", 503, { code: "general.unavailable" })
        : undefined;
      const error = useSwrLoadError(
        failure,
        "die Liste der Schulen",
        retrySchools,
      );
      return {
        schools: schoolsState.failed
          ? null
          : [
              { id: "7", name: "OGS Nord" },
              { id: "8", name: "OGS Süd" },
            ],
        error,
        fetchError: failure,
        mutate: retrySchools,
      };
    },
  };
});

const { default: TraegerDocumentPage } = await import("./page");

function renderPage() {
  render(
    <ToastProvider>
      <TraegerDocumentPage />
    </ToastProvider>,
  );
}

beforeEach(() => {
  retrySchools.mockReset();
  schoolsState.failed = true;
});

// Without the school names two releases to different schools read alike, and
// revoking one could hit the wrong one (#4692 review): the page shows the
// failed load with a retry and offers no revocation until the names are back.
it("shows a failed load of the schools and offers no revocation", async () => {
  renderPage();

  expect(
    await screen.findByText(
      catalogText("general.unavailable", "die Liste der Schulen"),
    ),
  ).toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: /Freigabe zurücknehmen/ }),
  ).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
  expect(retrySchools).toHaveBeenCalled();
});

it("names each school of a release once the schools are loaded", async () => {
  schoolsState.failed = false;
  renderPage();

  expect(
    await screen.findByRole("button", {
      name: "Freigabe zurücknehmen: OGS Nord",
    }),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("button", {
      name: "Freigabe zurücknehmen: OGS Süd",
    }),
  ).toBeInTheDocument();
});

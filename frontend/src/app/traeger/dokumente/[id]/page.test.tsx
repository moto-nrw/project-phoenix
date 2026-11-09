import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

const { retrySchools, schoolsState, sessionState, router, deleteDocument } =
  vi.hoisted(() => ({
    retrySchools: vi.fn(),
    schoolsState: { failed: true },
    sessionState: {
      permissions: ["traeger_overview:read", "traeger_documents:manage"],
    },
    router: { push: vi.fn(), replace: vi.fn() },
    deleteDocument: vi.fn(),
  }));

vi.mock("next/navigation", async (importOriginal) => ({
  ...(await importOriginal<typeof import("next/navigation")>()),
  useParams: () => ({ id: "4" }),
  useRouter: () => router,
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: {
        scope: "org",
        permissions: sessionState.permissions,
      },
    },
  }),
}));
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));
vi.mock("~/lib/traeger-documents-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/traeger-documents-api")>()),
  deleteTraegerDocument: deleteDocument,
}));

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
  router.replace.mockReset();
  deleteDocument.mockReset();
  schoolsState.failed = true;
  sessionState.permissions = [
    "traeger_overview:read",
    "traeger_documents:manage",
  ];
});

async function openDeleteDialog() {
  fireEvent.click(
    await screen.findByRole("button", {
      name: "Weitere Aktionen zum Dokument",
    }),
  );
  fireEvent.click(
    await screen.findByRole("menuitem", { name: /Dokument löschen/ }),
  );
  return screen.findByRole("dialog", { name: "Dokument löschen?" });
}

function confirmDeletion(dialog: HTMLElement) {
  const confirm = within(dialog).getByRole("button", {
    name: "Endgültig löschen",
  });
  expect(confirm).toBeDisabled();
  fireEvent.change(
    within(dialog).getByLabelText("Tippen Sie zur Bestätigung LÖSCHEN ein:"),
    { target: { value: "LÖSCHEN" } },
  );
  fireEvent.click(confirm);
}

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
      name: "Freigabe zurücknehmen: OGS-Leitung: OGS Nord",
    }),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("button", {
      name: "Freigabe zurücknehmen: OGS-Leitung: OGS Süd",
    }),
  ).toBeInTheDocument();
});

// #5993: the dialog names every release that ends with the document, and
// only the typed word lets the deletion run; afterwards the page leaves for
// the list.
it("deletes the document after naming every release that ends", async () => {
  schoolsState.failed = false;
  deleteDocument.mockResolvedValue(undefined);
  renderPage();

  const dialog = await openDeleteDialog();
  expect(
    within(dialog).getByText("Diese Freigaben enden sofort"),
  ).toBeInTheDocument();
  const releases = within(dialog).getByRole("list");
  expect(
    within(releases).getByText("OGS-Leitung: OGS Nord"),
  ).toBeInTheDocument();
  expect(
    within(releases).getByText("OGS-Leitung: OGS Süd"),
  ).toBeInTheDocument();
  expect(
    within(dialog).getByText(
      /Auch ein gespeicherter Link öffnet es nicht mehr/,
    ),
  ).toBeInTheDocument();
  confirmDeletion(dialog);

  expect(
    await screen.findByText("„Hausordnung“ gelöscht."),
  ).toBeInTheDocument();
  expect(deleteDocument).toHaveBeenCalledWith("4");
  expect(router.replace).toHaveBeenCalledWith("/traeger/dokumente");
  expect(
    screen.queryByText("Dieses Dokument gibt es nicht."),
  ).not.toBeInTheDocument();
});

// A refused deletion keeps the dialog open with the reason and stays on the
// page; the document is unchanged.
it("keeps the dialog with the reason when the deletion fails", async () => {
  schoolsState.failed = false;
  deleteDocument.mockRejectedValue(
    new ApiError("not found", 404, { code: "traeger.document_not_found" }),
  );
  renderPage();

  const dialog = await openDeleteDialog();
  confirmDeletion(dialog);

  expect(
    await within(dialog).findByText(
      catalogText("traeger.document_not_found", "das Dokument"),
    ),
  ).toBeInTheDocument();
  expect(router.replace).not.toHaveBeenCalled();
  expect(
    screen.getByRole("dialog", { name: "Dokument löschen?" }),
  ).toBeInTheDocument();
});

// Only the role Dokumente deletes: a role that only reads gets no menu.
it("offers no deletion without the right to manage", async () => {
  schoolsState.failed = false;
  sessionState.permissions = ["traeger_overview:read"];
  renderPage();

  expect(
    await screen.findByRole("button", { name: "Herunterladen" }),
  ).toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Weitere Aktionen zum Dokument" }),
  ).not.toBeInTheDocument();
});

// #5992: "Anmeldung verwalten" alone no longer writes documents. The
// Verwaltung without the role Dokumente reads and downloads, sees no write
// action and learns who may change the releases.
it("offers no write action to the Verwaltung without the role Dokumente", async () => {
  schoolsState.failed = false;
  sessionState.permissions = [
    "traeger_overview:read",
    "traeger_enrollment:manage",
  ];
  renderPage();

  expect(
    await screen.findByRole("button", { name: "Herunterladen" }),
  ).toBeInTheDocument();
  expect(
    screen.getByText(/Ändern kann, wer die Rolle „Dokumente“ hat\./),
  ).toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Freigeben" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Weitere Aktionen zum Dokument" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: /^Freigabe zurücknehmen/ }),
  ).not.toBeInTheDocument();
});

// #5993 review: without the school names the dialog could not say which
// schools lose the document, so no deletion is offered until they load.
it("offers no deletion while the school names are missing", async () => {
  renderPage();

  expect(
    await screen.findByText(
      catalogText("general.unavailable", "die Liste der Schulen"),
    ),
  ).toBeInTheDocument();
  // The head menu still offers the download, but no deletion.
  fireEvent.click(
    screen.getByRole("button", { name: "Weitere Aktionen zum Dokument" }),
  );
  expect(
    await screen.findByRole("menuitem", { name: "Herunterladen" }),
  ).toBeInTheDocument();
  expect(
    screen.queryByRole("menuitem", { name: /Dokument löschen/ }),
  ).not.toBeInTheDocument();
});

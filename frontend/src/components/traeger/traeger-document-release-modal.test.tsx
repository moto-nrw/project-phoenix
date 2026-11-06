import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { catalogText } from "~/test/error-catalog-text";
import { TraegerDocumentReleaseModal } from "./traeger-document-release-modal";

vi.mock("~/lib/traeger-documents-api", () => ({
  releaseTraegerDocument: vi.fn(),
}));

function ModalWithFailedSchools({ retry }: Readonly<{ retry: () => void }>) {
  const loadError = useSwrLoadError(
    new ApiError("boom", 503, { code: "general.unavailable" }),
    "die Liste der Schulen",
    retry,
  );
  return (
    <TraegerDocumentReleaseModal
      isOpen
      documentId="4"
      documentTitle="Hausordnung"
      schools={[]}
      schoolsLoading={false}
      schoolsLoadError={loadError}
      onClose={vi.fn()}
      onReleased={vi.fn()}
    />
  );
}

// Without its schools the release to one school or group cannot be chosen;
// the modal says why and offers a new try (#4692 review).
it("shows a failed load of the schools with a retry", async () => {
  const retry = vi.fn();
  render(
    <ToastProvider>
      <ModalWithFailedSchools retry={retry} />
    </ToastProvider>,
  );

  fireEvent.click(screen.getByRole("button", { name: "Eine Schule" }));

  expect(
    await screen.findByText(
      catalogText("general.unavailable", "die Liste der Schulen"),
    ),
  ).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
  expect(retry).toHaveBeenCalled();
});

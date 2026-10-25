import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError, unavailableApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

const {
  fetchRunningSupervision,
  handOverSupervision,
  releaseSupervision,
  showSuccess,
} = vi.hoisted(() => ({
  fetchRunningSupervision: vi.fn(),
  handOverSupervision: vi.fn(),
  releaseSupervision: vi.fn(),
  showSuccess: vi.fn(),
}));

vi.mock("~/lib/substitution-api", () => ({
  substitutionService: {
    fetchRunningSupervision,
    handOverSupervision,
    releaseSupervision,
  },
}));
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: showSuccess }),
}));
vi.mock("~/components/ui/modal", () => ({
  dialogAriaProps: { role: "dialog" as const, "aria-modal": true },
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
      <section aria-label={title}>
        {children}
        {footer}
      </section>
    ) : null,
}));
vi.mock("~/components/ui/custom-select", () => ({
  CustomSelect: ({
    value,
    options,
    onChange,
    ariaLabelledBy,
  }: {
    value: string;
    options: Array<{ value: string; label: string }>;
    onChange: (value: string) => void;
    ariaLabelledBy: string;
  }) => (
    <select
      aria-labelledby={ariaLabelledBy}
      value={value}
      onChange={(event) => onChange(event.target.value)}
    >
      {options.map((option) => (
        <option key={option.value} value={option.value}>
          {option.label}
        </option>
      ))}
    </select>
  ),
}));

import {
  HandOverSupervisionButton,
  HandOverSupervisionModal,
} from "./hand-over-supervision-modal";

const overview = {
  id: "41",
  name: "Freispiel",
  roomName: "Atelier",
  supervisors: [
    { id: "11", fullName: "Alex Alt" },
    { id: "12", fullName: "Bea Bleibt" },
  ],
  availableTargets: [{ id: "73", fullName: "Toni Test" }],
  isCurrentUserSupervising: true,
  canAssign: true,
};

function renderModal(props: { onDone?: () => Promise<unknown> } = {}) {
  const onDone = props.onDone ?? vi.fn().mockResolvedValue(undefined);
  const onClose = vi.fn();
  render(
    <HandOverSupervisionModal
      activeGroupId="41"
      currentStaffId="11"
      isOpen
      onDone={onDone}
      onClose={onClose}
    />,
  );
  return { onDone, onClose };
}

function chooseRelease() {
  fireEvent.click(screen.getByRole("button", { name: "Aufsicht abgeben" }));
}

describe("HandOverSupervisionButton", () => {
  it("opens the dialog for the supervised session only", () => {
    const onHandOver = vi.fn();
    const { rerender } = render(
      <HandOverSupervisionButton activeGroupId="41" onHandOver={onHandOver} />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Aufsicht übergeben" }));
    expect(onHandOver).toHaveBeenCalledWith("41");

    rerender(
      <HandOverSupervisionButton
        activeGroupId={null}
        onHandOver={onHandOver}
      />,
    );
    expect(
      screen.queryByRole("button", { name: "Aufsicht übergeben" }),
    ).not.toBeInTheDocument();
  });
});

describe("HandOverSupervisionModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    fetchRunningSupervision.mockResolvedValue(overview);
    handOverSupervision.mockResolvedValue({
      id: "91",
      targetName: "Toni Test",
    });
    releaseSupervision.mockResolvedValue(undefined);
  });

  it("hands the supervision over to the chosen person", async () => {
    const { onDone, onClose } = renderModal();

    expect(
      await screen.findByText(
        "Die Kinder bleiben im Raum und die Aktivität läuft weiter. Die Planung ändert sich nicht.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Übergeben" })).toBeDisabled();
    fireEvent.change(screen.getByLabelText("An wen übergeben Sie?"), {
      target: { value: "73" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Übergeben" }));

    await waitFor(() =>
      expect(handOverSupervision).toHaveBeenCalledWith("41", "73"),
    );
    expect(releaseSupervision).not.toHaveBeenCalled();
    expect(onDone).toHaveBeenCalledOnce();
    expect(showSuccess).toHaveBeenCalledWith(
      "Toni Test hat die Aufsicht übernommen.",
    );
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("releases the supervision while another supervisor stays", async () => {
    const { onDone, onClose } = renderModal();

    await screen.findByLabelText("An wen übergeben Sie?");
    chooseRelease();
    expect(
      screen.getByText(/Weiter in der Aufsicht:/).closest("p"),
    ).toHaveTextContent("Weiter in der Aufsicht: Bea Bleibt");
    expect(screen.queryByText(/Alex Alt/)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Abgeben" }));

    await waitFor(() => expect(releaseSupervision).toHaveBeenCalledWith("41"));
    expect(handOverSupervision).not.toHaveBeenCalled();
    expect(onDone).toHaveBeenCalledOnce();
    expect(showSuccess).toHaveBeenCalledWith(
      "Sie sind nicht mehr als Aufsicht eingetragen.",
    );
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("disables releasing with an explanation for the only supervisor", async () => {
    fetchRunningSupervision.mockResolvedValue({
      ...overview,
      supervisors: [{ id: "11", fullName: "Alex Alt" }],
    });
    renderModal();

    await screen.findByLabelText("An wen übergeben Sie?");
    chooseRelease();

    expect(
      screen.getByText(
        "Sie sind die einzige Aufsicht. Übergeben Sie die Aufsicht an eine Person oder beenden Sie die Aktivität.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Abgeben" })).toBeDisabled();
  });

  it("offers nothing when the user no longer supervises the session", async () => {
    fetchRunningSupervision.mockResolvedValue({
      ...overview,
      isCurrentUserSupervising: false,
    });
    renderModal();

    expect(
      await screen.findByText(
        "Sie beaufsichtigen diese Aktivität nicht mehr. Deshalb können Sie die Aufsicht nicht übergeben.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Übergeben" })).toBeDisabled();
    expect(
      screen.queryByLabelText("An wen übergeben Sie?"),
    ).not.toBeInTheDocument();
  });

  it("keeps the dialog open with the catalog text when the release is refused", async () => {
    releaseSupervision.mockRejectedValue(
      new ApiError("no other supervisor stays with the session", 409, {
        code: "substitutions.last_supervisor",
      }),
    );
    const { onDone, onClose } = renderModal();

    await screen.findByLabelText("An wen übergeben Sie?");
    chooseRelease();
    fireEvent.click(screen.getByRole("button", { name: "Abgeben" }));

    expect(
      await screen.findByText(
        catalogText(
          "substitutions.last_supervisor",
          "das Abgeben der Aufsicht",
        ),
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("no other supervisor stays with the session"),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Abgeben" })).toBeEnabled();
    expect(onDone).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("shows a failed load in the dialog with retry", async () => {
    fetchRunningSupervision.mockRejectedValueOnce(
      unavailableApiError(new TypeError("Failed to fetch")),
    );
    renderModal();

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die laufende Aufsicht"),
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(
      await screen.findByLabelText("An wen übergeben Sie?"),
    ).toBeInTheDocument();
  });
});

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import deMessages from "~/i18n/messages/de.json";
import { getChildConsents, withdrawChildPhotoConsent } from "~/lib/parent-api";
import { ChildConsentsSection } from "./child-consents-section";

vi.mock("~/lib/parent-api", () => ({
  getChildConsents: vi.fn(),
  withdrawChildPhotoConsent: vi.fn(),
}));

const mockedGet = vi.mocked(getChildConsents);
const mockedWithdraw = vi.mocked(withdrawChildPhotoConsent);

const granted = [
  {
    key: "agb" as const,
    state: "granted" as const,
    changed_at: "2026-08-25T10:00:00Z",
    can_withdraw: false,
  },
  {
    key: "data_processing" as const,
    state: "granted" as const,
    changed_at: "2026-08-25T10:00:00Z",
    can_withdraw: false,
  },
  {
    key: "email_contact" as const,
    state: "not_recorded" as const,
    can_withdraw: false,
  },
  {
    key: "photo" as const,
    state: "granted" as const,
    changed_at: "2026-08-25T10:00:00Z",
    can_withdraw: true,
  },
];

function renderSection() {
  return render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <ChildConsentsSection studentId="42" />
    </NextIntlClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mockedGet.mockResolvedValue(granted);
});

describe("ChildConsentsSection", () => {
  it("bietet nach einem Ladefehler einen neuen Versuch an", async () => {
    const user = userEvent.setup();
    mockedGet.mockRejectedValueOnce(new Error("offline"));
    renderSection();

    expect(
      await screen.findByText(
        "Die Einwilligungen konnten gerade nicht geladen werden. Bitte versuchen Sie es noch einmal.",
      ),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Erneut laden" }));

    expect(
      await screen.findByRole("heading", {
        name: "Einwilligungen und Bestätigungen",
      }),
    ).toBeInTheDocument();
    expect(mockedGet).toHaveBeenCalledTimes(2);
  });

  it("zeigt vier klar getrennte Zustände und nur bei Fotos eine Aktion", async () => {
    renderSection();

    expect(
      await screen.findByRole("heading", {
        name: "Einwilligungen und Bestätigungen",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Allgemeine Geschäftsbedingungen (AGB)"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Datenschutz zur Kenntnis genommen"),
    ).toBeInTheDocument();
    expect(screen.getByText("Kontakt per E-Mail erlaubt")).toBeInTheDocument();
    expect(screen.getByText("Foto-Einwilligung")).toBeInTheDocument();
    expect(screen.getAllByText("Hinterlegt")).toHaveLength(3);
    expect(screen.getByText("Nicht hinterlegt")).toBeInTheDocument();
    expect(
      screen.getAllByRole("button", { name: "Foto-Einwilligung widerrufen" }),
    ).toHaveLength(1);
  });

  it("erklärt die sofortige Wirkung und zeigt danach den Widerruf", async () => {
    const user = userEvent.setup();
    mockedWithdraw.mockResolvedValue([
      ...granted.slice(0, 3),
      {
        key: "photo",
        state: "withdrawn",
        changed_at: "2026-08-31T09:45:00Z",
        can_withdraw: false,
      },
    ]);
    renderSection();

    await user.click(
      await screen.findByRole("button", {
        name: "Foto-Einwilligung widerrufen",
      }),
    );
    expect(
      screen.getByRole("heading", { name: "Foto-Einwilligung widerrufen?" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Die Einwilligung endet sofort. Falls ein Foto gespeichert ist, wird es gelöscht.",
      ),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Jetzt widerrufen" }));

    await waitFor(() => expect(mockedWithdraw).toHaveBeenCalledWith("42"));
    const withdrawn = await screen.findByText("Widerrufen");
    const notRecorded = screen.getByText("Nicht hinterlegt");
    expect(withdrawn).not.toHaveAttribute(
      "style",
      notRecorded.getAttribute("style"),
    );
    expect(
      screen.queryByRole("button", { name: "Foto-Einwilligung widerrufen" }),
    ).not.toBeInTheDocument();
  });
});

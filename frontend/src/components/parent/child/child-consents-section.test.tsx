import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import deMessages from "~/i18n/messages/de.json";
import {
  getChildConsents,
  grantChildPhotoConsent,
  withdrawChildPhotoConsent,
} from "~/lib/parent-api";
import { ChildConsentsSection } from "./child-consents-section";

vi.mock("~/lib/parent-api", () => ({
  getChildConsents: vi.fn(),
  grantChildPhotoConsent: vi.fn(),
  withdrawChildPhotoConsent: vi.fn(),
}));

const mockedGet = vi.mocked(getChildConsents);
const mockedGrant = vi.mocked(grantChildPhotoConsent);
const mockedWithdraw = vi.mocked(withdrawChildPhotoConsent);

const granted = [
  {
    key: "agb" as const,
    state: "granted" as const,
    changed_at: "2026-08-25T10:00:00Z",
    can_withdraw: false,
    can_grant: false,
  },
  {
    key: "data_processing" as const,
    state: "granted" as const,
    changed_at: "2026-08-25T10:00:00Z",
    can_withdraw: false,
    can_grant: false,
  },
  {
    key: "email_contact" as const,
    state: "not_recorded" as const,
    can_withdraw: false,
    can_grant: false,
  },
  {
    key: "photo" as const,
    state: "granted" as const,
    changed_at: "2026-08-25T10:00:00Z",
    can_withdraw: true,
    can_grant: false,
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

  it("zeigt nur hinterlegte Einwilligungen und nur bei Fotos eine Aktion", async () => {
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
    expect(
      screen.queryByText("Kontakt per E-Mail erlaubt"),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Foto-Einwilligung")).toBeInTheDocument();
    expect(screen.getAllByText("Hinterlegt")).toHaveLength(3);
    expect(screen.queryByText("Nicht hinterlegt")).not.toBeInTheDocument();
    expect(
      screen.getByText(
        "Hier sehen Sie, was für Ihr Kind gespeichert ist. Die Foto-Einwilligung können Sie hier ändern.",
      ),
    ).toBeInTheDocument();
    const withdrawButton = screen.getByRole("button", {
      name: "Foto-Einwilligung widerrufen",
    });
    expect(withdrawButton).toHaveTextContent(/^Widerrufen$/);
  });

  it("nennt die Änderungsmöglichkeit nur bei einer änderbaren Foto-Einwilligung", async () => {
    mockedGet.mockResolvedValue(granted.slice(0, 2));

    renderSection();

    expect(
      await screen.findByText(
        "Hier sehen Sie, was für Ihr Kind gespeichert ist.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Die Foto-Einwilligung können Sie hier ändern."),
    ).not.toBeInTheDocument();
  });

  it("blendet den gesamten Bereich ohne hinterlegte Einwilligungen aus", async () => {
    mockedGet.mockResolvedValue(
      granted.map((consent) => ({
        ...consent,
        state: "not_recorded" as const,
        changed_at: undefined,
        can_withdraw: false,
        can_grant: false,
      })),
    );

    renderSection();

    await waitFor(() =>
      expect(
        screen.queryByTestId("parent-page-section-skeleton"),
      ).not.toBeInTheDocument(),
    );
    expect(
      screen.queryByRole("heading", {
        name: "Einwilligungen und Bestätigungen",
      }),
    ).not.toBeInTheDocument();
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
        can_grant: true,
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
    expect(await screen.findByText("Widerrufen")).toBeInTheDocument();
    expect(screen.queryByText("Nicht hinterlegt")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Foto-Einwilligung widerrufen" }),
    ).not.toBeInTheDocument();
  });

  it("bietet nach einem Widerruf eine neue Foto-Einwilligung an", async () => {
    const user = userEvent.setup();
    mockedGet.mockResolvedValue([
      ...granted.slice(0, 3),
      {
        key: "photo",
        state: "withdrawn",
        changed_at: "2026-08-31T09:45:00Z",
        can_withdraw: false,
        can_grant: true,
      },
    ]);
    mockedGrant.mockResolvedValue(granted);
    renderSection();

    await user.click(
      await screen.findByRole("button", {
        name: "Foto-Einwilligung erneut geben",
      }),
    );
    expect(
      screen.getByRole("heading", {
        name: "Foto-Einwilligung erneut geben?",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Die OGS darf danach wieder ein Foto hinterlegen."),
    ).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: "Einwilligung geben" }),
    );

    await waitFor(() => expect(mockedGrant).toHaveBeenCalledWith("42"));
    expect(await screen.findAllByText("Hinterlegt")).toHaveLength(3);
    expect(
      screen.getByText("Die Foto-Einwilligung wurde erneut gegeben."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Foto-Einwilligung widerrufen" }),
    ).toBeInTheDocument();
  });
});

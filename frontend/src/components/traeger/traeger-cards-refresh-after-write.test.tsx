// #6652: on every Träger card, a confirmed write whose reload fails is no
// failed action. The reload error stands above the card, says the change is
// saved, and its „Wiederholen“ only reloads; the write runs exactly once.
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { SAVED_BUT_STALE_SUFFIX } from "~/lib/hooks/use-refresh-after-write";
import { catalogText } from "~/test/error-catalog-text";

const api = vi.hoisted(() => ({
  saveHolidayNumberRange: vi.fn(),
  saveCareNumberRange: vi.fn(),
  updateTraegerBankAccount: vi.fn(),
  deleteTraegerBankAccount: vi.fn(),
  createTraegerBankAccount: vi.fn(),
  deleteContributionRule: vi.fn(),
  createContributionRule: vi.fn(),
  updateContributionRule: vi.fn(),
  saveCostBearer: vi.fn(),
  deleteCostBearer: vi.fn(),
  uploadTraegerLogo: vi.fn(),
  removeTraegerLogo: vi.fn(),
}));

vi.mock("~/lib/traeger-billing-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  saveHolidayNumberRange: api.saveHolidayNumberRange,
  saveCareNumberRange: api.saveCareNumberRange,
}));
vi.mock("~/lib/traeger-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  updateTraegerBankAccount: api.updateTraegerBankAccount,
  deleteTraegerBankAccount: api.deleteTraegerBankAccount,
  createTraegerBankAccount: api.createTraegerBankAccount,
  uploadTraegerLogo: api.uploadTraegerLogo,
  removeTraegerLogo: api.removeTraegerLogo,
}));
vi.mock("~/lib/traeger-contributions-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  deleteContributionRule: api.deleteContributionRule,
  createContributionRule: api.createContributionRule,
  updateContributionRule: api.updateContributionRule,
  saveCostBearer: api.saveCostBearer,
  deleteCostBearer: api.deleteCostBearer,
}));

import { ContributionRulesCard } from "./contribution-rules-card";
import { CostBearersCard } from "./cost-bearers-card";
import { NumberRangeCard } from "./number-range-card";
import { TraegerBankAccountsCard } from "./traeger-bank-accounts-card";
import { TraegerLogoCard } from "./traeger-logo-card";

const down = () => new ApiError("down", 503, { code: "general.unavailable" });
const reloadText = (object: string) =>
  `${catalogText("general.unavailable", object)} ${SAVED_BUT_STALE_SUFFIX}`;

function failingReload() {
  return vi.fn().mockRejectedValueOnce(down()).mockResolvedValue(undefined);
}

async function expectReloadOnly(
  object: string,
  onChanged: ReturnType<typeof vi.fn>,
  write: ReturnType<typeof vi.fn>,
) {
  expect(await screen.findByText(reloadText(object))).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
  await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(2));
  expect(write).toHaveBeenCalledTimes(1);
}

async function chooseMenuItem(trigger: string, item: string) {
  const [menu] = await screen.findAllByRole("button", { name: trigger });
  fireEvent.click(menu!);
  fireEvent.click(await screen.findByRole("menuitem", { name: item }));
}

async function confirmTwoSteps(first: string, second: string) {
  fireEvent.click(await screen.findByRole("button", { name: first }));
  fireEvent.click(await screen.findByRole("button", { name: second }));
}

describe("Träger cards reload after a confirmed write", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("keeps a saved number pattern saved when the reload fails", async () => {
    api.saveHolidayNumberRange.mockResolvedValue(undefined);
    const onChanged = failingReload();
    render(
      <NumberRangeCard
        numberRange={{ pattern: "", example: "" } as never}
        onChanged={onChanged}
      />,
      { wrapper: ToastProvider },
    );

    fireEvent.click(screen.getByRole("button", { name: "Muster festlegen" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Speichern" }));

    await expectReloadOnly(
      "die Anzeige des Nummernkreises",
      onChanged,
      api.saveHolidayNumberRange,
    );
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  const account = {
    id: "3",
    label: "Hauptkonto",
    accountHolder: "Träger",
    iban: "DE02120300000000202051",
    bic: "BYLADEM1001",
    bankName: "Bank",
    isDefault: false,
  };

  it("keeps a new default bank account when the reload fails", async () => {
    api.updateTraegerBankAccount.mockResolvedValue(undefined);
    const onChanged = failingReload();
    render(
      <TraegerBankAccountsCard
        accounts={[account]}
        canEdit
        onChanged={onChanged}
      />,
      { wrapper: ToastProvider },
    );

    await chooseMenuItem(
      "Aktionen für die Bankverbindung Hauptkonto",
      "Als Standard festlegen",
    );

    await expectReloadOnly(
      "die Anzeige der Bankverbindungen",
      onChanged,
      api.updateTraegerBankAccount,
    );
    expect(
      screen.queryByText(
        catalogText("general.unavailable", "die Bankverbindung"),
      ),
    ).toBeNull();
  });

  it("keeps a deleted bank account deleted when the reload fails", async () => {
    api.deleteTraegerBankAccount.mockResolvedValue(undefined);
    const onChanged = failingReload();
    render(
      <TraegerBankAccountsCard
        accounts={[account]}
        canEdit
        onChanged={onChanged}
      />,
      { wrapper: ToastProvider },
    );

    await chooseMenuItem(
      "Aktionen für die Bankverbindung Hauptkonto",
      "Löschen",
    );
    await confirmTwoSteps("Ja, löschen", "Löschen");

    await expectReloadOnly(
      "die Anzeige der Bankverbindungen",
      onChanged,
      api.deleteTraegerBankAccount,
    );
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("keeps a deleted contribution rule deleted when the reload fails", async () => {
    api.deleteContributionRule.mockResolvedValue(undefined);
    const onChanged = failingReload();
    render(
      <ContributionRulesCard
        kind="benefits_exemption"
        rules={[
          {
            id: "1",
            kind: "benefits_exemption",
            validFrom: "2026-01-01",
            validUntil: null,
            siblingFromChild: null,
            discountBasisPoints: null,
            discountCents: null,
            note: "Befreit",
          },
        ]}
        onChanged={onChanged}
      />,
      { wrapper: ToastProvider },
    );

    await chooseMenuItem(/Aktionen für die Regel/ as never, "Löschen");
    await confirmTwoSteps("Ja, löschen", "Löschen");

    await expectReloadOnly(
      "die Anzeige der Regeln",
      onChanged,
      api.deleteContributionRule,
    );
    expect(screen.queryByText("Regel löschen?")).toBeNull();
  });

  const bearer = {
    id: "5",
    name: "Jugendamt",
    address: "",
    fileReference: "",
    active: true,
    inUse: true,
  };

  it("keeps a switched-off cost bearer switched off when the reload fails", async () => {
    api.saveCostBearer.mockResolvedValue(undefined);
    const onChanged = failingReload();
    render(<CostBearersCard bearers={[bearer]} onChanged={onChanged} />, {
      wrapper: ToastProvider,
    });

    await chooseMenuItem("Aktionen für Jugendamt", "Ausschalten");

    await expectReloadOnly(
      "die Anzeige der Kostenträger",
      onChanged,
      api.saveCostBearer,
    );
  });

  it("keeps a deleted cost bearer deleted when the reload fails", async () => {
    api.deleteCostBearer.mockResolvedValue(undefined);
    const onChanged = failingReload();
    render(
      <CostBearersCard
        bearers={[{ ...bearer, inUse: false }]}
        onChanged={onChanged}
      />,
      { wrapper: ToastProvider },
    );

    await chooseMenuItem("Aktionen für Jugendamt", "Löschen");
    await confirmTwoSteps("Ja, löschen", "Löschen");

    await expectReloadOnly(
      "die Anzeige der Kostenträger",
      onChanged,
      api.deleteCostBearer,
    );
  });

  it("keeps a removed logo removed when the reload fails", async () => {
    api.removeTraegerLogo.mockResolvedValue(undefined);
    const onChanged = failingReload();
    render(
      <TraegerLogoCard
        logo={{
          contentType: "image/png",
          width: 100,
          height: 100,
          sizeBytes: 1000,
          updatedAt: "2026-09-01T10:00:00Z",
        }}
        canEdit
        onChanged={onChanged}
      />,
      { wrapper: ToastProvider },
    );

    fireEvent.click(screen.getByRole("button", { name: "Logo entfernen" }));
    await confirmTwoSteps("Ja, entfernen", "Entfernen");

    await expectReloadOnly(
      "die Anzeige des Logos",
      onChanged,
      api.removeTraegerLogo,
    );
  });

  it("keeps an uploaded logo saved when the reload fails", async () => {
    api.uploadTraegerLogo.mockResolvedValue(undefined);
    const onChanged = failingReload();
    render(<TraegerLogoCard logo={null} canEdit onChanged={onChanged} />, {
      wrapper: ToastProvider,
    });

    fireEvent.change(screen.getByLabelText("Logo-Datei auswählen"), {
      target: {
        files: [new File(["png"], "logo.png", { type: "image/png" })],
      },
    });

    await expectReloadOnly(
      "die Anzeige des Logos",
      onChanged,
      api.uploadTraegerLogo,
    );
  });
});

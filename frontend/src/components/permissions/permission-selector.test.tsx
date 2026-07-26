/**
 * Tests for PermissionSelector
 * Tests the rendering and functionality of the permission resource/action selector
 */
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { PermissionSelector } from "./permission-selector";
import type { PermissionSelectorValue } from "./permission-selector";

describe("PermissionSelector", () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders resource and action dropdowns", async () => {
    render(<PermissionSelector value={undefined} onChange={mockOnChange} />);

    await waitFor(() => {
      expect(screen.getByLabelText(/ressource/i)).toBeInTheDocument();
      expect(screen.getByLabelText(/aktion/i)).toBeInTheDocument();
    });
  });

  it("displays default placeholder for resource", async () => {
    render(<PermissionSelector value={undefined} onChange={mockOnChange} />);

    await waitFor(() => {
      const resourceSelect =
        screen.getByLabelText<HTMLSelectElement>(/ressource/i);
      expect(resourceSelect.value).toBe("");
      expect(screen.getByText("Ressource auswählen...")).toBeInTheDocument();
    });
  });

  it("displays default placeholder for action", async () => {
    render(<PermissionSelector value={undefined} onChange={mockOnChange} />);

    await waitFor(() => {
      const actionSelect = screen.getByLabelText<HTMLSelectElement>(/aktion/i);
      expect(actionSelect.value).toBe("");
      expect(screen.getByText("Zuerst Ressource wählen")).toBeInTheDocument();
    });
  });

  it("renders with initial value", async () => {
    const value: PermissionSelectorValue = {
      resource: "users",
      action: "read",
    };

    render(<PermissionSelector value={value} onChange={mockOnChange} />);

    await waitFor(() => {
      const resourceSelect = screen.getByLabelText(/ressource/i);
      const actionSelect = screen.getByLabelText(/aktion/i);
      expect(resourceSelect).toHaveTextContent("Benutzer");
      expect(actionSelect).toHaveTextContent("Lesen");
    });
  });

  it("enables action dropdown when resource is selected", async () => {
    render(<PermissionSelector value={undefined} onChange={mockOnChange} />);

    const resourceSelect = screen.getByLabelText(/ressource/i);
    fireEvent.click(resourceSelect);
    fireEvent.click(screen.getByRole("option", { name: "Benutzer" }));

    await waitFor(() => {
      const actionSelect = screen.getByLabelText(/aktion/i);
      expect(actionSelect).not.toBeDisabled();
    });
  });

  it("disables action dropdown when no resource is selected", async () => {
    render(<PermissionSelector value={undefined} onChange={mockOnChange} />);

    await waitFor(() => {
      const actionSelect = screen.getByLabelText(/aktion/i);
      expect(actionSelect).toBeDisabled();
    });
  });

  it("calls onChange when resource is selected", async () => {
    render(<PermissionSelector value={undefined} onChange={mockOnChange} />);

    const resourceSelect = screen.getByLabelText(/ressource/i);
    fireEvent.click(resourceSelect);
    fireEvent.click(screen.getByRole("option", { name: "Benutzer" }));

    // onChange is not called until action is also selected
    expect(mockOnChange).not.toHaveBeenCalled();
  });

  it("calls onChange when both resource and action are selected", async () => {
    render(<PermissionSelector value={undefined} onChange={mockOnChange} />);

    const resourceSelect = screen.getByLabelText(/ressource/i);
    fireEvent.click(resourceSelect);
    fireEvent.click(screen.getByRole("option", { name: "Benutzer" }));

    const actionSelect = screen.getByLabelText(/aktion/i);
    fireEvent.click(actionSelect);
    fireEvent.click(screen.getByRole("option", { name: "Lesen" }));

    await waitFor(() => {
      expect(mockOnChange).toHaveBeenCalledWith({
        resource: "users",
        action: "read",
      });
    });
  });

  it("displays permission preview when both values are set", async () => {
    const value: PermissionSelectorValue = {
      resource: "users",
      action: "read",
    };

    render(<PermissionSelector value={value} onChange={mockOnChange} />);

    await waitFor(() => {
      expect(screen.getByText("Berechtigungsname:")).toBeInTheDocument();
      expect(screen.getByText("users:read")).toBeInTheDocument();
    });
  });

  it("renders required indicators when required prop is true", async () => {
    render(
      <PermissionSelector
        value={undefined}
        onChange={mockOnChange}
        required={true}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText(/ressource\*/i)).toBeInTheDocument();
      expect(screen.getByText(/aktion\*/i)).toBeInTheDocument();
    });
  });

  it("renders all resource options", async () => {
    render(<PermissionSelector value={undefined} onChange={mockOnChange} />);

    const resourceSelect = screen.getByLabelText(/ressource/i);
    fireEvent.click(resourceSelect);

    await waitFor(() => {
      expect(screen.getAllByRole("option").length).toBeGreaterThan(1);
      expect(
        screen.getByRole("option", { name: "Benutzer" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("option", { name: "Aktivitäten" }),
      ).toBeInTheDocument();
      expect(screen.getByRole("option", { name: "Räume" })).toBeInTheDocument();
    });
  });

  it("updates action options based on selected resource", async () => {
    render(<PermissionSelector value={undefined} onChange={mockOnChange} />);

    const resourceSelect = screen.getByLabelText(/ressource/i);
    fireEvent.click(resourceSelect);
    fireEvent.click(screen.getByRole("option", { name: "Benutzer" }));

    const actionSelect = screen.getByLabelText(/aktion/i);
    fireEvent.click(actionSelect);

    await waitFor(() => {
      expect(screen.getAllByRole("option").length).toBeGreaterThan(1);
      expect(
        screen.getByRole("option", { name: "Erstellen" }),
      ).toBeInTheDocument();
      expect(screen.getByRole("option", { name: "Lesen" })).toBeInTheDocument();
      expect(
        screen.getByRole("option", { name: "Bearbeiten" }),
      ).toBeInTheDocument();
    });
  });

  it("resets action when resource changes to incompatible value", async () => {
    const value: PermissionSelectorValue = {
      resource: "users",
      action: "read",
    };

    render(<PermissionSelector value={value} onChange={mockOnChange} />);

    const resourceSelect = screen.getByLabelText(/ressource/i);
    fireEvent.click(resourceSelect);
    fireEvent.click(screen.getByRole("option", { name: "Authentifizierung" }));

    await waitFor(() => {
      const actionSelect = screen.getByLabelText(/aktion/i);
      // auth only has "manage" action, "read" should be reset -> placeholder
      expect(actionSelect).toHaveTextContent("Aktion auswählen...");
    });
  });

  it("maintains action when resource changes to compatible value", async () => {
    const value: PermissionSelectorValue = {
      resource: "users",
      action: "read",
    };

    render(<PermissionSelector value={value} onChange={mockOnChange} />);

    const resourceSelect = screen.getByLabelText(/ressource/i);
    fireEvent.click(resourceSelect);
    fireEvent.click(screen.getByRole("option", { name: "Räume" }));

    await waitFor(() => {
      const actionSelect = screen.getByLabelText(/aktion/i);
      // rooms also has "read" action, so it should be maintained
      expect(actionSelect).toHaveTextContent("Lesen");
    });
  });

  it("displays German action labels", async () => {
    const value: PermissionSelectorValue = {
      resource: "users",
      action: "create",
    };

    render(<PermissionSelector value={value} onChange={mockOnChange} />);

    const actionSelect = screen.getByLabelText(/aktion/i);
    fireEvent.click(actionSelect);

    await waitFor(() => {
      expect(
        screen.getByRole("option", { name: "Erstellen" }),
      ).toBeInTheDocument();
    });
  });
});

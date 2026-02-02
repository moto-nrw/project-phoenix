/**
 * Tests for PermissionSelector
 * Tests the rendering and functionality of the permission resource/action selector
 */
import {
  render,
  screen,
  waitFor,
  fireEvent,
  act,
} from "@testing-library/react";
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
      const resourceSelect =
        screen.getByLabelText<HTMLSelectElement>(/ressource/i);
      const actionSelect = screen.getByLabelText<HTMLSelectElement>(/aktion/i);
      expect(resourceSelect.value).toBe("users");
      expect(actionSelect.value).toBe("read");
    });
  });

  it("enables action dropdown when resource is selected", async () => {
    render(<PermissionSelector value={undefined} onChange={mockOnChange} />);

    const resourceSelect = screen.getByLabelText(/ressource/i);

    await act(async () => {
      fireEvent.change(resourceSelect, { target: { value: "users" } });
    });

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

    await act(async () => {
      fireEvent.change(resourceSelect, { target: { value: "users" } });
    });

    // onChange is not called until action is also selected
    expect(mockOnChange).not.toHaveBeenCalled();
  });

  it("calls onChange when both resource and action are selected", async () => {
    render(<PermissionSelector value={undefined} onChange={mockOnChange} />);

    const resourceSelect = screen.getByLabelText(/ressource/i);
    const actionSelect = screen.getByLabelText(/aktion/i);

    await act(async () => {
      fireEvent.change(resourceSelect, { target: { value: "users" } });
    });

    await act(async () => {
      fireEvent.change(actionSelect, { target: { value: "read" } });
    });

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
      expect(screen.getByText("Permission-Name:")).toBeInTheDocument();
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

    await waitFor(() => {
      const resourceSelect =
        screen.getByLabelText<HTMLSelectElement>(/ressource/i);
      const options = Array.from(resourceSelect.options);

      expect(options.length).toBeGreaterThan(1);
      expect(options.some((opt) => opt.value === "users")).toBe(true);
      expect(options.some((opt) => opt.value === "activities")).toBe(true);
      expect(options.some((opt) => opt.value === "rooms")).toBe(true);
    });
  });

  it("updates action options based on selected resource", async () => {
    render(<PermissionSelector value={undefined} onChange={mockOnChange} />);

    const resourceSelect = screen.getByLabelText(/ressource/i);

    await act(async () => {
      fireEvent.change(resourceSelect, { target: { value: "users" } });
    });

    await waitFor(() => {
      const actionSelect = screen.getByLabelText<HTMLSelectElement>(/aktion/i);
      const options = Array.from(actionSelect.options);

      expect(options.length).toBeGreaterThan(1);
      expect(options.some((opt) => opt.value === "create")).toBe(true);
      expect(options.some((opt) => opt.value === "read")).toBe(true);
      expect(options.some((opt) => opt.value === "update")).toBe(true);
    });
  });

  it("resets action when resource changes to incompatible value", async () => {
    const value: PermissionSelectorValue = {
      resource: "users",
      action: "read",
    };

    render(<PermissionSelector value={value} onChange={mockOnChange} />);

    const resourceSelect = screen.getByLabelText(/ressource/i);

    await act(async () => {
      fireEvent.change(resourceSelect, { target: { value: "auth" } });
    });

    await waitFor(() => {
      const actionSelect = screen.getByLabelText<HTMLSelectElement>(/aktion/i);
      // auth only has "manage" action, "read" should be reset
      expect(actionSelect.value).toBe("");
    });
  });

  it("maintains action when resource changes to compatible value", async () => {
    const value: PermissionSelectorValue = {
      resource: "users",
      action: "read",
    };

    render(<PermissionSelector value={value} onChange={mockOnChange} />);

    const resourceSelect = screen.getByLabelText(/ressource/i);

    await act(async () => {
      fireEvent.change(resourceSelect, { target: { value: "rooms" } });
    });

    await waitFor(() => {
      const actionSelect = screen.getByLabelText<HTMLSelectElement>(/aktion/i);
      // rooms also has "read" action, so it should be maintained
      expect(actionSelect.value).toBe("read");
    });
  });

  it("displays German action labels", async () => {
    const value: PermissionSelectorValue = {
      resource: "users",
      action: "create",
    };

    render(<PermissionSelector value={value} onChange={mockOnChange} />);

    await waitFor(() => {
      const actionSelect = screen.getByLabelText<HTMLSelectElement>(/aktion/i);
      const createOption = Array.from(actionSelect.options).find(
        (opt) => opt.value === "create",
      );
      expect(createOption?.textContent).toBe("Erstellen");
    });
  });
});

import React from "react";
import { render, type RenderOptions } from "@testing-library/react";
import { ToastProvider } from "~/contexts/ToastContext";

/**
 * Custom render function that wraps components with all necessary providers.
 * Use this instead of @testing-library/react's render for components that
 * need context providers (e.g., ToastContext).
 */
function AllTheProviders({ children }: { children: React.ReactNode }) {
  return <ToastProvider>{children}</ToastProvider>;
}

function customRender(
  ui: React.ReactElement,
  options?: Omit<RenderOptions, "wrapper">,
) {
  return render(ui, { wrapper: AllTheProviders, ...options });
}

// Re-export everything from @testing-library/react
export * from "@testing-library/react";

// Override render with our custom render
export { customRender as render };

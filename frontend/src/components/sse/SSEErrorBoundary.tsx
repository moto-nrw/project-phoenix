"use client";

import React from "react";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { logDisplayedCrash } from "~/lib/error-log";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "SSEErrorBoundary" });

interface SSEErrorBoundaryProps {
  readonly children: React.ReactNode;
  readonly fallback?: React.ReactNode;
}

interface SSEErrorBoundaryState {
  readonly hasError: boolean;
}

export class SSEErrorBoundary extends React.Component<
  SSEErrorBoundaryProps,
  SSEErrorBoundaryState
> {
  constructor(props: SSEErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(): SSEErrorBoundaryState {
    return { hasError: true };
  }

  componentDidCatch(error: unknown, errorInfo: unknown) {
    logger.error("SSE boundary caught an error", {
      error: error instanceof Error ? error.message : String(error),
      error_info: String(errorInfo),
    });
    logDisplayedCrash(error);
  }

  render() {
    if (this.state.hasError) {
      return (
        this.props.fallback ?? (
          <LoadErrorAlert error="Live-Updates sind derzeit nicht verfügbar." />
        )
      );
    }
    return this.props.children;
  }
}

"use client";

import * as Sentry from "@sentry/nextjs";
import React, { type ErrorInfo, type ReactNode } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentDepartureErrorBoundary" });

interface StudentDepartureErrorBoundaryProps {
  readonly children: ReactNode;
}

interface StudentDepartureErrorBoundaryState {
  readonly error: Error | null;
}

export class StudentDepartureErrorBoundary extends React.Component<
  StudentDepartureErrorBoundaryProps,
  StudentDepartureErrorBoundaryState
> {
  state: StudentDepartureErrorBoundaryState = { error: null };

  static getDerivedStateFromError(
    error: unknown,
  ): StudentDepartureErrorBoundaryState {
    return {
      error: error instanceof Error ? error : new Error(String(error)),
    };
  }

  componentDidCatch(error: unknown, _errorInfo: ErrorInfo): void {
    const normalized =
      error instanceof Error ? error : new Error(String(error));
    Sentry.captureException(normalized);
    logger.error("student_departure_render_failed", {
      error: normalized.message,
      error_name: normalized.name,
    });
  }

  private retry = (): void => {
    this.setState({ error: null });
  };

  render(): ReactNode {
    if (this.state.error) {
      return (
        <Alert
          type="error"
          message="Die erlaubten Heimwege konnten nicht angezeigt werden. Bitte versuchen Sie es erneut."
          action={
            <Button
              type="button"
              variant="outline"
              size="compact"
              onClick={this.retry}
            >
              Erneut versuchen
            </Button>
          }
        />
      );
    }

    return this.props.children;
  }
}

import { afterAll, describe, expect, it } from "vitest";
import { lintSource, removeProbeDirectories } from "./oxlint-probe";

afterAll(removeProbeDirectories);

// Every string method the match rule knows. The fixtures build their calls
// from this list, so the literal call text never sits in the repo.
const MATCH_METHODS = [
  "includes",
  "startsWith",
  "endsWith",
  "match",
  "matchAll",
  "search",
  "indexOf",
  "lastIndexOf",
];

function count(output: string, rule: string) {
  return output.split(`error-path(${rule})`).length - 1;
}

describe.concurrent("error-path rules", () => {
  describe("error-path/no-error-alert", () => {
    it("rejects a literal, a conditional and a const error type", async () => {
      const { status, output } = await lintSource(
        `import { Alert } from "~/components/ui/alert";
      export function Probe({ late }: { late: boolean }) {
        const tone = late ? "error" : "warning";
        return (
          <>
            <Alert type="error" message="Speichern fehlgeschlagen." />
            <Alert type={late ? "error" : "warning"} message="Frist" />
            <Alert type={tone} message="Frist" />
          </>
        );
      }`,
      );

      expect(status).toBe(1);
      expect(count(output, "no-error-alert")).toBe(3);
    });

    it("rejects a type it cannot resolve, also through an import alias", async () => {
      const { status, output } = await lintSource(
        `import { Alert as Banner } from "~/components/ui/alert";
      import { alertTypeFor } from "~/lib/somewhere";
      export function Probe({ notice }: { notice: { type: "error" | "info"; message: string } }) {
        return (
          <>
            <Banner type={alertTypeFor(notice)} message="x" />
            <Banner type={notice.type} message={notice.message} />
          </>
        );
      }`,
      );

      expect(status).toBe(1);
      expect(count(output, "no-error-alert")).toBe(2);
    });

    it("accepts hints that are not failures", async () => {
      const { status, output } = await lintSource(
        `import { Alert } from "~/components/ui/alert";
      export function Probe({ done, kind }: { done: boolean; kind: "info" | "success" }) {
        const tone: "warning" | "info" = done ? "info" : "warning";
        return (
          <>
            <Alert type="warning" message="Die Frist endet morgen." />
            <Alert type={done ? "success" : "info"} message="Stand" />
            <Alert type={tone} message="Stand" />
            <Alert type={kind} message="Stand" />
          </>
        );
      }`,
      );

      expect(status).toBe(0);
      expect(output).not.toContain("error-path");
    });

    it("accepts the modules that implement the display path", async () => {
      const { status, output } = await lintSource(
        `import { Alert } from "./alert";
      export function FormErrorAlert({ message }: { message: string }) {
        return <Alert type="error" message={message} />;
      }`,
        "src/components/ui/form-error-alert.tsx",
      );

      expect(status).toBe(0);
      expect(output).not.toContain("error-path");
    });

    it("ignores tests and an Alert that is not the kit Alert", async () => {
      const inTest = await lintSource(
        `import { Alert } from "~/components/ui/alert";
      export const probe = <Alert type="error" message="x" />;`,
        "src/components/probe.test.tsx",
      );
      const otherAlert = await lintSource(
        `function Alert(props: { type: string; message: string }) {
        return <p>{props.message}</p>;
      }
      export const probe = <Alert type="error" message="x" />;`,
      );

      expect(inTest.output).not.toContain("error-path");
      expect(otherAlert.output).not.toContain("error-path");
    });
  });

  describe("error-path/no-error-text-display", () => {
    it("rejects an error's text in toasts, setters and JSX", async () => {
      const { status, output } = await lintSource(
        `import { useState } from "react";
      import { useToast } from "~/contexts/ToastContext";
      export function Probe({ load }: { load: () => Promise<void> }) {
        const toast = useToast();
        const [error, setError] = useState<string | null>(null);
        const [state, setState] = useState({ error: "" });
        async function run() {
          try {
            await load();
          } catch (err) {
            toast.error(err instanceof Error ? err.message : "Fehler");
            setError(\`Fehler: \${(err as Error).message}\`);
            setState((prev) => ({ ...prev, error: (err as Error).message }));
          }
        }
        void run;
        void state;
        return <p>{error}</p>;
      }
      export function Shown({ error }: { error: Error }) {
        return <p title={error.message}>{error.message}</p>;
      }`,
      );

      expect(status).toBe(1);
      expect(count(output, "no-error-text-display")).toBe(5);
    });

    it("follows the text through a const copy and a helper", async () => {
      const { status, output } = await lintSource(
        `import { useState } from "react";
      function describe(cause: unknown) {
        return cause instanceof Error ? cause.message : "Unbekannt";
      }
      export function Probe({ load }: { load: () => Promise<void> }) {
        const [, setNotice] = useState("");
        async function run() {
          try {
            await load();
          } catch (e) {
            const text = e instanceof Error ? e.message : "";
            setNotice(text);
            setNotice(describe(e));
          }
        }
        void run;
        return null;
      }`,
      );

      expect(status).toBe(1);
      expect(count(output, "no-error-text-display")).toBe(2);
    });

    it("accepts logging, rethrowing and values from the display path", async () => {
      const { status, output } = await lintSource(
        `import { createLogger } from "~/lib/logger";
      import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
      import { FormErrorAlert } from "~/components/ui/form-error-alert";
      import { Alert } from "~/components/ui/alert";
      const logger = createLogger({ component: "Probe" });
      function validate(value: string) {
        return value ? null : { message: "Bitte einen Namen eingeben." };
      }
      export function Probe({ value, load }: { value: string; load: () => Promise<void> }) {
        const form = useApiFormError();
        const load2 = useApiLoadError();
        const { error: loadError } = load2;
        const result = validate(value);
        async function run() {
          try {
            await load();
          } catch (err) {
            logger.error("probe_failed", {
              error: err instanceof Error ? err.message : String(err),
            });
            await form.show(err);
            throw new Error(\`wrapped: \${(err as Error).message}\`, { cause: err });
          }
        }
        void run;
        return (
          <>
            <FormErrorAlert message={form.error?.message ?? ""} />
            <p>{loadError?.message}</p>
            {result ? <Alert type="warning" message={result.message} /> : null}
          </>
        );
      }`,
      );

      expect(status).toBe(0);
      expect(output).not.toContain("error-path");
    });
  });

  describe("error-path/no-error-text-display: the caught value", () => {
    it("rejects a caught value turned into text or shown as it is", async () => {
      const { status, output } = await lintSource(
        `import { useState } from "react";
      import { useToast } from "~/contexts/ToastContext";
      export function Probe({ load }: { load: () => Promise<void> }) {
        const toast = useToast();
        const [notice, setNotice] = useState("");
        const [shown, setShown] = useState<unknown>(null);
        async function run() {
          try {
            await load();
          } catch (e) {
            toast.error(String(e));
            setNotice(\`Fehler: \${e}\`);
            setNotice(e instanceof Error ? e.toString() : "");
            toast.error(e as string);
            setShown(e);
          }
        }
        void load().catch((failure) => setNotice(String(failure)));
        void run;
        void shown;
        return <p>{notice}</p>;
      }
      export function Inline({ load }: { load: () => void }) {
        try {
          load();
          return null;
        } catch (e) {
          return <p title={e as string}>{e as string}</p>;
        }
      }`,
      );

      expect(status).toBe(1);
      expect(count(output, "no-error-text-display")).toBe(7);
    });

    it("accepts the caught value on the path, in logs and in state", async () => {
      const { status, output } = await lintSource(
        `import { useState } from "react";
      import { createLogger } from "~/lib/logger";
      import { useApiErrorDisplay } from "~/contexts/ToastContext";
      const logger = createLogger({ component: "Probe" });
      export function Probe({ load, error }: { load: () => Promise<void>; error: string }) {
        const { show } = useApiErrorDisplay();
        const [, setFailure] = useState<unknown>(null);
        async function run() {
          try {
            await load();
          } catch (e) {
            logger.error("probe_failed", { error: String(e), text: \`\${e}\` });
            setFailure(e);
            await show(e, { object: "die Probe" });
          }
        }
        void run;
        return <p className="text-sm">{error}</p>;
      }`,
      );

      expect(status).toBe(0);
      expect(output).not.toContain("error-path");
    });
  });

  describe("error-path/no-hand-rolled-alert", () => {
    it("rejects a hand-written role=alert, also as a conditional branch", async () => {
      const { status, output } = await lintSource(
        `export function Probe({ data, live }: { data: { error: string }; live: boolean }) {
        return (
          <>
            <div role="alert">{data.error}</div>
            <span role={live ? "alert" : undefined}>Bitte prüfen.</span>
          </>
        );
      }`,
      );

      expect(status).toBe(1);
      expect(count(output, "no-hand-rolled-alert")).toBe(2);
    });

    it("accepts the kit, other roles and component props", async () => {
      const kit = await lintSource(
        `export function Input({ error }: { error?: string }) {
        return error ? <p role="alert">{error}</p> : null;
      }`,
        "src/components/ui/input.tsx",
      );
      const other = await lintSource(
        `function Banner(props: { role: string; children: string }) {
        return <section aria-label={props.role}>{props.children}</section>;
      }
      export function Probe() {
        return (
          <>
            <p role="status">Gespeichert.</p>
            <Banner role="alert">Hinweis</Banner>
          </>
        );
      }`,
      );

      expect(kit.output).not.toContain("error-path");
      expect(other.output).not.toContain("error-path");
    });
  });

  describe("error-path/no-raw-error-toast", () => {
    it("rejects an error toast by hand, also a literal German one", async () => {
      const { status, output } = await lintSource(
        `import { useToast } from "~/contexts/ToastContext";
      export function Probe({ save }: { save: () => Promise<void> }) {
        const toast = useToast();
        const api = useToast();
        const { error: toastError, success } = useToast();
        async function run() {
          try {
            await save();
            success("Gespeichert.");
          } catch {
            toast.error("Speichern fehlgeschlagen");
            api.error("Speichern fehlgeschlagen");
            toastError("Speichern fehlgeschlagen");
            useToast().error("Speichern fehlgeschlagen");
          }
        }
        void run;
        return null;
      }`,
      );

      expect(status).toBe(1);
      // toast.error, api.error, the destructured error, useToast().error;
      // the call through the alias counts once, at its destructuring.
      expect(count(output, "no-raw-error-toast")).toBe(4);
    });

    it("accepts the display path, warnings and ToastContext itself", async () => {
      const probe = await lintSource(
        `import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
      export function Probe({ copy, save }: { copy: () => Promise<boolean>; save: () => Promise<void> }) {
        const toast = useToast();
        const { show, showMessage } = useApiErrorDisplay();
        async function run() {
          try {
            await save();
          } catch (error) {
            await show(error, { object: "die Probe" });
          }
          if (!(await copy())) showMessage("Der Link konnte nicht kopiert werden.");
          toast.warning("Die Auswahl hat sich geändert.");
        }
        void run;
        return null;
      }`,
      );
      const context = await lintSource(
        `export function useApiErrorDisplay(toast: { error: (m: string) => void }) {
        return { showMessage: (message: string) => toast.error(message) };
      }`,
        "src/contexts/ToastContext.tsx",
      );

      expect(probe.output).not.toContain("error-path");
      expect(context.output).not.toContain("error-path");
    });
  });

  describe("error-path/no-error-text-match", () => {
    it("rejects string matching on an error's text", async () => {
      const methodChecks = MATCH_METHODS.flatMap((method) => [
        `if (err.message.${method}("(404)")) return "missing";`,
        `if (errorText.${method}("Konflikt")) return "conflict";`,
      ]);
      const { status, output } = await lintSource(
        `export function classify(err: unknown, errorText: string) {
        if (!(err instanceof Error)) return "other";
        const msg = err.message;
        ${methodChecks.join("\n        ")}
        if (err.message.toLowerCase().startsWith("forbidden")) return "denied";
        if (/timed out/i.test(err.message)) return "slow";
        if (msg.endsWith("!")) return "loud";
        if (err.message === "No authentication token available") return "auth";
        return "other";
      }`,
        "src/lib/probe.ts",
      );

      expect(status).toBe(1);
      expect(count(output, "no-error-text-match")).toBe(
        methodChecks.length + 4,
      );
    });

    it("accepts decisions on the code, the status and unrelated text", async () => {
      const unrelatedChecks = MATCH_METHODS.flatMap((method) => [
        `if (warning.message.${method}("Frist")) return "deadline";`,
        `if (codes.${method}("general.input")) return "input";`,
      ]);
      const { status, output } = await lintSource(
        `import { ApiError } from "~/lib/api-error";
      export function classify(err: unknown, warning: { message: string }, codes: string[]) {
        if (err instanceof ApiError && err.code === "general.permission") return "denied";
        if (err instanceof ApiError && err.status === 404) return "missing";
        ${unrelatedChecks.join("\n        ")}
        return "other";
      }`,
        "src/lib/probe.ts",
      );

      expect(status).toBe(0);
      expect(output).not.toContain("error-path");
    });
  });
});

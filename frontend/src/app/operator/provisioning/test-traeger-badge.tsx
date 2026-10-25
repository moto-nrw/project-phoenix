import { StatusBadge } from "~/components/ui/status-badge";

/** Marks a test Träger with invented sample data (#4658) in the operator portal. */
export function TestTraegerBadge() {
  return (
    <StatusBadge
      label="Test-Träger"
      tone="orange"
      title="Testsystem mit erfundenen Beispieldaten"
    />
  );
}

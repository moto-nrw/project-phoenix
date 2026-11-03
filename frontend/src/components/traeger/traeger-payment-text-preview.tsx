import { SectionCard } from "~/components/ui/section-card";
import { paymentTextPreview } from "~/lib/traeger-payment-text";

interface TraegerPaymentTextPreviewProps {
  /** The saved payment text of the Träger; empty keeps the built-in one. */
  readonly text: string;
  /** Whether the Träger asks for the debtor number at the confirmation. */
  readonly asksDebtorNumber: boolean;
}

/**
 * The payment sentence of the confirmation letter with example values
 * (#5622), bold as the letter prints it. A display only: the text is
 * changed in the field above.
 */
export function TraegerPaymentTextPreview({
  text,
  asksDebtorNumber,
}: TraegerPaymentTextPreviewProps) {
  return (
    <SectionCard
      title="Vorschau des Zahlungstexts"
      description="So steht der Text im Schreiben, mit Beispielwerten. Bei 0 € steht dort: „Sie müssen für diese Betreuung nichts bezahlen.“"
    >
      <figure aria-label="Vorschau des Zahlungstexts">
        <p className="font-semibold wrap-anywhere whitespace-pre-line text-gray-900">
          {paymentTextPreview(text, asksDebtorNumber)}
        </p>
      </figure>
    </SectionCard>
  );
}

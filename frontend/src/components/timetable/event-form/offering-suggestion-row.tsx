"use client";

import { Button } from "~/components/ui/button";
import type { OfferingSuggestion } from "~/lib/offering-suggestion";

/**
 * Offers to take the children from the offering whose bookings match the
 * planned list (#5104). A Klasse brings every child of the class; this line
 * names the narrower choice and switches to it in one step.
 */
export function OfferingSuggestionRow({
  suggestion,
  onApply,
}: Readonly<{
  suggestion: OfferingSuggestion;
  onApply: (suggestion: OfferingSuggestion) => void;
}>) {
  return (
    <div className="mt-2 flex flex-col gap-2 rounded-md border border-gray-200 bg-gray-50 p-3">
      <p className="text-xs text-gray-700">
        {`Sollen nur die für „${suggestion.offering.name}“ angemeldeten Kinder kommen? Dann verbinden Sie den Termin mit dem Angebot.`}
      </p>
      <Button
        type="button"
        variant="outline"
        size="compact"
        className="self-start"
        onClick={() => onApply(suggestion)}
      >
        Angebot übernehmen
      </Button>
    </div>
  );
}

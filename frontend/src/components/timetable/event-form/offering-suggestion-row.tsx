"use client";

import { Button } from "~/components/ui/button";
import type { OfferingSuggestion } from "~/lib/offering-suggestion";

/**
 * Offers to take the children from the offering whose bookings match the
 * planned list (#5104). A Klasse brings every child of the class; this line
 * names the narrower choice and switches to it in one step. When several
 * offerings fit equally (#5138), it names each as an equal choice and
 * preselects none.
 */
export function OfferingSuggestionRow({
  suggestion,
  onApply,
}: Readonly<{
  suggestion: OfferingSuggestion;
  onApply: (offeringId: string) => void;
}>) {
  const choices = suggestion.ambiguous ? suggestion.candidates : [];
  return (
    <div className="mt-2 flex flex-col gap-2 rounded-md border border-gray-200 bg-gray-50 p-3">
      <p className="text-xs text-gray-700">
        {choices.length > 1
          ? "Sollen nur Kinder kommen, die für ein Angebot angemeldet sind? Dazu passen mehrere Angebote. Wählen Sie eins."
          : `Sollen nur die für „${suggestion.offering.name}“ angemeldeten Kinder kommen? Dann verbinden Sie den Termin mit dem Angebot.`}
      </p>
      {choices.length > 1 ? (
        <div className="flex flex-wrap gap-2">
          {choices.map((choice) => (
            <Button
              key={choice.id}
              type="button"
              variant="outline"
              size="compact"
              onClick={() => onApply(choice.id)}
            >
              {`„${choice.name}“ übernehmen`}
            </Button>
          ))}
        </div>
      ) : (
        <Button
          type="button"
          variant="outline"
          size="compact"
          className="self-start"
          onClick={() => onApply(suggestion.offering.id)}
        >
          Angebot übernehmen
        </Button>
      )}
    </div>
  );
}

"use client";

import { Button } from "~/components/ui/button";
import { InsetPanel } from "~/components/ui/inset-panel";
import type { OfferingSuggestion } from "~/lib/offering-suggestion";
import { offeringNamesFor, offeringSetNames } from "~/lib/roster-deviation";

// Übernehmen ändert nur das Formular; gespeichert wird mit dem Termin.
function applyTooltip(offeringCount: number): string {
  return offeringCount > 1
    ? "Verbindet den Termin mit diesen Angeboten und nimmt deren Kinder."
    : "Verbindet den Termin mit dem Angebot und nimmt dessen Kinder.";
}

/**
 * Offers to take the children from the offerings whose bookings match the
 * planned list (#5104). A Klasse brings every child of the class; this line
 * names the narrower choice and switches to it in one step. Several
 * offerings can explain the list together (#5139), for example Randstunde
 * and Ganztag; they are taken over together. When another offering fits as
 * well (#5138), it names each as an equal choice and preselects none.
 */
export function OfferingSuggestionRow({
  suggestion,
  onApply,
}: Readonly<{
  suggestion: OfferingSuggestion;
  onApply: (offeringIds: string[]) => void;
}>) {
  const choices = suggestion.ambiguous
    ? suggestion.combinations?.length
      ? suggestion.combinations
      : suggestion.candidates.map((offering) => [offering])
    : [];
  const combinedChoices = choices.some((choice) => choice.length > 1);
  return (
    <InsetPanel framed className="mt-2 flex flex-col gap-2">
      <p className="text-xs text-gray-700">
        {choices.length > 1
          ? combinedChoices
            ? "Mehrere Angebotskombinationen passen zur Kinderliste. Wählen Sie eine Kombination."
            : "Sollen nur Kinder kommen, die für ein Angebot angemeldet sind? Dazu passen mehrere Angebote. Wählen Sie eins."
          : suggestion.offerings.length > 1
            ? `Sollen nur die für ${offeringNamesFor(suggestion.offerings)} angemeldeten Kinder kommen? Dann verbinden Sie den Termin mit diesen Angeboten.`
            : `Sollen nur die für ${offeringNamesFor(suggestion.offerings)} angemeldeten Kinder kommen? Dann verbinden Sie den Termin mit dem Angebot.`}
      </p>
      {choices.length > 1 ? (
        <div className="flex flex-wrap gap-2">
          {choices.map((choice) => (
            <Button
              key={choice.map((offering) => offering.id).join(",")}
              type="button"
              variant="outline"
              size="md"
              className="max-w-full text-left break-words whitespace-normal"
              onClick={() => onApply(choice.map((offering) => offering.id))}
              tooltip={applyTooltip(choice.length)}
            >
              {`${offeringSetNames(choice)} übernehmen`}
            </Button>
          ))}
        </div>
      ) : (
        <Button
          type="button"
          variant="outline"
          size="md"
          className="self-start"
          onClick={() =>
            onApply(suggestion.offerings.map((offering) => offering.id))
          }
          tooltip={applyTooltip(suggestion.offerings.length)}
        >
          {suggestion.offerings.length > 1
            ? "Angebote übernehmen"
            : "Angebot übernehmen"}
        </Button>
      )}
    </InsetPanel>
  );
}

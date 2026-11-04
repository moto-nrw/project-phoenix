package api

// seedEnrollmentFormFields are the questions of the demo form "Anmeldeformular
// Ganztag". Questions about the child are asked per child (#4947, #5027).
func seedEnrollmentFormFields(attachments []map[string]string) []map[string]any {
	const allergies = "Allergien und Unverträglichkeiten"
	const swimming = "Mein Kind darf schwimmen."
	swimmingTranslations := seedTranslations("label", swimming, map[string]string{
		"en": "My child is allowed to swim.",
		"ru": "Моему ребёнку разрешено плавать.",
		"uk": "Моїй дитині дозволено плавати.",
	})
	// One translation of an older wording, so the editor shows a text that
	// needs checking and the form falls back to German for that language.
	swimmingTranslations["pl"] = map[string]any{"label": map[string]string{
		"text": "Moje dziecko umie pływać.", "source": "Mein Kind kann schwimmen.",
	}}
	return append([]map[string]any{
		{"key": "ogs_information", "type": "information", "label": "Hinweise zur OGS", "content": "Hier finden Sie Informationen zum Schulstart und zum Abholen.", "sort_order": 0, "attachments": attachments},
		{
			"key": "allergies", "label": allergies, "type": "textarea", "sort_order": 10, "applies_to_child": true,
			"translations": seedTranslations("label", allergies, map[string]string{
				"en": "Allergies and intolerances",
				"ru": "Аллергии и непереносимости",
				"uk": "Алергії та непереносимості",
			}),
		},
		{
			"key": "swimming_permission", "label": swimming, "type": "boolean", "sort_order": 20, "applies_to_child": true,
			"translations": swimmingTranslations,
		},
	}, seedIncomeDeclarationFields()...)
}

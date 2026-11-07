package api

// occupancyDemoCareOfferings complete the demo care offerings for the
// Träger's occupancy (#5977): with OGS Ganztag and Kurzbetreuung, which have
// free places, it shows every state of a care form. The Spätbetreuung is full
// (seedEnrollmentCourseBookings), the Betreuung bis 15 Uhr runs without a
// booking.
func occupancyDemoCareOfferings() []seedCareOffering {
	return []seedCareOffering{
		{key: "ogs-15-uhr", name: "Betreuung bis 15 Uhr", description: "Neu in diesem Schuljahr", daysMode: "parent_choice", days: []string{"mon", "tue", "wed", "thu", "fri"}, lunch: true, price: 12000, capacity: intPtr(12), sort: 25, countsAsCare: true, pickupTime: "15:00", group: "Betreuungszeit"},
		{key: "spaetbetreuung", name: "Spätbetreuung", description: "Betreuung bis 17 Uhr", daysMode: "fixed", days: []string{"mon", "tue", "wed", "thu", "fri"}, price: 4000, capacity: intPtr(4), sort: 26, countsAsCare: true, pickupTime: "17:00"},
	}
}

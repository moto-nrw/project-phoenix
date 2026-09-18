package departure

// Plan is a child's departure plan as one write carries it: the unified
// per-weekday mode set plus the three legacy projections clients still send.
// A nil field means "not supplied", which is what keeps an update that touches
// unrelated columns from rewriting the stored plan.
//
// The rules below are the owner's, not a persistence detail: which of the four
// representations wins, what counts as an intentional change, and how a legacy
// per-day map merges into the mode set decide what a school's Stammdaten
// actually say about how a child goes home.
type Plan struct {
	AllowedDepartureModes AllowedDepartureModes
	DepartureDays         DepartureDays
	BusDays               BusDays
	PickupDays            PickupDays
	// PickupStatus is the oldest projection of all: one string for the whole
	// week. It only ever stands in for PickupDays.
	PickupStatus *string
}

// Touched reports whether this write carries a departure plan at all. Every
// resolution step keys off it: a caller that loaded no plan leaves all fields
// nil and must not have the stored plan rewritten.
func (p Plan) Touched() bool {
	return p.AllowedDepartureModes != nil ||
		p.DepartureDays != nil ||
		p.BusDays != nil ||
		p.PickupDays != nil ||
		p.PickupStatus != nil
}

// Normalized is the plan with every projection normalized, which is the shape
// the stored side is always compared in.
func (p Plan) Normalized() Plan {
	return Plan{
		AllowedDepartureModes: p.AllowedDepartureModes.Normalize(),
		DepartureDays:         p.DepartureDays.Normalize(),
		BusDays:               p.BusDays.Normalize(),
		PickupDays:            p.PickupDays.Normalize(),
		PickupStatus:          p.PickupStatus,
	}
}

// Effective resolves the three stored projections of a row into the one plan
// that is actually in effect. Every read hydrates through it, so a caller never
// has to know which column a given school's data happens to live in.
//
// Precedence: the mode set wins when it says anything at all; then
// departure_days, which is authoritative as soon as it carries a non-alone day.
// An empty departure_days cannot distinguish "walks alone every day" from "not
// backfilled yet" — a row written straight to bus_days would read as no plan —
// so the legacy maps answer last. For a genuinely all-alone child every branch
// gives the same result.
//
// PickupStatus is carried through untouched: it is an input projection, and
// nothing derives the stored plan from it at read time.
func (p Plan) Effective() Plan {
	if allowed := p.AllowedDepartureModes.Normalize(); allowed.HasAny() {
		return Plan{
			AllowedDepartureModes: allowed,
			DepartureDays:         allowed.DepartureDays(),
			BusDays:               allowed.BusDays(),
			PickupDays:            allowed.PickupDays(),
			PickupStatus:          p.PickupStatus,
		}
	}
	if days := p.DepartureDays.Normalize(); days.HasAny() {
		return Plan{
			AllowedDepartureModes: AllowedDepartureModesFromDeparture(days),
			DepartureDays:         days,
			BusDays:               days.BusDays(),
			PickupDays:            days.PickupDays(),
			PickupStatus:          p.PickupStatus,
		}
	}
	bus, pickup := p.BusDays.Normalize(), p.PickupDays.Normalize()
	return Plan{
		AllowedDepartureModes: AllowedDepartureModesFromLegacy(bus, pickup),
		DepartureDays:         DepartureDaysFromLegacy(bus, pickup),
		BusDays:               bus,
		PickupDays:            pickup,
		PickupStatus:          p.PickupStatus,
	}
}

// Rebase replaces every field that still carries exactly what the read
// hydrated with the state just read under the row lock.
//
// A row lock makes the reads agree with the write, but only for the stored
// side: the in-memory plan is whatever the caller loaded, possibly long before
// a concurrent companion edit committed. A caller that never touches the plan
// (a sickness auto-clear, a status day, an import) still carries all four
// hydrated fields, so Touched is true and Resolve would read the difference to
// the now-newer stored plan as an intentional change — reverting the committed
// edit, trimming its fresh edges, or refusing the unrelated update. Rebasing
// the untouched fields makes such a write a no-op re-persist instead.
//
// Fields the caller genuinely changed differ from the baseline and are left
// alone, so an intentional plan write still wins over the stored state (last
// writer wins, as before — this only stops a NON-writer from winning). Without
// a baseline (the caller built the plan itself, or the read predates the
// snapshot) nothing is rebased and the supplied fields are taken at face value.
func (p Plan) Rebase(baseline, current *Plan) Plan {
	if baseline == nil || current == nil {
		return p
	}
	if p.AllowedDepartureModes != nil && AllowedModesEqual(p.AllowedDepartureModes, baseline.AllowedDepartureModes) {
		p.AllowedDepartureModes = current.AllowedDepartureModes
	}
	if p.DepartureDays != nil && DaysEqual(p.DepartureDays, baseline.DepartureDays) {
		p.DepartureDays = current.DepartureDays
	}
	if p.BusDays != nil && BusEqual(p.BusDays, baseline.BusDays) {
		p.BusDays = current.BusDays
	}
	if p.PickupDays != nil && PickupEqual(p.PickupDays, baseline.PickupDays) {
		p.PickupDays = current.PickupDays
	}
	// PickupStatus needs no rebase: a hydrated read always leaves PickupDays
	// non-nil, and effectivePickupDays only falls back to the legacy status
	// string when PickupDays is nil.
	return p
}

// Align returns the plan with every projection set to the one that will
// actually be persisted, so a validation runs against the effective plan
// rather than a transient mix of a stale hydrated mode set and a freshly-set
// legacy map. Without it a legacy client that removes the accompanied mode via
// DepartureDays while clearing the "mit wem" note is rejected against the stale
// accompanied mode it never sent.
//
// An untouched plan is returned unchanged; there is nothing to align.
func (p Plan) Align(current *Plan) Plan {
	if !p.Touched() {
		return p
	}
	allowed := p.Resolve(current)
	p.AllowedDepartureModes = allowed
	p.DepartureDays = allowed.DepartureDays()
	p.BusDays = allowed.BusDays()
	p.PickupDays = allowed.PickupDays()
	return p
}

// Resolve answers the mode set that is in effect after this write, given the
// plan the row currently holds (nil on a create).
//
// The supplied mode set wins unless the caller also moved one of the legacy
// projections, because a legacy client sends its change there and carries the
// mode set along unchanged from its read.
func (p Plan) Resolve(current *Plan) AllowedDepartureModes {
	if p.AllowedDepartureModes != nil && p.preferSuppliedModes(current) {
		return p.AllowedDepartureModes.Normalize()
	}
	if current == nil {
		if p.DepartureDays != nil {
			return AllowedDepartureModesFromDeparture(p.DepartureDays).Normalize()
		}
		return AllowedDepartureModesFromLegacy(p.BusDays, p.effectivePickupDays()).Normalize()
	}

	pickup := p.effectivePickupDays()
	busChanged := p.BusDays != nil && !BusEqual(p.BusDays, current.BusDays)
	pickupChanged := pickup != nil && !PickupEqual(pickup, current.PickupDays)
	if busChanged || pickupChanged {
		return mergeLegacyModes(current.AllowedDepartureModes, p.BusDays, pickup, busChanged, pickupChanged)
	}

	if p.DepartureDays != nil && !DaysEqual(p.DepartureDays, current.DepartureDays) {
		return AllowedDepartureModesFromDeparture(p.DepartureDays).Normalize()
	}
	return current.AllowedDepartureModes.Normalize()
}

func (p Plan) preferSuppliedModes(current *Plan) bool {
	if current == nil {
		return true
	}
	if !AllowedModesEqual(p.AllowedDepartureModes, current.AllowedDepartureModes) {
		return true
	}
	pickup := p.effectivePickupDays()
	departureChanged := p.DepartureDays != nil && !DaysEqual(p.DepartureDays, current.DepartureDays)
	legacyChanged := (p.BusDays != nil && !BusEqual(p.BusDays, current.BusDays)) ||
		(pickup != nil && !PickupEqual(pickup, current.PickupDays))
	return !departureChanged && !legacyChanged
}

// effectivePickupDays is the per-day pickup map this write means, deriving it
// from the single legacy status string when no map was supplied.
func (p Plan) effectivePickupDays() PickupDays {
	if p.PickupDays != nil {
		return p.PickupDays
	}
	if p.PickupStatus != nil {
		return PickupDaysFromLegacyStatus(*p.PickupStatus)
	}
	return nil
}

// mergeLegacyModes folds a changed legacy map into the stored mode set per
// weekday, leaving the modes the caller said nothing about in place. A legacy
// client only ever speaks about bus or pickup, so overwriting the whole day
// would silently drop an accompanied or alone mode it never saw.
func mergeLegacyModes(current AllowedDepartureModes, bus BusDays, pickup PickupDays, busChanged, pickupChanged bool) AllowedDepartureModes {
	current = current.Normalize()
	out := AllowedDepartureModes{}
	for _, day := range PickupDayOrder {
		modes := map[DepartureMode]bool{}
		for _, mode := range current[day] {
			modes[mode] = true
		}
		if busChanged {
			modes[DepartureBus] = bus[day]
		}
		if pickupChanged {
			modes[DeparturePickup] = pickup[day]
		}
		for _, mode := range []DepartureMode{DepartureAlone, DepartureBus, DeparturePickup, DepartureAccompanied} {
			if modes[mode] {
				out[day] = append(out[day], mode)
			}
		}
	}
	return out.Normalize()
}

// AllowedModesEqual compares two mode sets weekday by weekday, normalizing
// both first so a differently ordered or differently spelled equivalent set
// does not read as a change.
func AllowedModesEqual(a, b AllowedDepartureModes) bool {
	a, b = a.Normalize(), b.Normalize()
	for _, day := range PickupDayOrder {
		left, right := a[day], b[day]
		if len(left) != len(right) {
			return false
		}
		for i := range left {
			if left[i] != right[i] {
				return false
			}
		}
	}
	return true
}

func DaysEqual(a, b DepartureDays) bool {
	a, b = a.Normalize(), b.Normalize()
	for _, day := range PickupDayOrder {
		if a.ModeFor(day) != b.ModeFor(day) {
			return false
		}
	}
	return true
}

func BusEqual(a, b BusDays) bool {
	a, b = a.Normalize(), b.Normalize()
	for _, day := range PickupDayOrder {
		if a[day] != b[day] {
			return false
		}
	}
	return true
}

func PickupEqual(a, b PickupDays) bool {
	a, b = a.Normalize(), b.Normalize()
	for _, day := range PickupDayOrder {
		if a[day] != b[day] {
			return false
		}
	}
	return true
}

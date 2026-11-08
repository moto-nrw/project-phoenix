package application

import (
	"slices"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The rule chain of the monthly care contribution (#4685, ADR 0164): income
// level, sibling, single parent, cap, in the order of ADR 0088 section 4.
// It is pure: the caller reads the rules, the evidences and the draft
// invoices of the month. It reuses the sibling discount and the ranking of
// the holiday care (ADR 0115): the first child is the one with the highest
// base price.

// careChainEntry is one invoice of the month as the chain sees it: the child
// it bills, the child's date of birth for a tie, and what is left of each
// position. An issued invoice of an earlier run of the month takes part in
// the ranking and the cap but gets no reduction (issued).
type careChainEntry struct {
	invoice   *parentbilling.CareInvoice
	child     string
	birthDate *calendar.Date
	issued    bool
	remaining []int64
}

// careChainInput are the rules and evidences of the Träger the chain reads.
type careChainInput struct {
	month     calendar.Date
	rules     []parentbilling.ContributionRule
	evidences []parentbilling.CareEvidence
	// familyChildren is the confirmed number of children valid in the
	// month per family (#5918); a sibling rule reads it only when it counts
	// the family.
	familyChildren map[string]int
	// uncounted collects the families a rule that counts the family ranked
	// without a confirmed number valid in the month; nil collects nothing.
	uncounted map[string]bool
}

// applyCareChain runs the chain over the invoices of one month and writes
// every reduction and the total of each draft.
func applyCareChain(entries []*careChainEntry, in careChainInput) {
	rules := careRulesInForce(in.rules, in.month)
	for _, entry := range entries {
		entry.initialize()
	}
	for _, entry := range entries {
		entry.applyIncome(in, rules)
	}
	for _, rule := range rulesOfKind(rules, parentbilling.RuleSibling) {
		applyCareSiblingRule(entries, rule, in)
	}
	for _, entry := range entries {
		entry.applySingleParent(in, rules)
	}
	for _, rule := range rulesOfKind(rules, parentbilling.RuleCap) {
		applyCareCap(entries, rule)
	}
	for _, entry := range entries {
		entry.finish()
	}
}

// careRulesInForce are the care rules valid for the whole month. A care
// rule starts on the first of a month and ends on the last day of one.
func careRulesInForce(rules []parentbilling.ContributionRule, month calendar.Date) []*parentbilling.ContributionRule {
	valid := []*parentbilling.ContributionRule{}
	for i := range rules {
		rule := &rules[i]
		if rule.Service != parentbilling.ServiceCare || month.Before(rule.ValidFrom) ||
			(rule.ValidUntil != nil && rule.ValidUntil.Before(monthEnd(month))) {
			continue
		}
		valid = append(valid, rule)
	}
	return valid
}

func rulesOfKind(rules []*parentbilling.ContributionRule, kind string) []*parentbilling.ContributionRule {
	of := []*parentbilling.ContributionRule{}
	for _, rule := range rules {
		if rule.Kind == kind {
			of = append(of, rule)
		}
	}
	return of
}

func ruleByID(rules []*parentbilling.ContributionRule, id *int64) *parentbilling.ContributionRule {
	for _, rule := range rules {
		if id != nil && rule.ID == *id {
			return rule
		}
	}
	return nil
}

// careEvidenceInMonth is the evidence of a kind that holds for a contract in
// the month: the one valid on its first day, else the first that starts
// within it, like the version of a month (ADR 0158 E2). Evidences of one
// kind and contract never overlap.
func careEvidenceInMonth(evidences []parentbilling.CareEvidence, contractID int64, kind string, month calendar.Date) *parentbilling.CareEvidence {
	end := monthEnd(month)
	var found *parentbilling.CareEvidence
	for i := range evidences {
		evidence := &evidences[i]
		if evidence.ContractID != contractID || evidence.Kind != kind || evidence.ValidFrom.After(end) ||
			(evidence.ValidUntil != nil && evidence.ValidUntil.Before(month)) {
			continue
		}
		if found == nil || evidence.ValidFrom.Before(found.ValidFrom) {
			found = evidence
		}
	}
	return found
}

func appliesAt(schoolIDs []int64, entry *careChainEntry) bool {
	return slices.Contains(schoolIDs, entry.invoice.SchoolID)
}

// inScope reports whether a rule applies to a module: a rule without
// modules applies to all.
func inScope(rule *parentbilling.ContributionRule, offeringID int64) bool {
	return len(rule.CareOfferingIDs) == 0 || slices.Contains(rule.CareOfferingIDs, offeringID)
}

// scoped is what is left of the positions the rule applies to.
func (e *careChainEntry) scoped(rule *parentbilling.ContributionRule) int64 {
	var sum int64
	for i, position := range e.invoice.Positions {
		if inScope(rule, position.CareOfferingID) {
			sum += e.remaining[i]
		}
	}
	return sum
}

// base is the base price of the positions the rule applies to, before any
// reduction: the sibling rule ranks by it (ADR 0115 point 3).
func (e *careChainEntry) base(rule *parentbilling.ContributionRule) int64 {
	var sum int64
	for _, position := range e.invoice.Positions {
		if inScope(rule, position.CareOfferingID) {
			sum += position.AmountCents
		}
	}
	return sum
}

// discountOf is what the rule takes from amount, rounded half up to whole
// cent and never more than amount: a share or a fixed amount like the
// holiday sibling rule, or the difference to a fixed end price.
func discountOf(amount int64, rule *parentbilling.ContributionRule) int64 {
	if amount <= 0 {
		return 0
	}
	if rule.PriceCents != nil {
		return max(amount-*rule.PriceCents, 0)
	}
	return siblingDiscount(amount, rule)
}

// reduce takes discount from the positions the rule applies to, in
// proportion to what is left of each (largest remainder, so the parts add up
// to the cent), and writes the reduction. Issued invoices keep their saved
// allocations and reductions.
func (e *careChainEntry) reduce(rule *parentbilling.ContributionRule, discount int64, rank, children *int) {
	if e.issued || discount <= 0 {
		return
	}
	left := e.scoped(rule)
	discount = min(discount, left)
	indices := []int{}
	numerators := []int64{}
	for i, position := range e.invoice.Positions {
		if inScope(rule, position.CareOfferingID) && e.remaining[i] > 0 {
			indices = append(indices, i)
			numerators = append(numerators, e.remaining[i]*discount)
		}
	}
	for k, part := range largestRemainder(numerators, left, discount) {
		e.remaining[indices[k]] -= part
	}
	e.invoice.Reductions = append(e.invoice.Reductions, parentbilling.CareReduction{
		Kind: rule.Kind, RuleID: rule.ID, Name: rule.Name, ChildRank: rank, FamilyChildren: children,
		DiscountBasisPoints: rule.DiscountBasisPoints, DiscountCents: rule.DiscountCents, PriceCents: rule.PriceCents,
		CapCents: rule.CapCents, CareOfferingIDs: slices.Clone(rule.CareOfferingIDs), AmountCents: -discount,
	})
}

// familyOf is the family of an invoice: its recipient's mail address,
// compared without case, like the families of ADR 0126 and #4691. An
// invoice without an address is a family of its own. #4689 replaces the key
// with the payer; the children the family states (#5918) use the same key.
func familyOf(entry *careChainEntry) string {
	if email := strings.ToLower(strings.TrimSpace(entry.invoice.Recipient.Email)); email != "" {
		return email
	}
	return "contract:" + strconv.FormatInt(entry.invoice.ContractID, 10)
}

// rankedCareChild is a child of a family as a rule counts it.
type rankedCareChild struct {
	key       string
	base      int64
	birthDate *calendar.Date
	entries   []*careChainEntry
}

// rankFamilies groups the entries by family and, in each family, by child,
// in the order of the sibling rule: the highest base price first, then the
// oldest, then the child key, so the order is the same on every run. The
// families come by their key.
func rankFamilies(entries []*careChainEntry, rule *parentbilling.ContributionRule) [][]*rankedCareChild {
	families := map[string]map[string]*rankedCareChild{}
	order := []string{}
	for _, entry := range entries {
		family := familyOf(entry)
		if families[family] == nil {
			families[family] = map[string]*rankedCareChild{}
			order = append(order, family)
		}
		child := families[family][entry.child]
		if child == nil {
			child = &rankedCareChild{key: entry.child, birthDate: entry.birthDate}
			families[family][entry.child] = child
		}
		child.base += entry.base(rule)
		child.entries = append(child.entries, entry)
	}
	slices.Sort(order)
	ranked := make([][]*rankedCareChild, 0, len(order))
	for _, family := range order {
		children := make([]*rankedCareChild, 0, len(families[family]))
		for _, child := range families[family] {
			children = append(children, child)
		}
		slices.SortFunc(children, compareCareChildren)
		ranked = append(ranked, children)
	}
	return ranked
}

func compareCareChildren(a, b *rankedCareChild) int {
	if a.base != b.base {
		return compareOrdered(b.base, a.base)
	}
	switch {
	case a.birthDate != nil && b.birthDate != nil && *a.birthDate != *b.birthDate:
		return a.birthDate.Compare(*b.birthDate)
	case a.birthDate != nil && b.birthDate == nil:
		return -1
	case a.birthDate == nil && b.birthDate != nil:
		return 1
	}
	return strings.Compare(a.key, b.key)
}

// countingSchools are the schools whose children a sibling rule counts:
// the chosen ones, else the rule's own schools.
func countingSchools(rule *parentbilling.ContributionRule) []int64 {
	if len(rule.CountSchoolIDs) > 0 {
		return rule.CountSchoolIDs
	}
	return rule.SchoolIDs
}

// applyCareSiblingRule ranks the children of every family that have a
// position of the rule at a counting school and reduces the invoices at the
// rule's schools of every child from the rule's rank on.
func applyCareSiblingRule(entries []*careChainEntry, rule *parentbilling.ContributionRule, in careChainInput) {
	counting := countingSchools(rule)
	counted := []*careChainEntry{}
	for _, entry := range entries {
		if appliesAt(counting, entry) && entry.base(rule) > 0 {
			counted = append(counted, entry)
		}
	}
	for _, children := range rankFamilies(counted, rule) {
		total, ahead := familySize(children, rule, in)
		for index, child := range children {
			rank := ahead + index + 1
			if rank < *rule.SiblingFromChild {
				continue
			}
			reduceCareSiblingChild(child, rule, rank, total)
		}
	}
}

// familySize is how many children a sibling rule counts in a family and how
// many of them, the siblings without a contract, come before its children
// with one (#5918). A rule that counts the family takes the confirmed number
// valid in the month, but never fewer than the children with a contract it
// counts; without a confirmed number it counts those children alone and
// notes the family as uncounted.
func familySize(children []*rankedCareChild, rule *parentbilling.ContributionRule, in careChainInput) (total, ahead int) {
	withContract := len(children)
	if !rule.CountsFamilyChildren {
		return withContract, 0
	}
	family := familyOf(children[0].entries[0])
	stated, ok := in.familyChildren[family]
	if !ok {
		if in.uncounted != nil && !strings.HasPrefix(family, "contract:") {
			in.uncounted[family] = true
		}
		return withContract, 0
	}
	total = max(stated, withContract)
	return total, total - withContract
}

// applyCareCap lowers what a family pays at the rule's schools to the cap:
// the excess comes off the last child of the sibling order first.
func applyCareCap(entries []*careChainEntry, rule *parentbilling.ContributionRule) {
	scope := []*careChainEntry{}
	for _, entry := range entries {
		if appliesAt(rule.SchoolIDs, entry) {
			scope = append(scope, entry)
		}
	}
	for _, children := range rankFamilies(scope, rule) {
		var paid int64
		for _, child := range children {
			for _, entry := range child.entries {
				paid += entry.scoped(rule)
			}
		}
		excess := paid - *rule.CapCents
		for c := len(children) - 1; c >= 0 && excess > 0; c-- {
			excess = children[c].takeCapExcess(rule, excess)
		}
	}
}

// reduceCareSiblingChild computes one discount per child, then shares it
// across eligible draft contracts by largest remainder. Issued contracts
// consume the fixed price or discount without being changed themselves.
func reduceCareSiblingChild(child *rankedCareChild, rule *parentbilling.ContributionRule, rank, children int) {
	var drafts []*careChainEntry
	var amounts []int64
	var draftTotal, issuedTotal, issuedDiscount int64
	for _, entry := range child.entries {
		if !appliesAt(rule.SchoolIDs, entry) {
			continue
		}
		if entry.issued {
			issuedTotal += entry.issuedAtSiblingStage(rule)
			issuedDiscount += entry.issuedSiblingDiscount(rule.ID)
		} else {
			drafts = append(drafts, entry)
			amounts = append(amounts, entry.scoped(rule))
			draftTotal += entry.scoped(rule)
		}
	}
	if draftTotal == 0 {
		return
	}
	discount := discountOf(draftTotal, rule)
	if rule.PriceCents != nil {
		discount = min(draftTotal, discountOf(draftTotal+issuedTotal, rule))
	} else if rule.DiscountCents != nil {
		discount = min(draftTotal, max(*rule.DiscountCents-issuedDiscount, 0))
	}
	for i := range amounts {
		amounts[i] *= discount
	}
	for i, part := range largestRemainder(amounts, draftTotal, discount) {
		drafts[i].reduce(rule, part, &rank, &children)
	}
}

func (e *careChainEntry) initialize() {
	e.remaining = make([]int64, len(e.invoice.Positions))
	for i, position := range e.invoice.Positions {
		e.remaining[i] = position.AmountCents
		if e.issued {
			e.remaining[i] = position.PayableCents
		}
	}
	if !e.issued {
		e.invoice.Reductions = []parentbilling.CareReduction{}
	}
}

func (e *careChainEntry) finish() {
	if e.issued {
		return
	}
	var total int64
	for i, left := range e.remaining {
		e.invoice.Positions[i].PayableCents = left
		total += left
	}
	e.invoice.TotalCents = total
}

// issuedAtSiblingStage is what an issued invoice billed for the rule's
// modules when the sibling rule ran: their base price less the income level
// and less this rule's discount. The income reduction is shared again over
// the positions of the modules its rule named (all without names), by base
// price and largest remainder, as the chain shared it when it issued the
// invoice; it never lands on a module its rule did not reduce. The
// single-parent reduction and the cap came later and do not count, so a
// later contract of the child never pays a fixed price twice.
func (e *careChainEntry) issuedAtSiblingStage(rule *parentbilling.ContributionRule) int64 {
	stage := e.base(rule)
	for _, reduction := range e.invoice.Reductions {
		if reduction.Kind != parentbilling.RuleIncomeLevel {
			continue
		}
		stage -= e.incomeOnScope(reduction, rule)
	}
	return max(stage-e.issuedSiblingDiscount(rule.ID), 0)
}

// incomeOnScope is the part of an issued income reduction that fell on the
// positions the sibling rule applies to.
func (e *careChainEntry) incomeOnScope(reduction parentbilling.CareReduction, rule *parentbilling.ContributionRule) int64 {
	reduced := &parentbilling.ContributionRule{CareOfferingIDs: reduction.CareOfferingIDs}
	var indices []int
	var numerators []int64
	var base int64
	discount := -reduction.AmountCents
	for i, position := range e.invoice.Positions {
		if inScope(reduced, position.CareOfferingID) && position.AmountCents > 0 {
			indices = append(indices, i)
			numerators = append(numerators, position.AmountCents*discount)
			base += position.AmountCents
		}
	}
	if base == 0 || discount <= 0 {
		return 0
	}
	var onScope int64
	for k, part := range largestRemainder(numerators, base, min(discount, base)) {
		if inScope(rule, e.invoice.Positions[indices[k]].CareOfferingID) {
			onScope += part
		}
	}
	return onScope
}

func (e *careChainEntry) issuedSiblingDiscount(ruleID int64) int64 {
	var discount int64
	for _, reduction := range e.invoice.Reductions {
		if reduction.Kind == parentbilling.RuleSibling && reduction.RuleID == ruleID {
			discount -= reduction.AmountCents
		}
	}
	return discount
}

func (e *careChainEntry) applyIncome(in careChainInput, rules []*parentbilling.ContributionRule) {
	if e.issued {
		return
	}
	evidence := careEvidenceInMonth(in.evidences, e.invoice.ContractID, parentbilling.EvidenceIncomeLevel, in.month)
	if evidence == nil {
		return
	}
	if rule := ruleByID(rules, evidence.RuleID); rule != nil && rule.Kind == parentbilling.RuleIncomeLevel && appliesAt(rule.SchoolIDs, e) {
		e.reduce(rule, discountOf(e.scoped(rule), rule), nil, nil)
	}
}

func (e *careChainEntry) applySingleParent(in careChainInput, rules []*parentbilling.ContributionRule) {
	if e.issued || careEvidenceInMonth(in.evidences, e.invoice.ContractID, parentbilling.EvidenceSingleParent, in.month) == nil {
		return
	}
	for _, rule := range rulesOfKind(rules, parentbilling.RuleSingleParent) {
		if appliesAt(rule.SchoolIDs, e) {
			e.reduce(rule, discountOf(e.scoped(rule), rule), nil, nil)
		}
	}
}

func (c *rankedCareChild) takeCapExcess(rule *parentbilling.ContributionRule, excess int64) int64 {
	for i := len(c.entries) - 1; i >= 0 && excess > 0; i-- {
		entry := c.entries[i]
		if entry.issued {
			continue
		}
		take := min(excess, entry.scoped(rule))
		entry.reduce(rule, take, nil, nil)
		excess -= take
	}
	return excess
}

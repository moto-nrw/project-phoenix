package architecture

import (
	"go/ast"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// parentChildWriteGuardViolations is the rule of #6529: every parent write
// for an existing child runs in a school unit opened by one of the guarded
// openers of the Parent Portal workflow, whose write argument runs only after
// guardChildWrite locked membership, child, profile and relationship and
// re-checked the permissions. The rule reads the flows' call graph, not
// names: from every exported method of the two flow services it follows the
// calls inside the two packages (closures and method values included) and
// reports every call of a port method the register below classifies as a
// write. The write argument of a guarded opener is not followed: whatever
// runs there is guarded. Every write reached another way is a violation,
// unless its entry is in the exact register of writes not bound to one child,
// with the reason. A port method the classification does not know is a
// violation as well, so a new port or a new method of a port is classified
// before it can be reached, and an exemption that reaches no write is stale.
func parentChildWriteGuardViolations(project, modulePath string, loaded []*packages.Package) []Violation {
	flows := parentFlowPackages(modulePath, loaded)
	if len(flows) == 0 {
		return nil
	}
	graph := newFlowGraph(flows)
	graph.project = project
	var violations []Violation
	reached := map[string]bool{}
	for _, root := range graph.roots() {
		violations = append(violations, graph.rootViolations(root, reached)...)
	}
	for name := range childWriteGuardExemptions {
		if !reached[name] {
			violations = append(violations, parentChildWriteViolation("parent-child-writes.stale-exemption", name, "no write", nil))
		}
	}
	return violations
}

func parentFlowPackages(modulePath string, loaded []*packages.Package) []*packages.Package {
	var flows []*packages.Package
	for _, pkg := range loaded {
		for _, path := range flowPackages {
			if pkg.PkgPath == modulePath+"/"+path {
				flows = append(flows, pkg)
			}
		}
	}
	return flows
}

// rootViolations reports the unclassified ports and the unguarded writes one
// entry reaches; reached records the entries that write at all.
func (g *flowGraph) rootViolations(root *types.Func, reached map[string]bool) []Violation {
	name := g.name(root)
	sinks, unknown := g.writesFrom(root)
	var violations []Violation
	for _, port := range unknown {
		violations = append(violations, parentChildWriteViolation("parent-child-writes.unclassified-port", name, port, g.location(root)))
	}
	if len(sinks) == 0 {
		return violations
	}
	reached[name] = true
	if strings.TrimSpace(childWriteGuardExemptions[name]) == "" {
		violations = append(violations, parentChildWriteViolation("parent-child-writes.unguarded", name, strings.Join(sinks, ","), g.location(root)))
	}
	return violations
}

func parentChildWriteViolation(rule, source, target string, locations []Location) Violation {
	return Violation{
		Scope: ScopeProduction, Rule: rule, Source: source, Target: target, Locations: locations,
		Detail: "parent writes for an existing child run behind the child write guard (#6529): open the unit with WithChildWrite or WithPermittedChildForWrite, or classify the port",
	}
}

var flowPackages = []string{"workflows/parentportal/internal/care", "workflows/parentportal/internal/messaging"}

// guardedOpeners open a school unit behind guardChildWrite; the value is the
// index of their write argument.
var guardedOpeners = map[string]int{
	"care.Service.WithChildWrite":                        5,
	"care.Service.WithPermittedChildForWrite":            5,
	"care.Service.WithParentConferenceBooking":           3,
	"messaging.ChildResolver.WithChildWrite":             5,
	"messaging.ChildResolver.WithPermittedChildForWrite": 5,
}

// childWriteGuardExemptions are the entries that write without a child of
// the account being the subject of the write. Each one names why.
var childWriteGuardExemptions = map[string]string{
	"care.Service.UpdatePortalLocale":                  "the account's own portal language in every school profile; no child is written",
	"care.Service.StateFamilyChildren":                 "the family's number of children on a Träger contract, authorized per contract in the Träger unit",
	"care.Service.DeclareContractNotice":               "withdrawal or termination of a Träger contract, authorized per contract in the Träger unit",
	"care.Service.SignContract":                        "signature of a Träger contract, authorized per contract in the Träger unit",
	"care.Service.GrantSEPAMandate":                    "the payer's SEPA mandate on a Träger contract",
	"care.Service.RevokeSEPAMandate":                   "the payer's SEPA mandate on a Träger contract",
	"care.Service.HolidayRegistrationFamily":           "records the prefill read of the family's children in the data access log and states a holiday wish; a read with its audit, no change of a child",
	"care.Service.ContractSigningView":                 "reads a Träger contract; it shares contractSigning with SignContract, which writes only when a signature is given",
	"care.Service.NotifyBillingDocument":               "push of a Träger document to its accounts, run by Delivery, not by a parent request",
	"care.Service.NotifyContractDocument":              "push of a Träger document to its accounts, run by Delivery, not by a parent request",
	"care.Service.NotifyTraegerDocumentRelease":        "push of a Träger release to its accounts, run by Delivery, not by a parent request",
	"care.Service.NotifyTraegerDocumentReleaseAccount": "push of a Träger release to one account, run by Delivery, not by a parent request",
	"messaging.Service.GetChildConversation":           "moves the reader's own read cursor while it reads the conversation (care.ReadReceiptOnly)",
	"messaging.Service.GetGroupThread":                 "moves the reader's own read cursor while it reads the group thread",
	"messaging.Service.MarkAnnouncementRead":           "the account's read receipt of an announcement of its audience",
	"messaging.Service.AcknowledgeAnnouncement":        "the account's acknowledgement of an announcement of its audience",
	"messaging.Service.SubmitAnnouncementSurvey":       "one survey answer per account of the announcement's audience",
	"messaging.Service.RespondToAnnouncement":          "poll answer for a child: HoldAnswerPermission share-locks relationship, profile and membership in the answer's own transaction",
	"messaging.Service.ShareRequestInTx":               "delegated: care calls it through RequestSharer, a write sink of care, only inside its guarded units",
}

// portClasses classifies the port methods the flows call: a whole port by
// its name, or one method. Only "write" is a write; "read" covers reads,
// locks, runtime and pure policy; "notice" covers losable after-commit
// notices, such as the chat pill that mirrors a committed self-service write
// in its own detached transaction.
var portClasses = map[string]string{
	"care.AbsenceNotifier":                                      "notice",
	"care.ArrivalSchedules":                                     "read",
	"care.AttendanceReader":                                     "read",
	"care.BatchSettingsService":                                 "read",
	"care.BillingDocumentNotifier":                              "write",
	"care.BillingDocuments.BillingDocumentLetter":               "read",
	"care.BillingDocuments.ListBillingDocuments":                "read",
	"care.BillingDocuments.FamilyChildren":                      "read",
	"care.BillingDocuments.StateFamilyChildren":                 "write",
	"care.CareExceptions":                                       "read",
	"care.CareOfferingReads":                                    "read",
	"care.CarePeriodReads":                                      "read",
	"care.CarePlanPolicy":                                       "read",
	"care.ChildDataAccessLog":                                   "write",
	"care.ChildEvents":                                          "notice",
	"care.ChildReads":                                           "read",
	"care.ContractDocuments.ContractDocumentFile":               "read",
	"care.ContractDocuments.ContractSigningView":                "read",
	"care.ContractDocuments.FamilyEmail":                        "read",
	"care.ContractDocuments.ListContractDocuments":              "read",
	"care.ContractDocuments.SignContract":                       "write",
	"care.ContractInboxNotices":                                 "read",
	"care.ContractNotices.ContractNoticeOptions":                "read",
	"care.ContractNotices.DeclareContractNotice":                "write",
	"care.DataRequestReads":                                     "read",
	"care.DeparturePolicy":                                      "read",
	"care.EnrollablePhaseReads":                                 "read",
	"care.EnrollmentRequestReads":                               "read",
	"care.EnrollmentSettings":                                   "read",
	"care.ExcusedAbsenceRequests.EditRequest":                   "write",
	"care.ExcusedAbsenceRequests.ListForStudent":                "read",
	"care.ExcusedAbsenceRequests.Submit":                        "write",
	"care.GuardianAbsenceReports":                               "write",
	"care.GuardianAccess":                                       "write",
	"care.GuardianChangeLog":                                    "write",
	"care.GuardianChangeSource":                                 "write",
	"care.GuardianInvitationReads":                              "read",
	"care.GuardianPhoneReads":                                   "read",
	"care.GuardianPickupExceptions":                             "write",
	"care.GuardianPolicy":                                       "read",
	"care.GuardianProfileReads":                                 "read",
	"care.GuardianRecords":                                      "write",
	"care.GuardianRecords.ListGuardianProfileTenants":           "read",
	"care.GuardianSource":                                       "write",
	"care.HolidayWishStatements":                                "write",
	"care.Locales":                                              "read",
	"care.MealPlan":                                             "read",
	"care.MealPlan.ClearParticipationForDay":                    "write",
	"care.MealPlan.ReplaceParticipationSchedule":                "write",
	"care.MealPlan.SetParticipationForDay":                      "write",
	"care.MealPlan.SetParticipationForWeek":                     "write",
	"care.MealPlanSource":                                       "read",
	"care.MealPlanSource.ClearParticipationForDay":              "write",
	"care.MealPlanSource.ReplaceParticipationSchedule":          "write",
	"care.MealPlanSource.SetParticipationForDay":                "write",
	"care.MealPlanSource.SetParticipationForWeek":               "write",
	"care.OfferingChangeRequests":                               "read",
	"care.OfferingChangeRequests.CreateCourseRequest":           "write",
	"care.OfferingChangeRequests.Edit":                          "write",
	"care.OfferingChangeRequests.SubmitOfferingChange":          "write",
	"care.OfferingChangeRequests.WithdrawCourseRequest":         "write",
	"care.OfferingHistoryReads":                                 "read",
	"care.ParentAccounts":                                       "read",
	"care.PersonReads":                                          "read",
	"care.PickupPermissionWrite":                                "write",
	"care.PickupSchedules":                                      "read",
	"care.PickupSignatureLedger.PickupSignatures":               "read",
	"care.PickupSignatureLedger.RecordPickupSignature":          "write",
	"care.ReleaseFanOut":                                        "write",
	"care.ReleaseNotices":                                       "read",
	"care.ReleaseRecipients":                                    "read",
	"care.ReleaseSchools":                                       "read",
	"care.RequestEventWriter":                                   "write",
	"care.RequestShareVisibility":                               "read",
	"care.RequestSharer.LoadRequestShareVisibility":             "read",
	"care.RequestSharer.ShareRequestInTx":                       "write",
	"care.Runtime":                                              "read",
	"care.SEPAMandates":                                         "read",
	"care.SEPAMandates.GrantSEPAMandate":                        "write",
	"care.SEPAMandates.RevokeSEPAMandate":                       "write",
	"care.HolidayRegistrationAnnouncements":                     "read",
	"care.SchoolTraeger":                                        "read",
	"care.SelfServiceEvents.EmitSelfServicePill":                "write",
	"care.SelfServiceEvents.WakeChildGuardians":                 "notice",
	"care.SettingsService":                                      "read",
	"care.SettingsSnapshot":                                     "read",
	"care.StaffParentMessageNotifier":                           "write",
	"care.StatusDayReads":                                       "read",
	"care.StudentCare":                                          "write",
	"care.StudentChangeRecorder":                                "write",
	"care.StudentConsentService.CurrentStates":                  "read",
	"care.StudentConsentService.RecordTransitions":              "write",
	"care.StudentDataRequestCommands":                           "write",
	"care.StudentGuardianReads":                                 "read",
	"care.StudentPeople":                                        "write",
	"care.StudentPhotoUnlinker":                                 "write",
	"care.StudentReads":                                         "read",
	"care.StudentRecords":                                       "write",
	"care.StudentUpdates":                                       "notice",
	"care.TraegerDocumentNotifier":                              "write",
	"care.TraegerRuntime":                                       "read",
	"care.TraegerSharedDocuments":                               "read",
	"care.Config.Memberships":                                   "read",
	"care.Config.InvitationAccounts":                            "read",
	"care.Config.InvalidateInvites":                             "write",
	"carerequests.Reads":                                        "read",
	"carerequests.Service":                                      "read",
	"carerequests.Service.CreatePickupChange":                   "write",
	"carerequests.Service.CreatePickupPermission":               "write",
	"carerequests.Service.CreateRequest":                        "write",
	"carerequests.Service.EditRequest":                          "write",
	"messaging.AnnouncementFeedAccess":                          "read",
	"messaging.AnnouncementFeedAccess.MarkAcknowledged":         "write",
	"messaging.AnnouncementFeedAccess.MarkRead":                 "write",
	"messaging.AnnouncementFeedAccess.SetResponse":              "write",
	"messaging.DeclarationCommands":                             "read",
	"messaging.DeclarationCommands.InsertDeclarationSubmission": "write",
	"messaging.DeclarationReads":                                "read",
	"messaging.PermissionValues":                                "read",
	"messaging.AttachmentAudience":                              "read",
	"messaging.AttachmentFiles":                                 "read",
	"messaging.CareRequestReads":                                "read",
	"messaging.ChildResolver":                                   "read",
	"messaging.ContentFingerprints":                             "read",
	"messaging.ConversationCore":                                "read",
	"messaging.ConversationCore.AppendMessage":                  "write",
	"messaging.ConversationCore.Broadcast":                      "notice",
	"messaging.ConversationCore.BroadcastRead":                  "notice",
	"messaging.ConversationCore.MarkReadToNewest":               "write",
	"messaging.DeclarationPolicy":                               "read",
	"messaging.ExcusedRequestReads":                             "read",
	"messaging.FamilyProtectionReads":                           "read",
	"messaging.GroupThreads":                                    "read",
	"messaging.GroupThreads.ReadGroupThread":                    "write",
	"messaging.GuardianProfileReads":                            "read",
	"messaging.MessageInboxReads":                               "read",
	"messaging.MessageReads":                                    "read",
	"messaging.MessageThreadAccess":                             "read",
	"messaging.MessageThreadAccess.GetOrCreate":                 "write",
	"messaging.OfferingChangeRequestReads":                      "read",
	"messaging.RequestEvents":                                   "read",
	"messaging.RequestSharePolicy":                              "read",
	"messaging.RequestShareRecords":                             "read",
	"messaging.RequestShareRecords.Create":                      "write",
	"messaging.SignatureLedger.DeclarationSignatures":           "read",
	"messaging.SignatureLedger.RecordDeclarationSignature":      "write",
	"messaging.Surveys":                                         "read",
	"messaging.Surveys.SubmitSurvey":                            "write",
}

// flowGraph is the call graph of the flow packages.
type flowGraph struct {
	decls   map[*types.Func]*ast.FuncDecl
	infos   map[*types.Func]*types.Info
	pkgs    map[*types.Func]*packages.Package
	project string
}

func newFlowGraph(pkgs []*packages.Package) *flowGraph {
	g := &flowGraph{decls: map[*types.Func]*ast.FuncDecl{}, infos: map[*types.Func]*types.Info{}, pkgs: map[*types.Func]*packages.Package{}}
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				obj, ok := pkg.TypesInfo.Defs[fn.Name].(*types.Func)
				if !ok {
					continue
				}
				g.decls[obj], g.infos[obj], g.pkgs[obj] = fn, pkg.TypesInfo, pkg
			}
		}
	}
	return g
}

// roots are the exported methods of the flow services.
func (g *flowGraph) roots() []*types.Func {
	var roots []*types.Func
	for fn := range g.decls {
		if !fn.Exported() {
			continue
		}
		recv := fn.Type().(*types.Signature).Recv()
		if recv == nil {
			continue
		}
		if named := namedOf(recv.Type()); named != nil && named.Obj().Name() == "Service" {
			roots = append(roots, fn)
		}
	}
	sort.Slice(roots, func(i, j int) bool { return g.name(roots[i]) < g.name(roots[j]) })
	return roots
}

// writesFrom returns the write sinks reachable from root outside guarded
// write arguments, and the port methods without a class.
func (g *flowGraph) writesFrom(root *types.Func) (sinks, unknown []string) {
	walk := flowWalk{graph: g, seen: map[*types.Func]bool{root: true}, queue: []*types.Func{root}, found: map[string]bool{}, unclassified: map[string]bool{}}
	for len(walk.queue) > 0 {
		fn := walk.queue[0]
		walk.queue = walk.queue[1:]
		walk.visit(fn)
	}
	return sortedPortKeys(walk.found), sortedPortKeys(walk.unclassified)
}

// flowWalk follows the calls of one entry through the flow packages.
type flowWalk struct {
	graph               *flowGraph
	seen                map[*types.Func]bool
	queue               []*types.Func
	found, unclassified map[string]bool
}

func (w *flowWalk) visit(fn *types.Func) {
	info := w.graph.infos[fn]
	skip := map[ast.Node]bool{}
	ast.Inspect(w.graph.decls[fn].Body, func(n ast.Node) bool {
		if n == nil || skip[n] {
			return false
		}
		skipGuardedWrite(info, skip, n)
		var ident *ast.Ident
		switch x := n.(type) {
		case *ast.Ident:
			ident = x
		case *ast.SelectorExpr:
			ident = x.Sel
		default:
			return true
		}
		switch obj := info.Uses[ident].(type) {
		case *types.Func:
			w.function(obj)
		case *types.Var:
			w.field(info, n, obj)
		}
		return true
	})
}

// skipGuardedWrite marks the write argument of a guarded opener: whatever
// runs there runs behind the guard.
func skipGuardedWrite(info *types.Info, skip map[ast.Node]bool, n ast.Node) {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return
	}
	index, guarded := guardedOpeners[calleeName(info, call.Fun)]
	if !guarded || index >= len(call.Args) || receiptOnly(info, call.Args[index+1:]) {
		return
	}
	skip[call.Args[index]] = true
}

// receiptOnly reports a guarded opener told to skip its guard for a read
// receipt (care.ReadReceiptOnly): its write argument is followed like
// unguarded code.
func receiptOnly(info *types.Info, options []ast.Expr) bool {
	for _, option := range options {
		if ident := identOf(option); ident != nil {
			if fn, ok := info.Uses[ident].(*types.Func); ok && funcName(fn) == "care.ReadReceiptOnly" {
				return true
			}
		}
	}
	return false
}

func identOf(expression ast.Expr) *ast.Ident {
	switch x := expression.(type) {
	case *ast.Ident:
		return x
	case *ast.SelectorExpr:
		return x.Sel
	}
	return nil
}

// function follows a function of the flows or records a port method.
func (w *flowWalk) function(obj *types.Func) {
	if _, local := w.graph.decls[obj]; local {
		if !w.seen[obj] {
			w.seen[obj] = true
			w.queue = append(w.queue, obj)
		}
		return
	}
	port := interfaceMethodName(obj)
	if port == "" {
		return
	}
	switch classify(port) {
	case "write":
		w.found[port] = true
	case "":
		w.unclassified[port] = true
	}
}

// field records a callable field of the configuration the register names a
// write.
func (w *flowWalk) field(info *types.Info, n ast.Node, obj *types.Var) {
	if !obj.IsField() || !isFunc(obj.Type()) {
		return
	}
	if owner := fieldOwner(info, n); owner != "" && classify(owner+"."+obj.Name()) == "write" {
		w.found[owner+"."+obj.Name()] = true
	}
}

func sortedPortKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func classify(port string) string {
	if class, ok := portClasses[port]; ok {
		return class
	}
	if i := strings.LastIndex(port, "."); i > 0 {
		return portClasses[port[:i]]
	}
	return ""
}

func (g *flowGraph) name(fn *types.Func) string {
	return funcName(fn)
}

func (g *flowGraph) location(fn *types.Func) []Location {
	pkg, decl := g.pkgs[fn], g.decls[fn]
	if pkg == nil || decl == nil {
		return nil
	}
	location, err := sourceLocation(g.project, pkg.Fset.PositionFor(decl.Pos(), false), funcName(fn))
	if err != nil {
		return nil
	}
	return []Location{location}
}

func funcName(fn *types.Func) string {
	if fn.Pkg() == nil {
		return fn.Name()
	}
	pkg := fn.Pkg().Name()
	if recv := fn.Type().(*types.Signature).Recv(); recv != nil {
		if named := namedOf(recv.Type()); named != nil {
			return pkg + "." + named.Obj().Name() + "." + fn.Name()
		}
	}
	return pkg + "." + fn.Name()
}

// calleeName names the function a call expression calls, as funcName or as
// interface method.
func calleeName(info *types.Info, fun ast.Expr) string {
	var ident *ast.Ident
	switch x := fun.(type) {
	case *ast.Ident:
		ident = x
	case *ast.SelectorExpr:
		ident = x.Sel
	default:
		return ""
	}
	fn, ok := info.Uses[ident].(*types.Func)
	if !ok {
		return ""
	}
	if port := interfaceMethodName(fn); port != "" {
		return port
	}
	return funcName(fn)
}

// interfaceMethodName names a method of a module interface as
// package.Interface.Method; "" for any other function.
func interfaceMethodName(fn *types.Func) string {
	recv := fn.Type().(*types.Signature).Recv()
	if recv == nil || fn.Pkg() == nil || !strings.HasPrefix(fn.Pkg().Path(), "github.com/moto-nrw/project-phoenix/") {
		return ""
	}
	named := namedOf(recv.Type())
	if named == nil {
		return ""
	}
	if _, ok := named.Underlying().(*types.Interface); !ok {
		return ""
	}
	return named.Obj().Pkg().Name() + "." + named.Obj().Name() + "." + fn.Name()
}

// fieldOwner names the struct type a func-typed field is selected from.
func fieldOwner(info *types.Info, n ast.Node) string {
	sel, ok := n.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	selection := info.Selections[sel]
	if selection == nil {
		return ""
	}
	index := selection.Index()
	typ := selection.Recv()
	for _, i := range index[:len(index)-1] {
		st, ok := derefStruct(typ)
		if !ok {
			return ""
		}
		typ = st.Field(i).Type()
	}
	named := namedOf(typ)
	if named == nil {
		return ""
	}
	return named.Obj().Pkg().Name() + "." + named.Obj().Name()
}

func derefStruct(typ types.Type) (*types.Struct, bool) {
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}
	st, ok := typ.Underlying().(*types.Struct)
	return st, ok
}

func namedOf(typ types.Type) *types.Named {
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}
	named, _ := typ.(*types.Named)
	return named
}

func isFunc(typ types.Type) bool {
	_, ok := typ.Underlying().(*types.Signature)
	return ok
}

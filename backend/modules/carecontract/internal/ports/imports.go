package ports

import (
	"time"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// ContractChild is the child of a contract of the Träger that was neither
// withdrawn nor ended, as its newest version names it, with the last day the
// contract holds: the end of a termination, else the end of that version.
type ContractChild struct {
	ContractID int64
	SchoolID   int64
	Status     string
	FirstName  string
	LastName   string
	BirthDate  *calendar.Date
	EndsOn     *calendar.Date
}

// ImportedContract is a contract an import file names (#4723): its school,
// its key, the instant of its conclusion, its first version and its party.
type ImportedContract struct {
	Line        int
	SchoolID    int64
	ImportKey   string
	ConcludedAt time.Time
	Version     carecontract.Version
	Party       NewParty
}

// ImportActionDetail is the key of a trail entry that names the file an
// imported contract came from.
const ImportActionDetail = "import_file"

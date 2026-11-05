package peopledirectory

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/sharedkernel/textnorm"
)

func (p *DirectoryPersonSnapshot) Validate() error {
	if p.FirstName == "" {
		return errors.New("first name is required")
	}

	if p.LastName == "" {
		return errors.New("last name is required")
	}

	p.FirstName = textnorm.Name(p.FirstName)
	p.LastName = textnorm.Name(p.LastName)

	return nil
}

func (p *DirectoryPersonSnapshot) GetFullName() string {
	return p.FirstName + " " + p.LastName
}

func (p *DirectoryPersonSnapshot) SetAccount(account *PersonAccountSnapshot) {
	p.Account = account
	if account != nil {
		p.AccountID = &account.ID
	} else {
		p.AccountID = nil
	}
}

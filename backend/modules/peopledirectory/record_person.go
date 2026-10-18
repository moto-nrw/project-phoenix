package peopledirectory

import (
	"errors"
	"strings"
)

func (p *DirectoryPersonSnapshot) Validate() error {
	if p.FirstName == "" {
		return errors.New("first name is required")
	}

	if p.LastName == "" {
		return errors.New("last name is required")
	}

	p.FirstName = strings.TrimSpace(p.FirstName)
	p.LastName = strings.TrimSpace(p.LastName)

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

func (p *DirectoryPersonSnapshot) SetRFIDCard(card *PersonRFIDSnapshot) {
	p.RFIDCard = card
	if card != nil {
		p.TagID = &card.ID
	} else {
		p.TagID = nil
	}
}

func (p *DirectoryPersonSnapshot) HasRFIDCard() bool {
	return p.TagID != nil && *p.TagID != ""
}

func (p *DirectoryPersonSnapshot) HasAccount() bool {
	return p.AccountID != nil && *p.AccountID > 0
}

package domain

import "github.com/moto-nrw/project-phoenix/internal/timezone"

type Date = timezone.Date

func ParseDate(value string) (Date, error) { return timezone.ParseDate(value) }

type Dish struct {
	Dish string
	Note *string
}

type Entry struct {
	Date     Date
	Position int
	Dish     string
	Note     *string
}

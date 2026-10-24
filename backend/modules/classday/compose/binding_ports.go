package compose

import app "github.com/moto-nrw/project-phoenix/modules/classday/internal/application"

type CallerReader interface{ app.CallerReader }
type SchoolWriteScope interface{ app.SchoolWriteScope }
type ClassTripStatusDays interface{ app.ClassTripStatusDays }
type ClassDayPickupReader interface{ app.ClassDayPickupReader }
type CareDayResolver interface{ app.CareDayResolver }
type PickupTimeReader interface{ app.PickupTimeReader }
type ArrivalTimeReader interface{ app.ArrivalTimeReader }
type PickupBaselineReader interface{ app.PickupBaselineReader }

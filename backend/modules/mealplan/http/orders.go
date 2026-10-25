package mealplan

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
)

// OrderChangeResponse is a numbered change of an order. The school's view
// names no child: the lists with names need meals:orders.
type OrderChangeResponse struct {
	Sequence    int       `json:"sequence"`
	SchoolClass string    `json:"school_class"`
	Delta       int       `json:"delta"`
	DetectedAt  time.Time `json:"detected_at"`
}

// ClassCountResponse is the number of meals of a class.
type ClassCountResponse struct {
	SchoolClass string `json:"school_class"`
	Count       int    `json:"count"`
}

// OrderDayResponse is one weekday of the orders.
type OrderDayResponse struct {
	Date        string                `json:"date"`
	Closed      bool                  `json:"closed"`
	Deadline    time.Time             `json:"deadline"`
	Status      string                `json:"status"`
	OrderID     string                `json:"order_id,omitempty"`
	ConfirmedAt *time.Time            `json:"confirmed_at,omitempty"`
	Total       int                   `json:"total"`
	Classes     []ClassCountResponse  `json:"classes"`
	Changes     []OrderChangeResponse `json:"changes"`
}

// OrdersResponse is the school's orders of a window (#3873).
type OrdersResponse struct {
	FormatVersion int                `json:"format_version"`
	Rhythm        string             `json:"rhythm"`
	Detail        string             `json:"detail"`
	Days          []OrderDayResponse `json:"days"`
}

// OrderDays lays out the days of an order week for the wire.
func OrderDays(week mealplanModule.OrderWeek) []OrderDayResponse {
	days := make([]OrderDayResponse, 0, len(week.Days))
	for _, day := range week.Days {
		response := OrderDayResponse{
			Date: string(day.Date), Closed: day.Closed, Deadline: day.Deadline, Status: string(day.Status),
			ConfirmedAt: day.ConfirmedAt, Total: day.Total,
			Classes: make([]ClassCountResponse, 0, len(day.Classes)), Changes: make([]OrderChangeResponse, 0, len(day.Changes)),
		}
		if day.OrderID > 0 {
			response.OrderID = strconv.FormatInt(day.OrderID, 10)
		}
		for _, count := range day.Classes {
			response.Classes = append(response.Classes, ClassCountResponse(count))
		}
		for _, change := range day.Changes {
			response.Changes = append(response.Changes, OrderChangeResponse{Sequence: change.Sequence, SchoolClass: change.Child.SchoolClass, Delta: change.Delta, DetectedAt: change.DetectedAt})
		}
		days = append(days, response)
	}
	return days
}

// getOrders returns the school's orders of [from, to]; reading fixes every
// due order (#3873).
func (rs *Resource) getOrders(w http.ResponseWriter, r *http.Request) {
	from, fromErr := mealplanModule.ParseDate(r.URL.Query().Get("from"))
	to, toErr := mealplanModule.ParseDate(r.URL.Query().Get("to"))
	if fromErr != nil || toErr != nil {
		rs.runtime.InvalidRequest(w, r, errors.New("from and to must be in YYYY-MM-DD format"))
		return
	}
	week, err := rs.module.Orders(r.Context(), from, to)
	if err != nil {
		rs.runtime.ModuleFailure(w, r, err, "failed to load meal orders")
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, OrdersResponse{FormatVersion: mealplanModule.OrderFormatVersion, Rhythm: week.Rhythm, Detail: week.Detail, Days: OrderDays(week)}, "Meal orders retrieved successfully")
}

// exportMonth exports the ordered meals per child of a month.
func (rs *Resource) exportMonth(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	from, to, err := mealplanModule.MonthWindow(mealplanModule.Date(month + "-01"))
	if err != nil {
		rs.runtime.InvalidRequest(w, r, errors.New("month must be in YYYY-MM format"))
		return
	}
	format := r.URL.Query().Get("format")
	if format != "pdf" && format != "xlsx" {
		rs.runtime.InvalidRequest(w, r, fmt.Errorf("unsupported format %q", format))
		return
	}
	week, err := rs.module.Orders(r.Context(), from, to)
	if err != nil {
		rs.runtime.ModuleFailure(w, r, err, "failed to load meal orders")
		return
	}
	export, err := week.MonthExport(from)
	if err != nil {
		rs.runtime.ModuleFailure(w, r, err, "failed to export the monthly overview")
		return
	}
	file, err := rs.runtime.RenderList(export, format)
	if err != nil {
		rs.runtime.ModuleFailure(w, r, err, "failed to export the monthly overview")
		return
	}
	writeFile(w, file)
}

func writeFile(w http.ResponseWriter, file ExportFile) {
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

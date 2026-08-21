package controlcenter

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func ParseDateRange(r *http.Request, loc *time.Location, now time.Time) (DateRange, error) {
	if loc == nil {
		loc = time.UTC
	}
	today := now.In(loc)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc)
	from, to := today.AddDate(0, 0, -29), today
	query := r.URL.Query()
	if raw := strings.TrimSpace(query.Get("from")); raw != "" {
		parsed, err := time.ParseInLocation("2006-01-02", raw, loc)
		if err != nil {
			return DateRange{}, &ValidationError{Message: "invalid date range", Fields: map[string]string{"from": "use YYYY-MM-DD"}}
		}
		from = parsed
	}
	if raw := strings.TrimSpace(query.Get("to")); raw != "" {
		parsed, err := time.ParseInLocation("2006-01-02", raw, loc)
		if err != nil {
			return DateRange{}, &ValidationError{Message: "invalid date range", Fields: map[string]string{"to": "use YYYY-MM-DD"}}
		}
		to = parsed
	}
	if to.Before(from) {
		return DateRange{}, &ValidationError{Message: "invalid date range", Fields: map[string]string{"to": "must be on or after from"}}
	}
	rng := DateRange{From: from, To: to}
	if rng.Days() > MaxRangeDays {
		return DateRange{}, &ValidationError{Message: "date range is too large", Fields: map[string]string{"from": fmt.Sprintf("range may contain at most %d inclusive local dates", MaxRangeDays)}}
	}
	if raw := strings.TrimSpace(query.Get("compare")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return DateRange{}, &ValidationError{Message: "invalid comparison flag", Fields: map[string]string{"compare": "use true or false"}}
		}
		rng.Compare = value
	}
	return rng, nil
}

func ParsePagination(r *http.Request, loc *time.Location) (Pagination, error) {
	page, pageSize := 1, 25
	fields := make(map[string]string)
	if raw := strings.TrimSpace(r.URL.Query().Get("page")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			fields["page"] = "must be a positive integer"
		} else {
			page = value
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("page_size")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > MaxPageSize {
			fields["page_size"] = fmt.Sprintf("must be between 1 and %d", MaxPageSize)
		} else {
			pageSize = value
		}
	}
	options := Pagination{Page: page, PageSize: pageSize, Search: strings.TrimSpace(r.URL.Query().Get("search")), Filters: map[string]string{}}
	for _, key := range []string{"status", "source", "exercise_id", "plan_id", "template_id", "date_basis"} {
		if value := strings.TrimSpace(r.URL.Query().Get(key)); value != "" {
			options.Filters[key] = value
		}
	}
	if status := options.Filters["status"]; status != "" && !validSessionStatus(status) {
		fields["status"] = "use scheduled, active, finished, cancelled, or excused"
	}
	if basis := options.Filters["date_basis"]; basis != "" && basis != "actual" && basis != "calendar" {
		fields["date_basis"] = "use actual or calendar"
	}
	for key, target := range map[string]**time.Time{"from": &options.From, "to": &options.To} {
		if raw := strings.TrimSpace(r.URL.Query().Get(key)); raw != "" {
			value, err := time.ParseInLocation("2006-01-02", raw, loc)
			if err != nil {
				fields[key] = "use YYYY-MM-DD"
				continue
			}
			*target = &value
		}
	}
	if (options.From == nil) != (options.To == nil) {
		fields["range"] = "use from and to together"
	} else if options.From != nil && options.To != nil {
		if options.To.Before(*options.From) {
			fields["to"] = "must be on or after from"
		} else if (DateRange{From: *options.From, To: *options.To}).Days() > MaxRangeDays {
			fields["from"] = fmt.Sprintf("range may contain at most %d inclusive local dates", MaxRangeDays)
		}
	}
	if len(fields) > 0 {
		return Pagination{}, &ValidationError{Message: "invalid pagination or filters", Fields: fields}
	}
	return options, nil
}

func ParseAnalyticsFilters(r *http.Request) (AnalyticsFilters, error) {
	filters := AnalyticsFilters{
		Status:  strings.TrimSpace(r.URL.Query().Get("status")),
		DayType: strings.TrimSpace(r.URL.Query().Get("day_type")),
	}
	fields := map[string]string{}
	for key, destination := range map[string]**int64{
		"exercise_id": &filters.ExerciseID,
		"plan_id":     &filters.PlanID,
		"template_id": &filters.TemplateID,
	} {
		raw := strings.TrimSpace(r.URL.Query().Get(key))
		if raw == "" {
			continue
		}
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			fields[key] = "must be a positive integer"
			continue
		}
		*destination = &value
	}
	if filters.Status != "" && !validSessionStatus(filters.Status) {
		fields["status"] = "use scheduled, active, finished, cancelled, or excused"
	}
	if filters.DayType != "" && filters.DayType != "training" && filters.DayType != "rest" {
		fields["day_type"] = "use training or rest"
	}
	if len(fields) > 0 {
		return AnalyticsFilters{}, &ValidationError{Message: "invalid analytics filters", Fields: fields}
	}
	return filters, nil
}

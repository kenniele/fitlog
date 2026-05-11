package fatsecret

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// flexFloat handles FatSecret's habit of quoting every number ("calories":"150"),
// plus the occasional missing/null/empty field.
type flexFloat float64

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	s = strings.Trim(s, `"`)
	if s == "" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("flexFloat parse %q: %w", s, err)
	}
	*f = flexFloat(v)
	return nil
}

// ptr returns nil if v is zero, else a pointer to v. Used when mapping flexFloat
// into the domain types where missing fields are *float64.
func (f flexFloat) ptr() *float64 {
	if f == 0 {
		return nil
	}
	v := float64(f)
	return &v
}

// asFloat returns v as float64 (0 for missing). Used for required fields.
func (f flexFloat) asFloat() float64 { return float64(f) }

// flexInt handles "20583" style stringified ints, plus null/empty.
type flexInt int64

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	s = strings.Trim(s, `"`)
	if s == "" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("flexInt parse %q: %w", s, err)
	}
	*f = flexInt(v)
	return nil
}

// normalizeList copes with FatSecret's three-way encoding of lists:
//   - 0 items → null, "", or missing
//   - 1 item  → bare object
//   - 2+ items → array
//
// It returns a slice of raw JSON objects ready for further unmarshalling.
func normalizeList(raw json.RawMessage) []json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 ||
		bytes.Equal(trimmed, []byte("null")) ||
		bytes.Equal(trimmed, []byte(`""`)) {
		return nil
	}
	if trimmed[0] == '[' {
		var arr []json.RawMessage
		_ = json.Unmarshal(trimmed, &arr)
		return arr
	}
	return []json.RawMessage{trimmed}
}

// foodEntryDTO mirrors the fields of a single food_entry node.
type foodEntryDTO struct {
	FoodEntryID          string    `json:"food_entry_id"`
	FoodEntryName        string    `json:"food_entry_name"`
	FoodEntryDescription string    `json:"food_entry_description"`
	DateInt              flexInt   `json:"date_int"`
	Meal                 string    `json:"meal"`
	NumberOfUnits        flexFloat `json:"number_of_units"`
	Calories             flexFloat `json:"calories"`
	Protein              flexFloat `json:"protein"`
	Carbohydrate         flexFloat `json:"carbohydrate"`
	Fat                  flexFloat `json:"fat"`
	Fiber                flexFloat `json:"fiber"`
	Sugar                flexFloat `json:"sugar"`
	Sodium               flexFloat `json:"sodium"`
	Potassium            flexFloat `json:"potassium"`
	Cholesterol          flexFloat `json:"cholesterol"`
	SaturatedFat         flexFloat `json:"saturated_fat"`
	MonounsaturatedFat   flexFloat `json:"monounsaturated_fat"`
	PolyunsaturatedFat   flexFloat `json:"polyunsaturated_fat"`
	TransFat             flexFloat `json:"trans_fat"`
	Calcium              flexFloat `json:"calcium"`
	Iron                 flexFloat `json:"iron"`
	VitaminA             flexFloat `json:"vitamin_a"`
	VitaminC             flexFloat `json:"vitamin_c"`
}

// foodEntriesResponse is the envelope returned by `food_entries.get`.
type foodEntriesResponse struct {
	FoodEntries struct {
		FoodEntry json.RawMessage `json:"food_entry"`
	} `json:"food_entries"`
}

// dayDTO mirrors one day in `food_entries.get_month`.
type dayDTO struct {
	DateInt      flexInt   `json:"date_int"`
	Calories     flexFloat `json:"calories"`
	Carbohydrate flexFloat `json:"carbohydrate"`
	Protein      flexFloat `json:"protein"`
	Fat          flexFloat `json:"fat"`
}

// monthResponse is the envelope returned by `food_entries.get_month`.
type monthResponse struct {
	Month struct {
		FromDateInt flexInt         `json:"from_date_int"`
		ToDateInt   flexInt         `json:"to_date_int"`
		Day         json.RawMessage `json:"day"`
	} `json:"month"`
}

// profileResponse is what `profile.get` returns. We only use it for health checks.
type profileResponse struct {
	Profile struct {
		UserID string `json:"user_id"`
	} `json:"profile"`
}

// errorResponse is FatSecret's error envelope: {"error":{"code":N,"message":"..."}}
type errorResponse struct {
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

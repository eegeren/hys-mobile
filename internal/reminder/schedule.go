package reminder

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrNoSchedule indicates that a store lacks a configured schedule.
var ErrNoSchedule = errors.New("schedule not found for store")

// Shift represents a start/end shift window.
type Shift struct {
	Start string
	End   string
}

// Label returns "HH:MM-HH:MM".
func (s Shift) Label() string {
	return fmt.Sprintf("%s-%s", s.Start, s.End)
}

// StoreSchedule represents store-specific shift windows.
type StoreSchedule struct {
	Weekday []Shift
	Sunday  []Shift
}

// DefaultSchedules holds the hard-coded mapping of store identifiers to schedules.
var DefaultSchedules = buildDefaultSchedules()

func buildDefaultSchedules() map[string]StoreSchedule {
	scheduleA := StoreSchedule{
		Weekday: []Shift{{Start: "09:00", End: "18:00"}, {Start: "10:30", End: "19:30"}},
		Sunday:  []Shift{{Start: "10:30", End: "19:30"}},
	}
	scheduleB := StoreSchedule{
		Weekday: []Shift{{Start: "09:00", End: "18:00"}, {Start: "11:00", End: "20:00"}},
		Sunday:  []Shift{{Start: "10:30", End: "19:30"}, {Start: "11:00", End: "20:00"}},
	}
	schedule26 := StoreSchedule{
		Weekday: []Shift{{Start: "09:00", End: "18:00"}, {Start: "11:30", End: "20:30"}},
		Sunday:  []Shift{{Start: "10:30", End: "19:30"}, {Start: "11:30", End: "20:30"}},
	}

	schedules := make(map[string]StoreSchedule)

	register := func(keys []string, schedule StoreSchedule) {
		for _, key := range keys {
			norm := normalizeStoreKey(key)
			schedules[norm] = schedule
		}
	}

	register([]string{"03", "04", "05", "13", "17", "19", "20", "21", "23", "25", "27", "bandirma hys"}, scheduleA)
	register([]string{"02", "07", "08", "09", "10", "14", "22"}, scheduleB)
	register([]string{"26", "bursa nilüfer hys"}, schedule26)

	return schedules
}

var storeReplacer = strings.NewReplacer(
	"ı", "i",
	"İ", "i",
	"ğ", "g",
	"Ğ", "g",
	"ş", "s",
	"Ş", "s",
	"ö", "o",
	"Ö", "o",
	"ü", "u",
	"Ü", "u",
)

func normalizeStoreKey(v string) string {
	lowered := strings.ToLower(strings.TrimSpace(v))
	return storeReplacer.Replace(lowered)
}

// ShiftsForStore returns today's shift windows for the given store identifier.
func ShiftsForStore(schedule map[string]StoreSchedule, sube string, date time.Time) ([]Shift, error) {
	if schedule == nil {
		schedule = DefaultSchedules
	}
	key := normalizeStoreKey(sube)
	store, ok := schedule[key]
	if !ok {
		return nil, ErrNoSchedule
	}
	if date.Weekday() == time.Sunday {
		return store.Sunday, nil
	}
	return store.Weekday, nil
}

// ShiftStartTimes returns HH:MM start times as time.Time values.
func ShiftStartTimes(schedule map[string]StoreSchedule, sube string, date time.Time, loc *time.Location) ([]time.Time, error) {
	shifts, err := ShiftsForStore(schedule, sube, date)
	if err != nil {
		return nil, err
	}
	out := make([]time.Time, 0, len(shifts))
	for _, shift := range shifts {
		t, err := parseClock(loc, date, shift.Start)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func parseClock(loc *time.Location, date time.Time, hhmm string) (time.Time, error) {
	parsed, err := time.ParseInLocation("15:04", hhmm, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse shift %s: %w", hhmm, err)
	}
	year, month, day := date.In(loc).Date()
	hour, min, _ := parsed.Clock()
	return time.Date(year, month, day, hour, min, 0, 0, loc), nil
}

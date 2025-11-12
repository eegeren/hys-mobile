package reminder

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"hys-backend-go/internal/repo"
	"hys-backend-go/internal/roles"
)

var reminderOffsets = []int{0, 45}

// CheckinLookup exposes read access to last check-in timestamps.
type CheckinLookup interface {
	Last(tc string) (time.Time, bool)
}

// Service periodically evaluates shift reminders.
type Service struct {
	source   *repo.Source
	checkins CheckinLookup
	schedule map[string]StoreSchedule
	loc      *time.Location

	sentMu sync.Mutex
	sent   map[string]map[string]map[int]bool // day -> tc -> offset -> sent
}

// NewService constructs a reminder service.
func NewService(source *repo.Source, checkins CheckinLookup, schedule map[string]StoreSchedule, loc *time.Location) (*Service, error) {
	if source == nil {
		return nil, errors.New("source is required")
	}
	if checkins == nil {
		return nil, errors.New("checkins is required")
	}
	if loc == nil {
		var err error
		loc, err = time.LoadLocation("Europe/Istanbul")
		if err != nil {
			return nil, err
		}
	}
	if schedule == nil {
		schedule = DefaultSchedules
	}
	return &Service{
		source:   source,
		checkins: checkins,
		schedule: schedule,
		loc:      loc,
		sent:     make(map[string]map[string]map[int]bool),
	}, nil
}

// Location returns the timezone used by the service.
func (s *Service) Location() *time.Location {
	return s.loc
}

// Start launches the reminder loop until the context is canceled.
func (s *Service) Start(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	go s.loop(ctx)
}

func (s *Service) loop(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	s.process(time.Now())

	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			s.process(t)
		}
	}
}

func (s *Service) process(now time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	persons, _, err := s.source.Fetch(ctx)
	if err != nil {
		log.Printf("reminder: fetch error: %v", err)
		return
	}

	current := now.In(s.loc).Truncate(time.Minute)

	for _, p := range persons {
		tc := strings.TrimSpace(p.TC)
		if tc == "" {
			continue
		}
		if roles.Determine(&p) != "personel" {
			continue
		}
		shiftTimes, err := ShiftStartTimes(s.schedule, p.Sube, current, s.loc)
		if err != nil {
			if !errors.Is(err, ErrNoSchedule) {
				log.Printf("reminder: shift lookup error for sube=%s: %v", p.Sube, err)
			}
			continue
		}
		for _, shift := range shiftTimes {
			graceStart := shift.Add(15 * time.Minute)
			s.evaluateShift(tc, p.Sube, shift, graceStart, current)
		}
	}
}

func (s *Service) evaluateShift(tc, sube string, shiftStart, graceStart, now time.Time) {
	for _, offset := range reminderOffsets {
		reminderTime := graceStart.Add(time.Duration(offset) * time.Minute)
		if !sameMinute(reminderTime, now) {
			continue
		}
		if !s.shouldSend(tc, shiftStart, offset) {
			continue
		}
		if s.hasCheckin(tc, shiftStart) {
			continue
		}
		log.Printf("[PUSH] tc=%s sube=%s vardiya=%s giris_saati=%s offset=%d", tc, sube, shiftStart.Format("15:04"), graceStart.Format("15:04"), offset)
	}
}

func (s *Service) shouldSend(tc string, shiftStart time.Time, offset int) bool {
	dayKey := shiftStart.In(s.loc).Format("2006-01-02")

	s.sentMu.Lock()
	defer s.sentMu.Unlock()

	perDay, ok := s.sent[dayKey]
	if !ok {
		perDay = make(map[string]map[int]bool)
		s.sent[dayKey] = perDay
	}
	perUser, ok := perDay[tc]
	if !ok {
		perUser = make(map[int]bool)
		perDay[tc] = perUser
	}
	if perUser[offset] {
		return false
	}
	perUser[offset] = true

	// prune older days
	for day := range s.sent {
		if day != dayKey {
			delete(s.sent, day)
		}
	}

	return true
}

func (s *Service) hasCheckin(tc string, shiftStart time.Time) bool {
	last, ok := s.checkins.Last(tc)
	if !ok {
		return false
	}
	return sameDay(last, shiftStart, s.loc)
}

// ShiftLabels returns formatted shift windows for the requested store.
func (s *Service) ShiftLabels(sube string, date time.Time) ([]string, error) {
	shifts, err := ShiftsForStore(s.schedule, sube, date)
	if err != nil {
		return nil, ErrNoSchedule
	}
	labels := make([]string, 0, len(shifts))
	for _, shift := range shifts {
		labels = append(labels, shift.Label())
	}
	return labels, nil
}

// ForceNotify bypasses timing checks and logs an immediate reminder for testing.
func (s *Service) ForceNotify(tc, note string) error {
	tc = strings.TrimSpace(tc)
	if tc == "" {
		return errors.New("tc is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	persons, _, err := s.source.Fetch(ctx)
	if err != nil {
		return err
	}

	found := false
	for i := range persons {
		if strings.EqualFold(persons[i].TC, tc) {
			if roles.Determine(&persons[i]) != "personel" {
				return errors.New("tc is not eligible for reminders")
			}
			found = true
			break
		}
	}
	if !found {
		return errors.New("tc not found")
	}

	if note == "" {
		note = "admin-triggered"
	}
	log.Printf("[PUSH][FORCE] tc=%s note=%s", tc, note)
	return nil
}

func sameMinute(a, b time.Time) bool {
	return a.Truncate(time.Minute).Equal(b.Truncate(time.Minute))
}

func sameDay(a, b time.Time, loc *time.Location) bool {
	ay, am, ad := a.In(loc).Date()
	by, bm, bd := b.In(loc).Date()
	return ay == by && am == bm && ad == bd
}

// ScheduleForStore exposes lookup for other packages.
func (s *Service) ScheduleForStore(sube string) (StoreSchedule, error) {
	key := normalizeStoreKey(sube)
	store, ok := s.schedule[key]
	if !ok {
		return StoreSchedule{}, ErrNoSchedule
	}
	return store, nil
}

// NextReminderTimes is exported for testing.
func (s *Service) NextReminderTimes(sube string, date time.Time) ([]time.Time, error) {
	return ShiftStartTimes(s.schedule, sube, date, s.loc)
}

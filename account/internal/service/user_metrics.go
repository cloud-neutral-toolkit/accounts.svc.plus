package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

var (
	// ErrUserRepositoryNotConfigured is returned when the metrics service lacks
	// a user repository dependency.
	ErrUserRepositoryNotConfigured = errors.New("user repository is not configured")
	// ErrSubscriptionProviderNotConfigured is returned when subscription data is
	// required but the dependency has not been provided.
	ErrSubscriptionProviderNotConfigured = errors.New("subscription provider is not configured")
)

// UserRepository provides access to persisted user records required for
// metrics aggregation.
type UserRepository interface {
	ListUsers(ctx context.Context) ([]UserRecord, error)
}

// SubscriptionProvider exposes subscription status information for a set of
// users.
type SubscriptionProvider interface {
	FetchSubscriptionStates(ctx context.Context, userIDs []string) (map[string]SubscriptionState, error)
}

// UserRecord represents the minimal attributes required to compute metrics for
// a user population.
type UserRecord struct {
	ID        string
	CreatedAt time.Time
	Active    bool
}

// SubscriptionState describes the subscription attributes associated with a
// user.
type SubscriptionState struct {
	Active    bool
	ExpiresAt *time.Time
}

// UserMetricsService aggregates user population metrics such as totals and
// activity trends across configurable time windows.
type UserMetricsService struct {
	Users         UserRepository
	Subscriptions SubscriptionProvider
	DailyPeriods  int
	WeeklyPeriods int
	now           func() time.Time
}

const (
	defaultDailyPeriods  = 30
	defaultWeeklyPeriods = 12
)

// UserMetrics captures the aggregated metrics returned to API consumers.
type UserMetrics struct {
	Overview MetricsOverview `json:"overview"`
	Series   MetricsSeries   `json:"series"`
}

// UserMetricsProvider defines the behaviour expected from metrics aggregators.
type UserMetricsProvider interface {
	Compute(ctx context.Context) (UserMetrics, error)
}

// MetricsOverview contains headline statistics for the user population.
type MetricsOverview struct {
	TotalUsers      int `json:"totalUsers"`
	ActiveUsers     int `json:"activeUsers"`
	SubscribedUsers int `json:"subscribedUsers"`
	NewUsersLast24h int `json:"newUsersLast24h"`
}

// MetricsSeries contains chronological breakdowns of user statistics.
type MetricsSeries struct {
	Daily  []MetricsPoint `json:"daily"`
	Weekly []MetricsPoint `json:"weekly"`
}

// MetricsPoint represents aggregated counts for a specific period.
type MetricsPoint struct {
	Period     string `json:"period"`
	Total      int    `json:"total"`
	Active     int    `json:"active"`
	Subscribed int    `json:"subscribed"`
}

// Compute aggregates the metrics using the configured repositories.
func (s *UserMetricsService) Compute(ctx context.Context) (UserMetrics, error) {
	if s == nil {
		return UserMetrics{}, ErrUserRepositoryNotConfigured
	}
	if s.Users == nil {
		return UserMetrics{}, ErrUserRepositoryNotConfigured
	}
	if s.Subscriptions == nil {
		return UserMetrics{}, ErrSubscriptionProviderNotConfigured
	}

	dailyPeriods := s.DailyPeriods
	if dailyPeriods <= 0 {
		dailyPeriods = defaultDailyPeriods
	}
	weeklyPeriods := s.WeeklyPeriods
	if weeklyPeriods <= 0 {
		weeklyPeriods = defaultWeeklyPeriods
	}

	now := s.nowFn()
	startOfToday := truncateToDay(now)
	dailyStart := startOfToday.AddDate(0, 0, -(dailyPeriods - 1))
	weekStart := truncateToWeek(now)
	weeklyStart := weekStart.AddDate(0, 0, -7*(weeklyPeriods-1))

	users, err := s.Users.ListUsers(ctx)
	if err != nil {
		return UserMetrics{}, err
	}

	userIDs := make([]string, 0, len(users))
	for _, user := range users {
		if user.ID != "" {
			userIDs = append(userIDs, user.ID)
		}
	}

	subscriptionStates := make(map[string]SubscriptionState)
	if len(userIDs) > 0 {
		states, err := s.Subscriptions.FetchSubscriptionStates(ctx, userIDs)
		if err != nil {
			return UserMetrics{}, err
		}
		if states != nil {
			subscriptionStates = states
		}
	}

	overview := MetricsOverview{}
	dailySeries := make([]MetricsPoint, dailyPeriods)
	weeklySeries := make([]MetricsPoint, weeklyPeriods)

	dailyIndex := make(map[string]*MetricsPoint, dailyPeriods)
	for i := 0; i < dailyPeriods; i++ {
		periodStart := dailyStart.AddDate(0, 0, i)
		label := periodStart.Format("2006-01-02")
		dailySeries[i] = MetricsPoint{Period: label}
		dailyIndex[label] = &dailySeries[i]
	}

	weeklyIndex := make(map[string]*MetricsPoint, weeklyPeriods)
	for i := 0; i < weeklyPeriods; i++ {
		periodStart := weeklyStart.AddDate(0, 0, i*7)
		year, week := periodStart.ISOWeek()
		label := fmt.Sprintf("%04d-W%02d", year, week)
		weeklySeries[i] = MetricsPoint{Period: label}
		weeklyIndex[label] = &weeklySeries[i]
	}

	newUserThreshold := now.Add(-24 * time.Hour)

	for _, user := range users {
		overview.TotalUsers++

		subscribed := false
		if state, ok := subscriptionStates[user.ID]; ok {
			subscribed = state.Active
		}

		if user.Active {
			overview.ActiveUsers++
		}
		if subscribed {
			overview.SubscribedUsers++
		}
		if !user.CreatedAt.IsZero() && !user.CreatedAt.Before(newUserThreshold) {
			overview.NewUsersLast24h++
		}

		if user.CreatedAt.IsZero() {
			continue
		}

		createdAtUTC := user.CreatedAt.UTC()

		day := truncateToDay(createdAtUTC)
		if !day.Before(dailyStart) && !day.After(startOfToday) {
			label := day.Format("2006-01-02")
			if point, ok := dailyIndex[label]; ok {
				point.Total++
				if user.Active {
					point.Active++
				}
				if subscribed {
					point.Subscribed++
				}
			}
		}

		week := truncateToWeek(createdAtUTC)
		if !week.Before(weeklyStart) && !week.After(weekStart) {
			year, weekNumber := week.ISOWeek()
			label := fmt.Sprintf("%04d-W%02d", year, weekNumber)
			if point, ok := weeklyIndex[label]; ok {
				point.Total++
				if user.Active {
					point.Active++
				}
				if subscribed {
					point.Subscribed++
				}
			}
		}
	}

	// Ensure chronological order.
	sort.SliceStable(dailySeries, func(i, j int) bool {
		return dailySeries[i].Period < dailySeries[j].Period
	})
	sort.SliceStable(weeklySeries, func(i, j int) bool {
		return weeklySeries[i].Period < weeklySeries[j].Period
	})

	metrics := UserMetrics{
		Overview: overview,
		Series: MetricsSeries{
			Daily:  dailySeries,
			Weekly: weeklySeries,
		},
	}

	return metrics, nil
}

func (s *UserMetricsService) nowFn() time.Time {
	if s != nil && s.now != nil {
		return s.now()
	}
	return time.Now().UTC()
}

func truncateToDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func truncateToWeek(t time.Time) time.Time {
	t = truncateToDay(t)
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return t.AddDate(0, 0, -(weekday - 1))
}

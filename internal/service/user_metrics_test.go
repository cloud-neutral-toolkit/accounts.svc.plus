package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

type stubUserRepository struct {
	users []UserRecord
	err   error
}

func (s stubUserRepository) ListUsers(context.Context) ([]UserRecord, error) {
	if s.err != nil {
		return nil, s.err
	}
	cloned := make([]UserRecord, len(s.users))
	copy(cloned, s.users)
	return cloned, nil
}

type stubSubscriptionProvider struct {
	states map[string]SubscriptionState
	err    error
}

func (s stubSubscriptionProvider) FetchSubscriptionStates(context.Context, []string) (map[string]SubscriptionState, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.states == nil {
		return map[string]SubscriptionState{}, nil
	}
	cloned := make(map[string]SubscriptionState, len(s.states))
	for k, v := range s.states {
		cloned[k] = v
	}
	return cloned, nil
}

func TestUserMetricsServiceCompute(t *testing.T) {
	now := time.Date(2024, time.March, 18, 15, 30, 0, 0, time.UTC)
	dailyPeriods := 5
	weeklyPeriods := 4

	repo := stubUserRepository{users: []UserRecord{
		{ID: "u1", CreatedAt: now.Add(-2 * time.Hour), Active: true},
		{ID: "u2", CreatedAt: now.Add(-26 * time.Hour), Active: false},
		{ID: "u3", CreatedAt: now.AddDate(0, 0, -7), Active: true},
		{ID: "u4", CreatedAt: now.AddDate(0, 0, -20), Active: false},
		{ID: "u5", CreatedAt: time.Time{}, Active: true},
	}}
	provider := stubSubscriptionProvider{states: map[string]SubscriptionState{
		"u1": {Active: true},
		"u3": {Active: true},
		"u4": {Active: false},
	}}

	service := UserMetricsService{
		Users:         repo,
		Subscriptions: provider,
		DailyPeriods:  dailyPeriods,
		WeeklyPeriods: weeklyPeriods,
		now:           func() time.Time { return now },
	}

	metrics, err := service.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}

	if metrics.Overview.TotalUsers != 5 {
		t.Fatalf("expected total users 5, got %d", metrics.Overview.TotalUsers)
	}
	if metrics.Overview.ActiveUsers != 3 {
		t.Fatalf("expected active users 3, got %d", metrics.Overview.ActiveUsers)
	}
	if metrics.Overview.SubscribedUsers != 2 {
		t.Fatalf("expected subscribed users 2, got %d", metrics.Overview.SubscribedUsers)
	}
	if metrics.Overview.NewUsersLast24h != 1 {
		t.Fatalf("expected new users in last 24h to be 1, got %d", metrics.Overview.NewUsersLast24h)
	}

	if len(metrics.Series.Daily) != dailyPeriods {
		t.Fatalf("expected %d daily points, got %d", dailyPeriods, len(metrics.Series.Daily))
	}
	lastDay := metrics.Series.Daily[dailyPeriods-1]
	if lastDay.Total != 1 || lastDay.Active != 1 || lastDay.Subscribed != 1 {
		t.Fatalf("unexpected latest daily metrics: %+v", lastDay)
	}

	secondDay := metrics.Series.Daily[dailyPeriods-2]
	if secondDay.Total != 1 || secondDay.Active != 0 || secondDay.Subscribed != 0 {
		t.Fatalf("unexpected previous daily metrics: %+v", secondDay)
	}

	if len(metrics.Series.Weekly) != weeklyPeriods {
		t.Fatalf("expected %d weekly points, got %d", weeklyPeriods, len(metrics.Series.Weekly))
	}
	latestWeek := metrics.Series.Weekly[weeklyPeriods-1]
	if latestWeek.Total != 1 || latestWeek.Active != 1 || latestWeek.Subscribed != 1 {
		t.Fatalf("unexpected latest weekly metrics: %+v", latestWeek)
	}

	weekOfU4 := truncateToWeek(repo.users[3].CreatedAt)
	year, weekNum := weekOfU4.ISOWeek()
	expectedLabel := fmt.Sprintf("%04d-W%02d", year, weekNum)
	var weekPoint *MetricsPoint
	for i := range metrics.Series.Weekly {
		if metrics.Series.Weekly[i].Period == expectedLabel {
			weekPoint = &metrics.Series.Weekly[i]
			break
		}
	}
	if weekPoint == nil {
		t.Fatalf("expected weekly metrics for label %s", expectedLabel)
	}
	if weekPoint.Total == 0 {
		t.Fatalf("expected at least one signup recorded for %s", expectedLabel)
	}
}

func TestUserMetricsServiceMissingDependencies(t *testing.T) {
	svc := UserMetricsService{}
	if _, err := svc.Compute(context.Background()); !errors.Is(err, ErrUserRepositoryNotConfigured) {
		t.Fatalf("expected ErrUserRepositoryNotConfigured, got %v", err)
	}

	svc.Users = stubUserRepository{}
	if _, err := svc.Compute(context.Background()); !errors.Is(err, ErrSubscriptionProviderNotConfigured) {
		t.Fatalf("expected ErrSubscriptionProviderNotConfigured, got %v", err)
	}
}

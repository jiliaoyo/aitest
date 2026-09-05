package admin

import "context"

type Service struct{ store *Store }

func NewService(store *Store) *Service { return &Service{store: store} }

func (s *Service) ListUsers(ctx context.Context, f UserListFilter) (UsersPage, error) {
	summary, err := s.store.Summary(ctx, f.DateRange)
	if err != nil {
		return UsersPage{}, err
	}
	users, nextCursor, err := s.store.ListUsers(ctx, f)
	if err != nil {
		return UsersPage{}, err
	}
	return UsersPage{Summary: summary, Users: users, NextCursor: nextCursor}, nil
}

func (s *Service) UserDetail(ctx context.Context, userID string, dateRange DateRange) (UserDetail, error) {
	profile, err := s.store.Profile(ctx, userID)
	if err != nil {
		return UserDetail{}, err
	}
	usage, err := s.store.Usage(ctx, dateRange, userID)
	if err != nil {
		return UserDetail{}, err
	}
	profile.LastActiveAt = usage.LastActiveAt
	profile.LastLoginAt = usage.LastLoginAt
	byKind, err := s.store.AIByKind(ctx, dateRange, userID)
	if err != nil {
		return UserDetail{}, err
	}
	byModel, err := s.store.AIByModel(ctx, dateRange, userID)
	if err != nil {
		return UserDetail{}, err
	}
	daily, err := s.store.AIDaily(ctx, dateRange, userID)
	if err != nil {
		return UserDetail{}, err
	}
	runs, err := s.store.RecentAIRuns(ctx, dateRange, userID)
	if err != nil {
		return UserDetail{}, err
	}
	sessions, err := s.store.RecentPracticeSessions(ctx, dateRange, userID)
	if err != nil {
		return UserDetail{}, err
	}
	return UserDetail{
		User: profile, Usage: usage, AIByKind: byKind, AIByModel: byModel,
		AIDaily: daily, RecentAIRuns: runs, RecentPractice: sessions,
	}, nil
}

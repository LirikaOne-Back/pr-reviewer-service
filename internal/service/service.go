package service

import (
	"errors"
	"math/rand"
	"time"

	"pr-reviewer-service/internal/model"
	"pr-reviewer-service/internal/storage"
)

type Service struct {
	store *storage.Storage
	rng   *rand.Rand
}

func New(store *storage.Storage) *Service {
	return &Service{
		store: store,
		rng:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *Service) CreateTeam(team model.Team) (*model.Team, error) {
	exists, err := s.store.TeamExists(team.TeamName)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New(model.ErrTeamExists)
	}

	if err := s.store.CreateTeam(team.TeamName); err != nil {
		return nil, err
	}

	for _, member := range team.Members {
		user := model.User{
			UserID:   member.UserID,
			Username: member.Username,
			TeamName: team.TeamName,
			IsActive: member.IsActive,
		}
		if err := s.store.UpsertUser(user); err != nil {
			return nil, err
		}
	}

	return s.store.GetTeam(team.TeamName)
}

func (s *Service) GetTeam(teamName string) (*model.Team, error) {
	team, err := s.store.GetTeam(teamName)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, errors.New(model.ErrNotFound)
	}
	return team, nil
}

func (s *Service) SetUserActive(userID string, isActive bool) (*model.User, error) {
	err := s.store.SetUserActive(userID, isActive)
	if err != nil {
		return nil, errors.New(model.ErrNotFound)
	}
	return s.store.GetUser(userID)
}

func (s *Service) CreatePR(prID, prName, authorID string) (*model.PullRequest, error) {
	exists, err := s.store.PRExists(prID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New(model.ErrPRExists)
	}

	author, err := s.store.GetUser(authorID)
	if err != nil {
		return nil, err
	}
	if author == nil {
		return nil, errors.New(model.ErrNotFound)
	}

	activeMembers, err := s.store.GetActiveTeamMembers(author.TeamName, authorID)
	if err != nil {
		return nil, err
	}

	reviewers := s.selectRandomReviewers(activeMembers, 2)

	pr := model.PullRequest{
		PullRequestID:     prID,
		PullRequestName:   prName,
		AuthorID:          authorID,
		Status:            model.StatusOpen,
		AssignedReviewers: reviewers,
	}

	if err := s.store.CreatePR(pr); err != nil {
		return nil, err
	}

	return s.store.GetPR(prID)
}

func (s *Service) MergePR(prID string) (*model.PullRequest, error) {
	pr, err := s.store.GetPR(prID)
	if err != nil {
		return nil, err
	}
	if pr == nil {
		return nil, errors.New(model.ErrNotFound)
	}

	if pr.Status == model.StatusMerged {
		return pr, nil
	}

	if err := s.store.MergePR(prID); err != nil {
		return nil, err
	}

	return s.store.GetPR(prID)
}

func (s *Service) ReassignReviewer(prID, oldUserID string) (*model.PullRequest, string, error) {
	pr, err := s.store.GetPR(prID)
	if err != nil {
		return nil, "", err
	}
	if pr == nil {
		return nil, "", errors.New(model.ErrNotFound)
	}

	if pr.Status == model.StatusMerged {
		return nil, "", errors.New(model.ErrPRMerged)
	}

	found := false
	for _, reviewerID := range pr.AssignedReviewers {
		if reviewerID == oldUserID {
			found = true
			break
		}
	}
	if !found {
		return nil, "", errors.New(model.ErrNotAssigned)
	}

	oldUser, err := s.store.GetUser(oldUserID)
	if err != nil {
		return nil, "", err
	}
	if oldUser == nil {
		return nil, "", errors.New(model.ErrNotFound)
	}

	excludeUsers := map[string]bool{oldUserID: true, pr.AuthorID: true}
	for _, reviewerID := range pr.AssignedReviewers {
		excludeUsers[reviewerID] = true
	}

	candidates, err := s.store.GetActiveTeamMembers(oldUser.TeamName, "")
	if err != nil {
		return nil, "", err
	}

	availableCandidates := []model.User{}
	for _, c := range candidates {
		if !excludeUsers[c.UserID] {
			availableCandidates = append(availableCandidates, c)
		}
	}

	if len(availableCandidates) == 0 {
		return nil, "", errors.New(model.ErrNoCandidate)
	}

	newReviewer := availableCandidates[s.rng.Intn(len(availableCandidates))]

	if err := s.store.ReassignReviewer(prID, oldUserID, newReviewer.UserID); err != nil {
		return nil, "", err
	}

	pr, err = s.store.GetPR(prID)
	return pr, newReviewer.UserID, err
}

func (s *Service) GetUserReviews(userID string) ([]model.PullRequestShort, error) {
	return s.store.GetPRsByReviewer(userID)
}

func (s *Service) selectRandomReviewers(users []model.User, maxCount int) []string {
	if len(users) == 0 {
		return []string{}
	}

	count := maxCount
	if len(users) < count {
		count = len(users)
	}

	indices := s.rng.Perm(len(users))
	reviewers := make([]string, count)
	for i := 0; i < count; i++ {
		reviewers[i] = users[indices[i]].UserID
	}
	return reviewers
}

func (s *Service) GetStatistics() (map[string]interface{}, error) {
	return s.store.GetStatistics()
}

func (s *Service) DeactivateTeam(teamName string) (map[string]interface{}, error) {
	team, err := s.store.GetTeam(teamName)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, errors.New(model.ErrNotFound)
	}

	deactivatedUserIDs, err := s.store.DeactivateTeam(teamName)
	if err != nil {
		return nil, err
	}

	if len(deactivatedUserIDs) == 0 {
		return map[string]interface{}{
			"team_name":            teamName,
			"deactivated_users":    []string{},
			"reassigned_prs":       []string{},
			"failed_reassignments": []string{},
		}, nil
	}

	prIDs, err := s.store.GetOpenPRsForReviewers(deactivatedUserIDs)
	if err != nil {
		return nil, err
	}

	reassignedPRs := []string{}
	failedPRs := []string{}

	for _, prID := range prIDs {
		pr, err := s.store.GetPR(prID)
		if err != nil || pr == nil {
			failedPRs = append(failedPRs, prID)
			continue
		}

		hasDeactivated := false
		for _, reviewerID := range pr.AssignedReviewers {
			for _, deactivatedID := range deactivatedUserIDs {
				if reviewerID == deactivatedID {
					hasDeactivated = true
					break
				}
			}
			if hasDeactivated {
				break
			}
		}

		if !hasDeactivated {
			continue
		}

		for _, reviewerID := range pr.AssignedReviewers {
			isDeactivated := false
			for _, deactivatedID := range deactivatedUserIDs {
				if reviewerID == deactivatedID {
					isDeactivated = true
					break
				}
			}

			if isDeactivated {
				oldUser, err := s.store.GetUser(reviewerID)
				if err != nil || oldUser == nil {
					continue
				}

				candidates, err := s.store.GetActiveTeamMembers(oldUser.TeamName, "")
				if err != nil {
					continue
				}

				excludeUsers := map[string]bool{pr.AuthorID: true}
				for _, rid := range pr.AssignedReviewers {
					excludeUsers[rid] = true
				}

				availableCandidates := []model.User{}
				for _, c := range candidates {
					if !excludeUsers[c.UserID] {
						availableCandidates = append(availableCandidates, c)
					}
				}

				if len(availableCandidates) > 0 {
					newReviewer := availableCandidates[s.rng.Intn(len(availableCandidates))]
					err = s.store.ReassignReviewer(prID, reviewerID, newReviewer.UserID)
					if err == nil {
						reassignedPRs = append(reassignedPRs, prID)
						break
					}
				}
			}
		}
	}

	return map[string]interface{}{
		"team_name":            teamName,
		"deactivated_users":    deactivatedUserIDs,
		"reassigned_prs":       reassignedPRs,
		"failed_reassignments": failedPRs,
	}, nil
}

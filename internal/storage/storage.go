package storage

import (
	"database/sql"
	"fmt"
	"time"

	"pr-reviewer-service/internal/model"

	_ "github.com/lib/pq"
)

type Storage struct {
	db *sql.DB
}

func New(host, port, user, password, dbname string) (*Storage, error) {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &Storage{db: db}, nil
}

func (s *Storage) Close() error {
	return s.db.Close()
}

func (s *Storage) CreateTeam(teamName string) error {
	_, err := s.db.Exec("INSERT INTO teams (team_name) VALUES ($1)", teamName)
	return err
}

func (s *Storage) TeamExists(teamName string) (bool, error) {
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM teams WHERE team_name = $1)", teamName).Scan(&exists)
	return exists, err
}

func (s *Storage) GetTeam(teamName string) (*model.Team, error) {
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM teams WHERE team_name = $1)", teamName).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}

	rows, err := s.db.Query(`
		SELECT user_id, username, is_active 
		FROM users 
		WHERE team_name = $1
		ORDER BY user_id`, teamName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []model.TeamMember{}
	for rows.Next() {
		var m model.TeamMember
		if err := rows.Scan(&m.UserID, &m.Username, &m.IsActive); err != nil {
			return nil, err
		}
		members = append(members, m)
	}

	return &model.Team{
		TeamName: teamName,
		Members:  members,
	}, nil
}

func (s *Storage) UpsertUser(user model.User) error {
	_, err := s.db.Exec(`
		INSERT INTO users (user_id, username, team_name, is_active, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE SET
			username = EXCLUDED.username,
			team_name = EXCLUDED.team_name,
			is_active = EXCLUDED.is_active,
			updated_at = EXCLUDED.updated_at`,
		user.UserID, user.Username, user.TeamName, user.IsActive, time.Now())
	return err
}

func (s *Storage) GetUser(userID string) (*model.User, error) {
	var user model.User
	err := s.db.QueryRow(`
		SELECT user_id, username, team_name, is_active 
		FROM users WHERE user_id = $1`, userID).
		Scan(&user.UserID, &user.Username, &user.TeamName, &user.IsActive)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Storage) SetUserActive(userID string, isActive bool) error {
	result, err := s.db.Exec(`
		UPDATE users SET is_active = $1, updated_at = $2 
		WHERE user_id = $3`, isActive, time.Now(), userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Storage) GetActiveTeamMembers(teamName, excludeUserID string) ([]model.User, error) {
	rows, err := s.db.Query(`
		SELECT user_id, username, team_name, is_active 
		FROM users 
		WHERE team_name = $1 AND is_active = true AND user_id != $2
		ORDER BY user_id`, teamName, excludeUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []model.User{}
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.UserID, &u.Username, &u.TeamName, &u.IsActive); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (s *Storage) CreatePR(pr model.PullRequest) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	createdAt := time.Now()
	_, err = tx.Exec(`
		INSERT INTO pull_requests (pull_request_id, pull_request_name, author_id, status, created_at)
		VALUES ($1, $2, $3, $4, $5)`,
		pr.PullRequestID, pr.PullRequestName, pr.AuthorID, pr.Status, createdAt)
	if err != nil {
		return err
	}

	for _, reviewerID := range pr.AssignedReviewers {
		_, err = tx.Exec(`
			INSERT INTO pr_reviewers (pull_request_id, user_id)
			VALUES ($1, $2)`, pr.PullRequestID, reviewerID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Storage) PRExists(prID string) (bool, error) {
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM pull_requests WHERE pull_request_id = $1)", prID).Scan(&exists)
	return exists, err
}

func (s *Storage) GetPR(prID string) (*model.PullRequest, error) {
	var pr model.PullRequest
	var createdAt, mergedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT pull_request_id, pull_request_name, author_id, status, created_at, merged_at
		FROM pull_requests WHERE pull_request_id = $1`, prID).
		Scan(&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &pr.Status, &createdAt, &mergedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if createdAt.Valid {
		t := model.FormatTime(createdAt.Time)
		pr.CreatedAt = &t
	}
	if mergedAt.Valid {
		t := model.FormatTime(mergedAt.Time)
		pr.MergedAt = &t
	}

	rows, err := s.db.Query(`
		SELECT user_id FROM pr_reviewers 
		WHERE pull_request_id = $1 
		ORDER BY assigned_at`, prID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reviewers := []string{}
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		reviewers = append(reviewers, userID)
	}
	pr.AssignedReviewers = reviewers

	return &pr, nil
}

func (s *Storage) MergePR(prID string) error {
	mergedAt := time.Now()
	result, err := s.db.Exec(`
		UPDATE pull_requests 
		SET status = $1, merged_at = $2 
		WHERE pull_request_id = $3`, model.StatusMerged, mergedAt, prID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Storage) ReassignReviewer(prID, oldUserID, newUserID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		DELETE FROM pr_reviewers 
		WHERE pull_request_id = $1 AND user_id = $2`, prID, oldUserID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	_, err = tx.Exec(`
		INSERT INTO pr_reviewers (pull_request_id, user_id)
		VALUES ($1, $2)`, prID, newUserID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Storage) GetPRsByReviewer(userID string) ([]model.PullRequestShort, error) {
	rows, err := s.db.Query(`
		SELECT p.pull_request_id, p.pull_request_name, p.author_id, p.status
		FROM pull_requests p
		JOIN pr_reviewers pr ON p.pull_request_id = pr.pull_request_id
		WHERE pr.user_id = $1
		ORDER BY p.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prs := []model.PullRequestShort{}
	for rows.Next() {
		var pr model.PullRequestShort
		if err := rows.Scan(&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &pr.Status); err != nil {
			return nil, err
		}
		prs = append(prs, pr)
	}
	return prs, nil
}

func (s *Storage) GetStatistics() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	var totalPRs, openPRs, mergedPRs int
	err := s.db.QueryRow(`
		SELECT 
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE status = 'OPEN') as open,
			COUNT(*) FILTER (WHERE status = 'MERGED') as merged
		FROM pull_requests`).Scan(&totalPRs, &openPRs, &mergedPRs)
	if err != nil {
		return nil, err
	}

	stats["total_prs"] = totalPRs
	stats["open_prs"] = openPRs
	stats["merged_prs"] = mergedPRs

	rows, err := s.db.Query(`
		SELECT u.user_id, u.username, COUNT(pr.pull_request_id) as review_count
		FROM users u
		LEFT JOIN pr_reviewers pr ON u.user_id = pr.user_id
		GROUP BY u.user_id, u.username
		HAVING COUNT(pr.pull_request_id) > 0
		ORDER BY review_count DESC
		LIMIT 10`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	topReviewers := []map[string]interface{}{}
	for rows.Next() {
		var userID, username string
		var count int
		if err := rows.Scan(&userID, &username, &count); err != nil {
			return nil, err
		}
		topReviewers = append(topReviewers, map[string]interface{}{
			"user_id":      userID,
			"username":     username,
			"review_count": count,
		})
	}
	stats["top_reviewers"] = topReviewers

	return stats, nil
}

func (s *Storage) DeactivateTeam(teamName string) ([]string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`SELECT user_id FROM users WHERE team_name = $1 AND is_active = true`, teamName)
	if err != nil {
		return nil, err
	}

	userIDs := []string{}
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			rows.Close()
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	rows.Close()

	if len(userIDs) == 0 {
		return userIDs, nil
	}

	_, err = tx.Exec(`
		UPDATE users 
		SET is_active = false, updated_at = $1 
		WHERE team_name = $2 AND is_active = true`, time.Now(), teamName)
	if err != nil {
		return nil, err
	}

	return userIDs, tx.Commit()
}

func (s *Storage) GetOpenPRsForReviewers(userIDs []string) ([]string, error) {
	if len(userIDs) == 0 {
		return []string{}, nil
	}

	query := `
		SELECT DISTINCT p.pull_request_id
		FROM pull_requests p
		JOIN pr_reviewers pr ON p.pull_request_id = pr.pull_request_id
		WHERE p.status = 'OPEN' AND pr.user_id IN (`

	args := []interface{}{}
	for i, userID := range userIDs {
		if i > 0 {
			query += ", "
		}
		query += fmt.Sprintf("$%d", i+1)
		args = append(args, userID)
	}
	query += ")"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prIDs := []string{}
	for rows.Next() {
		var prID string
		if err := rows.Scan(&prID); err != nil {
			return nil, err
		}
		prIDs = append(prIDs, prID)
	}
	return prIDs, nil
}

package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

const baseURL = "http://localhost:8080"

func TestFullWorkflow(t *testing.T) {
	time.Sleep(2 * time.Second)

	teamName := fmt.Sprintf("test_team_%d", time.Now().Unix())

	t.Run("CreateTeam", func(t *testing.T) {
		team := map[string]interface{}{
			"team_name": teamName,
			"members": []map[string]interface{}{
				{"user_id": "test_u1", "username": "TestUser1", "is_active": true},
				{"user_id": "test_u2", "username": "TestUser2", "is_active": true},
				{"user_id": "test_u3", "username": "TestUser3", "is_active": true},
				{"user_id": "test_u4", "username": "TestUser4", "is_active": true},
			},
		}

		resp, err := postJSON("/team/add", team)
		if err != nil {
			t.Fatalf("Failed to create team: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, string(body))
		}
	})

	t.Run("GetTeam", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/team/get?team_name=%s", baseURL, teamName))
		if err != nil {
			t.Fatalf("Failed to get team: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		if result["team_name"] != teamName {
			t.Errorf("Expected team_name %s, got %v", teamName, result["team_name"])
		}
	})

	prID := fmt.Sprintf("test_pr_%d", time.Now().Unix())

	t.Run("CreatePR_WithAutoAssignment", func(t *testing.T) {
		pr := map[string]interface{}{
			"pull_request_id":   prID,
			"pull_request_name": "Test PR",
			"author_id":         "test_u1",
		}

		resp, err := postJSON("/pullRequest/create", pr)
		if err != nil {
			t.Fatalf("Failed to create PR: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, string(body))
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		prData := result["pr"].(map[string]interface{})
		reviewers := prData["assigned_reviewers"].([]interface{})

		if len(reviewers) != 2 {
			t.Errorf("Expected 2 reviewers, got %d", len(reviewers))
		}
	})

	t.Run("GetUserReviews", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/users/getReview?user_id=test_u2", baseURL))
		if err != nil {
			t.Fatalf("Failed to get user reviews: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		prs := result["pull_requests"].([]interface{})
		if len(prs) == 0 {
			t.Error("Expected at least 1 PR for reviewer")
		}
	})

	t.Run("ReassignReviewer", func(t *testing.T) {
		reassign := map[string]interface{}{
			"pull_request_id": prID,
			"old_user_id":     "test_u2",
		}

		resp, err := postJSON("/pullRequest/reassign", reassign)
		if err != nil {
			t.Fatalf("Failed to reassign: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
		}
	})

	t.Run("MergePR", func(t *testing.T) {
		merge := map[string]interface{}{
			"pull_request_id": prID,
		}

		resp, err := postJSON("/pullRequest/merge", merge)
		if err != nil {
			t.Fatalf("Failed to merge PR: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		prData := result["pr"].(map[string]interface{})
		if prData["status"] != "MERGED" {
			t.Errorf("Expected status MERGED, got %v", prData["status"])
		}
	})

	t.Run("ReassignAfterMerge_ShouldFail", func(t *testing.T) {
		reassign := map[string]interface{}{
			"pull_request_id": prID,
			"old_user_id":     "test_u3",
		}

		resp, err := postJSON("/pullRequest/reassign", reassign)
		if err != nil {
			t.Fatalf("Failed request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("Expected status 409, got %d", resp.StatusCode)
		}
	})

	t.Run("MergePR_Idempotent", func(t *testing.T) {
		merge := map[string]interface{}{
			"pull_request_id": prID,
		}

		resp, err := postJSON("/pullRequest/merge", merge)
		if err != nil {
			t.Fatalf("Failed to merge PR: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("DeactivateUser", func(t *testing.T) {
		deactivate := map[string]interface{}{
			"user_id":   "test_u3",
			"is_active": false,
		}

		resp, err := postJSON("/users/setIsActive", deactivate)
		if err != nil {
			t.Fatalf("Failed to deactivate user: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		userData := result["user"].(map[string]interface{})
		if userData["is_active"] != false {
			t.Error("User should be inactive")
		}
	})

	t.Run("GetStatistics", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/statistics", baseURL))
		if err != nil {
			t.Fatalf("Failed to get statistics: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		var stats map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&stats)

		if _, ok := stats["total_prs"]; !ok {
			t.Error("Statistics should contain total_prs")
		}
		if _, ok := stats["top_reviewers"]; !ok {
			t.Error("Statistics should contain top_reviewers")
		}
	})
}

func TestEdgeCases(t *testing.T) {
	t.Run("CreateTeam_AlreadyExists", func(t *testing.T) {
		teamName := fmt.Sprintf("duplicate_team_%d", time.Now().Unix())
		team := map[string]interface{}{
			"team_name": teamName,
			"members": []map[string]interface{}{
				{"user_id": "dup_u1", "username": "DupUser1", "is_active": true},
			},
		}

		postJSON("/team/add", team)

		resp, err := postJSON("/team/add", team)
		if err != nil {
			t.Fatalf("Failed request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("CreatePR_SoloTeam", func(t *testing.T) {
		soloTeam := fmt.Sprintf("solo_%d", time.Now().Unix())
		team := map[string]interface{}{
			"team_name": soloTeam,
			"members": []map[string]interface{}{
				{"user_id": "solo_u1", "username": "SoloUser", "is_active": true},
			},
		}
		postJSON("/team/add", team)

		pr := map[string]interface{}{
			"pull_request_id":   fmt.Sprintf("solo_pr_%d", time.Now().Unix()),
			"pull_request_name": "Solo PR",
			"author_id":         "solo_u1",
		}

		resp, err := postJSON("/pullRequest/create", pr)
		if err != nil {
			t.Fatalf("Failed to create PR: %v", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		prData := result["pr"].(map[string]interface{})
		reviewers := prData["assigned_reviewers"].([]interface{})

		if len(reviewers) != 0 {
			t.Errorf("Expected 0 reviewers for solo team, got %d", len(reviewers))
		}
	})
}

func postJSON(path string, data interface{}) (*http.Response, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return http.Post(baseURL+path, "application/json", bytes.NewBuffer(jsonData))
}

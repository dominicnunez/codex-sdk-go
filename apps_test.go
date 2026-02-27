package codex_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

// TestAppsList verifies the apps/list method round-trip using MockTransport.
func TestAppsList(t *testing.T) {
	t.Run("minimal", func(t *testing.T) {
		mock := NewMockTransport()
		client := codex.NewClient(mock)

		// Mock response with single app
		_ = mock.SetResponseData("app/list", map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":   "test-app-123",
					"name": "Test App",
				},
			},
		})

		resp, err := client.Apps.List(context.Background(), codex.AppsListParams{})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if len(resp.Data) != 1 {
			t.Fatalf("expected 1 app, got %d", len(resp.Data))
		}

		app := resp.Data[0]
		if app.ID != "test-app-123" {
			t.Errorf("expected ID test-app-123, got %s", app.ID)
		}
		if app.Name != "Test App" {
			t.Errorf("expected Name 'Test App', got %s", app.Name)
		}
	})

	t.Run("with pagination and options", func(t *testing.T) {
		mock := NewMockTransport()
		client := codex.NewClient(mock)

		// Mock response with multiple apps and pagination
		_ = mock.SetResponseData("app/list", map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":               "app-1",
					"name":             "App One",
					"description":      "First app",
					"isAccessible":     true,
					"isEnabled":        true,
					"logoUrl":          "https://example.com/logo.png",
					"logoUrlDark":      "https://example.com/logo-dark.png",
					"installUrl":       "https://example.com/install",
					"distributionChannel": "chatgpt",
					"labels": map[string]string{
						"category": "productivity",
					},
					"branding": map[string]interface{}{
						"isDiscoverableApp": true,
						"category":          "productivity",
						"developer":         "Acme Inc",
						"website":           "https://acme.com",
						"privacyPolicy":     "https://acme.com/privacy",
						"termsOfService":    "https://acme.com/tos",
					},
					"appMetadata": map[string]interface{}{
						"categories":    []string{"productivity", "tools"},
						"subCategories": []string{"automation"},
						"developer":     "Acme Inc",
						"version":       "1.0.0",
						"versionId":     "v1",
						"versionNotes":  "Initial release",
						"seoDescription": "A productivity app",
						"firstPartyType": "connector",
						"firstPartyRequiresInstall": true,
						"showInComposerWhenUnlinked": false,
						"screenshots": []map[string]interface{}{
							{
								"userPrompt": "Screenshot 1",
								"url":        "https://example.com/screenshot1.png",
								"fileId":     "file-123",
							},
						},
						"review": map[string]interface{}{
							"status": "approved",
						},
					},
				},
			},
			"nextCursor": "cursor-abc",
		})

		params := codex.AppsListParams{
			Cursor:       ptr("previous-cursor"),
			Limit:        ptr(uint32(10)),
			ForceRefetch: true,
			ThreadID:     ptr("thread-123"),
		}

		resp, err := client.Apps.List(context.Background(), params)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if len(resp.Data) != 1 {
			t.Fatalf("expected 1 app, got %d", len(resp.Data))
		}

		app := resp.Data[0]
		if app.ID != "app-1" {
			t.Errorf("expected ID app-1, got %s", app.ID)
		}
		if app.Description == nil || *app.Description != "First app" {
			t.Errorf("expected description 'First app', got %v", app.Description)
		}
		if !app.IsAccessible {
			t.Error("expected IsAccessible true")
		}
		if !app.IsEnabled {
			t.Error("expected IsEnabled true")
		}
		if app.Branding == nil {
			t.Fatal("expected Branding to be non-nil")
		}
		if !app.Branding.IsDiscoverableApp {
			t.Error("expected IsDiscoverableApp true")
		}
		if app.AppMetadata == nil {
			t.Fatal("expected AppMetadata to be non-nil")
		}
		if app.AppMetadata.Categories == nil || len(*app.AppMetadata.Categories) != 2 {
			t.Error("expected 2 categories")
		}
		if app.AppMetadata.Screenshots == nil || len(*app.AppMetadata.Screenshots) != 1 {
			t.Error("expected 1 screenshot")
		}
		if app.AppMetadata.Review == nil {
			t.Error("expected review to be non-nil")
		}

		if resp.NextCursor == nil || *resp.NextCursor != "cursor-abc" {
			t.Errorf("expected nextCursor 'cursor-abc', got %v", resp.NextCursor)
		}

		// Verify params were serialized correctly
		req := mock.GetSentRequest(0)
		if req == nil {
			t.Fatal("expected request to be sent")
		}
		if req.Method != "app/list" {
			t.Errorf("expected method apps/list, got %s", req.Method)
		}

		var sentParams codex.AppsListParams
		if err := json.Unmarshal(req.Params, &sentParams); err != nil {
			t.Fatalf("failed to unmarshal params: %v", err)
		}
		if sentParams.Cursor == nil || *sentParams.Cursor != "previous-cursor" {
			t.Error("cursor not serialized correctly")
		}
		if sentParams.Limit == nil || *sentParams.Limit != 10 {
			t.Error("limit not serialized correctly")
		}
		if !sentParams.ForceRefetch {
			t.Error("forceRefetch not serialized correctly")
		}
		if sentParams.ThreadID == nil || *sentParams.ThreadID != "thread-123" {
			t.Error("threadId not serialized correctly")
		}
	})

	t.Run("empty list", func(t *testing.T) {
		mock := NewMockTransport()
		client := codex.NewClient(mock)

		_ = mock.SetResponseData("app/list", map[string]interface{}{
			"data": []map[string]interface{}{},
		})

		resp, err := client.Apps.List(context.Background(), codex.AppsListParams{})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if len(resp.Data) != 0 {
			t.Errorf("expected empty data, got %d items", len(resp.Data))
		}
		if resp.NextCursor != nil {
			t.Error("expected nil nextCursor for empty list")
		}
	})
}

// TestAppListUpdatedNotification verifies the app/listUpdated notification dispatch.
func TestAppListUpdatedNotification(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	var receivedNotif *codex.AppListUpdatedNotification
	client.OnAppListUpdated(func(notif codex.AppListUpdatedNotification) {
		receivedNotif = &notif
	})

	// Inject notification
	notifData := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"id":   "new-app-456",
				"name": "New App",
			},
		},
	}
	notifJSON, _ := json.Marshal(notifData)
	mock.InjectServerNotification(context.Background(), codex.Notification{
		JSONRPC: "2.0",
		Method:  "app/list/updated",
		Params:  notifJSON,
	})

	if receivedNotif == nil {
		t.Fatal("notification handler was not called")
	}

	if len(receivedNotif.Data) != 1 {
		t.Fatalf("expected 1 app, got %d", len(receivedNotif.Data))
	}

	app := receivedNotif.Data[0]
	if app.ID != "new-app-456" {
		t.Errorf("expected ID new-app-456, got %s", app.ID)
	}
	if app.Name != "New App" {
		t.Errorf("expected Name 'New App', got %s", app.Name)
	}
}

// TestAppsServiceMethodSignatures is a compile-time test to verify the AppsService has the expected methods.
func TestAppsServiceMethodSignatures(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// This will fail to compile if the method signature is wrong
	var _ func(context.Context, codex.AppsListParams) (codex.AppsListResponse, error) = client.Apps.List
}

// Copyright The Linux Foundation and contributors.
// SPDX-License-Identifier: MIT

package tools

import (
	"context"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestOCGTools_RegisterReadOnly(t *testing.T) {
	for _, tc := range []struct {
		name     string
		register func(*mcp.Server)
	}{
		{"search_ocg_meetups", RegisterSearchOCGMeetups},
		{"list_ocg_meetup_filters", RegisterListOCGMeetupFilters},
	} {
		tool := listRegisteredTool(t, tc.name, tc.register)
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("%s must carry ReadOnlyHint", tc.name)
		}
	}
}

// The description must route away from the three surfaces an agent confuses
// meetups with, and toward the filter tool for community spelling.
func TestSearchOCGMeetupsDescription_RoutesCorrectly(t *testing.T) {
	tool := listRegisteredTool(t, "search_ocg_meetups", RegisterSearchOCGMeetups)
	for _, want := range []string{
		"ocgroups.dev",
		"search_meetings",
		"search_committees",
		"KubeCon",
		"list_ocg_meetup_filters",
		"limit/offset",
	} {
		if !strings.Contains(tool.Description, want) {
			t.Errorf("search_ocg_meetups description missing %q", want)
		}
	}
}

func TestSearchOCGMeetups_SendsArgumentsAsQueryParams(t *testing.T) {
	captured := setupLensTest(t)

	res, _, err := handleSearchOCGMeetups(context.Background(), &mcp.CallToolRequest{}, SearchOCGMeetupsArgs{
		Community: "CNCF",
		Query:     "kubernetes",
		Location:  "Austin",
		Since:     "2026-09-01",
		Until:     "2026-12-31",
		Limit:     25,
		Offset:    50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %s", resultText(t, res))
	}
	if captured.Method != http.MethodGet || captured.Path != "/lfx-lens/ocg/meetups" {
		t.Errorf("unexpected request: %s %s", captured.Method, captured.Path)
	}
	want := url.Values{
		"community": {"CNCF"},
		"q":         {"kubernetes"},
		"location":  {"Austin"},
		"since":     {"2026-09-01"},
		"until":     {"2026-12-31"},
		"limit":     {"25"},
		"offset":    {"50"},
	}
	if !reflect.DeepEqual(captured.Query, want) {
		t.Errorf("query = %v, want %v", captured.Query, want)
	}
}

func TestSearchOCGMeetups_OmitsUnsetArguments(t *testing.T) {
	captured := setupLensTest(t)

	if _, _, err := handleSearchOCGMeetups(context.Background(), &mcp.CallToolRequest{}, SearchOCGMeetupsArgs{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(captured.Query) != 0 {
		t.Errorf("empty args must send no query params, got %v", captured.Query)
	}
}

func TestListOCGMeetupFilters_CallsTheFiltersEndpoint(t *testing.T) {
	captured := setupLensTest(t)

	res, _, err := handleListOCGMeetupFilters(context.Background(), &mcp.CallToolRequest{}, ListOCGMeetupFiltersArgs{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %s", resultText(t, res))
	}
	if captured.Method != http.MethodGet || captured.Path != "/lfx-lens/ocg/meetup-filters" {
		t.Errorf("unexpected request: %s %s", captured.Method, captured.Path)
	}
}

// The lens words its own rejections; the detail is returned verbatim.
func TestOCGTools_ReturnLensDetailOnError(t *testing.T) {
	setupFakeLens(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"detail":"unknown community 'cncf'; call list_ocg_meetup_filters"}`))
	})

	res, _, err := handleSearchOCGMeetups(context.Background(), &mcp.CallToolRequest{}, SearchOCGMeetupsArgs{Community: "cncf"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error result")
	}
	if got, want := resultText(t, res), "unknown community 'cncf'; call list_ocg_meetup_filters"; got != want {
		t.Errorf("error text = %q, want %q", got, want)
	}
}

func TestOCGTools_UnconfiguredLensIsAnError(t *testing.T) {
	prev := lensConfig
	lensConfig = nil
	t.Cleanup(func() { lensConfig = prev })

	if _, _, err := handleSearchOCGMeetups(context.Background(), &mcp.CallToolRequest{}, SearchOCGMeetupsArgs{}); err == nil {
		t.Error("search_ocg_meetups must fail when lens is not configured")
	}
	if _, _, err := handleListOCGMeetupFilters(context.Background(), &mcp.CallToolRequest{}, ListOCGMeetupFiltersArgs{}); err == nil {
		t.Error("list_ocg_meetup_filters must fail when lens is not configured")
	}
}

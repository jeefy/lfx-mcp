// Copyright The Linux Foundation and contributors.
// SPDX-License-Identifier: MIT

package tools

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// setupCommitteeTest points the committee tools at a stub LFX API.
func setupCommitteeTest(t *testing.T) *stubLFXAPI {
	t.Helper()
	api := newStubLFXAPI(t)
	prev := committeeConfig
	SetCommitteeConfig(&CommitteeConfig{Clients: api.Clients})
	t.Cleanup(func() { committeeConfig = prev })
	return api
}

// setupMailingListTest points the mailing list tools at a stub LFX API.
func setupMailingListTest(t *testing.T) *stubLFXAPI {
	t.Helper()
	api := newStubLFXAPI(t)
	prev := mailingListConfig
	SetMailingListConfig(&MailingListConfig{Clients: api.Clients})
	t.Cleanup(func() { mailingListConfig = prev })
	return api
}

// assertTagsAllQuery checks that the recorded query-resources request carries
// the given filters in tags_all (AND) and nothing in tags (OR).
func assertTagsAllQuery(t *testing.T, r stubAPIRequest, wantTagsAll []string) {
	t.Helper()
	if r.Method != http.MethodGet || r.Path != resourcesPath {
		t.Fatalf("expected GET %s, got %s %s", resourcesPath, r.Method, r.Path)
	}
	assertExchangedAuth(t, r)
	if got := r.Query["tags_all"]; strings.Join(got, "|") != strings.Join(wantTagsAll, "|") {
		t.Errorf("query tags_all: want %v, got %v", wantTagsAll, got)
	}
	if got, ok := r.Query["tags"]; ok {
		t.Errorf("query must not carry tags (OR); got %v", got)
	}
}

func TestSearchCommitteeMembers_FiltersCombineWithAND(t *testing.T) {
	api := setupCommitteeTest(t)
	api.Respond(resourcesPath, page(nil, ""))

	res, _, err := handleSearchCommitteeMembers(context.Background(), stubCallToolRequest(), SearchCommitteeMembersArgs{
		CommitteeUID: "C1",
		ProjectUID:   "P1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %s", allResultText(t, res))
	}
	assertTagsAllQuery(t, api.LastRequest(), []string{"committee_uid:C1", "project_uid:P1"})
}

func TestSearchGroupMembers_ForwardsFiltersAsAND(t *testing.T) {
	api := setupCommitteeTest(t)
	api.Respond(resourcesPath, page(nil, ""))

	res, _, err := handleSearchCommitteeMembersGroupMode(context.Background(), stubCallToolRequest(), SearchGroupMembersArgs{
		GroupUID:   "C1",
		ProjectUID: "P1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %s", allResultText(t, res))
	}
	assertTagsAllQuery(t, api.LastRequest(), []string{"committee_uid:C1", "project_uid:P1"})
}

func TestSearchPastMeetings_FiltersCombineWithAND(t *testing.T) {
	api := setupParticipantTest(t)
	api.Respond(resourcesPath, page(nil, ""))

	res, _, err := handleSearchPastMeetings(context.Background(), stubCallToolRequest(), SearchPastMeetingsArgs{
		CommitteeUID: "C1",
		MeetingID:    "M1",
		ProjectUID:   "P1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %s", allResultText(t, res))
	}
	r := api.LastRequest()
	assertTagsAllQuery(t, r, []string{"committee_uid:C1", "meeting_id:M1"})
	if got := r.Query.Get("parent"); got != "project:P1" {
		t.Errorf("query parent: want project:P1, got %q", got)
	}
}

func TestSearchMailingListMembers_FiltersCombineWithAND(t *testing.T) {
	api := setupMailingListTest(t)
	api.Respond(resourcesPath, page(nil, ""))

	res, _, err := handleSearchMailingListMembers(context.Background(), stubCallToolRequest(), SearchMailingListMembersArgs{
		MailingListID: "145670",
		ProjectUID:    "P1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %s", allResultText(t, res))
	}
	assertTagsAllQuery(t, api.LastRequest(), []string{"mailing_list_uid:145670", "project_uid:P1"})
}

// TestNarrowingSearchDescriptionsStateANDFilters pins the clause that tells
// callers two filters intersect rather than union.
func TestNarrowingSearchDescriptionsStateANDFilters(t *testing.T) {
	const want = "Filters combine with AND: a record must match every filter given."
	for _, tc := range []struct {
		name     string
		register func(*mcp.Server)
	}{
		{"search_committee_members", func(s *mcp.Server) { RegisterSearchCommitteeMembers(s, false) }},
		{"search_group_members", func(s *mcp.Server) { RegisterSearchCommitteeMembers(s, true) }},
		{"search_past_meetings", func(s *mcp.Server) { RegisterSearchPastMeetings(s, false) }},
		{"search_past_meetings", func(s *mcp.Server) { RegisterSearchPastMeetings(s, true) }},
		{"search_mailing_list_members", RegisterSearchMailingListMembers},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tool := listRegisteredTool(t, tc.name, tc.register)
			if !strings.Contains(tool.Description, want) {
				t.Errorf("%s description missing %q", tc.name, want)
			}
		})
	}
}

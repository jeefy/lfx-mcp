// Copyright The Linux Foundation and contributors.
// SPDX-License-Identifier: MIT

// Package tools provides MCP tool implementations for the LFX MCP server.
package tools

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// Meeting-side fixtures for the LFX Self Serve parity rules. Every address is
// under example.test so a test can assert that none is returned.

// registrantDoc is one v1_meeting_registrant query-service resource.
func registrantDoc(uid, meetingID, email string, host bool) string {
	return fmt.Sprintf(`{
	  "type": "v1_meeting_registrant",
	  "id": %q,
	  "data": {
	    "uid": %q,
	    "meeting_id": %q,
	    "occurrence": "1771596000000",
	    "email": %q,
	    "case_insensitive_email": %q,
	    "first_name": "Reg",
	    "last_name": "Istrant",
	    "username": "user-%s",
	    "user_id": "auth0|%s",
	    "avatar_url": "https://avatars.example.test/%s.png",
	    "host": %t,
	    "job_title": "Engineer",
	    "org_name": "Example Org",
	    "org_is_member": true,
	    "type": "committee",
	    "committee_uid": "9f0c2e6a-6b1e-4c3e-9d1a-0c1b2a3d4e5f",
	    "last_invite_sent_at": "2026-06-01T00:00:00Z",
	    "last_invite_delivery_status": "delivered",
	    "created_by": {"email": "creator@example.test", "name": "Creator"},
	    "updated_by": {"email": "editor@example.test", "name": "Editor"}
	  }
	}`, uid, uid, meetingID, email, strings.ToLower(email), uid, uid, uid, host)
}

// meetingDocWithPeople is a v1_meeting record carrying every people field
// the rule touches, plus the join-page password that stays.
const meetingDocWithPeople = `{
  "id": "meeting-1",
  "title": "Weekly sync",
  "project_uid": "11111111-1111-1111-1111-111111111111",
  "password": "join-page-password",
  "user_id": "auth0|owner",
  "registrant_count": 12,
  "organizers": ["auth0|a", "auth0|b"],
  "created_by": {"user_id": "u1", "username": "creator", "email": "creator@example.test", "name": "Creator", "profile_picture": "p"},
  "updated_by": {"user_id": "u2", "username": "editor", "email": "editor@example.test", "name": "Editor"},
  "updated_by_list": [{"email": "editor@example.test", "name": "Editor"}],
  "owner": {"user_id": "u1", "username": "creator", "email": "owner@example.test", "name": "Owner", "profile_picture": "p"},
  "occurrences": [
    {"occurrence_id": "1", "start_time": "2026-07-01T15:00:00Z", "registrant_count": 7, "response_count_yes": 3},
    {"occurrence_id": "2", "start_time": "2026-07-08T15:00:00Z", "registrant_count": 8}
  ]
}`

// assertMeetingPeopleTrimmed checks the non-full-view projection of a
// v1_meeting record built from meetingDocWithPeople.
func assertMeetingPeopleTrimmed(t *testing.T, data map[string]any) {
	t.Helper()
	for _, key := range []string{"updated_by", "updated_by_list", "organizers", "user_id", "registrant_count"} {
		if _, has := data[key]; has {
			t.Errorf("%s must be removed, got %v", key, data[key])
		}
	}
	for _, key := range []string{"created_by", "owner"} {
		obj, _ := data[key].(map[string]any)
		if len(obj) != 2 || obj["name"] == nil || obj["email"] == nil {
			t.Errorf("%s must be reduced to name and email, got %v", key, data[key])
		}
	}
	if data["password"] != "join-page-password" {
		t.Error("the join-page password stays")
	}
	occurrences, _ := data["occurrences"].([]any)
	if len(occurrences) != 2 {
		t.Fatalf("occurrences must stay, got %v", data["occurrences"])
	}
	for _, o := range occurrences {
		occ := o.(map[string]any)
		if _, has := occ["registrant_count"]; has {
			t.Error("occurrence registrant_count must be removed")
		}
		if occ["start_time"] == nil {
			t.Error("other occurrence fields stay")
		}
	}
}

func TestSearchMeetings_PeopleFieldsFollowFullView(t *testing.T) {
	for _, full := range []bool{true, false} {
		t.Run(fmt.Sprintf("full view %v", full), func(t *testing.T) {
			api := setupMeetingLookupTest(t)
			pinMeetingSearchNow(t, beforeJoinFieldsOccurrences)
			api.Respond(resourcesPath, singleResourcePage("v1_meeting", "meeting-1", meetingDocWithPeople))
			ctx := context.Background()
			if full {
				ctx = fullViewCtx()
			}
			res, _, _ := handleSearchMeetings(ctx, stubCallToolRequest(), SearchMeetingsArgs{ProjectUID: "11111111-1111-1111-1111-111111111111"})
			if res.IsError {
				t.Fatalf("unexpected error result: %s", allResultText(t, res))
			}
			data := resultJSON(t, res)["resources"].([]any)[0].(map[string]any)["Data"].(map[string]any)
			if full {
				if data["updated_by_list"] == nil || data["organizers"] == nil || data["registrant_count"] != float64(12) {
					t.Errorf("full view must keep every field: %v", data)
				}
				return
			}
			assertMeetingPeopleTrimmed(t, data)
			if len(api.RequestsTo(accessCheckPath)) != 0 {
				t.Error("meeting records need no relation check")
			}
		})
	}
}

func TestGetMeeting_PeopleFieldsTrimmedWithoutFullView(t *testing.T) {
	api := setupMeetingLookupTest(t)
	api.Respond(resourcesPath, singleResourcePage("v1_meeting", "meeting-1", meetingDocWithPeople))
	res, _, _ := handleGetMeeting(context.Background(), stubCallToolRequest(), GetMeetingArgs{UID: "meeting-1"})
	if res.IsError {
		t.Fatalf("unexpected error result: %s", allResultText(t, res))
	}
	assertMeetingPeopleTrimmed(t, resultJSON(t, res)["Data"].(map[string]any))
}

// --- registrants ---

const (
	meetingOrganized  = "11111111111"
	meetingRegistered = "22222222222"
	meetingHidden     = "33333333333"
)

// registrantPage is one registrant per meeting plus the caller's own record
// in the registered meeting.
func registrantPage(token string, meetings ...string) string {
	var docs []string
	for i, m := range meetings {
		docs = append(docs, registrantDoc(fmt.Sprintf("r%d", i), m, fmt.Sprintf("person%d@example.test", i), i == 0))
	}
	docs = append(docs, registrantDoc("self", meetingRegistered, stubCallerEmail, false))
	return page(docs, token)
}

// selfRegistrantLookup is the answer to the caller's own-registration query.
func selfRegistrantLookup(meetings ...string) string {
	var docs []string
	for _, m := range meetings {
		docs = append(docs, registrantDoc("self-"+m, m, stubCallerEmail, false))
	}
	return page(docs, "")
}

func TestSearchMeetingRegistrants_FullViewIsUnchangedAndMakesNoChecks(t *testing.T) {
	api := setupMeetingLookupTest(t)
	api.GrantRelations()
	api.Respond(resourcesPath, registrantPage("", meetingOrganized, meetingHidden))
	_, out, err := handleSearchMeetingRegistrants(fullViewCtx(), stubCallToolRequest(), SearchMeetingRegistrantsArgs{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Resources) != 3 || out.Resources[0].Data["email"] != "person0@example.test" || len(api.Requests()) != 1 {
		t.Errorf("full view must return the page unchanged with one call: %+v / %d requests", out.Resources, len(api.Requests()))
	}
}

func TestSearchMeetingRegistrants_ViewsPerMeeting(t *testing.T) {
	api := setupMeetingLookupTest(t)
	api.GrantRelations("v1_meeting:" + meetingOrganized + "#organizer")
	api.Respond(resourcesPath, registrantPage("next", meetingOrganized, meetingRegistered, meetingHidden))
	// A self-registration answer naming the organized meeting (outside the
	// asked filter) must not lower the organizer's view.
	api.Respond(resourcesPath, selfRegistrantLookup(meetingRegistered, meetingOrganized))

	res, out, err := handleSearchMeetingRegistrants(context.Background(), stubCallToolRequest(), SearchMeetingRegistrantsArgs{PageSize: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := allResultText(t, res)
	if strings.Contains(text, "person1@example.test") || strings.Contains(text, "person2@example.test") {
		t.Errorf("an address the rule hides reached the caller:\n%s", text)
	}
	if out.PageToken != "next" {
		t.Error("the page token must be kept")
	}
	if len(out.Resources) != 3 {
		t.Fatalf("expected organizer record + roster record + own record, got %d:\n%s", len(out.Resources), text)
	}
	byUID := map[string]map[string]any{}
	for _, r := range out.Resources {
		byUID[r.Data["uid"].(string)] = r.Data
	}
	if organized := byUID["r0"]; organized["email"] != "person0@example.test" || organized["last_invite_delivery_status"] == nil {
		t.Errorf("organizer record must be unchanged: %v", organized)
	}
	roster := byUID["r1"]
	want := []string{"avatar_url", "committee_uid", "first_name", "host", "job_title", "last_name", "meeting_id", "occurrence", "org_name", "type", "uid"}
	if keys := sortedKeys(roster); strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Errorf("roster keys = %v, want %v", keys, want)
	}
	if own := byUID["self"]; own["email"] != stubCallerEmail || own["username"] == nil {
		t.Errorf("the caller's own record must be unchanged: %v", own)
	}
	if _, hidden := byUID["r2"]; hidden {
		t.Error("the hidden meeting's record must be dropped")
	}
	// One access-check for the three meetings, one self lookup for the two
	// the caller does not organize, on the e-mail tag raw and lowercased.
	bodies := api.AccessCheckBodies()
	if len(bodies) != 1 || len(bodies[0]) != 3 {
		t.Errorf("expected one batch of three organizer checks, got %v", bodies)
	}
	assertExchangedAuth(t, api.RequestsTo(accessCheckPath)[0])
	lookups := api.RequestsTo(resourcesPath)
	if len(lookups) != 2 {
		t.Fatalf("expected the search and one self lookup, got %d", len(lookups))
	}
	self := lookups[1]
	if self.Query.Get("type") != "v1_meeting_registrant" {
		t.Errorf("self lookup type = %q", self.Query.Get("type"))
	}
	if tags := self.Query["tags"]; strings.Join(tags, " ") != "email:"+stubCallerEmail+" email:"+strings.ToLower(stubCallerEmail) {
		t.Errorf("self lookup tags = %v", tags)
	}
	if filters := self.Query["filters_or"]; strings.Join(filters, " ") != "meeting_id:"+meetingRegistered+" meeting_id:"+meetingHidden {
		t.Errorf("self lookup filters_or = %v", filters)
	}
	assertExchangedAuth(t, self)
	if len(out.Warnings) == 0 {
		t.Error("a short page with a token carries the visibility warning")
	}
}

func TestSearchMeetingRegistrants_NoEmailClaimMeansNoRosterView(t *testing.T) {
	api := setupMeetingLookupTest(t)
	api.GrantRelations()
	api.Respond(resourcesPath, registrantPage("", meetingRegistered))
	req := stubCallToolRequest()
	delete(req.Extra.TokenInfo.Extra, ClaimEmail)
	_, out, err := handleSearchMeetingRegistrants(context.Background(), req, SearchMeetingRegistrantsArgs{MeetingID: meetingRegistered})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Resources) != 0 || len(api.RequestsTo(resourcesPath)) != 1 {
		t.Errorf("without an e-mail claim nothing is shown and no self lookup runs: %d records, %d queries", len(out.Resources), len(api.RequestsTo(resourcesPath)))
	}
}

func TestSearchMeetingRegistrants_FailsClosed(t *testing.T) {
	cases := []struct {
		name  string
		setup func(api *stubLFXAPI)
	}{
		{"access-check 503", func(api *stubLFXAPI) {
			api.FailAccessCheck(http.StatusServiceUnavailable)
			api.Respond(resourcesPath, registrantPage("", meetingHidden))
		}},
		{"self lookup 502", func(api *stubLFXAPI) {
			api.GrantRelations()
			api.Respond(resourcesPath, registrantPage("", meetingHidden))
			api.RespondStatus(resourcesPath, http.StatusBadGateway, "")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := setupMeetingLookupTest(t)
			tc.setup(api)
			_, out, err := handleSearchMeetingRegistrants(context.Background(), stubCallToolRequest(), SearchMeetingRegistrantsArgs{})
			if err == nil || err.Error() != peopleVisibilityUnavailableMessage || len(out.Resources) != 0 {
				t.Fatalf("expected the unavailable error and no records, got %v %v", err, out.Resources)
			}
		})
	}
}

func TestSearchMeetingRegistrants_NameFilterNeedsAnOrganizedMeeting(t *testing.T) {
	t.Run("no meeting_id", func(t *testing.T) {
		api := setupMeetingLookupTest(t)
		_, _, err := handleSearchMeetingRegistrants(context.Background(), stubCallToolRequest(), SearchMeetingRegistrantsArgs{CommitteeUID: "C1", Name: "Reg"})
		if err == nil || !strings.Contains(err.Error(), "meeting_id") || len(api.Requests()) != 0 {
			t.Fatalf("expected a refusal before any call, got %v", err)
		}
	})
	t.Run("registrant, not organizer", func(t *testing.T) {
		api := setupMeetingLookupTest(t)
		api.GrantRelations()
		api.Respond(resourcesPath, selfRegistrantLookup(meetingRegistered))
		_, _, err := handleSearchMeetingRegistrants(context.Background(), stubCallToolRequest(), SearchMeetingRegistrantsArgs{MeetingID: meetingRegistered, Name: "Reg"})
		if err == nil || !strings.Contains(err.Error(), "organize") {
			t.Fatalf("expected a refusal, got %v", err)
		}
	})
	t.Run("organizer passes", func(t *testing.T) {
		api := setupMeetingLookupTest(t)
		api.GrantRelations("v1_meeting:" + meetingOrganized + "#organizer")
		api.Respond(resourcesPath, registrantPage("", meetingOrganized))
		_, out, err := handleSearchMeetingRegistrants(context.Background(), stubCallToolRequest(), SearchMeetingRegistrantsArgs{MeetingID: meetingOrganized, Name: "Reg"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// The own record in the fixture belongs to another meeting and is
		// outside the checked scope, so it is not shown.
		if len(out.Resources) != 1 || len(api.AccessCheckBodies()) != 1 {
			t.Errorf("expected the organized meeting's record after one check, got %d records, %d checks", len(out.Resources), len(api.AccessCheckBodies()))
		}
	})
}

func TestGetMeetingRegistrant_Views(t *testing.T) {
	t.Run("organizer unchanged", func(t *testing.T) {
		api := setupMeetingLookupTest(t)
		api.GrantRelations("v1_meeting:" + meetingOrganized + "#organizer")
		api.Respond(resourcesPath, page([]string{registrantDoc("r1", meetingOrganized, "person@example.test", false)}, ""))
		res, _, _ := handleGetMeetingRegistrant(context.Background(), stubCallToolRequest(), GetMeetingRegistrantArgs{UID: "r1"})
		if res.IsError || resultJSON(t, res)["Data"].(map[string]any)["email"] != "person@example.test" {
			t.Errorf("organizer gets the record unchanged: %s", allResultText(t, res))
		}
	})
	t.Run("registrant gets the roster view", func(t *testing.T) {
		api := setupMeetingLookupTest(t)
		api.GrantRelations()
		api.Respond(resourcesPath, page([]string{registrantDoc("r1", meetingRegistered, "person@example.test", true)}, ""))
		api.Respond(resourcesPath, selfRegistrantLookup(meetingRegistered))
		res, _, _ := handleGetMeetingRegistrant(context.Background(), stubCallToolRequest(), GetMeetingRegistrantArgs{UID: "r1"})
		text := allResultText(t, res)
		if res.IsError {
			t.Fatalf("unexpected error: %s", text)
		}
		assertNoEmail(t, text)
		if data := resultJSON(t, res)["Data"].(map[string]any); data["host"] != true || data["first_name"] == nil || data["username"] != nil {
			t.Errorf("roster view wrong: %v", data)
		}
	})
	t.Run("hidden reads as not visible", func(t *testing.T) {
		api := setupMeetingLookupTest(t)
		api.GrantRelations()
		api.Respond(resourcesPath, page([]string{registrantDoc("r1", meetingHidden, "person@example.test", false)}, ""))
		api.Respond(resourcesPath, page(nil, ""))
		res, out, _ := handleGetMeetingRegistrant(context.Background(), stubCallToolRequest(), GetMeetingRegistrantArgs{UID: "r1"})
		if !res.IsError || strings.TrimSpace(allResultText(t, res)) != lookupNotVisibleMessage("meeting registrant", "r1") || out != nil {
			t.Fatalf("expected the not-visible message, got %s", allResultText(t, res))
		}
	})
	t.Run("fails closed", func(t *testing.T) {
		api := setupMeetingLookupTest(t)
		api.FailAccessCheck(http.StatusBadGateway)
		api.Respond(resourcesPath, page([]string{registrantDoc("r1", meetingHidden, "person@example.test", false)}, ""))
		res, _, _ := handleGetMeetingRegistrant(context.Background(), stubCallToolRequest(), GetMeetingRegistrantArgs{UID: "r1"})
		if !res.IsError || strings.TrimSpace(allResultText(t, res)) != peopleVisibilityUnavailableMessage {
			t.Fatalf("expected the unavailable error, got %s", allResultText(t, res))
		}
	})
}

// --- past meeting records, artifacts, summaries ---

func TestSearchPastMeetings_PeopleFieldsTrimmedWithoutFullView(t *testing.T) {
	const id = "91461158520-1771596000000"
	for _, full := range []bool{true, false} {
		t.Run(fmt.Sprintf("full view %v", full), func(t *testing.T) {
			api := setupMeetingLookupTest(t)
			api.Respond(resourcesPath, page([]string{pastMeetingDocWith(id, "public", false)}, ""))
			ctx := context.Background()
			if full {
				ctx = fullViewCtx()
			}
			res, out, err := handleSearchPastMeetings(ctx, stubCallToolRequest(), SearchPastMeetingsArgs{ProjectUID: "P1"})
			if err != nil || res.IsError {
				t.Fatalf("unexpected error: %v %s", err, allResultText(t, res))
			}
			data := out.Resources[0].Data
			if full {
				if data["updated_by_list"] == nil || len(data["created_by"].(map[string]any)) != 5 {
					t.Errorf("full view keeps every field: %v", data)
				}
				return
			}
			if _, has := data["updated_by"]; has {
				t.Error("updated_by must be removed")
			}
			if _, has := data["updated_by_list"]; has {
				t.Error("updated_by_list must be removed")
			}
			if cb := data["created_by"].(map[string]any); len(cb) != 2 || cb["name"] != "Creator" || cb["email"] != "creator@example.test" {
				t.Errorf("created_by must be name and email: %v", cb)
			}
			if data["visibility"] != "public" || data["title"] == nil {
				t.Error("other fields stay")
			}
		})
	}
}

func TestGetPastMeeting_ArtifactPeopleFieldsTrimmedWithoutFullView(t *testing.T) {
	artifact := func(typ, id string) stubAPIResponse {
		return stubAPIResponse{Status: http.StatusOK, Body: singleResourcePage(typ, id, `{"uid": "`+id+`", "host_email": "host@example.test", "host_id": "h1", "created_by": {"email": "creator@example.test"}, "updated_by": {"email": "editor@example.test"}, "share_url": "https://zoom.example.test/rec"}`)}
	}
	res, out := getPastMeetingWith(t, artifact("v1_past_meeting_recording", "rec-1"), artifact("v1_past_meeting_transcript", "tr-1"))
	assertNoEmail(t, allResultText(t, res))
	for name, r := range map[string]map[string]any{"recording": resourceData(out.Recording), "transcript": resourceData(out.Transcript)} {
		for _, key := range []string{"host_email", "host_id", "created_by", "updated_by"} {
			if _, has := r[key]; has {
				t.Errorf("%s %s must be removed", name, key)
			}
		}
		if r["share_url"] == nil {
			t.Errorf("%s links stay", name)
		}
	}
}

func TestGetPastMeeting_ArtifactPeopleFieldsKeptWithFullView(t *testing.T) {
	api := setupMeetingLookupTest(t)
	api.Respond(pastMeetingPath, pastMeetingBody)
	api.Respond(resourcesPath, singleResourcePage("v1_past_meeting_recording", "rec-1", `{"uid": "rec-1", "host_email": "host@example.test"}`))
	api.Respond(resourcesPath, emptyResourcePage)
	_, out, err := handleGetPastMeeting(fullViewCtx(), stubCallToolRequest(), GetPastMeetingArgs{UID: "past-1"})
	if err != nil || resourceData(out.Recording)["host_email"] != "host@example.test" {
		t.Fatalf("full view keeps host_email: %v %v", err, out.Recording)
	}
}

const summaryDoc = `{
  "type": "v1_past_meeting_summary",
  "id": "sum-1",
  "data": {
    "uid": "sum-1",
    "past_meeting_uid": "pm-1",
    "summary_title": "Weekly sync summary",
    "content": "Discussed the roadmap.",
    "edited_content": "",
    "approved": true,
    "zoom_meeting_host_id": "zh1",
    "zoom_meeting_host_email": "host@example.test",
    "created_by": {"email": "creator@example.test", "name": "Creator"},
    "updated_by": {"email": "editor@example.test", "name": "Editor"}
  }
}`

func assertSummaryPeopleTrimmed(t *testing.T, data map[string]any) {
	t.Helper()
	for _, key := range []string{"zoom_meeting_host_email", "zoom_meeting_host_id", "created_by", "updated_by"} {
		if _, has := data[key]; has {
			t.Errorf("%s must be removed", key)
		}
	}
	if data["content"] != "Discussed the roadmap." || data["summary_title"] == nil {
		t.Error("content and title stay")
	}
}

func TestSummaries_PeopleFieldsFollowFullView(t *testing.T) {
	t.Run("search without full view", func(t *testing.T) {
		api := setupMeetingLookupTest(t)
		api.Respond(resourcesPath, page([]string{summaryDoc}, ""))
		res, out, err := handleSearchPastMeetingSummaries(context.Background(), stubCallToolRequest(), SearchPastMeetingSummariesArgs{ProjectUID: "P1"})
		if err != nil {
			t.Fatal(err)
		}
		assertNoEmail(t, allResultText(t, res))
		assertSummaryPeopleTrimmed(t, out.Resources[0].Data)
	})
	t.Run("search with full view", func(t *testing.T) {
		api := setupMeetingLookupTest(t)
		api.Respond(resourcesPath, page([]string{summaryDoc}, ""))
		_, out, err := handleSearchPastMeetingSummaries(fullViewCtx(), stubCallToolRequest(), SearchPastMeetingSummariesArgs{ProjectUID: "P1"})
		if err != nil || out.Resources[0].Data["zoom_meeting_host_email"] != "host@example.test" {
			t.Fatalf("full view keeps the host: %v %v", err, out.Resources[0].Data)
		}
	})
	t.Run("get without full view", func(t *testing.T) {
		api := setupMeetingLookupTest(t)
		api.Respond(resourcesPath, page([]string{summaryDoc}, ""))
		res, _, _ := handleGetPastMeetingSummary(context.Background(), stubCallToolRequest(), GetPastMeetingSummaryArgs{UID: "sum-1"})
		if res.IsError {
			t.Fatal(allResultText(t, res))
		}
		assertNoEmail(t, allResultText(t, res))
		assertSummaryPeopleTrimmed(t, resultJSON(t, res)["Data"].(map[string]any))
		if len(api.RequestsTo(accessCheckPath)) != 0 {
			t.Error("summaries need no relation check")
		}
	})
}

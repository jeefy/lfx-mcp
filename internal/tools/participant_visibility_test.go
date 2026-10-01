// Copyright The Linux Foundation and contributors.
// SPDX-License-Identifier: MIT

package tools

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// Past-meeting participant fixtures for the LFX Self Serve parity rule.

const (
	pastOrganized  = "11111111111-1771596000000"
	pastPublic     = "22222222222-1771596000000"
	pastRestricted = "33333333333-1771596000000"
	pastCommittee  = "44444444444-1771596000000"
	pastHidden     = "55555555555-1771596000000"
	memberOfGroup  = "9f0c2e6a-6b1e-4c3e-9d1a-0c1b2a3d4e5f"
)

// participantDocFor is participantDoc with an explicit past meeting, host
// flag and username.
func participantDocFor(uid, occurrenceID, email, first, last, username string, host, attended bool) string {
	return fmt.Sprintf(`{
	  "type": "v1_past_meeting_participant",
	  "id": %q,
	  "data": {
	    "uid": %q,
	    "meeting_and_occurrence_id": %q,
	    "meeting_id": %q,
	    "project_uid": "a0941000002wBz4AAE",
	    "committee_uid": %q,
	    "email": %q,
	    "first_name": %q,
	    "last_name": %q,
	    "username": %q,
	    "host": %t,
	    "job_title": "Engineer",
	    "org_name": "Example Org",
	    "avatar_url": "https://avatars.example.test/%s.png",
	    "is_invited": true,
	    "is_attended": %t,
	    "zoom_user_name": %q,
	    "sessions": [{"uid": "s1", "join_time": "2026-06-10T15:02:11Z"}]
	  }
	}`, uid, uid, occurrenceID, strings.Split(occurrenceID, "-")[0], memberOfGroup, email, first, last, username, host, uid, attended, first+" "+last)
}

// pastDocs answers the past-meeting record lookup for the five fixtures.
func pastDocs(ids ...string) string {
	var docs []string
	for _, id := range ids {
		switch id {
		case pastPublic:
			docs = append(docs, pastMeetingDocWith(id, "public", false))
		case pastRestricted:
			docs = append(docs, pastMeetingDocWith(id, "public", true))
		case pastCommittee:
			docs = append(docs, pastMeetingDocWith(id, "private", false, memberOfGroup))
		default:
			docs = append(docs, pastMeetingDocWith(id, "private", false))
		}
	}
	return page(docs, "")
}

// meetingRoster is a host, an attendee and the caller for one past meeting.
func meetingRoster(id string) []string {
	n := id[:2]
	return []string{
		participantDocFor("host-"+n, id, "host"+n+"@example.test", "Hosty", n, "", true, true),
		participantDocFor("att-"+n, id, "att"+n+"@example.test", "Atty", n, "", false, true),
		participantDocFor("self-"+n, id, stubCallerEmail, "Stub", n, "", false, true),
	}
}

func uids(rs []map[string]any) string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r["uid"].(string))
	}
	return strings.Join(out, " ")
}

func participantsOf(t *testing.T, out any) []map[string]any {
	t.Helper()
	res, ok := out.(participantSearchResult)
	if !ok {
		t.Fatalf("unexpected output %T", out)
	}
	data := make([]map[string]any, 0, len(res.Resources))
	for _, r := range res.Resources {
		data = append(data, resourceData(r))
	}
	return data
}

func TestParticipants_FullViewIsUnchangedAndMakesNoChecks(t *testing.T) {
	api := setupParticipantTest(t)
	api.GrantRelations()
	api.Respond(resourcesPath, page(meetingRoster(pastHidden), ""))
	_, out, _ := handleSearchPastMeetingParticipants(fullViewCtx(), stubCallToolRequest(), SearchPastMeetingParticipantsArgs{PastMeetingID: pastHidden})
	if got := participantsOf(t, out); len(got) != 3 || got[1]["email"] != "att55@example.test" {
		t.Errorf("full view returns the page unchanged: %v", got)
	}
	if len(api.Requests()) != 1 {
		t.Errorf("full view makes the one search call, got %d", len(api.Requests()))
	}
}

func TestParticipants_SingleMeetingViews(t *testing.T) {
	cases := []struct {
		name     string
		id       string
		grants   []string
		wantUIDs string
	}{
		{"organizer sees everything", pastOrganized, []string{"v1_past_meeting:" + pastOrganized + "#organizer"}, "host-11 att-11 self-11"},
		{"public meeting: hosts and self", pastPublic, nil, "host-22 self-22"},
		{"restricted public meeting: self only", pastRestricted, nil, "self-33"},
		{"group member: hosts and self", pastCommittee, []string{"committee:" + memberOfGroup + "#member"}, "host-44 self-44"},
		{"direct attendee relation: hosts and self", pastHidden, []string{"v1_past_meeting:" + pastHidden + "#attendee"}, "host-55 self-55"},
		{"private meeting without relations: self only", pastHidden, nil, "self-55"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := setupParticipantTest(t)
			api.GrantRelations(tc.grants...)
			api.Respond(resourcesPath, pastDocs(tc.id))                // the view lookup, before the search
			api.Respond(resourcesPath, page(meetingRoster(tc.id), "")) // the search
			res, out, _ := handleSearchPastMeetingParticipants(context.Background(), stubCallToolRequest(), SearchPastMeetingParticipantsArgs{PastMeetingID: tc.id})
			if res.IsError {
				t.Fatalf("unexpected error: %s", allResultText(t, res))
			}
			got := participantsOf(t, out)
			if uids(got) != tc.wantUIDs {
				t.Fatalf("records = %q, want %q", uids(got), tc.wantUIDs)
			}
			text := allResultText(t, res)
			for _, r := range got {
				switch {
				case strings.HasPrefix(r["uid"].(string), "self-"):
					if r["email"] != stubCallerEmail || r["sessions"] == nil {
						t.Errorf("the caller's own record is unchanged: %v", r)
					}
				case tc.id == pastOrganized:
					if r["email"] == nil || r["sessions"] == nil {
						t.Errorf("organizer records are unchanged: %v", r)
					}
				default:
					want := []string{"first_name", "host", "is_attended", "last_name", "meeting_and_occurrence_id", "uid"}
					if keys := sortedKeys(r); strings.Join(keys, ",") != strings.Join(want, ",") {
						t.Errorf("host stub keys = %v, want %v", keys, want)
					}
				}
			}
			if tc.id != pastOrganized && strings.Contains(text, "host"+tc.id[:2]+"@example.test") {
				t.Error("a host's e-mail reached a non-organizer")
			}
			if strings.Contains(text, "att"+tc.id[:2]+"@example.test") && tc.id != pastOrganized {
				t.Error("an attendee reached a non-organizer")
			}
			result := out.(participantSearchResult)
			if result.People == nil || *result.People != len(got) || result.Records == nil || *result.Records != len(got) {
				t.Errorf("people/records must describe what is returned: %v %v", result.People, result.Records)
			}
			// One record lookup, one relation batch holding organizer, host,
			// invitee, attendee and the group membership where the record
			// names a group.
			bodies := api.AccessCheckBodies()
			if len(bodies) != 1 {
				t.Fatalf("expected one access-check batch, got %v", bodies)
			}
			wantReqs := 4
			if tc.id == pastCommittee {
				wantReqs = 5
			}
			if len(bodies[0]) != wantReqs {
				t.Errorf("batch = %v", bodies[0])
			}
		})
	}
}

func TestParticipants_DedupeRunsBeforeProjection(t *testing.T) {
	// The same host appears on the invitee side (host:true, not attended)
	// and the join side (host:false, attended): merged, the stub carries
	// both flags, which projecting first would have lost.
	api := setupParticipantTest(t)
	api.GrantRelations()
	api.Respond(resourcesPath, pastDocs(pastPublic))
	api.Respond(resourcesPath, page([]string{
		participantDocFor("inv", pastPublic, "Host@example.test", "Hosty", "H", "", true, false),
		participantDocFor("join", pastPublic, "host@example.test", "Hosty", "H", "", false, true),
	}, ""))
	_, out, _ := handleSearchPastMeetingParticipants(context.Background(), stubCallToolRequest(), SearchPastMeetingParticipantsArgs{PastMeetingID: pastPublic})
	got := participantsOf(t, out)
	if len(got) != 1 || got[0]["host"] != true || got[0]["is_attended"] != true {
		t.Fatalf("expected one merged host stub with both flags, got %v", got)
	}
	if got[0]["email"] != nil {
		t.Error("the merged stub carries no e-mail")
	}
}

func TestParticipants_DedupeFalseProjectsRawRecords(t *testing.T) {
	api := setupParticipantTest(t)
	api.GrantRelations()
	api.Respond(resourcesPath, pastDocs(pastPublic))
	api.Respond(resourcesPath, page(meetingRoster(pastPublic), ""))
	_, out, _ := handleSearchPastMeetingParticipants(context.Background(), stubCallToolRequest(), SearchPastMeetingParticipantsArgs{PastMeetingID: pastPublic, Dedupe: boolPtrT(false)})
	result := out.(participantSearchResult)
	if uids(participantsOf(t, out)) != "host-22 self-22" || result.People != nil {
		t.Errorf("raw records are projected the same way: %v", result.Resources)
	}
}

func TestParticipants_OwnRecordMatchesByUsername(t *testing.T) {
	api := setupParticipantTest(t)
	api.GrantRelations()
	api.Respond(resourcesPath, pastDocs(pastHidden))
	api.Respond(resourcesPath, page([]string{
		participantDocFor("mine", pastHidden, "other-address@example.test", "Stub", "User", stubCallerUsername, false, true),
		participantDocFor("theirs", pastHidden, "att@example.test", "Atty", "A", "someone-else", false, true),
	}, ""))
	_, out, _ := handleSearchPastMeetingParticipants(context.Background(), stubCallToolRequest(), SearchPastMeetingParticipantsArgs{PastMeetingID: pastHidden})
	if got := participantsOf(t, out); uids(got) != "mine" || got[0]["email"] != "other-address@example.test" {
		t.Errorf("own record by username must be returned in full: %v", got)
	}
}

func TestParticipants_PlainScopeDecidesPerMeetingOnThePage(t *testing.T) {
	api := setupParticipantTest(t)
	api.GrantRelations("v1_past_meeting:" + pastOrganized + "#organizer")
	roster := append(meetingRoster(pastOrganized), meetingRoster(pastPublic)...)
	roster = append(roster, meetingRoster(pastHidden)...)
	api.Respond(resourcesPath, page(roster, "next"))                            // the search runs first
	api.Respond(resourcesPath, pastDocs(pastOrganized, pastPublic, pastHidden)) // then the record lookup
	res, out, _ := handleSearchPastMeetingParticipants(context.Background(), stubCallToolRequest(), SearchPastMeetingParticipantsArgs{ProjectUID: "P1", PageSize: 10})
	if res.IsError {
		t.Fatal(allResultText(t, res))
	}
	// The caller's three own records merge into one person (dedupe keys on
	// e-mail across the page, as for every caller); the hidden meeting's
	// other records are dropped.
	if got := uids(participantsOf(t, out)); got != "host-11 att-11 self-11 host-22" {
		t.Errorf("records = %q", got)
	}
	result := out.(participantSearchResult)
	if result.PageToken == nil || *result.PageToken != "next" {
		t.Error("the page token is kept")
	}
	lookup := api.RequestsTo(resourcesPath)[1]
	if lookup.Query.Get("type") != "v1_past_meeting" || len(lookup.Query["filters_or"]) != 3 {
		t.Errorf("record lookup = %v", lookup.Query)
	}
	assertExchangedAuth(t, lookup)
}

func TestParticipants_DateRangeAppliesTheRulePerMeeting(t *testing.T) {
	api := setupParticipantTest(t)
	api.GrantRelations("v1_past_meeting:" + pastOrganized + "#organizer")
	api.Respond(resourcesPath, page([]string{pastMeetingDoc(pastOrganized), pastMeetingDoc(pastPublic), pastMeetingDoc(pastHidden)}, "")) // resolve ids
	api.Respond(resourcesPath, pastDocs(pastOrganized, pastPublic, pastHidden))                                                           // record lookup
	api.Respond(resourcesPath, page(meetingRoster(pastOrganized), ""))                                                                    // drains
	api.Respond(resourcesPath, page(meetingRoster(pastPublic), ""))
	api.Respond(resourcesPath, page(meetingRoster(pastHidden), ""))
	res, out, _ := handleSearchPastMeetingParticipants(context.Background(), stubCallToolRequest(), SearchPastMeetingParticipantsArgs{ProjectUID: "P1", DateFrom: "2026-06-01", DateTo: "2026-06-30"})
	if res.IsError {
		t.Fatal(allResultText(t, res))
	}
	if got := uids(participantsOf(t, out)); got != "host-11 att-11 self-11 host-22" {
		t.Errorf("records = %q", got)
	}
	result := out.(participantSearchResult)
	if result.Meetings == nil || *result.Meetings != 3 || *result.People != 4 || *result.Records != 4 {
		t.Errorf("totals must describe what is returned: meetings=%v people=%v records=%v", result.Meetings, result.People, result.Records)
	}
	if len(api.AccessCheckBodies()) != 1 {
		t.Error("one relation batch for every resolved meeting")
	}
}

func TestParticipants_PersonFiltersNeedOrganizedMeetings(t *testing.T) {
	t.Run("project scope without a range", func(t *testing.T) {
		api := setupParticipantTest(t)
		res, _, _ := handleSearchPastMeetingParticipants(context.Background(), stubCallToolRequest(), SearchPastMeetingParticipantsArgs{ProjectUID: "P1", Name: "Hosty"})
		if !res.IsError || strings.TrimSpace(allResultText(t, res)) != participantFilterRefusal || len(api.Requests()) != 0 {
			t.Fatalf("expected the refusal before any call, got %s", allResultText(t, res))
		}
	})
	t.Run("org_name on a public meeting", func(t *testing.T) {
		api := setupParticipantTest(t)
		api.GrantRelations()
		api.Respond(resourcesPath, pastDocs(pastPublic))
		res, _, _ := handleSearchPastMeetingParticipants(context.Background(), stubCallToolRequest(), SearchPastMeetingParticipantsArgs{PastMeetingID: pastPublic, OrgName: "Example Org"})
		if !res.IsError || strings.TrimSpace(allResultText(t, res)) != participantFilterRefusal {
			t.Fatalf("expected the refusal, got %s", allResultText(t, res))
		}
		if len(api.RequestsTo(resourcesPath)) != 1 {
			t.Error("no participant data may be read when the filter is refused")
		}
	})
	t.Run("name on an organized meeting", func(t *testing.T) {
		api := setupParticipantTest(t)
		api.GrantRelations("v1_past_meeting:" + pastOrganized + "#organizer")
		api.Respond(resourcesPath, pastDocs(pastOrganized))
		api.Respond(resourcesPath, page(meetingRoster(pastOrganized), ""))
		res, out, _ := handleSearchPastMeetingParticipants(context.Background(), stubCallToolRequest(), SearchPastMeetingParticipantsArgs{PastMeetingID: pastOrganized, Name: "Hosty"})
		if res.IsError || len(participantsOf(t, out)) != 3 {
			t.Fatalf("an organizer keeps the filter: %s", allResultText(t, res))
		}
	})
	t.Run("date range with one unorganized meeting", func(t *testing.T) {
		api := setupParticipantTest(t)
		api.GrantRelations("v1_past_meeting:" + pastOrganized + "#organizer")
		api.Respond(resourcesPath, page([]string{pastMeetingDoc(pastOrganized), pastMeetingDoc(pastPublic)}, ""))
		api.Respond(resourcesPath, pastDocs(pastOrganized, pastPublic))
		res, _, _ := handleSearchPastMeetingParticipants(context.Background(), stubCallToolRequest(), SearchPastMeetingParticipantsArgs{ProjectUID: "P1", DateFrom: "2026-06-01", Name: "Hosty"})
		if !res.IsError || strings.TrimSpace(allResultText(t, res)) != participantFilterRefusal {
			t.Fatalf("expected the refusal, got %s", allResultText(t, res))
		}
		if len(api.RequestsTo(resourcesPath)) != 2 {
			t.Error("the drain must not start when the filter is refused")
		}
	})
}

func TestParticipants_CountOnlyFollowsTheView(t *testing.T) {
	t.Run("single public meeting counts", func(t *testing.T) {
		api := setupParticipantTest(t)
		api.GrantRelations()
		api.Respond(resourcesPath, pastDocs(pastPublic))
		api.Respond(countPath, `{"count": 9, "has_more": false}`)
		res, _, _ := handleSearchPastMeetingParticipants(context.Background(), stubCallToolRequest(), SearchPastMeetingParticipantsArgs{PastMeetingID: pastPublic, CountOnly: true, AttendedOnly: true})
		if res.IsError || resultJSON(t, res)["count"] != float64(9) {
			t.Fatalf("expected the count, got %s", allResultText(t, res))
		}
	})
	t.Run("single hidden meeting is refused, not zero", func(t *testing.T) {
		api := setupParticipantTest(t)
		api.GrantRelations()
		api.Respond(resourcesPath, pastDocs(pastHidden))
		res, _, _ := handleSearchPastMeetingParticipants(context.Background(), stubCallToolRequest(), SearchPastMeetingParticipantsArgs{PastMeetingID: pastHidden, CountOnly: true})
		if !res.IsError || strings.TrimSpace(allResultText(t, res)) != participantCountNotShownMessage || len(api.RequestsTo(countPath)) != 0 {
			t.Fatalf("expected the refusal and no count call, got %s", allResultText(t, res))
		}
	})
	t.Run("project scope resolves meetings and counts the shown ones", func(t *testing.T) {
		api := setupParticipantTest(t)
		api.GrantRelations("v1_past_meeting:" + pastOrganized + "#organizer")
		api.Respond(resourcesPath, page([]string{pastMeetingDoc(pastOrganized), pastMeetingDoc(pastPublic), pastMeetingDoc(pastHidden)}, ""))
		api.Respond(resourcesPath, pastDocs(pastOrganized, pastPublic, pastHidden))
		api.Respond(countPath, `{"count": 5, "has_more": false}`)
		api.Respond(countPath, `{"count": 7, "has_more": false}`)
		res, _, _ := handleSearchPastMeetingParticipants(context.Background(), stubCallToolRequest(), SearchPastMeetingParticipantsArgs{ProjectUID: "P1", CountOnly: true})
		if res.IsError {
			t.Fatal(allResultText(t, res))
		}
		out := resultJSON(t, res)
		if out["count"] != float64(12) || out["complete"] != true {
			t.Errorf("count = %v complete = %v", out["count"], out["complete"])
		}
		if note, _ := out["note"].(string); !strings.Contains(note, strings.TrimSpace(participantCountScopeNote)) {
			t.Errorf("note must say which meetings are counted: %q", note)
		}
		counts := api.RequestsTo(countPath)
		if len(counts) != 2 || counts[0].Query.Get("parent") != "past_meeting:"+pastOrganized || counts[1].Query.Get("parent") != "past_meeting:"+pastPublic {
			t.Errorf("only shown meetings are counted: %+v", counts)
		}
	})
	t.Run("full view keeps the single count call", func(t *testing.T) {
		api := setupParticipantTest(t)
		api.Respond(countPath, `{"count": 3, "has_more": false}`)
		res, _, _ := handleSearchPastMeetingParticipants(fullViewCtx(), stubCallToolRequest(), SearchPastMeetingParticipantsArgs{ProjectUID: "P1", CountOnly: true})
		if res.IsError || len(api.Requests()) != 1 {
			t.Fatalf("full view counts the scope in one call: %s", allResultText(t, res))
		}
		if note, _ := resultJSON(t, res)["note"].(string); strings.Contains(note, strings.TrimSpace(participantCountScopeNote)) {
			t.Error("full view carries no scope note")
		}
	})
}

func TestParticipants_FailsClosed(t *testing.T) {
	cases := []struct {
		name  string
		args  SearchPastMeetingParticipantsArgs
		setup func(api *stubLFXAPI)
	}{
		{"access-check 503 on a single meeting", SearchPastMeetingParticipantsArgs{PastMeetingID: pastPublic}, func(api *stubLFXAPI) {
			api.FailAccessCheck(http.StatusServiceUnavailable)
			api.Respond(resourcesPath, pastDocs(pastPublic))
		}},
		{"record lookup error on a single meeting", SearchPastMeetingParticipantsArgs{PastMeetingID: pastPublic}, func(api *stubLFXAPI) {
			api.GrantRelations()
			api.RespondStatus(resourcesPath, http.StatusBadGateway, "")
		}},
		{"access-check 503 on a plain page", SearchPastMeetingParticipantsArgs{ProjectUID: "P1"}, func(api *stubLFXAPI) {
			api.FailAccessCheck(http.StatusServiceUnavailable)
			api.Respond(resourcesPath, page(meetingRoster(pastPublic), ""))
			api.Respond(resourcesPath, pastDocs(pastPublic))
		}},
		{"access-check 503 on a count", SearchPastMeetingParticipantsArgs{PastMeetingID: pastPublic, CountOnly: true}, func(api *stubLFXAPI) {
			api.FailAccessCheck(http.StatusServiceUnavailable)
			api.Respond(resourcesPath, pastDocs(pastPublic))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := setupParticipantTest(t)
			tc.setup(api)
			res, out, _ := handleSearchPastMeetingParticipants(context.Background(), stubCallToolRequest(), tc.args)
			if !res.IsError || strings.TrimSpace(allResultText(t, res)) != peopleVisibilityUnavailableMessage || out != nil {
				t.Fatalf("expected the unavailable error and no records, got %s / %v", allResultText(t, res), out)
			}
			if len(api.RequestsTo(countPath)) != 0 {
				t.Error("no count may run after a predicate failure")
			}
		})
	}
}

func TestGetPastMeetingParticipant_Views(t *testing.T) {
	t.Run("full access host stub", func(t *testing.T) {
		api := setupParticipantTest(t)
		api.GrantRelations()
		api.Respond(resourcesPath, page([]string{participantDocFor("h", pastPublic, "host@example.test", "Hosty", "H", "", true, true)}, ""))
		api.Respond(resourcesPath, pastDocs(pastPublic))
		res, _, _ := handleGetPastMeetingParticipant(context.Background(), stubCallToolRequest(), GetPastMeetingParticipantArgs{UID: "h"})
		if res.IsError {
			t.Fatal(allResultText(t, res))
		}
		assertNoEmail(t, allResultText(t, res))
		if data := resultJSON(t, res)["Data"].(map[string]any); data["host"] != true || data["first_name"] != "Hosty" || data["job_title"] != nil {
			t.Errorf("host stub wrong: %v", data)
		}
	})
	t.Run("full access non-host reads as not visible", func(t *testing.T) {
		api := setupParticipantTest(t)
		api.GrantRelations()
		api.Respond(resourcesPath, page([]string{participantDocFor("a", pastPublic, "att@example.test", "Atty", "A", "", false, true)}, ""))
		api.Respond(resourcesPath, pastDocs(pastPublic))
		res, _, _ := handleGetPastMeetingParticipant(context.Background(), stubCallToolRequest(), GetPastMeetingParticipantArgs{UID: "a"})
		if !res.IsError || strings.TrimSpace(allResultText(t, res)) != lookupNotVisibleMessage("past meeting participant", "a") {
			t.Fatalf("expected the not-visible message, got %s", allResultText(t, res))
		}
	})
	t.Run("own record in a hidden meeting is returned in full", func(t *testing.T) {
		api := setupParticipantTest(t)
		api.GrantRelations()
		api.Respond(resourcesPath, page([]string{participantDocFor("me", pastHidden, strings.ToUpper(stubCallerEmail), "Stub", "U", "", false, true)}, ""))
		api.Respond(resourcesPath, pastDocs(pastHidden))
		res, _, _ := handleGetPastMeetingParticipant(context.Background(), stubCallToolRequest(), GetPastMeetingParticipantArgs{UID: "me"})
		if res.IsError || resultJSON(t, res)["Data"].(map[string]any)["sessions"] == nil {
			t.Fatalf("own record (matched case-insensitively) is unchanged: %s", allResultText(t, res))
		}
	})
	t.Run("fails closed", func(t *testing.T) {
		api := setupParticipantTest(t)
		api.FailAccessCheck(http.StatusBadGateway)
		api.Respond(resourcesPath, page([]string{participantDocFor("h", pastPublic, "host@example.test", "Hosty", "H", "", true, true)}, ""))
		api.Respond(resourcesPath, pastDocs(pastPublic))
		res, _, _ := handleGetPastMeetingParticipant(context.Background(), stubCallToolRequest(), GetPastMeetingParticipantArgs{UID: "h"})
		if !res.IsError || strings.TrimSpace(allResultText(t, res)) != peopleVisibilityUnavailableMessage {
			t.Fatalf("expected the unavailable error, got %s", allResultText(t, res))
		}
	})
}

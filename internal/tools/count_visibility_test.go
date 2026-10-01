// Copyright The Linux Foundation and contributors.
// SPDX-License-Identifier: MIT

package tools

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// countArgsCase is one count_lfx_resources argument set and whether a caller
// without full view may run it.
type countArgsCase struct {
	name    string
	args    CountLFXResourcesArgs
	allowed bool
}

// runCountGateCases runs each case against a stub that answers every count
// with 1, asserting the refusal (an error result naming the type and the
// allowed form, before any count call) or the pass-through.
func runCountGateCases(t *testing.T, api *stubLFXAPI, cases []countArgsCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := len(api.RequestsTo(countPath))
			api.Respond(countPath, `{"count": 1, "has_more": false}`)
			res, _, _ := handleCountLFXResources(context.Background(), stubCallToolRequest(), tc.args)
			counted := len(api.RequestsTo(countPath)) - before
			text := allResultText(t, res)
			if tc.allowed {
				if res.IsError || counted != 1 {
					t.Fatalf("expected the count to run, got isError=%v counts=%d: %s", res.IsError, counted, text)
				}
				return
			}
			if !res.IsError || counted != 0 {
				t.Fatalf("expected a refusal before any count, got isError=%v counts=%d: %s", res.IsError, counted, text)
			}
			if !strings.Contains(text, tc.args.Type) || !strings.Contains(text, "LFX Self Serve") {
				t.Errorf("refusal must name the type and the allowed form: %s", text)
			}
			assertNoEmail(t, text)
		})
	}
}

func TestCountLFXResources_CommitteeMemberGate(t *testing.T) {
	api := setupCountTest(t)
	const typ = committeeMemberResourceType
	runCountGateCases(t, api, []countArgsCase{
		{"bare type", CountLFXResourcesArgs{Type: typ}, true},
		{"committee parent", CountLFXResourcesArgs{Type: typ, Parent: "committee:C1"}, true},
		{"project parent", CountLFXResourcesArgs{Type: typ, Parent: "project:P1"}, true},
		{"structural tags", CountLFXResourcesArgs{Type: typ, Tags: []string{"committee_category:Technical"}, TagsAll: []string{"project_uid:P1", "voting_status:Voting Rep"}}, true},
		{"name", CountLFXResourcesArgs{Type: typ, Parent: "committee:C1", Name: "Pat"}, false},
		{"email tag", CountLFXResourcesArgs{Type: typ, Tags: []string{"email:someone@example.test"}}, false},
		{"username tags_all", CountLFXResourcesArgs{Type: typ, TagsAll: []string{"committee_uid:C1", "username:someone"}}, false},
		{"organization tag", CountLFXResourcesArgs{Type: typ, TagsAll: []string{"organization_name:Example Org"}}, false},
		{"filters_all email", CountLFXResourcesArgs{Type: typ, FiltersAll: []string{"email:someone@example.test"}}, false},
		{"filters_or", CountLFXResourcesArgs{Type: typ, FiltersOr: []string{"first_name:Pat"}}, false},
		{"other parent", CountLFXResourcesArgs{Type: typ, Parent: "meeting:M1"}, false},
		{"date range", CountLFXResourcesArgs{Type: typ, DateField: "created_at", DateFrom: "2026-01-01"}, false},
	})
	if n := len(api.RequestsTo(accessCheckPath)); n != 0 {
		t.Errorf("committee_member counts need no relation check, got %d", n)
	}
}

func TestCountLFXResources_NonPeopleTypesAreUngated(t *testing.T) {
	api := setupCountTest(t)
	runCountGateCases(t, api, []countArgsCase{
		{"committee with name", CountLFXResourcesArgs{Type: committeeResourceType, Name: "TOC"}, true},
		{"meeting with filters", CountLFXResourcesArgs{Type: meetingResourceType, FiltersAll: []string{"visibility:public"}}, true},
		{"project with tags", CountLFXResourcesArgs{Type: projectResourceType, Tags: []string{"slug:cncf"}}, true},
		{"mailing list member with name", CountLFXResourcesArgs{Type: mailingListMemberResourceType, Name: "pat"}, true},
	})
}

func TestCountLFXResources_FullViewSkipsTheGate(t *testing.T) {
	api := setupCountTest(t)
	api.Respond(countPath, `{"count": 3, "has_more": false}`)
	res, _, _ := handleCountLFXResources(fullViewCtx(), stubCallToolRequest(), CountLFXResourcesArgs{
		Type: committeeMemberResourceType, FiltersAll: []string{"email:someone@example.test"},
	})
	if res.IsError {
		t.Fatalf("full view must keep every filter: %s", allResultText(t, res))
	}
	if len(api.RequestsTo(accessCheckPath)) != 0 || len(api.RequestsTo(countPath)) != 1 {
		t.Error("full view makes the count call and nothing else")
	}
}

func TestCountLFXResources_RegistrantGate(t *testing.T) {
	const typ = meetingRegistrantResourceType
	t.Run("forms", func(t *testing.T) {
		api := setupCountTest(t)
		api.GrantRelations("v1_meeting:M1#organizer")
		runCountGateCases(t, api, []countArgsCase{
			{"organized meeting", CountLFXResourcesArgs{Type: typ, Parent: "meeting:M1"}, true},
			{"no parent", CountLFXResourcesArgs{Type: typ}, false},
			{"committee parent", CountLFXResourcesArgs{Type: typ, Parent: "committee:C1"}, false},
			{"with a tag", CountLFXResourcesArgs{Type: typ, Parent: "meeting:M1", Tags: []string{"host:true"}}, false},
			{"with name", CountLFXResourcesArgs{Type: typ, Parent: "meeting:M1", Name: "Pat"}, false},
			{"with filters_all", CountLFXResourcesArgs{Type: typ, Parent: "meeting:M1", FiltersAll: []string{"email:x@example.test"}}, false},
		})
	})
	t.Run("registrant of the meeting", func(t *testing.T) {
		api := setupCountTest(t)
		api.GrantRelations()
		api.Respond(resourcesPath, page([]string{`{"type":"v1_meeting_registrant","id":"r","data":{"uid":"r","meeting_id":"M1","email":"` + stubCallerEmail + `"}}`}, ""))
		runCountGateCases(t, api, []countArgsCase{
			{"registered meeting", CountLFXResourcesArgs{Type: typ, Parent: "meeting:M1"}, true},
		})
	})
	t.Run("neither", func(t *testing.T) {
		api := setupCountTest(t)
		api.GrantRelations()
		api.Respond(resourcesPath, page(nil, ""))
		runCountGateCases(t, api, []countArgsCase{
			{"hidden meeting", CountLFXResourcesArgs{Type: typ, Parent: "meeting:M1"}, false},
		})
	})
	t.Run("predicate failure fails closed", func(t *testing.T) {
		api := setupCountTest(t)
		api.FailAccessCheck(http.StatusServiceUnavailable)
		api.Respond(countPath, `{"count": 1, "has_more": false}`)
		res, _, _ := handleCountLFXResources(context.Background(), stubCallToolRequest(), CountLFXResourcesArgs{Type: typ, Parent: "meeting:M1"})
		if !res.IsError || allResultText(t, res) != peopleVisibilityUnavailableMessage+"\n" || len(api.RequestsTo(countPath)) != 0 {
			t.Fatalf("expected the unavailable error and no count, got %s", allResultText(t, res))
		}
	})
}

func TestCountLFXResources_ParticipantGate(t *testing.T) {
	const typ = pastMeetingParticipantResourceType
	const id = "91461158520-1771596000000"
	t.Run("organizer forms", func(t *testing.T) {
		api := setupCountTest(t)
		api.GrantRelations("v1_past_meeting:" + id + "#organizer")
		for range 3 {
			api.Respond(resourcesPath, page([]string{pastMeetingDoc(id)}, "")) // the past-meeting record lookup
		}
		runCountGateCases(t, api, []countArgsCase{
			{"parent alone", CountLFXResourcesArgs{Type: typ, Parent: "past_meeting:" + id}, true},
			{"attended tag", CountLFXResourcesArgs{Type: typ, Parent: "past_meeting:" + id, Tags: []string{"is_attended:true"}}, true},
			{"attended tags_all", CountLFXResourcesArgs{Type: typ, Parent: "past_meeting:" + id, TagsAll: []string{"is_attended:true"}}, true},
			{"no parent", CountLFXResourcesArgs{Type: typ}, false},
			{"project parent", CountLFXResourcesArgs{Type: typ, Parent: "project:P1"}, false},
			{"email tag", CountLFXResourcesArgs{Type: typ, Parent: "past_meeting:" + id, Tags: []string{"email:x@example.test"}}, false},
			{"name", CountLFXResourcesArgs{Type: typ, Parent: "past_meeting:" + id, Name: "Pat"}, false},
			{"filters_or", CountLFXResourcesArgs{Type: typ, Parent: "past_meeting:" + id, FiltersOr: []string{"org_name:Example"}}, false},
			{"date range", CountLFXResourcesArgs{Type: typ, Parent: "past_meeting:" + id, DateField: "created_at", DateTo: "2026-12-31"}, false},
		})
	})
	t.Run("public meeting is full access", func(t *testing.T) {
		api := setupCountTest(t)
		api.GrantRelations()
		api.Respond(resourcesPath, page([]string{pastMeetingDocWith(id, "public", false)}, ""))
		runCountGateCases(t, api, []countArgsCase{
			{"public", CountLFXResourcesArgs{Type: typ, Parent: "past_meeting:" + id}, true},
		})
	})
	t.Run("restricted public meeting without a relation is refused", func(t *testing.T) {
		api := setupCountTest(t)
		api.GrantRelations()
		api.Respond(resourcesPath, page([]string{pastMeetingDocWith(id, "public", true)}, ""))
		runCountGateCases(t, api, []countArgsCase{
			{"restricted", CountLFXResourcesArgs{Type: typ, Parent: "past_meeting:" + id}, false},
		})
	})
	t.Run("record not readable is refused", func(t *testing.T) {
		api := setupCountTest(t)
		api.GrantRelations()
		api.Respond(resourcesPath, page(nil, ""))
		runCountGateCases(t, api, []countArgsCase{
			{"unknown meeting", CountLFXResourcesArgs{Type: typ, Parent: "past_meeting:" + id}, false},
		})
	})
}

// pastMeetingDocWith is pastMeetingDoc with explicit visibility, restricted
// and committees values, the fields the participant rule reads.
func pastMeetingDocWith(occurrenceID, visibility string, restricted bool, committees ...string) string {
	cs := make([]string, 0, len(committees))
	for _, c := range committees {
		cs = append(cs, `{"uid": "`+c+`"}`)
	}
	return `{
	  "type": "v1_past_meeting",
	  "id": "` + occurrenceID + `",
	  "data": {
	    "meeting_id": "91461158520",
	    "meeting_and_occurrence_id": "` + occurrenceID + `",
	    "project_uid": "a0941000002wBz4AAE",
	    "title": "CNCF TOC",
	    "visibility": "` + visibility + `",
	    "restricted": ` + map[bool]string{true: "true", false: "false"}[restricted] + `,
	    "committees": [` + strings.Join(cs, ",") + `],
	    "start_time": "2026-06-10T15:00:00Z",
	    "created_by": {"user_id": "u1", "username": "creator", "email": "creator@example.test", "name": "Creator", "profile_picture": "p"},
	    "updated_by": {"user_id": "u2", "username": "editor", "email": "editor@example.test", "name": "Editor"},
	    "updated_by_list": [{"email": "editor@example.test", "name": "Editor"}]
	  }
	}`
}

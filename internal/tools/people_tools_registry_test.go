// Copyright The Linux Foundation and contributors.
// SPDX-License-Identifier: MIT

package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// peopleToolCase is one tool that returns people data, with the upstream
// answers it needs for one call and the call itself. The registry below is
// walked by the tests that pin the two invariants every people tool shares:
// a caller without full view whom every predicate denies gets no people
// data, and a caller with full view gets the upstream records unchanged
// without any predicate call.
type peopleToolCase struct {
	name string
	// setup configures the stub (the shared meeting, committee and project
	// configs all point at it) and queues the upstream answers for one call
	// under the given view; the order of query-service pages depends on it.
	setup func(api *stubLFXAPI, full bool)
	call  func(ctx context.Context) (*mcp.CallToolResult, any, error)
	// emails are the addresses the fixtures carry; none is the caller's.
	emails []string
}

const (
	registryCommittee = "66666666-6666-4666-8666-666666666666"
	registryMember    = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	registryMeeting   = "77777777777"
	registryPast      = "77777777777-1771596000000"
)

// peopleTools is the registry. A tool that returns people records or people
// fields is added here the day it is added to the server.
var peopleTools = []peopleToolCase{
	{
		name: "search_committee_members",
		setup: func(api *stubLFXAPI, _ bool) {
			api.Respond(resourcesPath, page([]string{groupMemberDoc(registryMember, registryCommittee, "None", "member@example.test")}, ""))
		},
		call: func(ctx context.Context) (*mcp.CallToolResult, any, error) {
			return handleSearchCommitteeMembers(ctx, stubCallToolRequest(), SearchCommitteeMembersArgs{CommitteeUID: registryCommittee})
		},
		emails: []string{"member@example.test"},
	},
	{
		name: "get_committee_member",
		setup: func(api *stubLFXAPI, _ bool) {
			api.Respond("/committees/"+registryCommittee+"/members/"+registryMember, committeeMemberRecord(registryMember, registryCommittee, "None", "member@example.test"))
		},
		call: func(ctx context.Context) (*mcp.CallToolResult, any, error) {
			return handleGetCommitteeMember(ctx, stubCallToolRequest(), GetCommitteeMemberArgs{CommitteeUID: registryCommittee, MemberUID: registryMember})
		},
		emails: []string{"member@example.test"},
	},
	{
		name: "get_committee",
		setup: func(api *stubLFXAPI, _ bool) {
			api.Respond("/committees/"+registryCommittee, `{"uid": "`+registryCommittee+`", "name": "TSC"}`)
			api.Respond("/committees/"+registryCommittee+"/settings", committeeSettingsRecord(registryCommittee, "hidden"))
		},
		call: func(ctx context.Context) (*mcp.CallToolResult, any, error) {
			return handleGetCommittee(ctx, stubCallToolRequest(), GetCommitteeArgs{UID: registryCommittee})
		},
		emails: []string{"writer@example.test", "auditor@example.test"},
	},
	{
		name: "search_meeting_registrants",
		setup: func(api *stubLFXAPI, full bool) {
			api.Respond(resourcesPath, page([]string{registrantDoc("reg", registryMeeting, "registrant@example.test", true)}, ""))
			if !full {
				api.Respond(resourcesPath, page(nil, "")) // the caller's own registrations
			}
		},
		call: func(ctx context.Context) (*mcp.CallToolResult, any, error) {
			return handleSearchMeetingRegistrants(ctx, stubCallToolRequest(), SearchMeetingRegistrantsArgs{MeetingID: registryMeeting})
		},
		emails: []string{"registrant@example.test", "creator@example.test", "editor@example.test"},
	},
	{
		name: "get_meeting_registrant",
		setup: func(api *stubLFXAPI, full bool) {
			api.Respond(resourcesPath, page([]string{registrantDoc("reg", registryMeeting, "registrant@example.test", true)}, ""))
			if !full {
				api.Respond(resourcesPath, page(nil, ""))
			}
		},
		call: func(ctx context.Context) (*mcp.CallToolResult, any, error) {
			return handleGetMeetingRegistrant(ctx, stubCallToolRequest(), GetMeetingRegistrantArgs{UID: "reg"})
		},
		emails: []string{"registrant@example.test", "creator@example.test", "editor@example.test"},
	},
	{
		name: "search_meetings",
		setup: func(api *stubLFXAPI, _ bool) {
			api.Respond(resourcesPath, singleResourcePage("v1_meeting", "meeting-1", meetingDocWithPeople))
		},
		call: func(ctx context.Context) (*mcp.CallToolResult, any, error) {
			return handleSearchMeetings(ctx, stubCallToolRequest(), SearchMeetingsArgs{ProjectUID: "11111111-1111-1111-1111-111111111111"})
		},
		// The organiser's name and e-mail stay (rendered with a mailto link);
		// editors go.
		emails: []string{"editor@example.test"},
	},
	{
		name: "get_meeting",
		setup: func(api *stubLFXAPI, _ bool) {
			api.Respond(resourcesPath, singleResourcePage("v1_meeting", "meeting-1", meetingDocWithPeople))
		},
		call: func(ctx context.Context) (*mcp.CallToolResult, any, error) {
			return handleGetMeeting(ctx, stubCallToolRequest(), GetMeetingArgs{UID: "meeting-1"})
		},
		emails: []string{"editor@example.test"},
	},
	{
		name: "search_past_meeting_participants",
		setup: func(api *stubLFXAPI, full bool) {
			if !full {
				api.Respond(resourcesPath, pastDocs(registryPast)) // private, no groups
			}
			api.Respond(resourcesPath, page([]string{participantDocFor("p", registryPast, "participant@example.test", "Par", "T", "", true, true)}, ""))
		},
		call: func(ctx context.Context) (*mcp.CallToolResult, any, error) {
			return handleSearchPastMeetingParticipants(ctx, stubCallToolRequest(), SearchPastMeetingParticipantsArgs{PastMeetingID: registryPast})
		},
		emails: []string{"participant@example.test"},
	},
	{
		name: "get_past_meeting_participant",
		setup: func(api *stubLFXAPI, full bool) {
			api.Respond(resourcesPath, page([]string{participantDocFor("p", registryPast, "participant@example.test", "Par", "T", "", true, true)}, ""))
			if !full {
				api.Respond(resourcesPath, pastDocs(registryPast))
			}
		},
		call: func(ctx context.Context) (*mcp.CallToolResult, any, error) {
			return handleGetPastMeetingParticipant(ctx, stubCallToolRequest(), GetPastMeetingParticipantArgs{UID: "p"})
		},
		emails: []string{"participant@example.test"},
	},
	{
		name: "search_past_meetings",
		setup: func(api *stubLFXAPI, _ bool) {
			api.Respond(resourcesPath, page([]string{pastMeetingDocWith(registryPast, "private", false)}, ""))
		},
		call: func(ctx context.Context) (*mcp.CallToolResult, any, error) {
			return handleSearchPastMeetings(ctx, stubCallToolRequest(), SearchPastMeetingsArgs{ProjectUID: "P1"})
		},
		emails: []string{"editor@example.test"},
	},
	{
		name: "get_past_meeting",
		setup: func(api *stubLFXAPI, _ bool) {
			api.Respond(pastMeetingPath, pastMeetingBody)
			api.Respond(resourcesPath, singleResourcePage("v1_past_meeting_recording", "rec-1", `{"uid": "rec-1", "host_email": "host@example.test", "created_by": {"email": "creator@example.test"}}`))
			api.Respond(resourcesPath, singleResourcePage("v1_past_meeting_transcript", "tr-1", `{"uid": "tr-1", "host_email": "host@example.test", "updated_by": {"email": "editor@example.test"}}`))
		},
		call: func(ctx context.Context) (*mcp.CallToolResult, any, error) {
			return handleGetPastMeeting(ctx, stubCallToolRequest(), GetPastMeetingArgs{UID: "past-1"})
		},
		emails: []string{"host@example.test", "creator@example.test", "editor@example.test"},
	},
	{
		name: "search_past_meeting_summaries",
		setup: func(api *stubLFXAPI, _ bool) {
			api.Respond(resourcesPath, page([]string{summaryDoc}, ""))
		},
		call: func(ctx context.Context) (*mcp.CallToolResult, any, error) {
			return handleSearchPastMeetingSummaries(ctx, stubCallToolRequest(), SearchPastMeetingSummariesArgs{ProjectUID: "P1"})
		},
		emails: []string{"host@example.test", "creator@example.test", "editor@example.test"},
	},
	{
		name: "get_past_meeting_summary",
		setup: func(api *stubLFXAPI, _ bool) {
			api.Respond(resourcesPath, page([]string{summaryDoc}, ""))
		},
		call: func(ctx context.Context) (*mcp.CallToolResult, any, error) {
			return handleGetPastMeetingSummary(ctx, stubCallToolRequest(), GetPastMeetingSummaryArgs{UID: "sum-1"})
		},
		emails: []string{"host@example.test", "creator@example.test", "editor@example.test"},
	},
}

// setupPeopleToolsTest points every tool family at one stub.
func setupPeopleToolsTest(t *testing.T) *stubLFXAPI {
	t.Helper()
	api := newStubLFXAPI(t)
	prevMeeting, prevCommittee, prevProject := meetingConfig, committeeConfig, projectConfig
	SetMeetingConfig(&MeetingConfig{Clients: api.Clients})
	SetCommitteeConfig(&CommitteeConfig{Clients: api.Clients})
	SetProjectConfig(&ProjectConfig{Clients: api.Clients})
	t.Cleanup(func() { meetingConfig, committeeConfig, projectConfig = prevMeeting, prevCommittee, prevProject })
	return api
}

// everything returns the text blocks, the structured output and the error
// of a tool result as one searchable string.
func everything(t *testing.T, res *mcp.CallToolResult, out any, err error) string {
	t.Helper()
	var sb strings.Builder
	if res != nil {
		sb.WriteString(allResultText(t, res))
	}
	if out != nil {
		b, merr := json.Marshal(out)
		if merr != nil {
			t.Fatalf("structured output does not marshal: %v", merr)
		}
		sb.Write(b)
	}
	if err != nil {
		sb.WriteString(err.Error())
	}
	return sb.String()
}

// predicateRequests counts the access-check calls. Together with the queue
// check in the full-view test (every queued upstream answer consumed, and an
// unqueued call 404s and fails the tool), it pins that a full-view caller
// causes exactly the upstream calls the tool made before this change.
func predicateRequests(api *stubLFXAPI) int {
	return len(api.RequestsTo(accessCheckPath))
}

// TestPeopleTools_DeniedCallerGetsNoPeopleData walks the registry as a
// caller without full view whom every relation check denies, whose own
// lookups are empty and whose past meetings are private: no fixture address
// appears anywhere in the text, the structured output or the error.
func TestPeopleTools_DeniedCallerGetsNoPeopleData(t *testing.T) {
	for _, tc := range peopleTools {
		t.Run(tc.name, func(t *testing.T) {
			api := setupPeopleToolsTest(t)
			api.GrantRelations()
			tc.setup(api, false)
			res, out, err := tc.call(context.Background())
			all := everything(t, res, out, err)
			for _, email := range tc.emails {
				if strings.Contains(all, email) {
					t.Errorf("%s reached a denied caller:\n%s", email, all)
				}
			}
			if strings.Contains(all, stubCallerEmail) {
				t.Errorf("the fixtures must not carry the caller's own address")
			}
		})
	}
}

// TestPeopleTools_FullViewIsUnchangedWithoutPredicateCalls walks the
// registry as a caller with full view: every fixture address is returned,
// as before this change, and no predicate call is made.
func TestPeopleTools_FullViewIsUnchangedWithoutPredicateCalls(t *testing.T) {
	for _, tc := range peopleTools {
		t.Run(tc.name, func(t *testing.T) {
			api := setupPeopleToolsTest(t)
			api.GrantRelations()
			tc.setup(api, true)
			res, out, err := tc.call(fullViewCtx())
			if err != nil || res == nil || res.IsError {
				t.Fatalf("unexpected failure: %v %s", err, allResultText(t, res))
			}
			all := everything(t, res, out, nil)
			for _, email := range tc.emails {
				if !strings.Contains(all, email) {
					t.Errorf("full view must keep %s:\n%s", email, all)
				}
			}
			if n := predicateRequests(api); n != 0 {
				t.Errorf("full view made %d predicate calls: %+v", n, api.Requests())
			}
			// The upstream answers queued for full view are exactly the calls
			// the tool made before this change; a leftover means a call was
			// skipped, an extra one 404s and fails the call above.
			for path, queue := range api.queues {
				if len(queue) != 0 {
					t.Errorf("%d queued answers for %s were not consumed", len(queue), path)
				}
			}
		})
	}
}

// TestPeopleTools_MissingFlagBehavesAsNoFullView walks the registry with a
// bare context: the outcome matches the denied caller, never the full view.
func TestPeopleTools_MissingFlagBehavesAsNoFullView(t *testing.T) {
	for _, tc := range peopleTools {
		t.Run(tc.name, func(t *testing.T) {
			api := setupPeopleToolsTest(t)
			api.GrantRelations()
			tc.setup(api, false)
			res, out, err := tc.call(WithFullView(context.Background(), false))
			all := everything(t, res, out, err)
			for _, email := range tc.emails {
				if strings.Contains(all, email) {
					t.Errorf("an explicit false flag leaked %s", email)
				}
			}
		})
	}
}

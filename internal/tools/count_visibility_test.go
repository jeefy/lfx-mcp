// Copyright The Linux Foundation and contributors.
// SPDX-License-Identifier: MIT

package tools

import (
	"context"
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

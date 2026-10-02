// Copyright The Linux Foundation and contributors.
// SPDX-License-Identifier: MIT

package tools

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// Mailing lists of one project, as the caller's relations to them make them:
// one the caller manages, one it audits, and one it only subscribes to.
const (
	mlWriterUID     = "150001"
	mlAuditorUID    = "150002"
	mlSubscriberUID = "150003"
)

// mailingListDoc is a groupsio_mailing_list query record; the query
// service's ID is the object ref.
func mailingListDoc(uid string) string {
	return fmt.Sprintf(`{"type": "groupsio_mailing_list", "id": "groupsio_mailing_list:%s", "data": {"uid": %q, "group_id": %s, "project_uid": "P1", "public": true}}`, uid, uid, uid)
}

// mailingListMemberDoc is a groupsio_member query record of list listUID.
func mailingListMemberDoc(uid, listUID, email string) string {
	return fmt.Sprintf(`{"type": "groupsio_member", "id": "groupsio_member:%s", "data": {"uid": %q, "mailing_list_uid": %q, "group_id": %s, "username": "user-%s", "first_name": "Sam", "last_name": "Subscriber", "email": %q, "project_uid": "P1"}}`, uid, uid, listUID, listUID, uid, email)
}

// listRelation is the relation request the rule asks for one list.
func listRelation(uid, relation string) string {
	return "groupsio_mailing_list:" + uid + "#" + relation
}

// memberQueries returns the groupsio_member queries the stub saw.
func mailingListMemberQueries(api *stubLFXAPI) []stubAPIRequest {
	var out []stubAPIRequest
	for _, r := range api.RequestsTo(resourcesPath) {
		if r.Query.Get("type") == mailingListMemberResourceType {
			out = append(out, r)
		}
	}
	return out
}

func TestSearchMailingListMembers_ManagersAndAuditorsGetTheList(t *testing.T) {
	for _, tc := range []struct {
		name    string
		listUID string
		grant   string
	}{
		{"writer", mlWriterUID, listRelation(mlWriterUID, "writer")},
		{"auditor", mlAuditorUID, listRelation(mlAuditorUID, "auditor")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := setupMailingListTest(t)
			api.GrantRelations(tc.grant)
			doc := mailingListMemberDoc("gm-1", tc.listUID, "subscriber@example.test")
			api.Respond(resourcesPath, page([]string{doc}, ""))

			_, out, err := handleSearchMailingListMembers(context.Background(), stubCallToolRequest(), SearchMailingListMembersArgs{MailingListID: tc.listUID, Name: "Sam"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out.Resources) != 1 || dataString(out.Resources[0].Data, "email") != "subscriber@example.test" {
				t.Fatalf("expected the member unchanged, got %+v", out.Resources)
			}
			// Both relations are asked: auditor does not include writer.
			bodies := api.AccessCheckBodies()
			if len(bodies) != 1 || !slices.Contains(bodies[0], listRelation(tc.listUID, "writer")) || !slices.Contains(bodies[0], listRelation(tc.listUID, "auditor")) {
				t.Errorf("expected one access-check asking writer and auditor, got %v", bodies)
			}
			q := mailingListMemberQueries(api)
			if len(q) != 1 || strings.Join(q[0].Query["tags_all"], ",") != "mailing_list_uid:"+tc.listUID || q[0].Query.Get("name") != "Sam" {
				t.Errorf("expected the one-list query as before, got %+v", q)
			}
			if _, ok := q[0].Query["filters_or"]; ok {
				t.Errorf("a shown list needs no narrowing clause, got %v", q[0].Query["filters_or"])
			}
		})
	}
}

func TestSearchMailingListMembers_SubscriberIsRefusedBeforeTheQuery(t *testing.T) {
	api := setupMailingListTest(t)
	// A subscriber is a member, and so a viewer, of the list: neither shows
	// its member list.
	api.GrantRelations(listRelation(mlSubscriberUID, "member"), listRelation(mlSubscriberUID, "viewer"))
	api.Respond(resourcesPath, page([]string{mailingListMemberDoc("gm-1", mlSubscriberUID, "subscriber@example.test")}, ""))

	_, out, err := handleSearchMailingListMembers(context.Background(), stubCallToolRequest(), SearchMailingListMembersArgs{MailingListID: mlSubscriberUID})
	if err == nil || err.Error() != mailingListMembersRefusal {
		t.Fatalf("expected the refusal, got %v", err)
	}
	if len(out.Resources) != 0 || len(mailingListMemberQueries(api)) != 0 {
		t.Errorf("a refused search must not query members, got %d queries", len(mailingListMemberQueries(api)))
	}
}

func TestSearchMailingListMembers_RefusesUnscopedSearches(t *testing.T) {
	for _, args := range []SearchMailingListMembersArgs{{}, {Name: "subscriber@example.test"}} {
		api := setupMailingListTest(t)
		api.GrantRelations()
		api.Respond(resourcesPath, page([]string{mailingListMemberDoc("gm-1", mlWriterUID, "subscriber@example.test")}, ""))
		_, _, err := handleSearchMailingListMembers(context.Background(), stubCallToolRequest(), args)
		if err == nil || err.Error() != mailingListMembersRefusal {
			t.Fatalf("%+v: expected the refusal, got %v", args, err)
		}
		if n := len(api.Requests()); n != 0 {
			t.Errorf("%+v: an unscoped search must make no upstream call, got %d", args, n)
		}
	}
}

func TestSearchMailingListMembers_FailsClosed(t *testing.T) {
	t.Run("relation check fails", func(t *testing.T) {
		api := setupMailingListTest(t)
		api.FailAccessCheck(http.StatusInternalServerError)
		api.Respond(resourcesPath, page([]string{mailingListMemberDoc("gm-1", mlWriterUID, "subscriber@example.test")}, ""))
		_, out, err := handleSearchMailingListMembers(context.Background(), stubCallToolRequest(), SearchMailingListMembersArgs{MailingListID: mlWriterUID})
		if err == nil || err.Error() != peopleVisibilityUnavailableMessage || len(out.Resources) != 0 {
			t.Fatalf("expected the unavailable message and no records, got %v %+v", err, out)
		}
		if n := len(mailingListMemberQueries(api)); n != 0 {
			t.Errorf("no member query after a failed check, got %d", n)
		}
	})
	t.Run("project list lookup fails", func(t *testing.T) {
		api := setupMailingListTest(t)
		api.GrantRelations(listRelation(mlWriterUID, "writer"))
		api.RespondStatus(resourcesPath, http.StatusInternalServerError, `{"message":"boom"}`)
		_, _, err := handleSearchMailingListMembers(context.Background(), stubCallToolRequest(), SearchMailingListMembersArgs{ProjectUID: "P1"})
		if err == nil || err.Error() != peopleVisibilityUnavailableMessage {
			t.Fatalf("expected the unavailable message, got %v", err)
		}
	})
}

func TestSearchMailingListMembers_ProjectIsNarrowedToShownLists(t *testing.T) {
	api := setupMailingListTest(t)
	api.GrantRelations(listRelation(mlWriterUID, "writer"), listRelation(mlAuditorUID, "auditor"), listRelation(mlSubscriberUID, "member"))
	api.Respond(resourcesPath, page([]string{mailingListDoc(mlWriterUID), mailingListDoc(mlAuditorUID)}, "more"))
	api.Respond(resourcesPath, page([]string{mailingListDoc(mlSubscriberUID)}, ""))
	// The page carries a record of the subscriber's list, which the
	// narrowing would not return: the second check drops it.
	api.Respond(resourcesPath, page([]string{
		mailingListMemberDoc("gm-1", mlWriterUID, "one@example.test"),
		mailingListMemberDoc("gm-2", mlAuditorUID, "two@example.test"),
		mailingListMemberDoc("gm-3", mlSubscriberUID, "three@example.test"),
	}, ""))

	_, out, err := handleSearchMailingListMembers(context.Background(), stubCallToolRequest(), SearchMailingListMembersArgs{ProjectUID: "P1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lookups := api.RequestsTo(resourcesPath)
	if len(lookups) != 3 || lookups[0].Query.Get("type") != mailingListResourceType || lookups[0].Query.Get("parent") != "project:P1" || lookups[1].Query.Get("page_token") != "more" {
		t.Fatalf("expected the paged list lookup on the project parent, got %+v", lookups)
	}
	q := mailingListMemberQueries(api)
	if len(q) != 1 {
		t.Fatalf("expected one member query, got %d", len(q))
	}
	if got := strings.Join(q[0].Query["filters_or"], ","); got != "mailing_list_uid:"+mlWriterUID+",mailing_list_uid:"+mlAuditorUID {
		t.Errorf("the query must be narrowed to the shown lists, got filters_or %q", got)
	}
	if got := strings.Join(q[0].Query["tags_all"], ","); got != "project_uid:P1" {
		t.Errorf("the project tag stays, got %q", got)
	}
	var emails []string
	for _, r := range out.Resources {
		emails = append(emails, dataString(r.Data, "email"))
	}
	if strings.Join(emails, ",") != "one@example.test,two@example.test" {
		t.Errorf("expected only the shown lists' members, got %v", emails)
	}
	if all := everything(t, nil, out, nil); strings.Contains(all, "three@example.test") || strings.Contains(all, mlSubscriberUID) {
		t.Errorf("the withheld list must be neither returned nor named:\n%s", all)
	}
}

func TestSearchMailingListMembers_ProjectRefusals(t *testing.T) {
	t.Run("no shown list", func(t *testing.T) {
		api := setupMailingListTest(t)
		api.GrantRelations(listRelation(mlSubscriberUID, "member"))
		api.Respond(resourcesPath, page([]string{mailingListDoc(mlSubscriberUID)}, ""))
		_, _, err := handleSearchMailingListMembers(context.Background(), stubCallToolRequest(), SearchMailingListMembersArgs{ProjectUID: "P1", Name: "Sam"})
		if err == nil || err.Error() != mailingListMembersRefusal || len(mailingListMemberQueries(api)) != 0 {
			t.Fatalf("expected the refusal and no member query, got %v", err)
		}
	})
	t.Run("no visible list", func(t *testing.T) {
		api := setupMailingListTest(t)
		api.GrantRelations()
		api.Respond(resourcesPath, page(nil, ""))
		_, _, err := handleSearchMailingListMembers(context.Background(), stubCallToolRequest(), SearchMailingListMembersArgs{ProjectUID: "P1"})
		if err == nil || err.Error() != mailingListMembersRefusal || len(mailingListMemberQueries(api)) != 0 {
			t.Fatalf("expected the refusal and no member query, got %v", err)
		}
	})
	t.Run("list lookup beyond its page cap", func(t *testing.T) {
		api := setupMailingListTest(t)
		api.GrantRelations()
		for i := 0; i < peopleLookupMaxPages; i++ {
			api.Respond(resourcesPath, page([]string{mailingListDoc(fmt.Sprint(160000 + i))}, fmt.Sprint("p", i)))
		}
		_, _, err := handleSearchMailingListMembers(context.Background(), stubCallToolRequest(), SearchMailingListMembersArgs{ProjectUID: "P1"})
		if err == nil || err.Error() != mailingListLookupCapRefusal {
			t.Fatalf("expected the lookup-cap refusal, got %v", err)
		}
		if len(mailingListMemberQueries(api)) != 0 || len(api.RequestsTo(accessCheckPath)) != 0 {
			t.Error("a refused search must check and query nothing further")
		}
	})
	t.Run("shown lists beyond one clause", func(t *testing.T) {
		api := setupMailingListTest(t)
		var docs, grants []string
		for i := 0; i <= peopleFilterChunk; i++ {
			uid := fmt.Sprint(170000 + i)
			docs = append(docs, mailingListDoc(uid))
			grants = append(grants, listRelation(uid, "writer"))
		}
		api.GrantRelations(grants...)
		api.Respond(resourcesPath, page(docs, ""))
		_, _, err := handleSearchMailingListMembers(context.Background(), stubCallToolRequest(), SearchMailingListMembersArgs{ProjectUID: "P1"})
		if err == nil || err.Error() != mailingListFilterCapRefusal || len(mailingListMemberQueries(api)) != 0 {
			t.Fatalf("expected the filter-cap refusal and no member query, got %v", err)
		}
	})
}

func TestGetMailingListMember_ManagersAndAuditorsGetTheRecord(t *testing.T) {
	for _, grant := range []string{listRelation(fullViewMailingListID, "writer"), listRelation(fullViewMailingListID, "auditor")} {
		t.Run(grant, func(t *testing.T) {
			api := setupMailingListTest(t)
			api.GrantRelations(grant)
			api.Respond(fullViewMailingListMemberPath, fullViewMailingListMemberRecord)
			res, _, err := handleGetMailingListMember(context.Background(), stubCallToolRequest(), GetMailingListMemberArgs{MailingListID: fullViewMailingListID, MemberID: fullViewMailingListMemberID})
			if err != nil || res.IsError || !strings.Contains(allResultText(t, res), "subscriber@example.test") {
				t.Fatalf("expected the record, got %v %s", err, allResultText(t, res))
			}
		})
	}
}

func TestGetMailingListMember_NotShownReadsAsNotFound(t *testing.T) {
	// The text a caller with full view gets for a member that does not
	// exist: the service's 404 through friendlyAPIError.
	api := setupMailingListTest(t)
	api.RespondStatus(fullViewMailingListMemberPath, http.StatusNotFound, `{"name":"NotFound","message":"member not found"}`)
	missing, _, _ := handleGetMailingListMember(fullViewCtx(), stubCallToolRequest(), GetMailingListMemberArgs{MailingListID: fullViewMailingListID, MemberID: fullViewMailingListMemberID})
	if !missing.IsError {
		t.Fatal("expected an error for the missing member")
	}

	api = setupMailingListTest(t)
	api.GrantRelations(listRelation(fullViewMailingListID, "member"), listRelation(fullViewMailingListID, "viewer"))
	api.Respond(fullViewMailingListMemberPath, fullViewMailingListMemberRecord)
	hidden, _, err := handleGetMailingListMember(context.Background(), stubCallToolRequest(), GetMailingListMemberArgs{MailingListID: fullViewMailingListID, MemberID: fullViewMailingListMemberID})
	if err != nil || !hidden.IsError {
		t.Fatalf("expected an error result, got %v %s", err, allResultText(t, hidden))
	}
	if got, want := allResultText(t, hidden), allResultText(t, missing); got != want {
		t.Errorf("a member not shown must read as a missing one:\n got %q\nwant %q", got, want)
	}
	if n := len(api.RequestsTo(fullViewMailingListMemberPath)); n != 0 {
		t.Errorf("the record must not be fetched for a caller it is not shown to, got %d fetches", n)
	}
}

func TestGetMailingListMember_CheckFailureFailsClosed(t *testing.T) {
	api := setupMailingListTest(t)
	api.FailAccessCheck(http.StatusBadGateway)
	api.Respond(fullViewMailingListMemberPath, fullViewMailingListMemberRecord)
	res, _, _ := handleGetMailingListMember(context.Background(), stubCallToolRequest(), GetMailingListMemberArgs{MailingListID: fullViewMailingListID, MemberID: fullViewMailingListMemberID})
	if !res.IsError || strings.TrimSpace(allResultText(t, res)) != peopleVisibilityUnavailableMessage {
		t.Fatalf("expected the unavailable message, got %s", allResultText(t, res))
	}
	if strings.Contains(allResultText(t, res), "subscriber@example.test") {
		t.Error("no record may be returned with the failure")
	}
}

func TestCountLFXResources_MailingListMembersFollowTheRule(t *testing.T) {
	refusal := countRefusal(mailingListMemberResourceType, "parent=groupsio_mailing_list:<uid> for a mailing list you manage or audit")
	for _, tc := range []struct {
		name    string
		grant   string
		args    CountLFXResourcesArgs
		allowed bool
	}{
		{"writer parent", listRelation(mlWriterUID, "writer"), CountLFXResourcesArgs{Parent: "groupsio_mailing_list:" + mlWriterUID, Tags: []string{"status:normal"}}, true},
		{"auditor parent", listRelation(mlAuditorUID, "auditor"), CountLFXResourcesArgs{Parent: "groupsio_mailing_list:" + mlAuditorUID}, true},
		{"subscriber parent", listRelation(mlSubscriberUID, "member"), CountLFXResourcesArgs{Parent: "groupsio_mailing_list:" + mlSubscriberUID}, false},
		{"no parent", listRelation(mlWriterUID, "writer"), CountLFXResourcesArgs{TagsAll: []string{"mailing_list_uid:" + mlWriterUID}}, false},
		{"project parent", listRelation(mlWriterUID, "writer"), CountLFXResourcesArgs{Parent: "project:P1"}, false},
		{"empty list uid", listRelation(mlWriterUID, "writer"), CountLFXResourcesArgs{Parent: "groupsio_mailing_list:"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := setupCountTest(t)
			api.GrantRelations(tc.grant)
			api.Respond(countPath, `{"count": 3, "has_more": false}`)
			args := tc.args
			args.Type = mailingListMemberResourceType
			res, _, _ := handleCountLFXResources(context.Background(), stubCallToolRequest(), args)
			counted := len(api.RequestsTo(countPath))
			if tc.allowed {
				if res.IsError || counted != 1 {
					t.Fatalf("expected the count, got %s", allResultText(t, res))
				}
				return
			}
			if !res.IsError || strings.TrimSpace(allResultText(t, res)) != refusal || counted != 0 {
				t.Fatalf("expected the refusal and no count, got %s (%d counts)", allResultText(t, res), counted)
			}
		})
	}
	t.Run("relation check fails", func(t *testing.T) {
		api := setupCountTest(t)
		api.FailAccessCheck(http.StatusInternalServerError)
		res, _, _ := handleCountLFXResources(context.Background(), stubCallToolRequest(), CountLFXResourcesArgs{Type: mailingListMemberResourceType, Parent: "groupsio_mailing_list:" + mlWriterUID})
		if !res.IsError || strings.TrimSpace(allResultText(t, res)) != peopleVisibilityUnavailableMessage || len(api.RequestsTo(countPath)) != 0 {
			t.Fatalf("expected the unavailable message, got %s", allResultText(t, res))
		}
	})
}

func TestSearchMailingListMembersDescribesTheRule(t *testing.T) {
	const want = "You get members only of mailing lists you manage or audit: set mailing_list_id, or project_uid for all such lists in a project."
	tool := listRegisteredTool(t, "search_mailing_list_members", RegisterSearchMailingListMembers)
	if !strings.Contains(tool.Description, want) {
		t.Errorf("description missing %q", want)
	}
	if n := len(tool.Description); n > schemaDescriptionBudget {
		t.Errorf("description is %d bytes, over the %d budget", n, schemaDescriptionBudget)
	}
	for _, banned := range []string{"Insights", "%", "staff"} {
		if strings.Contains(tool.Description, banned) {
			t.Errorf("description must not contain %q", banned)
		}
	}
}

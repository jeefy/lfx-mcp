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

// testSFID is a well-formed 18-character Salesforce Account SFID.
const testSFID = "001B000000IqhSLIAZ"

// seatsPath is the committee-service seats route for testSFID.
var seatsPath = "/committees/b2b-org/" + testSFID + "/seats"

// seatDoc is one OrgCommitteeSeat as committee-service 0.4.22 serves it
// (cmd/committee-api/design/type.go OrgCommitteeSeatType, decoded by the
// vendored client of the same version). uid/committee_uid/project_uid are
// uuid-formatted and avatar is omitted when empty, as the service does;
// values are test data.
func seatDoc(uid, committeeUID, committeeName, category, projectUID, projectSlug, first, last, email, role string, editable bool) string {
	reason := ""
	if !editable {
		reason = "This seat is foundation-controlled."
	}
	appointedBy := "Membership Entitlement"
	if !editable {
		appointedBy = "Community"
	}
	return fmt.Sprintf(`{
	  "uid": %q,
	  "committee_uid": %q,
	  "committee_name": %q,
	  "committee_category": %q,
	  "project_uid": %q,
	  "project_slug": %q,
	  "first_name": %q,
	  "last_name": %q,
	  "email": %q,
	  "job_title": "Director",
	  "role_name": %q,
	  "voting_status": "Voting Rep",
	  "appointed_by": %q,
	  "organization_id": %q,
	  "is_org_editable": %t,
	  "reason": %q,
	  "username": %q
	}`, uuidFor(uid), uuidFor(committeeUID), committeeName, category, uuidFor(projectUID), projectSlug, first, last, email, role, appointedBy, testSFID, editable, reason, strings.ToLower(first))
}

// uuidFor derives a deterministic, well-formed UUID from a short label so
// fixtures read naturally while satisfying the client's uuid format checks.
func uuidFor(label string) string {
	h := 0
	for _, c := range label {
		h = h*131 + int(c)
	}
	return fmt.Sprintf("%08x-%04x-4%03x-8%03x-%012x", h&0xffffffff, (h>>8)&0xffff, (h>>4)&0xfff, h&0xfff, h&0xffffffffffff)
}

// tenSeatsFixture: ten seats, two categories (Board / Technical), two
// projects (cncf / kubernetes), one duplicate e-mail (ann@x.org holds two
// seats), six editable / four foundation-controlled.
func tenSeatsFixture() []string {
	return []string{
		seatDoc("s01", "c-gb", "Governing Board", "Board", "p-cncf", "cncf", "Ann", "Alpha", "ann@x.org", "Chair", true),
		seatDoc("s02", "c-gb", "Governing Board", "Board", "p-cncf", "cncf", "Bob", "Beta", "bob@x.org", "None", true),
		seatDoc("s03", "c-gb", "Governing Board", "board ", "p-cncf", "cncf", "Cid", "Gamma", "cid@x.org", "None", true),
		seatDoc("s04", "c-toc", "TOC", "Technical", "p-cncf", "cncf", "Ann", "Alpha", "ANN@x.org", "None", false),
		seatDoc("s05", "c-toc", "TOC", "Technical", "p-cncf", "cncf", "Dee", "Delta", "dee@x.org", "Vice Chair", false),
		seatDoc("s06", "c-sc", "Steering", "Technical", "p-k8s", "kubernetes", "Eve", "Epsilon", "eve@x.org", "None", false),
		seatDoc("s07", "c-sc", "Steering", "Technical", "p-k8s", "kubernetes", "Fay", "Zeta", "fay@x.org", "None", false),
		seatDoc("s08", "c-sc", "Steering", "Technical", "p-k8s", "kubernetes", "Gus", "Eta", "gus@x.org", "None", true),
		seatDoc("s09", "c-k8b", "K8s Board", "Board", "p-k8s", "kubernetes", "Hal", "Theta", "hal@x.org", "None", true),
		seatDoc("s10", "c-k8b", "K8s Board", "Board", "p-k8s", "kubernetes", "Ivy", "Iota", "ivy@x.org", "Chair", true),
	}
}

func seatsPage(seats []string, token string) string {
	body := `{"seats": [` + strings.Join(seats, ",") + `]`
	if token != "" {
		body += fmt.Sprintf(`, "page_token": %q`, token)
	}
	return body + "}"
}

func setupOrgSeatsTest(t *testing.T) *stubLFXAPI {
	t.Helper()
	api := newStubLFXAPI(t)
	prev := orgSeatsConfig
	SetOrgSeatsConfig(&OrgSeatsConfig{Clients: api.Clients})
	t.Cleanup(func() { orgSeatsConfig = prev })
	return api
}

func TestOrgSeats_RejectsBadSFID(t *testing.T) {
	api := setupOrgSeatsTest(t)
	for _, bad := range []string{"", "001B000000IqhSLIA", "001B000000IqhSLIAZ1", "001B000000IqhSLIA-"} {
		res, _, _ := handleGetOrgCommitteeSeats(context.Background(), stubCallToolRequest(), GetOrgCommitteeSeatsArgs{B2bOrgUID: bad})
		if !res.IsError || !strings.Contains(allResultText(t, res), "search_b2b_orgs") {
			t.Errorf("b2b_org_uid %q must be rejected with a pointer to search_b2b_orgs, got %q", bad, allResultText(t, res))
		}
	}
	if len(api.Requests()) != 0 {
		t.Error("validation must not reach the API")
	}
}

func TestOrgSeats_SummaryArithmetic(t *testing.T) {
	api := setupOrgSeatsTest(t)
	api.Respond(seatsPath, seatsPage(tenSeatsFixture(), ""))

	res, _, _ := handleGetOrgCommitteeSeats(context.Background(), stubCallToolRequest(), GetOrgCommitteeSeatsArgs{B2bOrgUID: testSFID})
	if res.IsError {
		t.Fatalf("unexpected error: %s", allResultText(t, res))
	}
	r := api.LastRequest()
	if r.Path != seatsPath || r.Query.Get("v") != "1" || r.Query.Get("page_size") != "500" {
		t.Errorf("seats request wrong: %s %v", r.Path, r.Query)
	}
	if _, has := r.Query["project_uids"]; has {
		t.Error("org-wide call must not send project_uids")
	}
	assertExchangedAuth(t, r)

	out := resultJSON(t, res)
	checks := map[string]float64{
		"seats_total": 10, "people": 9, "board_seats": 5, "committee_seats": 5, "editable": 6, "foundation_controlled": 4,
	}
	for k, want := range checks {
		if out[k] != want {
			t.Errorf("%s: want %v got %v", k, want, out[k])
		}
	}
	byCategory := out["by_category"].(map[string]any)
	if byCategory["Board"] != float64(4) || byCategory["board "] != float64(1) || byCategory["Technical"] != float64(5) {
		t.Errorf("by_category keeps stored spelling: %v", byCategory)
	}
	byProject := out["by_project"].(map[string]any)
	if byProject["cncf"] != float64(5) || byProject["kubernetes"] != float64(5) {
		t.Errorf("by_project: %v", byProject)
	}
	byRole := out["by_role"].(map[string]any)
	if byRole["Chair"] != float64(2) || byRole["Vice Chair"] != float64(1) || byRole["None"] != float64(7) {
		t.Errorf("by_role: %v", byRole)
	}
	if out["visibility"] != "organization" || !strings.Contains(out["note"].(string), "Board & Committee tab") {
		t.Errorf("visibility/note: %v %v", out["visibility"], out["note"])
	}
	if _, has := out["seats"]; has {
		t.Error("seats rows must be omitted without include_seats")
	}
}

func TestOrgSeats_IncludeSeatsAndCategoryFilter(t *testing.T) {
	api := setupOrgSeatsTest(t)
	api.Respond(seatsPath, seatsPage(tenSeatsFixture(), ""))

	res, _, _ := handleGetOrgCommitteeSeats(context.Background(), stubCallToolRequest(), GetOrgCommitteeSeatsArgs{B2bOrgUID: testSFID, Category: "bOaRd", IncludeSeats: true})
	out := resultJSON(t, res)
	if out["seats_total"] != float64(5) || out["board_seats"] != float64(5) || out["committee_seats"] != float64(0) {
		t.Errorf("category=Board must keep the five board seats (incl. 'board '), got %v", out)
	}
	if out["people"] != float64(5) {
		t.Errorf("people within the board scope: want 5, got %v", out["people"])
	}
	seats := out["seats"].([]any)
	if len(seats) != 5 {
		t.Fatalf("include_seats must return the five rows, got %d", len(seats))
	}
	row := seats[0].(map[string]any)
	for _, k := range []string{"first_name", "last_name", "email", "role_name", "voting_status", "appointed_by", "committee_name", "project_slug", "is_org_editable"} {
		if _, has := row[k]; !has {
			t.Errorf("seat row missing %s: %v", k, row)
		}
	}
	if row["project_slug"] == "" {
		t.Error("project_slug must survive decoding (the v0.4.0 client dropped it; v0.4.22 carries it)")
	}
}

func TestOrgSeats_FoundationFamilyResolution(t *testing.T) {
	api := setupOrgSeatsTest(t)
	// Two pages of children; ROOT skipped.
	api.Respond(resourcesPath, page([]string{projectDoc("p-k8s", "kubernetes", "Kubernetes", "p-cncf", ""), projectDoc("p-root", "ROOT", "ROOT", "p-cncf", "")}, "more"))
	api.Respond(resourcesPath, page([]string{projectDoc("p-env", "envoy", "Envoy", "p-cncf", "")}, ""))
	api.Respond(seatsPath, seatsPage(tenSeatsFixture()[:3], ""))

	res, _, _ := handleGetOrgCommitteeSeats(context.Background(), stubCallToolRequest(), GetOrgCommitteeSeatsArgs{B2bOrgUID: testSFID, FoundationUID: "p-cncf"})
	if res.IsError {
		t.Fatalf("unexpected error: %s", allResultText(t, res))
	}
	projReqs := api.RequestsTo(resourcesPath)
	if len(projReqs) != 2 || projReqs[0].Query.Get("type") != "project" || projReqs[0].Query.Get("parent") != "project:p-cncf" || projReqs[1].Query.Get("page_token") != "more" {
		t.Errorf("family resolution requests wrong: %+v", projReqs)
	}
	seatReq := api.RequestsTo(seatsPath)[0]
	want := []string{"p-cncf", "p-k8s", "p-env"}
	if got := seatReq.Query["project_uids"]; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("project_uids: want %v got %v (ROOT must be skipped, root first)", want, got)
	}
	out := resultJSON(t, res)
	if out["project_uids_in_scope"] != float64(3) || out["foundation_uid"] != "p-cncf" {
		t.Errorf("scope echo wrong: %v", out)
	}
}

func TestOrgSeats_FamilyResolutionFailureFailsClosed(t *testing.T) {
	api := setupOrgSeatsTest(t)
	api.RespondStatus(resourcesPath, http.StatusInternalServerError, `{"message":"boom"}`)
	res, _, _ := handleGetOrgCommitteeSeats(context.Background(), stubCallToolRequest(), GetOrgCommitteeSeatsArgs{B2bOrgUID: testSFID, FoundationUID: "p-cncf"})
	if !res.IsError {
		t.Fatal("a failed family lookup must not fall back to the root alone")
	}
	if len(api.RequestsTo(seatsPath)) != 0 {
		t.Error("seats must not be fetched after a failed family lookup")
	}
}

func TestOrgSeats_DrainsPagesAndErrorsAtCap(t *testing.T) {
	// Drain: three pages.
	api := setupOrgSeatsTest(t)
	fx := tenSeatsFixture()
	api.Respond(seatsPath, seatsPage(fx[:4], "t1"))
	api.Respond(seatsPath, seatsPage(fx[4:8], "t2"))
	api.Respond(seatsPath, seatsPage(fx[8:], ""))
	res, _, _ := handleGetOrgCommitteeSeats(context.Background(), stubCallToolRequest(), GetOrgCommitteeSeatsArgs{B2bOrgUID: testSFID})
	if out := resultJSON(t, res); out["seats_total"] != float64(10) {
		t.Errorf("drain must collect every page, got %v", out["seats_total"])
	}
	reqs := api.RequestsTo(seatsPath)
	if len(reqs) != 3 || reqs[1].Query.Get("page_token") != "t1" || reqs[2].Query.Get("page_token") != "t2" {
		t.Errorf("page tokens not followed: %+v", reqs)
	}

	// Cap: every page returns a token; must error, never a partial roster.
	api2 := setupOrgSeatsTest(t)
	for i := 0; i < orgSeatsMaxPages+5; i++ {
		api2.Respond(seatsPath, seatsPage(fx[:1], fmt.Sprintf("t%d", i)))
	}
	res2, _, _ := handleGetOrgCommitteeSeats(context.Background(), stubCallToolRequest(), GetOrgCommitteeSeatsArgs{B2bOrgUID: testSFID})
	if !res2.IsError || !strings.Contains(allResultText(t, res2), "foundation_uid") {
		t.Errorf("cap must produce an error pointing at foundation_uid, got %q", allResultText(t, res2))
	}
	if n := len(api2.RequestsTo(seatsPath)); n != orgSeatsMaxPages {
		t.Errorf("must stop at exactly %d pages, made %d", orgSeatsMaxPages, n)
	}
}

func TestOrgSeats_ForbiddenMapsToOrgGrantMessage(t *testing.T) {
	api := setupOrgSeatsTest(t)
	api.RespondStatus(seatsPath, http.StatusForbidden, `{"message":"forbidden"}`)
	res, _, _ := handleGetOrgCommitteeSeats(context.Background(), stubCallToolRequest(), GetOrgCommitteeSeatsArgs{B2bOrgUID: testSFID})
	if !res.IsError {
		t.Fatal("expected an error result")
	}
	text := allResultText(t, res)
	if !strings.Contains(text, "organisation grant") || !strings.Contains(text, "auditor or writer") {
		t.Errorf("403 must explain the org grant, got %q", text)
	}
	// Heimdall answers 403 for an unknown SFID too, so the text must also point
	// at the identifier check.
	if !strings.Contains(text, "not a known organisation") || !strings.Contains(text, "search_b2b_orgs") {
		t.Errorf("403 must carry the unknown-SFID hint, got %q", text)
	}
	if strings.Contains(text, accessDeniedMessage) {
		t.Error("403 on seats must use the org-grant wording, not the generic access-denied message")
	}
}

func TestOrgSeats_OtherErrorsAreFriendly(t *testing.T) {
	api := setupOrgSeatsTest(t)
	api.RespondStatus(seatsPath, http.StatusNotFound, `{"message":"org not found"}`)
	// Goa's default branch wraps unknown statuses as "invalid response code N".
	res, _, _ := handleGetOrgCommitteeSeats(context.Background(), stubCallToolRequest(), GetOrgCommitteeSeatsArgs{B2bOrgUID: testSFID})
	if !res.IsError || !strings.Contains(allResultText(t, res), "404") {
		t.Errorf("404 must pass through friendlyAPIError, got %q", allResultText(t, res))
	}
}

func TestOrgSeats_DescriptionBudgetAndContent(t *testing.T) {
	tool := listRegisteredTool(t, "get_org_committee_seats", RegisterGetOrgCommitteeSeats)
	if n := len(tool.Description); n > 1000 {
		t.Errorf("description is %d bytes, keep it under 1000", n)
	}
	for _, want := range []string{"search_b2b_orgs", "foundation_uid", "category", "organization grant", "include_seats", "Board & Committee", "direct child projects as visible to the caller", "the way LFX Self Serve scopes it", "an organization grant does not make project discovery exhaustive"} {
		if !strings.Contains(tool.Description, want) {
			t.Errorf("description missing %q", want)
		}
	}
	if strings.Contains(tool.Description, "every descendant") {
		t.Error("description must not claim descendants beyond direct children")
	}
	for _, banned := range []string{"Insights", "Jim", "because", "65 KB"} {
		if strings.Contains(tool.Description, banned) {
			t.Errorf("description must not contain %q", banned)
		}
	}
	if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
		t.Error("tool must be read-only")
	}
}

func TestOrgSeats_FamilyResolutionIsCapped(t *testing.T) {
	api := setupOrgSeatsTest(t)
	for i := 0; i < participantMaxDrainPages+5; i++ {
		api.Respond(resourcesPath, page(nil, fmt.Sprintf("t%d", i)))
	}
	res, _, _ := handleGetOrgCommitteeSeats(context.Background(), stubCallToolRequest(), GetOrgCommitteeSeatsArgs{B2bOrgUID: testSFID, FoundationUID: "p"})
	if !res.IsError || !strings.Contains(allResultText(t, res), "page cap") {
		t.Errorf("expected a page-cap error, got %q", allResultText(t, res))
	}
	if len(api.RequestsTo(seatsPath)) != 0 {
		t.Error("seats must not be fetched after a capped family resolution")
	}
}

func TestOrgSeats_ByProjectFallbacksAndRowOrder(t *testing.T) {
	api := setupOrgSeatsTest(t)
	// Same committee + last name twice to exercise the first-name/e-mail tie-breakers;
	// one seat with no project_slug (keyed by uid) and one with neither (keyed "(none)").
	noSlug := strings.Replace(seatDoc("s20", "c-x", "Zed Committee", "Technical", "p-only-uid", "", "Bea", "Same", "bea@x.org", "None", true), `"project_slug": "",`, "", 1)
	noProject := strings.Replace(strings.Replace(seatDoc("s21", "c-x", "Zed Committee", "Technical", "p-none", "", "Abe", "Same", "abe@x.org", "None", true), `"project_slug": "",`, "", 1), fmt.Sprintf(`"project_uid": %q,`, uuidFor("p-none")), "", 1)
	api.Respond(seatsPath, seatsPage([]string{noSlug, noProject}, ""))
	res, _, _ := handleGetOrgCommitteeSeats(context.Background(), stubCallToolRequest(), GetOrgCommitteeSeatsArgs{B2bOrgUID: testSFID, IncludeSeats: true})
	if res.IsError {
		t.Fatalf("unexpected error: %s", allResultText(t, res))
	}
	out := resultJSON(t, res)
	byProject := out["by_project"].(map[string]any)
	if byProject[uuidFor("p-only-uid")] != float64(1) || byProject["(none)"] != float64(1) {
		t.Errorf("by_project must fall back to project_uid then \"(none)\": %v", byProject)
	}
	rows := out["seats"].([]any)
	first, second := rows[0].(map[string]any), rows[1].(map[string]any)
	if first["first_name"] != "Abe" || second["first_name"] != "Bea" {
		t.Errorf("rows with equal committee and last name must order by first name: %v, %v", first["first_name"], second["first_name"])
	}
}

func TestOrgSeats_UpstreamErrorsAreNeverBlank(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		want   string
	}{
		{http.StatusBadRequest, `{"message":"page_size out of range"}`, "page_size"},
		{http.StatusInternalServerError, `{"message":"kv unavailable"}`, "kv unavailable"},
		{http.StatusServiceUnavailable, `{"message":"try again"}`, "try again"},
	} {
		api := setupOrgSeatsTest(t)
		api.RespondStatus(seatsPath, tc.status, tc.body)
		res, _, _ := handleGetOrgCommitteeSeats(context.Background(), stubCallToolRequest(), GetOrgCommitteeSeatsArgs{B2bOrgUID: testSFID})
		text := allResultText(t, res)
		if !res.IsError || strings.TrimSpace(strings.TrimPrefix(text, "Failed to get organization committee seats:")) == "" {
			t.Errorf("%d: blank error text: %q", tc.status, text)
		}
		if !strings.Contains(text, tc.want) {
			t.Errorf("%d: upstream message %q missing from %q", tc.status, tc.want, text)
		}
	}
}

// familyOf returns n child project docs of root plus the expected family
// uids (root first, children in page order).
func familyOf(root string, n int) (docs []string, family []string) {
	family = []string{root}
	for i := 0; i < n; i++ {
		uid := fmt.Sprintf("%s-child-%03d", root, i)
		docs = append(docs, projectDoc(uid, fmt.Sprintf("child%03d", i), fmt.Sprintf("Child %03d", i), root, ""))
		family = append(family, uid)
	}
	return docs, family
}

func TestChunkStrings(t *testing.T) {
	in := []string{"a", "b", "c", "d", "e"}
	got := chunkStrings(in, 2)
	if len(got) != 3 || strings.Join(got[0], "") != "ab" || strings.Join(got[1], "") != "cd" || strings.Join(got[2], "") != "e" {
		t.Errorf("chunkStrings(5, 2): %v", got)
	}
	if got := chunkStrings(in, 5); len(got) != 1 || len(got[0]) != 5 {
		t.Errorf("exact fit must be one chunk: %v", got)
	}
	if got := chunkStrings(in, 10); len(got) != 1 || len(got[0]) != 5 {
		t.Errorf("size larger than input must be one chunk: %v", got)
	}
	if got := chunkStrings(nil, 3); got != nil {
		t.Errorf("empty input must yield no chunks: %v", got)
	}
	if got := chunkStrings(in, 0); len(got) != 1 || len(got[0]) != 5 {
		t.Errorf("size below one must yield the whole input: %v", got)
	}
}

func TestOrgSeats_LargeFamilyIsReadInChunks(t *testing.T) {
	api := setupOrgSeatsTest(t)
	// Root plus 89 children = 90 uids -> chunks of 40, 40, 10.
	docs, family := familyOf("p-big", 89)
	api.Respond(resourcesPath, page(docs, ""))
	fx := tenSeatsFixture()
	// Chunk 1: two pages. Chunk 2: two pages. Chunk 3: one page.
	api.Respond(seatsPath, seatsPage(fx[:2], "c1p2"))
	api.Respond(seatsPath, seatsPage(fx[2:4], ""))
	api.Respond(seatsPath, seatsPage(fx[4:6], "c2p2"))
	api.Respond(seatsPath, seatsPage(fx[6:8], ""))
	api.Respond(seatsPath, seatsPage(fx[8:], ""))

	res, _, _ := handleGetOrgCommitteeSeats(context.Background(), stubCallToolRequest(), GetOrgCommitteeSeatsArgs{B2bOrgUID: testSFID, FoundationUID: "p-big"})
	if res.IsError {
		t.Fatalf("unexpected error: %s", allResultText(t, res))
	}
	reqs := api.RequestsTo(seatsPath)
	if len(reqs) != 5 {
		t.Fatalf("expected five seats requests (3 chunks, two of them paged), got %d", len(reqs))
	}
	// project_uids partition the family in order across the first request of
	// each chunk; continuation pages repeat their chunk's uids.
	var seen []string
	for i, want := range [][]string{family[:40], family[:40], family[40:80], family[40:80], family[80:]} {
		if got := reqs[i].Query["project_uids"]; strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("request %d project_uids: want %d uids starting %s, got %d starting %s", i, len(want), want[0], len(got), firstOf(got))
		}
	}
	for _, i := range []int{0, 2, 4} {
		seen = append(seen, reqs[i].Query["project_uids"]...)
	}
	if strings.Join(seen, ",") != strings.Join(family, ",") {
		t.Error("the chunks' first requests must partition the family in order without gaps or repeats")
	}
	if reqs[1].Query.Get("page_token") != "c1p2" || reqs[3].Query.Get("page_token") != "c2p2" || reqs[0].Query.Get("page_token") != "" || reqs[2].Query.Get("page_token") != "" || reqs[4].Query.Get("page_token") != "" {
		t.Errorf("page tokens must be followed within each chunk and reset between chunks: %+v", reqs)
	}
	out := resultJSON(t, res)
	if out["seats_total"] != float64(10) || out["people"] != float64(9) || out["project_uids_in_scope"] != float64(90) {
		t.Errorf("merged summary must count every seat once across chunks: %v", out)
	}
}

func firstOf(s []string) string {
	if len(s) == 0 {
		return "(none)"
	}
	return s[0]
}

func TestOrgSeats_ChunkForbiddenFailsClosed(t *testing.T) {
	api := setupOrgSeatsTest(t)
	docs, _ := familyOf("p-big", 89)
	api.Respond(resourcesPath, page(docs, ""))
	api.Respond(seatsPath, seatsPage(tenSeatsFixture()[:3], ""))
	api.RespondStatus(seatsPath, http.StatusForbidden, `{"message":"forbidden"}`)
	res, _, _ := handleGetOrgCommitteeSeats(context.Background(), stubCallToolRequest(), GetOrgCommitteeSeatsArgs{B2bOrgUID: testSFID, FoundationUID: "p-big"})
	if !res.IsError || !strings.Contains(allResultText(t, res), "organisation grant") {
		t.Errorf("a 403 on the second chunk must return the forbidden message, got %q", allResultText(t, res))
	}
	if n := len(api.RequestsTo(seatsPath)); n != 2 {
		t.Errorf("must stop at the failing chunk, made %d seats requests", n)
	}
}

func TestOrgSeats_SmallFamilyIsOneRequest(t *testing.T) {
	api := setupOrgSeatsTest(t)
	// Root plus 39 children = exactly the chunk size: one request, unchanged.
	docs, family := familyOf("p-mid", 39)
	api.Respond(resourcesPath, page(docs, ""))
	api.Respond(seatsPath, seatsPage(tenSeatsFixture(), ""))
	res, _, _ := handleGetOrgCommitteeSeats(context.Background(), stubCallToolRequest(), GetOrgCommitteeSeatsArgs{B2bOrgUID: testSFID, FoundationUID: "p-mid"})
	if res.IsError {
		t.Fatalf("unexpected error: %s", allResultText(t, res))
	}
	reqs := api.RequestsTo(seatsPath)
	if len(reqs) != 1 {
		t.Fatalf("a family at or under the chunk size must be one request, made %d", len(reqs))
	}
	if got := reqs[0].Query["project_uids"]; len(got) != orgSeatsProjectChunk || strings.Join(got, ",") != strings.Join(family, ",") {
		t.Errorf("the one request must carry every uid in order: %d uids", len(got))
	}
	if out := resultJSON(t, res); out["seats_total"] != float64(10) {
		t.Errorf("summary: %v", out)
	}
}

func TestOrgSeats_ChunkPageCapIsAnError(t *testing.T) {
	api := setupOrgSeatsTest(t)
	docs, _ := familyOf("p-big", 89)
	api.Respond(resourcesPath, page(docs, ""))
	// First chunk drains in one page; the second never ends.
	api.Respond(seatsPath, seatsPage(tenSeatsFixture()[:1], ""))
	for i := 0; i < orgSeatsMaxPages+5; i++ {
		api.Respond(seatsPath, seatsPage(tenSeatsFixture()[1:2], fmt.Sprintf("t%d", i)))
	}
	res, _, _ := handleGetOrgCommitteeSeats(context.Background(), stubCallToolRequest(), GetOrgCommitteeSeatsArgs{B2bOrgUID: testSFID, FoundationUID: "p-big"})
	if !res.IsError || !strings.Contains(allResultText(t, res), "page cap") {
		t.Errorf("the page cap applies per chunk and is an error, got %q", allResultText(t, res))
	}
	if n := len(api.RequestsTo(seatsPath)); n != 1+orgSeatsMaxPages {
		t.Errorf("must stop at the cap of the second chunk: made %d requests, want %d", n, 1+orgSeatsMaxPages)
	}
}

func TestOrgSeats_PersonSeatedAcrossChunksCountsOnce(t *testing.T) {
	api := setupOrgSeatsTest(t)
	docs, family := familyOf("p-big", 89) // chunks: [0:40], [40:80], [80:90]
	api.Respond(resourcesPath, page(docs, ""))
	// ann@x.org holds a board seat on a project of chunk 1 and a technical
	// seat on a project of chunk 3; bob@x.org one seat in chunk 2.
	api.Respond(seatsPath, seatsPage([]string{
		seatDoc("x01", "c-gb", "Governing Board", "Board", family[3], "chunk1-proj", "Ann", "Alpha", "ann@x.org", "Chair", true),
	}, ""))
	api.Respond(seatsPath, seatsPage([]string{
		seatDoc("x02", "c-sc", "Steering", "Technical", family[45], "chunk2-proj", "Bob", "Beta", "bob@x.org", "None", false),
	}, ""))
	api.Respond(seatsPath, seatsPage([]string{
		seatDoc("x03", "c-toc", "TOC", "Technical", family[85], "chunk3-proj", "Ann", "Alpha", "ANN@x.org", "None", false),
	}, ""))

	res, _, _ := handleGetOrgCommitteeSeats(context.Background(), stubCallToolRequest(), GetOrgCommitteeSeatsArgs{B2bOrgUID: testSFID, FoundationUID: "p-big", IncludeSeats: true})
	if res.IsError {
		t.Fatalf("unexpected error: %s", allResultText(t, res))
	}
	// The three chunks must not overlap and must cover the family exactly.
	reqs := api.RequestsTo(seatsPath)
	if len(reqs) != 3 {
		t.Fatalf("expected three chunk requests, got %d", len(reqs))
	}
	seen := map[string]int{}
	var union []string
	for i, r := range reqs {
		uids := r.Query["project_uids"]
		wantLen := []int{40, 40, 10}[i]
		if len(uids) != wantLen {
			t.Errorf("chunk %d carries %d uids, want %d", i, len(uids), wantLen)
		}
		for _, u := range uids {
			seen[u]++
			union = append(union, u)
		}
	}
	for u, n := range seen {
		if n != 1 {
			t.Errorf("uid %s sent in %d chunks; chunks must not overlap", u, n)
		}
	}
	if strings.Join(union, ",") != strings.Join(family, ",") {
		t.Error("the union of the chunks must be the family, in order")
	}

	// One summary over the merged rows: the person counts once, both seats count.
	out := resultJSON(t, res)
	if out["seats_total"] != float64(3) || out["people"] != float64(2) || out["board_seats"] != float64(1) || out["committee_seats"] != float64(2) {
		t.Errorf("merged summary wrong: seats_total=%v people=%v board=%v committee=%v", out["seats_total"], out["people"], out["board_seats"], out["committee_seats"])
	}
	if out["editable"] != float64(1) || out["foundation_controlled"] != float64(2) || out["project_uids_in_scope"] != float64(90) {
		t.Errorf("merged arithmetic wrong: %v", out)
	}
	byProject := out["by_project"].(map[string]any)
	if byProject["chunk1-proj"] != float64(1) || byProject["chunk2-proj"] != float64(1) || byProject["chunk3-proj"] != float64(1) {
		t.Errorf("by_project must span every chunk: %v", byProject)
	}
	if rows := out["seats"].([]any); len(rows) != 3 || rows[0].(map[string]any)["committee_name"] != "Governing Board" {
		t.Errorf("rows must be merged and sorted once (committee name first): %v", rows)
	}
}

func TestOrgSeats_RepeatedFamilyUIDIsSentOnce(t *testing.T) {
	api := setupOrgSeatsTest(t)
	// The index echoes child p-k8s on both pages and the root as its own child.
	api.Respond(resourcesPath, page([]string{projectDoc("p-k8s", "kubernetes", "Kubernetes", "p-cncf", ""), projectDoc("p-cncf", "cncf", "CNCF", "p-cncf", "")}, "more"))
	api.Respond(resourcesPath, page([]string{projectDoc("p-k8s", "kubernetes", "Kubernetes", "p-cncf", ""), projectDoc("p-env", "envoy", "Envoy", "p-cncf", "")}, ""))
	api.Respond(seatsPath, seatsPage(tenSeatsFixture()[:3], ""))

	res, _, _ := handleGetOrgCommitteeSeats(context.Background(), stubCallToolRequest(), GetOrgCommitteeSeatsArgs{B2bOrgUID: testSFID, FoundationUID: "p-cncf"})
	if res.IsError {
		t.Fatalf("unexpected error: %s", allResultText(t, res))
	}
	want := []string{"p-cncf", "p-k8s", "p-env"}
	if got := api.RequestsTo(seatsPath)[0].Query["project_uids"]; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("each family uid must be sent once, root first: want %v got %v", want, got)
	}
	if out := resultJSON(t, res); out["project_uids_in_scope"] != float64(3) {
		t.Errorf("project_uids_in_scope counts each uid once, got %v", out["project_uids_in_scope"])
	}
}

func TestDedupeStrings(t *testing.T) {
	got := dedupeStrings([]string{"a", "b", "a", "c", "b"})
	if strings.Join(got, "") != "abc" {
		t.Errorf("dedupeStrings keeps the first occurrence in order, got %v", got)
	}
	if got := dedupeStrings(nil); len(got) != 0 {
		t.Errorf("empty input yields empty output, got %v", got)
	}
}

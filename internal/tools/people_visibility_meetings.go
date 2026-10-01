// Copyright The Linux Foundation and contributors.
// SPDX-License-Identifier: MIT

// Package tools provides MCP tool implementations for the LFX MCP server.
//
// This file holds the meeting half of the LFX Self Serve parity rules (see
// people_visibility.go): registrants of upcoming meetings, participants of
// past meetings, and the people fields of meeting, past-meeting, artifact
// and summary records.
package tools

import (
	"context"
	"strings"

	"github.com/linuxfoundation/lfx-mcp/internal/lfxv2"
	querysvc "github.com/linuxfoundation/lfx-v2-query-service/gen/query_svc"
	"github.com/modelcontextprotocol/go-sdk/auth"
)

// --- upcoming meetings (registrants) ---

// registrantView is what LFX Self Serve shows a caller of one meeting's
// registrants.
type registrantView int

const (
	// registrantHidden: nothing. Only organizers and registrants see a list.
	registrantHidden registrantView = iota
	// registrantRoster is the join-page drawer a registrant sees: names,
	// avatars, the organizer badge, job title and organisation, no e-mail.
	registrantRoster
	// registrantOrganizer is the organizer's view: records unchanged.
	registrantOrganizer
)

// registrantViews decides the registrantView of each meeting ID for the
// caller: one access-check batch for organizer, then one registrant query,
// as the caller, for the caller's own registrant records (matched on the
// e-mail tag, raw and lowercased, as Self Serve matches on e-mail) in the
// remaining meetings.
func registrantViews(ctx context.Context, clients *lfxv2.Clients, tokenInfo *auth.TokenInfo, meetingIDs []string) (map[string]registrantView, error) {
	views := make(map[string]registrantView, len(meetingIDs))
	ids := dedupeStrings(meetingIDs)
	if len(ids) == 0 {
		return views, nil
	}
	reqs := make([]string, 0, len(ids))
	for _, id := range ids {
		reqs = append(reqs, "v1_meeting:"+id+"#organizer")
	}
	relations, err := clients.CheckRelations(ctx, reqs)
	if err != nil {
		return nil, err
	}
	var remaining []string
	for _, id := range ids {
		if relations["v1_meeting:"+id+"#organizer"] {
			views[id] = registrantOrganizer
			continue
		}
		views[id] = registrantHidden
		remaining = append(remaining, id)
	}
	email := callerEmail(tokenInfo)
	if len(remaining) == 0 || email == "" {
		return views, nil
	}
	registered, err := callerRegistrantMeetings(ctx, clients, email, remaining)
	if err != nil {
		return nil, err
	}
	for id := range registered {
		if view, known := views[id]; known && view == registrantHidden {
			views[id] = registrantRoster
		}
	}
	return views, nil
}

// callerRegistrantMeetings returns the meeting IDs, among meetingIDs, that
// hold a registrant record for email, from registrant queries as the caller:
// the e-mail tag (raw and lowercased) must match and the meeting_id must be
// one of the IDs, sent in chunks of peopleFilterChunk.
func callerRegistrantMeetings(ctx context.Context, clients *lfxv2.Clients, email string, meetingIDs []string) (map[string]bool, error) {
	resourceType := meetingRegistrantResourceType
	emailTags := dedupeStrings([]string{"email:" + email, "email:" + strings.ToLower(email)})
	registered := make(map[string]bool)
	for _, chunk := range chunkStrings(meetingIDs, peopleFilterChunk) {
		filtersOr := make([]string, 0, len(chunk))
		for _, id := range chunk {
			filtersOr = append(filtersOr, "meeting_id:"+id)
		}
		var pageToken *string
		for pages := 0; ; pages++ {
			if pages >= peopleLookupMaxPages {
				return nil, errPeopleVisibilityCap
			}
			result, err := clients.QuerySvc.QueryResources(ctx, &querysvc.QueryResourcesPayload{
				Version:   "1",
				Type:      &resourceType,
				Tags:      emailTags,
				FiltersOr: filtersOr,
				PageSize:  peopleLookupPageSize,
				Sort:      "name_asc",
				PageToken: pageToken,
			})
			if err != nil {
				return nil, err
			}
			for _, r := range result.Resources {
				if id := dataString(resourceData(r), "meeting_id"); id != "" {
					registered[id] = true
				}
			}
			if !hasPageToken(result.PageToken) {
				break
			}
			pageToken = result.PageToken
		}
	}
	return registered, nil
}

// projectRegistrant reduces one v1_meeting_registrant record to the view. The
// caller's own record is never reduced. ok is false when the record is not
// shown at all.
func projectRegistrant(data map[string]any, view registrantView, tokenInfo *auth.TokenInfo) (out map[string]any, ok bool) {
	switch view {
	case registrantOrganizer:
		return data, true
	case registrantRoster:
		if isOwnRecord(data, tokenInfo) {
			return data, true
		}
		return keepFields(data,
			"uid", "meeting_id", "occurrence", "first_name", "last_name", "avatar_url",
			"host", "job_title", "org_name", "type", "committee_uid",
		), true
	}
	return nil, false
}

// filterRegistrants applies the registrant rule to a page of
// v1_meeting_registrant resources. views must cover every meeting_id on the
// page; a record without one is not shown.
func filterRegistrants(resources []*querysvc.Resource, views map[string]registrantView, tokenInfo *auth.TokenInfo) []*querysvc.Resource {
	out := make([]*querysvc.Resource, 0, len(resources))
	for _, r := range resources {
		data := resourceData(r)
		view, known := views[dataString(data, "meeting_id")]
		if data == nil || !known {
			continue
		}
		projected, ok := projectRegistrant(data, view, tokenInfo)
		if !ok {
			continue
		}
		out = append(out, &querysvc.Resource{Type: r.Type, ID: r.ID, Data: projected})
	}
	return out
}

// trimMeetingPeopleFields reduces a v1_meeting record, in place, to the
// people fields LFX Self Serve renders to a signed-in viewer: the organiser's
// name and e-mail (a mailto link) and nothing about editors, organizer
// accounts or registrant counts.
func trimMeetingPeopleFields(data any) {
	m, ok := data.(map[string]any)
	if !ok {
		return
	}
	dropFields(m, "updated_by", "updated_by_list", "organizers", "user_id", "registrant_count")
	reduceUserField(m, "created_by")
	reduceUserField(m, "owner")
	if occurrences, ok := m["occurrences"].([]any); ok {
		for _, o := range occurrences {
			if occurrence, ok := o.(map[string]any); ok {
				dropFields(occurrence, "registrant_count")
			}
		}
	}
}

// --- past meetings (participants, records, artifacts, summaries) ---

// participantView is what LFX Self Serve shows a caller of one past
// meeting's participants.
type participantView int

const (
	// participantOwnOnly: no list, no counts; the caller's own records only.
	participantOwnOnly participantView = iota
	// participantFullAccess is the join page of a meeting the caller has full
	// access to: attendance counts and the names of the host participants.
	participantFullAccess
	// participantOrganizer is the organizer's view: records unchanged.
	participantOrganizer
)

// pastMeetingAccessDoc is what the rule needs from a v1_past_meeting record.
type pastMeetingAccessDoc struct {
	public     bool
	committees []string
}

// participantViews decides the participantView of each past meeting ID
// (meeting_and_occurrence_id) for the caller: one query for the past-meeting
// records (visibility, restricted, committees), as the caller, then one
// access-check batch for organizer, the direct host/invitee/attendee
// relations and membership of each of the meeting's committees. A past
// meeting whose record the caller cannot read counts as not public.
func participantViews(ctx context.Context, clients *lfxv2.Clients, pastMeetingIDs []string) (map[string]participantView, error) {
	views := make(map[string]participantView, len(pastMeetingIDs))
	ids := dedupeStrings(pastMeetingIDs)
	if len(ids) == 0 {
		return views, nil
	}
	docs, err := pastMeetingAccessDocs(ctx, clients, ids)
	if err != nil {
		return nil, err
	}
	var reqs []string
	for _, id := range ids {
		reqs = append(reqs,
			"v1_past_meeting:"+id+"#organizer",
			"v1_past_meeting:"+id+"#host",
			"v1_past_meeting:"+id+"#invitee",
			"v1_past_meeting:"+id+"#attendee",
		)
		for _, c := range docs[id].committees {
			reqs = append(reqs, "committee:"+c+"#member")
		}
	}
	relations, err := clients.CheckRelations(ctx, reqs)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		switch {
		case relations["v1_past_meeting:"+id+"#organizer"]:
			views[id] = participantOrganizer
		case docs[id].public,
			relations["v1_past_meeting:"+id+"#host"],
			relations["v1_past_meeting:"+id+"#invitee"],
			relations["v1_past_meeting:"+id+"#attendee"],
			anyCommitteeMember(relations, docs[id].committees):
			views[id] = participantFullAccess
		default:
			views[id] = participantOwnOnly
		}
	}
	return views, nil
}

// anyCommitteeMember reports whether relations grants committee:<c>#member
// for any of committees.
func anyCommitteeMember(relations map[string]bool, committees []string) bool {
	for _, c := range committees {
		if relations["committee:"+c+"#member"] {
			return true
		}
	}
	return false
}

// pastMeetingAccessDocs reads the v1_past_meeting records for ids, as the
// caller, in chunks of peopleFilterChunk on meeting_and_occurrence_id. The
// meeting's groups are its committees list plus the single committee_uid
// the record also carries. An id without a record maps to the zero value
// (not public, no committees).
func pastMeetingAccessDocs(ctx context.Context, clients *lfxv2.Clients, ids []string) (map[string]pastMeetingAccessDoc, error) {
	resourceType := pastMeetingResourceType
	docs := make(map[string]pastMeetingAccessDoc, len(ids))
	for _, chunk := range chunkStrings(ids, peopleFilterChunk) {
		filtersOr := make([]string, 0, len(chunk))
		for _, id := range chunk {
			filtersOr = append(filtersOr, "meeting_and_occurrence_id:"+id)
		}
		var pageToken *string
		for pages := 0; ; pages++ {
			if pages >= peopleLookupMaxPages {
				return nil, errPeopleVisibilityCap
			}
			result, err := clients.QuerySvc.QueryResources(ctx, &querysvc.QueryResourcesPayload{
				Version:   "1",
				Type:      &resourceType,
				FiltersOr: filtersOr,
				PageSize:  peopleLookupPageSize,
				Sort:      "name_asc",
				PageToken: pageToken,
			})
			if err != nil {
				return nil, err
			}
			for _, r := range result.Resources {
				data := resourceData(r)
				id := dataString(data, "meeting_and_occurrence_id")
				if id == "" {
					continue
				}
				restricted, _ := data["restricted"].(bool)
				doc := pastMeetingAccessDoc{public: dataString(data, "visibility") == "public" && !restricted}
				if uid := dataString(data, "committee_uid"); uid != "" {
					doc.committees = append(doc.committees, uid)
				}
				if committees, ok := data["committees"].([]any); ok {
					for _, c := range committees {
						if committee, ok := c.(map[string]any); ok {
							if uid := dataString(committee, "uid"); uid != "" {
								doc.committees = append(doc.committees, uid)
							}
						}
					}
				}
				doc.committees = dedupeStrings(doc.committees)
				docs[id] = doc
			}
			if !hasPageToken(result.PageToken) {
				break
			}
			pageToken = result.PageToken
		}
	}
	return docs, nil
}

// participantShown reports whether one raw v1_past_meeting_participant record
// is shown at all under the view: every record of a meeting the caller
// organizes, the caller's own record (matched on the raw record), and the
// host records of a meeting the caller has full access to.
func participantShown(data map[string]any, view participantView, tokenInfo *auth.TokenInfo) bool {
	if view == participantOrganizer || isOwnRecord(data, tokenInfo) {
		return true
	}
	if view != participantFullAccess {
		return false
	}
	host, _ := data["host"].(bool)
	return host
}

// projectParticipant reduces one shown record to the view. The caller's own
// record and an organizer's records are never reduced; a host record of a
// full-access meeting keeps the name and flags the join page renders.
func projectParticipant(data map[string]any, view participantView, tokenInfo *auth.TokenInfo) map[string]any {
	if view == participantOrganizer || isOwnRecord(data, tokenInfo) {
		return data
	}
	return keepFields(data, "uid", "meeting_and_occurrence_id", "first_name", "last_name", "host", "is_attended")
}

// selectParticipants is the first of the two passes the participant rule
// takes over raw records: it drops every record participantShown rejects,
// with full fields intact, before any de-duplication. Identity matching then
// only ever merges records the caller is shown. views must cover every
// meeting_and_occurrence_id present; a record without one is not shown.
func selectParticipants(resources []*querysvc.Resource, views map[string]participantView, tokenInfo *auth.TokenInfo) []*querysvc.Resource {
	out := make([]*querysvc.Resource, 0, len(resources))
	for _, r := range resources {
		data := resourceData(r)
		view, known := views[dataString(data, "meeting_and_occurrence_id")]
		if data == nil || !known || !participantShown(data, view, tokenInfo) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// filterParticipants is the second pass: it reduces each shown record's
// fields to its meeting's view. It also re-applies selectParticipants, so a
// record that was not selected first is never returned.
func filterParticipants(resources []*querysvc.Resource, views map[string]participantView, tokenInfo *auth.TokenInfo) []*querysvc.Resource {
	selected := selectParticipants(resources, views, tokenInfo)
	out := make([]*querysvc.Resource, 0, len(selected))
	for _, r := range selected {
		data := resourceData(r)
		view := views[dataString(data, "meeting_and_occurrence_id")]
		out = append(out, &querysvc.Resource{Type: r.Type, ID: r.ID, Data: projectParticipant(data, view, tokenInfo)})
	}
	return out
}

// dedupeParticipantsPerMeeting de-duplicates people within each past
// meeting, never across meetings, keeping the meetings and the records in
// first-encounter order. A caller without full view may hold different
// views of the meetings on one page; merging across them would let one
// meeting's record decide, or fill in, another's.
func dedupeParticipantsPerMeeting(resources []*querysvc.Resource) []*querysvc.Resource {
	// Group by meeting, remembering where each group's records sat, then
	// place every merged record at the position of its group's next
	// original slot so the page keeps its sort order across meetings.
	groups := make(map[string][]*querysvc.Resource)
	positions := make(map[string][]int)
	for i, r := range resources {
		id := dataString(resourceData(r), "meeting_and_occurrence_id")
		groups[id] = append(groups[id], r)
		positions[id] = append(positions[id], i)
	}
	slots := make([]*querysvc.Resource, len(resources))
	for id, group := range groups {
		for i, merged := range dedupeParticipants(group) {
			slots[positions[id][i]] = merged
		}
	}
	out := make([]*querysvc.Resource, 0, len(resources))
	for _, r := range slots {
		if r != nil {
			out = append(out, r)
		}
	}
	return out
}

// distinctMeetings counts the distinct meeting_and_occurrence_id values.
func distinctMeetings(resources []*querysvc.Resource) int {
	return len(dataStrings(resources, "meeting_and_occurrence_id"))
}

// trimPastMeetingPeopleFields reduces a v1_past_meeting record in place:
// editors are rendered nowhere; the creator is rendered as a name with a
// mailto link.
func trimPastMeetingPeopleFields(data any) {
	m, ok := data.(map[string]any)
	if !ok {
		return
	}
	dropFields(m, "updated_by", "updated_by_list")
	reduceUserField(m, "created_by")
}

// trimPastMeetingArtifactPeopleFields reduces a recording or transcript
// record in place: no screen renders its host or editors.
func trimPastMeetingArtifactPeopleFields(data any) {
	if m, ok := data.(map[string]any); ok {
		dropFields(m, "host_email", "host_id", "created_by", "updated_by")
	}
}

// trimSummaryPeopleFields reduces a v1_past_meeting_summary record in place:
// the summary modal renders the title and content only.
func trimSummaryPeopleFields(data any) {
	if m, ok := data.(map[string]any); ok {
		dropFields(m, "zoom_meeting_host_email", "zoom_meeting_host_id", "created_by", "updated_by")
	}
}

// trimResourcesData applies trim to every resource's Data on a page.
func trimResourcesData(resources []*querysvc.Resource, trim func(any)) {
	for _, r := range resources {
		if r != nil {
			trim(r.Data)
		}
	}
}

// --- helpers shared by the meeting rules ---

// peopleFilterChunk is the most values sent in one filters_or clause.
const peopleFilterChunk = 100

// dropFields removes keys from data in place. A nil map is left alone.
func dropFields(data map[string]any, keys ...string) {
	for _, key := range keys {
		delete(data, key)
	}
}

// reduceUserField replaces data[key], when it is an object, by its name and
// email only: what LFX Self Serve renders for "Organized by" (the name, with
// a mailto link).
func reduceUserField(data map[string]any, key string) {
	obj, ok := data[key].(map[string]any)
	if !ok {
		return
	}
	data[key] = keepFields(obj, "name", "email")
}

// isOwnRecord reports whether a people record belongs to the caller: the
// record's username equals the caller's, or its email equals the caller's
// e-mail claim case-insensitively.
func isOwnRecord(data map[string]any, tokenInfo *auth.TokenInfo) bool {
	if username := callerUsername(tokenInfo); username != "" && dataString(data, "username") == username {
		return true
	}
	if email := callerEmail(tokenInfo); email != "" {
		return strings.EqualFold(strings.TrimSpace(dataString(data, "email")), email)
	}
	return false
}

// tagsAreExactly reports whether every tag is one of values.
func tagsAreExactly(tags []string, values ...string) bool {
	for _, tag := range tags {
		ok := false
		for _, v := range values {
			if tag == v {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

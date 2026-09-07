// Copyright The Linux Foundation and contributors.
// SPDX-License-Identifier: MIT

package tools

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The guidance tools replaced the explore tool's help action: tool results
// carry no byte budget, and tool descriptions/results are the only MCP
// surfaces that reach the model in every client. These tests pin the content
// the same way TestDoctrineHelp pinned the help action — if a token
// disappears, a failure pattern that produced wrong answers in the evals
// loses its recipe.

// TestGuidanceDescriptions_ShortAndFunctional keeps the guidance tools'
// descriptions small: their job is to be found and read, not to compete with
// the query tools for routing budget.
func TestGuidanceDescriptions_ShortAndFunctional(t *testing.T) {
	for name, desc := range map[string]string{
		"read_lfx_semantic_layer_guidance":   semanticLayerGuidanceDescription,
		"read_lfx_standard_metrics_guidance": standardMetricsGuidanceDescription,
	} {
		if got := len(desc); got > 400 {
			t.Errorf("%s description is %d bytes; guidance descriptions stay short (<=400)", name, got)
		}
		if !strings.Contains(desc, "Read") {
			t.Errorf("%s description should instruct when to read it", name)
		}
	}
	// The semantic layer guidance is shared by explore and query; its
	// description names the tools it covers so one read is understood to be
	// enough.
	for _, want := range []string{"explore_lfx_semantic_layer", "query_lfx_semantic_layer", "query_lfx_lens", "once per session"} {
		if !strings.Contains(semanticLayerGuidanceDescription, want) {
			t.Errorf("semantic layer guidance description missing %q", want)
		}
	}
}

// TestGuidanceTools_RegisterReadOnly checks both tools register under their
// gateable names with read-only annotations.
func TestGuidanceTools_RegisterReadOnly(t *testing.T) {
	for _, tc := range []struct {
		name     string
		register func(*mcp.Server)
	}{
		{"read_lfx_semantic_layer_guidance", RegisterSemanticLayerGuidance},
		{"read_lfx_standard_metrics_guidance", RegisterStandardMetricsGuidance},
	} {
		tool := listRegisteredTool(t, tc.name, tc.register)
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("%s must carry ReadOnlyHint", tc.name)
		}
	}
}

// TestGuidanceHandlers_ReturnTheDocuments checks the handlers hand back the
// embedded documents verbatim.
func TestGuidanceHandlers_ReturnTheDocuments(t *testing.T) {
	res, _, err := handleSemanticLayerGuidance(context.Background(), nil, GuidanceArgs{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := resultText(t, res); got != semanticLayerGuidance || len(got) < 5000 {
		t.Errorf("semantic layer guidance result is not the embedded document (len %d)", len(got))
	}
	res, _, err = handleStandardMetricsGuidance(context.Background(), nil, GuidanceArgs{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := resultText(t, res); got != standardMetricsGuidance || len(got) < 1500 {
		t.Errorf("standard metric guidance result is not the embedded document (len %d)", len(got))
	}
}

// TestSemanticLayerGuidanceContent pins the doctrine: every recipe verified
// against the live layer during the August 2026 evals, plus the failure
// modes the 2026-08-31 post-deploy eval rounds surfaced (rollup direction,
// person-grain rankings, events/training account dimensions, LF-wide scope,
// one-hop standard metric recipe filters).
func TestSemanticLayerGuidanceContent(t *testing.T) {
	text := semanticLayerGuidance
	for _, want := range []string{
		// windows and membership parity
		"trailing 12 months",
		"Membership\n  counts as of a past date or by year",
		"are standard metrics, not lens questions",
		"no metric, dimension or\ncolumn names, keys, SQL or tool names",
		"asset_id__end_date",
		"current_membership_count",
		"future-dated",
		// scoping and hierarchy
		"project__foundation_slug",
		"spine_hierarchy_level = 2",
		"risc-v-international/riscv",
		"There is no separate project parameter",
		"conformed lens",
		"asset_id__project_slug",
		"carry the conformed project entity",
		"event_id__project_name",
		"maintainer_key__project_slug",
		"returns ZERO",
		"health_metric_key__foundation_slug",
		"counts only, never sums",
		"__segment_slug",
		"PCC-style foundation rollups",
		`"Direct children of X"`,
		// LF-wide scope ambiguity (round-2 eval divergence)
		"'tlf' slug is the umbrella",
		// syntax
		"metric_time__year",
		"yyyy-mm-dd",
		"omitted = EVERY row",
		// value discovery
		"'Asia Pacific'",
		"Viet Nam",
		"asset_id__billing_country",
		"zero rows",
		// bots
		"member_is_bot",
		"bot_activities",
		"roughly 1.8x",
		// org shares and headcounts
		"org-ATTRIBUTED",
		"Individual - No Account",
		"2-4x",
		// name discovery and rollups
		"International Business Machines Corporation",
		"Red Hat LLC",
		"account__account_rollup_name",
		"subsidiaries INTO parents",
		// tiers, health, value
		"Premier Membership",
		"health_metric_key__health_score_category_v2') }} IS NOT NULL",
		"(Excellent, Healthy, Fair, Concerning, Critical)",
		"current_avg_health_score and current_software_value",
		"answer current-state questions without a pin",
		"total_software_value",
		"COCOMO",
		// populations, maintainers, regions, person grain
		"total_contributors_with_collaboration",
		"2000-01-01 sentinel",
		"maintainer_key__is_lf_project",
		"organization_lf_region",
		"activity_project_id__member_display_name",
		"not identity keys",
		// governance and meetings routing
		"search_committees",
		"search_committee_members",
		"Never infer a roster",
		"search_past_meetings",
		"attendance\nRECORDS, not unique people",
		"'Individual - No Account'",
		"there is no account entity, so no rollup",
		// events/training/sponsorships account entities and tiers
		"account__account_name",
		"NULL bucket",
		"sponsorship__sponsorship_tier_type",
		"'package_tier'",
		// governed org-attribution filter, and the account-attributed superset
		"activity_project_id__is_org_contribution = true",
		"SUPERSET of is_org_contribution",
		// account vs rollup doctrine, in full, once
		"is its parent",
		"returns only what rolls up to THAT subsidiary",
		"Red Hat LLC is itself a rollup parent",
		"Always filter the TOP\nparent",
		"value-searching 'IBM' finds",
		"their OWN rollup",
		"headcounts may NOT",
		// the fuzzy did-you-mean trap
		"get_dimensions(search=) is authoritative",
		// as-of maintainers
		"As of date D: total_maintainers where",
		"one as-of reading per period",
		// events/training/sponsorships are standard metrics now; the ad-hoc
		// shape of each cut stays here for the slices they lack
		"event_registrations,\nevent_sponsorships, speakers, training_enrollments and certifications cover",
		"ACCEPTED registrations",
		"enrollment records only",
		"scope them with project__foundation_slug",
		"leaf project's own slug returns NOTHING",
		"registration_id__event_start_date",
		"and metric_time, the sign-up\ndate",
		"event_id__event_name",
		"enrollment_id__course_name",
		"enrollment_id__product_type",
		"every org-scoped figure here is a\nfloor",
		// other surfaces are never a reconciliation target
		"is code contributions, bots excluded",
		"Do NOT reconcile figures against other\n  dashboards or pages",
		"PCC-style reporting is the\n  reconciliation surface",
		// worked examples stay live-verified
		"## Worked examples (verified live)",
		// resolve-first: a guessed slug or account name is a silent wrong answer
		"ALWAYS resolve names first",
		"has not come back from them",
		// the layer's own reach is one hop / one level, and it says so, with
		// the standard metrics as the any-depth route
		"REACH — how deep this layer's own dimensions go",
		"ONE HOP",
		"not their own acquisitions",
		"for the whole company at any depth use the standard\nmetric",
		"already resolved to the parent account FOR THAT PROJECT",
		// where each domain attaches
		"ATTACHMENT LEVELS",
		"legitimately\n  near-empty",
		// windows cut on Pacific days, layer-wide
		"Day boundaries are US-Pacific",
		// standard metric calls
		"STANDARD METRIC CALLS take uniform parameters",
		"this means INDIVIDUALS by contribution volume — run it, do not ask",
		"16. SOCIAL LISTENING. The standard metrics social_mentions (by total,\nproject, network, sentiment) and social_reach (by total, project) cover the\ncommon readings — prefer them",
		"Compose here only for a slice they lack",
		"stored as 'Twitter', not 'X'",
		"no filter means ALL of LF",
		"17. WHAT GOES IN THE ANSWER",
		"There is no free filter on a standard metric",
		"default combined",
		"default excluded",
		"start_date,\nend_date, period (day|week|month|quarter|year)",
		"there is no\nsince, until or as_of",
		"DEFAULTS are the plain reading",
		"applied block",
		"a WINDOW family counts between start_date and end_date",
		"an AT-DATE family (memberships, maintainers,\nproject_health, software_value) reports the state on end_date",
		"no lens call needed",
		"Every date is a UTC calendar day",
		"separate and combined cover a named node's tree and a company's subsidiaries\nat ANY depth",
		"maintainer_contributions (by=project or by=org",
		"PEOPLE is maintainer_contributions by=maintainer",
		"query_lfx_standard_metrics",
		"call query_lfx_standard_metrics directly; do not explore this layer or the\n  lens first",
		"Where this layer and the standard metrics read differently (both are\n  right; say which one you used)",
		"the family counts LF projects only",
		"the family uses the event start date",
		"the family counts\n  Accepted only",
		"the family omits them",
		"are unattributed there",
		"(asset_id__end_date IS NULL OR asset_id__end_date >= 'D')",
		"The 'tlf' slug is the umbrella's own\n  bucket, not the LF-wide scope; state which population you used.",
		"An LF-wide total takes NO project; the\nfoundation's own slug (tlf) is one bucket, not the LF-wide scope.",
		"an end\ndate that is NULL means still active",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("semantic layer guidance missing %q", want)
		}
	}
}

// TestStandardMetricsGuidanceContent pins what the standard-metric guidance
// must say: resolve names before calling, the contract table with the three
// date parameters and the removed ones, the two kinds with their examples,
// the defaults and the applied block, the two switches and their opposite
// defaults, the inventory with every family and grouping, the exact-literal
// rules and what a guard's candidates mean, how to read results, the worked
// calls and the rejections. No absolute figure anywhere (a figure in the
// guidance goes stale and gets quoted), and function, not rationale.
func TestStandardMetricsGuidanceContent(t *testing.T) {
	text := standardMetricsGuidance
	for _, want := range []string{
		// resolve names first, always
		"## Resolve names first — ALWAYS",
		"search_projects",
		"search_b2b_orgs",
		"has\nnot come back from them",
		"never a zero",
		"never pass the everyday name again",
		// the contract table and the removed parameters
		"## The contract",
		"| start_date | yyyy-mm-dd, a UTC calendar day | the family's window (below) |",
		"| end_date | yyyy-mm-dd, a UTC calendar day; for \"each year since X\" or \"through now\" leave it unset",
		"| period | day, week, month, quarter or year: one row per period | none = one figure |",
		"There is no since, until or as_of",
		"rejected by the\nrequest schema as an unexpected property; the words are start_date and\nend_date",
		"There is no free-form filter",
		// the four questions
		"\"TOP CONTRIBUTORS\" with nothing\n   more said means INDIVIDUALS",
		"run it, do not ask which reading was meant",
		"subprojects: excluded | separate | combined, DEFAULT combined",
		"subsidiaries: excluded | separate | combined, DEFAULT excluded",
		"a subsidiary is a different company",
		"a subproject is part of its project",
		"rejects org",
		// the two kinds
		"A WINDOW family counts what happened between start_date and end_date\n     inclusive",
		"An AT-DATE family reports the state on end_date",
		"partial_last_period",
		"the state on a single day is end_date alone",
		"## The two kinds, with examples",
		"end_date=2022-12-31. Any day but today is read date-based",
		"reads a few percent above the status-based current count",
		"people on TODAY's\n  roster with a code contribution in each year",
		"The roster itself has no\n  honest history",
		"applied.coverage says how many LF-hosted projects carry a v2 score; give\n  it when the count is the answer",
		// defaults and the applied block
		"## Defaults and the applied block",
		"end_date defaults to today (UTC)",
		"includes_future_dated",
		"the trailing 365 days before end_date",
		"all history on new_members and\nmembership_churn",
		"the trailing year on any day or week series",
		"runs from\nthe first row of data",
		"timezone (always UTC)",
		"definition (one sentence",
		"defaulted (the list of parameters the lens\nchose)",
		"truncated (limit cut\nrows off)",
		"`engine` is provenance\nfor you and never goes in an answer",
		"Do not compare the figure\nwith a number from another dashboard",
		// the switches
		"## The switches, row by row",
		"| combined (default) | X plus everything under it, any depth | folded together: the project columns leave the result; the rows are whatever by groups",
		"| separate | X plus everything under it, any depth | as the metric groups them, one row each: the breakdown |",
		"| excluded | X's own bucket only, nothing under it |",
		"| excluded (default) | the Y account only |",
		"| combined | Y plus every subsidiary under it, any depth | folded together: the org columns leave the result; the rows are whatever by groups",
		"one row per parent organization",
		"MOST QUESTIONS WANT BOTH",
		"Never derive one from the other: distinct counts do\nnot sum",
		// inventory: every family, kind and grouping list — derived below from
		// standardMetricNames and standardMetricGroupings, the single source
		"## Inventory",
		// the definitions and caveats that change how a figure is read
		"revenue is LIST PRICE, not dues billed — never divide one by the other",
		"membership_count, membership_revenue on any day but today and on a series",
		"region is provisional",
		"the churn date is the day AFTER the term ended",
		"known for about a third of contributors",
		"one row per GitHub identity (`handle`, a profile URL)",
		"Organizations from the enrichment vocabulary, not CRM accounts",
		"A superset of contributors",
		"today's roster active in each period",
		"Maintainership as of the build, contributions in the window",
		"category is the stored v2 band name (Excellent, Healthy, Fair, Concerning, Critical), never a threshold",
		"project_health_count, avg_project_health_score",
		"additive across projects, never across days",
		"totals read low, never inflated",
		"The window is the EVENT start date",
		"distinct people by email, not registrations: never sum them across rows",
		"Distinct people with an Accepted speaker status",
		"rejected and in-review proposals are excluded",
		"the edX branch carries no account",
		"neutral or unknown sentiment is in neither positive nor negative",
		"The sum counts a prolific author once per mention",
		// organizations: exact literals, the guard, the candidates
		"names its organization column `account`",
		"ANY DEPTH:\nseparate and combined walk the account hierarchy to the bottom",
		"LITERALS ARE EXACT",
		"STRAY SAME-COMPANY ACCOUNT",
		"the rejection lists\nup to five data-bearing candidates",
		"the parent legal name first",
		"never present a\ncandidate as the caller's own choice",
		// projects
		"AT ANY DEPTH\n(grandchildren included)",
		"never sum to a subtree total",
		"Memberships attach at FOUNDATION level",
		"'kubernetes' is\nnot a slug, 'k8s' is",
		// what goes in the answer
		"## What goes in the answer",
		"only the caveats that change how THIS figure is read",
		"in the reader's words",
		"Column names, keys, engines, SQL and the other tools\nstay out",
		// reading results
		"## Reading results",
		"Every date is a UTC calendar day, on both engines",
		"`period` is the first day of each period",
		"`period_end` is the day the state was read on",
		"never an organization, never\n  folded into a parent",
		"Present them side by side, never as a ratio",
		"rows are GitHub identities",
		"Two identities sharing a display name are two\n  rows",
		"never as a contact list",
		"never mix\n  the two in one answer",
		"truncated=true means limit cut rows off",
		// worked calls, one per kind, a series, an org, a rejection
		"## Worked calls",
		"One figure, window: contributors, project=cncf, start_date=2025-01-01",
		"One figure, at-date: memberships, project=cncf",
		"A series, LF-wide: new_members, period=year, no project",
		"An org with subsidiaries: contributions, org=International Business\n  Machines Corporation, subsidiaries=combined",
		"A rejection: memberships, start_date=2020-01-01",
		// errors
		"## Errors",
		"read it and change the call, do\nnot retry the same one",
		"the message names only\n  the property you sent, so the replacement is the contract table above",
		"an org that matches no data-bearing account: 400 with candidates",
		"an unknown project slug: 400 with candidates",
		"an order_by field that is not one of the result columns",
		"no snapshot on or before end_date",
		// round 9: date-free health disclosure; the tool reports the version
		"- project_health: The count and the categories are v2. The average reads\n  the v1 score now and will read the v2 score normalized to a hundred-point\n  scale",
		"applied.definition says which one\n  this call read",
		"v2 snapshots have a short history",
		"In the answer: the figure, the scope,\n  the snapshot day, and the version applied.definition gives — one line.",
		"applied.definition says which average this call read (by=population reads the normalized v2 average already)",
		"snapshots have a short history).",
		// round 9: answer economy, the umbrella slug, placeholders, one lens query
		"When unsure whether a note belongs, leave it\nout: the reader can ask.",
		"\"How many members\ndoes the LF have\" →\nmemberships, no project",
		"leave project unset. The foundation's own slug (tlf) names one bucket",
		"A search_projects hit for the foundation's name\nis not a reason to scope.",
		"- A series, LF-wide: new_members, period=year, no project",
		"that is the bucket, not the LF.",
		"naming one and dropping the other is the same failure as naming neither",
		"report it as returned and name the\n  cause (a stray same-name account, say) as the caveat; do not re-issue it\n  with different filter logic",
		"A scored project without a stored maximum counts in the total\n  but not in the average.",
		"a scored project without a stored maximum counts in the total but not in the average; v2 snapshots have a short history;",
		// round 7 addendum: preconditions, not background
		"if a\nsibling with a '-fund' slug is there, the memberships sit on it",
		"stating the stored spelling here does not exempt\nit: k8s, cncf and tlf still come back from search_projects in-session",
		"Do not compute a\nstart_date by counting back N years from today",
		"report every row the series returned, or say how many rows\nthere are and which ones you show",
		// round 11: executive readings vs the layer's (Part E)
		"an organization with memberships on three projects counts three times",
		"a reading this\n  family does not give yet (it arrives as its own metric)",
		"say the organization reading is not\n  available rather than deriving it",
		"a reading this family does not give yet (it arrives as its own metric): report memberships, say the grain",
		"count with\n  by=total, list with limit and order_by (applied.truncated says when the\n  list is partial)",
		"last N years, last N months, trailing quarter",
		"paying-only, new-to-the-LF and lost organizations are the same kind of reading",
		"\"new logos\" (organizations new to the LF altogether) is a different reading this family does not give",
		"an organization that dropped one project but kept another still counts here and is not a lost member",
		"the LF region rollup lists China, India and Japan beside Asia Pacific",
		"check-in data exists only for some registration sources, so an event whose source carries none shows zero attendees, not low attendance",
		"also holds accounts whose stored country spelling the lens does not resolve",
		"also those whose stored country spelling the lens does not\n  resolve",
		// round 10: series order and flagged rows
		"A series arrives in period order, oldest first",
		"a figure that is not in a row does\n  not exist",
		"start_date=2020-01-01, period=year, no end_date: one row per year end",
		"the last one today's state with\n  partial_last_period set",
		"a future end_date reads the scheduled state on that day and sets includes_future_dated, it is not the to-date row",
		"A flagged row (partial_last_period,\nincludes_future_dated) is reported with the applied block's wording, never\ndropped",
		"do not trim the table to a rounder\nwindow when writing up",
		"check the by=org rows for BOTH names before finalizing a table",
		"never silently dropped",
		"Decide the lens query's\n  scope before issuing it, not after seeing the result",
		// verification exercise (round 5): the disclosed lines
		"means participants (a\n  code contribution OR a collaboration activity)",
		"show the call shape only",
		"only when the applied block sets the\nflag",
		"a computed start clips the first period",
		"a top-N cut hides subsidiaries a hand-sum would miss",
		"sibling fund\nproject",
		"'Individual - No Account' and 'TI Account' are placeholder accounts",
		"a spelling variant (Redhat, Micro Soft) returns none",
		"since, until, as_of, group_by, where: rejected by the request schema",
		"A future end_date does not move the default window\nstart",
		"omit accounts and courses with no enrollment in the window",
		"by=org omits accounts with no certification in the window",
		"each LF-hosted project's own latest snapshot row on or before end_date",
		"applied.snapshot_date is null",
		"an ad hoc query by registration date reads differently",
		"an ad hoc count over all statuses reads higher",
		"an ad hoc count over the whole maintainers index reads higher",
		"With no rollup asked for, the reading is subsidiaries=excluded",
		"because a rollup was asked for; absent that, start from\n  excluded",
		"ONE lens query whose filter mirrors the family's stated definition",
		"subsidiaries=separate needs by=org and subprojects=separate\nneeds by=project",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("standard metric guidance missing %q", want)
		}
	}
	// Every family's inventory row, in the guidance's own table shape, from the
	// one list the description and the schema tests use too: a grouping
	// added on one surface and not the other fails here.
	for _, name := range standardMetricNames {
		row := "| " + name + " | " + standardMetricKinds[name] + " | " + standardMetricGroupings[name] + " |"
		if !strings.Contains(text, row) {
			t.Errorf("standard metric guidance missing inventory row %q", row)
		}
	}
	// Nothing from the old contract survives in the document as a live
	// instruction (as_of, since and until are named only as the words a
	// rejection replaces).
	for _, gone := range []string{"since/until", "takes as_of", "FLOW", "SNAPSHOT", "US-Pacific", "display name — \"top", "one-hop"} {
		if strings.Contains(text, gone) {
			t.Errorf("standard metric guidance still says %q, which the contract removed", gone)
		}
	}
}

// TestStandardMetricsGuidanceCarriesNoFigure pins that the guidance quotes
// no absolute figure: a number in the guidance goes stale the day after it
// is written and gets quoted as if it were the answer. Dates, parameter
// counts, HTTP statuses and the 365-day default are the only numbers.
func TestStandardMetricsGuidanceCarriesNoFigure(t *testing.T) {
	allowed := regexp.MustCompile(`^(20\d\d(-\d\d(-\d\d)?)?|365|400|404|1|2|3|4|5|31|01)$`)
	// A date is one token, not three: consume yyyy-mm-dd before bare numbers.
	for _, match := range regexp.MustCompile(`\b\d{4}-\d\d-\d\d\b|\b\d[\d,.]*\b`).FindAllString(standardMetricsGuidance, -1) {
		match = strings.TrimRight(match, ".,")
		if !allowed.MatchString(match) {
			t.Errorf("standard metric guidance carries the figure %q; figures go stale and get quoted", match)
		}
	}
}

// TestGuidanceNamesNoOtherSurface pins a product decision: the guidance
// describes exactly what a figure covers and offers the breakdown or another
// window, and never sends the model to compare with, or explain away, a
// number on another LFX surface by name.
func TestGuidanceNamesNoOtherSurface(t *testing.T) {
	for name, text := range map[string]string{
		"semantic layer guidance":  semanticLayerGuidance,
		"standard metric guidance": standardMetricsGuidance,
		"tool description":         standardMetricsDescription,
	} {
		if strings.Contains(text, "Insights") {
			t.Errorf("%s names Insights; describe what the figure covers instead", name)
		}
	}
}

// TestGuidanceHealthIsV2ByName pins the health vocabulary: the layer is
// undeclaring the v1 category dimension, so the guidance names only the v2
// one, and categories are the stored band names, never a score threshold.
func TestGuidanceHealthIsV2ByName(t *testing.T) {
	v1 := regexp.MustCompile(`health_score_category\b[^_]|Stable|Unsteady|Critical <|\b20-39\b`)
	for name, text := range map[string]string{
		"semantic layer guidance":  semanticLayerGuidance,
		"standard metric guidance": standardMetricsGuidance,
		"tool description":         standardMetricsDescription,
	} {
		if m := v1.FindString(text); m != "" {
			t.Errorf("%s still carries the v1 health vocabulary: %q", name, m)
		}
	}
}

// TestGuidanceCarriesNoCalendarDate pins Josep's rule for disclosures: a
// sentence says how a thing reads now and what it changes to, and the tool
// reports which state applied; a calendar date or a deployment week makes
// the sentence undecidable on the day it names. Years are allowed only as
// worked-call parameters, MetricFlow date literals, quoted example questions
// and the layer's stored sentinel value.
func TestGuidanceCarriesNoCalendarDate(t *testing.T) {
	year := regexp.MustCompile(`\b20\d\d\b`)
	allowed := regexp.MustCompile(`_date=|'20\d\d-\d\d-\d\d'|"[^"]*20\d\d[^"]*"|2000-01-01 sentinel|in 20\d\d"`)
	for name, text := range map[string]string{
		"semantic layer guidance":  semanticLayerGuidance,
		"standard metric guidance": standardMetricsGuidance,
	} {
		for n, line := range strings.Split(text, "\n") {
			if year.MatchString(line) && !allowed.MatchString(line) {
				t.Errorf("%s line %d carries a calendar date outside an example: %q", name, n+1, line)
			}
		}
	}
	for _, banned := range []string{"week of", "DBT-1 deployment lands", "from that deployment", "count them"} {
		if strings.Contains(standardMetricsGuidance, banned) {
			t.Errorf("standard metric guidance still says %q", banned)
		}
	}
}

// TestCombinedFoldsAHierarchyNotTheResult pins the Copilot round-3 fix: a
// combined fold removes the project or org columns and keeps whatever the by
// grouping produces; only by=total is one figure. No client text may say
// combined is "one row" or "one figure", which had callers reading a valid
// by=org breakdown as a wrong shape.
func TestCombinedFoldsAHierarchyNotTheResult(t *testing.T) {
	tool := listRegisteredTool(t, "query_lfx_standard_metrics", RegisterStandardMetrics)
	raw, err := json.Marshal(tool.InputSchema)
	if err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{
		"standard metrics schema":   string(raw),
		"standard metrics guidance": standardMetricsGuidance,
		"semantic layer guidance":   semanticLayerGuidance,
	} {
		for _, banned := range []string{
			"folded into ONE figure", "folded into ONE row", "folded into one row",
			"those folded into one row", "one figure is combined",
		} {
			if strings.Contains(text, banned) {
				t.Errorf("%s still says %q: combined folds a hierarchy, not the result", name, banned)
			}
		}
	}
	for _, want := range []string{"the project columns leave the result", "the org columns leave the result"} {
		if !strings.Contains(string(raw), want) || !strings.Contains(standardMetricsGuidance, want) {
			t.Errorf("schema and guidance must both say %q", want)
		}
	}
}

// Copyright The Linux Foundation and contributors.
// SPDX-License-Identifier: MIT

package tools

import (
	"regexp"
	"strings"
	"testing"
)

// A maintainer activity series is not a reconstruction of past rosters. Put
// the exception beside each generic at-date rule, including the required
// metric field that survives schema compaction, rather than only in an example.
func TestMaintainerExceptionBesideEveryAtDateRule(t *testing.T) {
	const exception = "maintainers is the exception: today's roster only; with period, one row per period of today's maintainers active in it, not the roster at that time"
	tool := listStandardMetricsTool(t)
	for name, text := range map[string]string{
		"metric schema":             schemaPropertyDescription(t, tool, "metric"),
		"period schema":             schemaPropertyDescription(t, tool, "period"),
		"standard metrics guidance": standardMetricsGuidance,
		"semantic layer guidance":   semanticLayerGuidance,
	} {
		t.Run(name, func(t *testing.T) {
			foundRule := false
			for _, paragraph := range regexp.MustCompile(`\n\s*\n`).Split(text, -1) {
				paragraph = strings.Join(strings.Fields(paragraph), " ")
				if !strings.Contains(paragraph, "AT-DATE family") &&
					!strings.Contains(paragraph, "AT-DATE:") &&
					!strings.Contains(paragraph, "at-date families report") {
					continue
				}
				foundRule = true
				if !strings.Contains(paragraph, exception) {
					t.Errorf("at-date rule/list lacks the adjacent maintainer exception: %s", paragraph)
				}
			}
			if !foundRule {
				t.Fatal("no at-date rule found; retain the generic contract and its exception")
			}
		})
	}
}

func TestYearIsDocumentedOnceAsACompatibilityAlias(t *testing.T) {
	text := strings.Join(strings.Fields(standardMetricsGuidance), " ")
	const alias = "by=year is accepted as a compatibility alias of period=year on new_members and membership_churn; it is not a grouping and the family lists do not include it"
	if !strings.Contains(text, alias) {
		t.Error("guidance must distinguish the year compatibility alias from canonical groupings")
	}
	if got := strings.Count(text, "by=year"); got != 1 {
		t.Errorf("by=year occurs %d times; explain the alias once, use period=year in examples", got)
	}
}

func TestMembershipAsOfUsesTheNonNullTermEnd(t *testing.T) {
	text := strings.Join(strings.Fields(semanticLayerGuidance), " ")
	for _, want := range []string{
		"Members as of date D: membership_count with metric_time <= 'D' AND asset_id__end_date >= 'D'",
		"end_date is never NULL; open-ended terms carry a far-future placeholder",
		"never churn_date, which is derived from a different end column and undercounts",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("membership as-of guidance missing %q", want)
		}
	}
	if strings.Contains(text, "asset_id__end_date IS NULL") {
		t.Error("membership as-of guidance still invents NULL-ended terms")
	}
}

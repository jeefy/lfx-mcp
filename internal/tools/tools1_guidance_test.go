// Copyright The Linux Foundation and contributors.
// SPDX-License-Identifier: MIT

// Package tools provides MCP tool implementations for the LFX MCP server.
package tools

import (
	"strings"
	"testing"
)

// TestTools1GuidanceDistinguishesCountsFromPagedListings pins the tool
// contracts in recipe 12: the listing route is not a count/completeness API.
func TestTools1GuidanceDistinguishesCountsFromPagedListings(t *testing.T) {
	text := strings.Join(strings.Fields(semanticLayerGuidance), " ")
	for _, want := range []string{
		"TWO ROUTES, TWO DEFINITIONS.",
		"search_past_meetings returns a paged listing.",
		"count_lfx_resources provides meeting counts",
		"search_past_meeting_participants with count_only=true provides participant counts",
		"both count what the caller's identity may see",
		"and say whether the count is complete",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("recipe 12 missing %q", want)
		}
	}
	if strings.Contains(text, "count_lfx_resources, search_past_meetings and search_past_meeting_participants count") {
		t.Error("recipe 12 must not promise counts or completeness from search_past_meetings")
	}
}

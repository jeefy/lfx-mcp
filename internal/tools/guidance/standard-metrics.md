# LFX standard metrics — agent guidance

Read this before your first query_lfx_standard_metrics call, and again
whenever a call surprises you. A standard metric is a governed recipe: its
metrics, its grouping and its built-in filters are fixed, and a caller
chooses only the grouping (by), the scope and the dates, so the same
question re-run gives the same figure. When one matches the question, prefer
it over the explore+query flow, and never rewrite it as an ad-hoc query just
to sort, filter or scope it — order_by, the scope switches and the dates do
that on the governed recipe. A standard metric reaches a project's tree and a
company's subsidiaries AT ANY DEPTH, which the explore+query flow does not
(see "Organizations" and "Projects" below).

## Resolve names first — ALWAYS

project takes a stored project slug from search_projects; org takes an
organization's legal name, in its stored spelling, from search_b2b_orgs.
Stored spellings are not the everyday ones ('k8s', 'ptproject', 'Red Hat
LLC'). A name either tool has returned in this session may be reused as is;
what is never acceptable is passing a slug or an organization name that has
not come back from them. The lens guards both: an unknown slug, or an
organization name that matches no account carrying data, is a rejection
with up to five candidates — never a zero. Pick from the candidates or go
back to the search tool; never pass the everyday name again. The slugs in
this document's worked calls and examples (cncf, tlf, k8s) show the call shape only;
resolve them in-session like any other name. Before any project-scoped
membership figure, look at what search_projects returned for the name: if a
sibling with a '-fund' slug is there, the memberships sit on it — query both
and report the combined figure, saying so.

## The contract

Every family takes the same parameters, and nothing per family:

| Parameter | Meaning | Default |
|---|---|---|
| metric | the family (inventory below) | required |
| by | exactly one grouping from the family's list | the family's first (total) |
| project | one slug from search_projects, exact | none = LF-wide |
| subprojects | excluded, separate or combined: what the project name covers | combined |
| org | one stored legal account name from search_b2b_orgs, exact | none |
| subsidiaries | excluded, separate or combined: what the org name covers | excluded |
| start_date | yyyy-mm-dd, a UTC calendar day | the family's window (below) |
| end_date | yyyy-mm-dd, a UTC calendar day; for "each year since X" or "through now" leave it unset — a future end_date reads the scheduled state on that day and sets includes_future_dated, it is not the to-date row | today (UTC) |
| period | day, week, month, quarter or year: adds a time dimension to by | none = no time series |
| order_by | result columns, - prefix for descending | none |
| limit | maximum rows; the result says whether it was cut | none = every row |

"The Linux Foundation" or "Linux Foundation X" as a whole means LF-wide:
leave project unset. The foundation's own slug (tlf) names one bucket of a
few dozen projects among the hosted ones, never the umbrella; pass it only
when the question is about that bucket, and the result's applied.note says
so whenever it is passed. A search_projects hit for the foundation's name
is not a reason to scope.

There is no since, until or as_of: a call that names one is rejected by the
request schema as an unexpected property; the words are start_date and
end_date. There is no free-form filter.

by=year is accepted as a compatibility alias of period=year on new_members
and membership_churn; it is not a grouping and the family lists do not
include it.

Answer four questions, then call once.

1. WHICH METRIC, GROUPED HOW? Pick the family from the inventory by what it
   answers, then its grouping with by: a one-figure question is by=total; a
   "which ... most" or "per ..." question is by=org, by=project, by=tier,
   by=country, by=event and so on, as the family offers. Left out, by is the
   family's first grouping. The scope supplies the other axis, so the two
   combine: "which CNCF projects does IBM work on" is contributions
   by=project with project=cncf, org=<IBM's legal name>,
   subprojects=separate; "which companies contribute to Kubernetes" is
   contributions by=org with project=k8s. "TOP CONTRIBUTORS" with nothing
   more said means INDIVIDUALS ranked by volume: contributions
   by=contributor with order_by=-code_contribution_activities and a limit —
   run it, do not ask which reading was meant. "Top maintainers" likewise is
   maintainer_contributions by=maintainer. The other readings each have a
   governed metric — name them as follow-ups, do not ask first: top
   organizations by volume = contributions by=org, by headcount =
   contributors by=org; top projects likewise by=project.
2. WHICH SCOPE? project, org, both, or neither for an LF-wide figure. A
   family whose model carries no account (speakers, project_health,
   software_value, social_mentions, social_reach) rejects org.
3. WHAT DOES THE NAME COVER? Two switches, and their defaults differ on
   purpose — a subsidiary is a different company ("Red Hat" means Red Hat),
   a subproject is part of its project ("CNCF" means all of CNCF).
   - subprojects: excluded | separate | combined, DEFAULT combined.
   - subsidiaries: excluded | separate | combined, DEFAULT excluded.
4. WHICH DATES? Two kinds of family, and the same three parameters on both.
   - A WINDOW family counts what happened between start_date and end_date
     inclusive. period adds a time dimension to by: by=org with period=month
     is one row per organization per month; column `period` is the period's
     first day. Without period the by grouping remains.
   - An AT-DATE family (memberships, member_organizations,
     paying_member_organizations, maintainers, project_health,
     software_value) reports the state on end_date. With period, one row
     per period end from start_date to end_date; if end_date falls inside a
     period the last row is the state on end_date and the applied block says
     partial_last_period. maintainers is the exception: today's roster only;
     with period, one row per period of today's maintainers active in it,
     not the roster at that time. start_date without period on an at-date
     family is a rejection: the state on a single day is end_date alone.

## The two kinds, with examples

WINDOW: new_members (install date), membership_churn (churn date),
contributors, contributions, contributing_organizations, participants,
maintainer_contributions (activity date), event_registrations,
event_sponsorships, speakers (the EVENT's start date), training_enrollments,
certifications (enrollment date), social_mentions, social_reach (mention
date).

- "How many contributors did CNCF have in 2025" → contributors,
  project=cncf, start_date=2025-01-01, end_date=2025-12-31.
- "Contributions per month this year" → contributions, start_date=<1 Jan>,
  period=month. Each row is one month; the last one is month-to-date.
- "New members per year" → new_members, period=year.

- "How many members does CNCF have" → memberships, project=cncf. Today's
  state, status-based.
- "How many members did the LF have at the end of 2022" → memberships,
  end_date=2022-12-31. Any day but today is read date-based: installed on
  or before the day and not ended by it; the applied block's definition
  says so, and it reads a few percent above the status-based current count.
- "Members at each year end since 2020" → memberships,
  start_date=2020-01-01, period=year, no end_date: one row per year end,
  each the state on 31 December, the last one today's state with
  partial_last_period set.
- "Maintainers over time" → maintainers, period=year: people on TODAY's
  roster with a code contribution in each year. The roster itself has no
  honest history, so maintainers without period and with an end_date other
  than today is a rejection that says exactly this.
- project_health: The count, the categories and the average are v2; the
  average is the v2 score normalized to a hundred-point scale (raw v2
  points over each project's coverage-dependent maximum) on both engines,
  and applied.definition says which column this call read. A scored
  project without a stored maximum counts in the total but not in the
  average. It is read on the latest snapshot on or before
  end_date (applied.snapshot_date); v2 snapshots have a short history, and
  an end_date before the first one is a rejection that says so, not a zero.
  applied.coverage says how many LF-hosted projects carry a v2 score; give
  it when the count is the answer. In the answer: the figure, the scope,
  the snapshot day, and the version applied.definition gives — one line.
- software_value is not pinned to one day: each LF-hosted project is read
  as of its own latest snapshot row on or before end_date, and a project
  whose latest row carries no value contributes nothing, so
  applied.snapshot_date stays null and applied.coverage says how many
  LF-hosted projects carry a value. Say the coverage with the total.

## Defaults and the applied block

end_date defaults to today (UTC); an explicit future end_date is honoured
and flagged includes_future_dated. start_date defaults to the family's
window: the trailing 365 days before end_date on the activity, event,
training and social families; all history on new_members and
membership_churn; the trailing year on any day or week series. A month,
quarter or year series on an at-date family with no start_date runs from
the first row of data. A future end_date does not move the default window
start; a window that starts after today returns zero rows with
includes_future_dated set — set start_date explicitly for a future-dated
window. For a relative phrase (last N years, last N months, trailing quarter) take the
family's default window or a bare period series rather than a hand-picked
start_date: a computed start clips the first period. Do not compute a
start_date by counting back N years from today for "last N years": that
arithmetic is the mistake the rule exists to prevent; a bare period=year
series returns every year and the reader picks. The same discipline applies
to the answer: report every row the series returned, or say how many rows
there are and which ones you show; do not trim the table to a rounder
window when writing up.

Every result carries an `applied` block: metric, by, kind (window or
at-date), project, subprojects, org, subsidiaries, start_date, end_date,
period, timezone (always UTC), definition (one sentence: which stored
definition produced the figure), defaulted (the list of parameters the lens
chose), partial_last_period, includes_future_dated, truncated (limit cut
rows off), snapshot_date, coverage, engine. Read the scope and the window
from it, never from the request you think you sent. The sentence under a
figure comes from `definition` and `defaulted`. Say partial_last_period,
includes_future_dated or truncated only when the applied block sets the
flag; never infer one by comparing dates yourself. `engine` is provenance
for you and never goes in an answer. Then offer what a reader most often wants
next — the breakdown, another window, the series. Do not compare the figure
with a number from another dashboard.

## The switches, row by row

For a named project X:

| subprojects | rows read | rows returned |
|---|---|---|
| combined (default) | X plus everything under it, any depth | folded together: the project columns leave the result; the rows are whatever by groups (by=total one figure, by=org one row per organization) |
| separate | X plus everything under it, any depth | as the metric groups them, one row each: the breakdown |
| excluded | X's own bucket only, nothing under it | as the metric groups them |

For a named organization Y:

| subsidiaries | rows read | rows returned |
|---|---|---|
| excluded (default) | the Y account only | as the metric groups them |
| separate | Y plus every subsidiary under it, any depth | one row each, with parent_org alongside |
| combined | Y plus every subsidiary under it, any depth | folded together: the org columns leave the result; the rows are whatever by groups (by=total one figure, by=project one row per project) |

With no rollup asked for, the reading is subsidiaries=excluded (the named
account alone); choosing combined or separate to answer a broader question
is a scope choice the answer names in words, not something left to the
applied block. separate is the breakdown, so it needs the grouping that
carries it: subsidiaries=separate needs by=org and subprojects=separate
needs by=project — on any other grouping the call is rejected; combined
folds the hierarchy and keeps that grouping's rows, and by=total is what
asks for one figure. With no org named, subsidiaries=combined on a by=org
reading is one row per parent organization, resolved to the top of each
chain. MOST QUESTIONS WANT BOTH the headline and the breakdown — two calls,
combined then separate. Never derive one from the other: distinct counts do
not sum.

## Inventory

Result columns are given in the vocabulary the results come back in. A
series adds `period` in front; an at-date series adds `period_end` too.

| Family | Kind | by | What it answers | Result columns | Definition and caveat |
|---|---|---|---|---|---|
| memberships | at-date | total, org, tier, project, country, region | Memberships and list-price dues on end_date | [account, parent_org / tier / project, project_name / country / region,] current_membership_count, current_membership_revenue (membership_count, membership_revenue on any day but today and on a series) | Distinct project-account pairs with an active term: an organization with memberships on three projects counts three times; "how many members" in the everyday sense is distinct member organizations — that is member_organizations (paying-only: paying_member_organizations), never derived from these rows; say which reading you report; revenue is LIST PRICE, not dues billed — never divide one by the other; memberships attach at foundation level; country and region are the account's billing country and its LF region (region is provisional); the LF region rollup lists China, India and Japan beside Asia Pacific — "APAC" in the everyday sense is those four rows together: report the sum and name the rows, or the stored row and say it excludes them; the NULL country row also holds accounts whose stored country spelling the lens does not resolve, not only accounts with no country |
| new_members | window | total, org, project | Memberships sold as new business, by install date | [account, parent_org / project, project_name,] new_membership_count | New business is first-per-project: an organization joining a second project counts again, and a lapsed account that rejoins counts again; "new logos" (organizations new to the LF altogether) is new_member_organizations — say which one you report; all history unless start_date |
| membership_churn | window | total, org, project | Memberships that ended without renewal, by churn date | [...,] churned_membership_count | Churned memberships are project-account pairs that ended: an organization that dropped one project but kept another still counts here and is not a lost member (lost organizations are lost_member_organizations) — say which reading you report; the churn date is the day AFTER the term ended, so a term ending on 31 December counts in the following year; zero-revenue and quasi-associate memberships excluded |
| member_organizations | at-date | total, project, foundation, country, region | Distinct member organizations on end_date | [project, project_name / foundation / country / region,] member_organizations | ORGANIZATIONS, not memberships: a Salesforce account holding memberships in several projects counts once here and once per project on memberships — "how many members do we have" is this reading; today is status-based, any other day and a series are date-based (installed on or before the day and not ended by it; reads a few percent above the status-based figure; applied.definition and applied.engine say which); an organization in several projects appears in every matching row of a breakdown, so rows never sum to the total; no by=org (that is memberships by=org) and no by=tier (an organization can hold several tiers); memberships attach at foundation level; country and region are the account's billing country and its LF region (region is provisional; the NULL row also holds unresolved spellings) |
| new_member_organizations | window | total, project, foundation, country, region | Organizations whose FIRST membership was installed in the window | [project, project_name / foundation / country / region,] new_member_organizations | Firstness follows the scope and applied.firstness names it: no project = first LF membership ever ("new logos"); project with subprojects=excluded = first membership in that project; project that is a foundation root (default combined) = first membership in that foundation; a foundation root that belongs to a consortium (a JDF series and its -fund project) = first membership in ANY of the consortium's projects, applied.consortium_members listing them; a project below foundation level with combined is REJECTED (see Errors) — pass subprojects=excluded or name the foundation; same-day installs in two projects both count as first; a lapsed account that rejoins is not new (contrast new_members); with an org named, total reads zero or one per window — the answer to "when did X first join"; all history unless start_date; rows never sum to the total |
| lost_member_organizations | window | total, project, foundation, country, region | Organizations whose LAST membership lapsed in the window | [project, project_name / foundation / country / region,] lost_member_organizations | The organization grain of churn: an organization that dropped one project but kept another is churned on membership_churn and NOT lost here; counted once, at the churn date of its last term (the day AFTER it ended, so a term ending on 31 December counts in the following year), and only while it holds no active membership; by=project and by=foundation are where that last membership sat; all history unless start_date; rows never sum to the total |
| paying_member_organizations | at-date | total, project, foundation, country, region | Distinct member organizations with a paying membership on end_date | [project, project_name / foundation / country / region,] paying_member_organizations | The paying subset of member_organizations: at least one active membership whose LIST PRICE is above zero (not dues billed or collected; zero-price and associate memberships do not count); today status-based, any other day and a series date-based; the same grouping rules and the same "rows never sum" caveat as member_organizations |
| contributors | window | total, org, project, country, region | Distinct code contributors | [account, parent_org / project, project_name / country / region,] total_contributors | Distinct people, bots excluded: never sum rows; country and region follow the PERSON and are known for about a third of contributors — the NULL row is the rest, and the NULL country row also holds people whose stored country spelling the lens does not resolve; the LF region rollup lists China, India and Japan beside Asia Pacific — "APAC" in the everyday sense is those four rows together: report the sum and name the rows, or the stored row and say it excludes them |
| contributions | window | total, org, project, contributor, type, platform, org_region | Code contribution volume | [account, parent_org / project, project_name / handle, contributor, account / type / platform / org_region,] code_contribution_activities | Additive, bots excluded; by=contributor is one row per GitHub identity (`handle`, a profile URL) with the display name and the resolved account; org_region is the LF region of the employer's HQ |
| contributing_organizations | window | total, project | Distinct organizations credited with a code contribution | [project, project_name,] total_contributing_organizations | Organizations from the enrichment vocabulary, not CRM accounts; a distinct count: never sum rows |
| participants | window | total, org, project | Distinct non-bot people with ANY activity in the window | [...,] total_participants | The broadest people count and much larger than contributors: code, issues, reviews, comments, stars, forks, meeting invitations and attendance, training and exams, Hacker News all count; contributors = code, participants = anyone who did anything; a superset of contributors; distinct people: never sum rows |
| maintainers | at-date | total, org, project, maintainer | Active maintainers today (LF projects only); with period, today's roster active in each period | [account / foundation, project, project_name / project, project_name, maintainer, account, role,] active_maintainers | Distinct people; LF projects only: an ad hoc count over the whole maintainers index reads higher; the NULL account row is maintainers with no resolved employer; today only unless period; by=maintainer has no series |
| maintainer_contributions | window | total, org, project, maintainer | Code contributions by people on the CURRENT maintainer roster of the activity's project | [...,] maintainer_contributions[, contributing_maintainers] | Maintainership as of the build, contributions in the window; maintainer_contributions is additive, contributing_maintainers a distinct count; "share of work" is this over contributions for the same scope and window |
| project_health | at-date | total, foundation, category, population | Projects with a v2 health score and their mean score, on the latest snapshot on or before end_date | [foundation / category / population,] project_health_count, avg_project_health_score | The count, the categories and the average are v2 (the average normalized to a hundred-point scale on both engines; applied.definition says which column this call read); a scored project without a stored maximum counts in the total but not in the average; v2 snapshots have a short history; a subset of LF-hosted projects (applied.coverage); LF-hosted unless by=population (rows lf_hosted and index); category is the stored v2 band name (Excellent, Healthy, Fair, Concerning, Critical), never a threshold; a distinct-project count: never sum rows; a mean of project scores: never re-average |
| software_value | at-date | total, foundation, population | COCOMO software value summed over each LF-hosted project's own latest snapshot row on or before end_date, whatever day that row is on | [foundation / population,] total_software_value | USD; not pinned to one day (applied.snapshot_date is null): each project as of its own latest row; a project whose latest row is a health-only day contributes nothing, so totals read low, never inflated — applied.coverage says how many LF-hosted projects carry a value; additive across projects, never across days |
| event_registrations | window | total, event, org | Accepted registrations of events starting in the window, and the distinct people behind them | [event / account, parent_org,] total_registrations, total_unique_registrants, total_checked_in_attendees | The window is the EVENT start date (an ad hoc query by registration date reads differently); registrants and checked-in attendees are distinct people by email, not registrations: never sum them across rows; check-in data exists only for some registration sources, so an event whose source carries none shows zero attendees, not low attendance; by=org is the registrant's account, NULL = unattributed |
| event_sponsorships | window | total, org, event | Sponsorship revenue and count of sponsorships, for events starting in the window (one event can carry several) | [account, parent_org / event,] total_sponsorship_revenue, total_sponsorship_count | Additive; USD; all tier types |
| speakers | window | total, event, org | Accepted speakers of events starting in the window | [event / account, parent_org,] total_speakers | Distinct people with an Accepted speaker status (Sessionize proposals accepted, Bevy listed speakers); rejected and in-review proposals are excluded, so an ad hoc count over all statuses reads higher; by=org is the speaker's account as resolved from the proposal — about one accepted speaker in five resolves to none and sits in the NULL account row; placeholder accounts fold into that row |
| training_enrollments | window | total, org, course | Enrollments and enrolled users by enrollment date | [account, parent_org / course,] total_enrollments, total_enrolled_users | Platform data only (TI + edX), so lifetime totals read below the official trained figure; the edX branch carries no account and sits in the NULL account row; placeholder accounts ('Individual - No Account', 'TI Account') are folded into the NULL account row on by=org: report that row as unattributed learners, never as an organization, never silently dropped; enrolled users is a distinct count |
| certifications | window | total, org | Completed certifications by enrollment date | [account, parent_org,] total_certifications | Additive; counted at enrollment time; the edX branch has no account (the NULL account row); placeholder accounts ('Individual - No Account', 'TI Account') are folded into the NULL account row on by=org: report that row as unattributed learners, never as an organization, never silently dropped |
| social_mentions | window | total, project, network, sentiment | Social listening mentions, distinct authors and sentiment by mention date | [project, project_name / network / sentiment,] social_listening_mentions, social_listening_unique_authors, social_listening_positive_mentions, social_listening_negative_mentions | Mention counts are additive; unique_authors is a distinct count; neutral or unknown sentiment is in neither positive nor negative; no org scope |
| social_reach | window | total, project | Potential reach of the mentions by mention date | [project, project_name,] social_listening_total_author_followers, social_listening_avg_author_followers | The sum counts a prolific author once per mention; the average is per mention; NULL follower counts excluded; no org scope |

## Organizations: account and parent_org

Every by=org reading names its organization column `account`: the account
that holds the record, as the CRM spells it. Most carry a second column,
`parent_org`: with an org named it is the account's direct parent; without
one (a parent leaderboard) it is the top of the account's chain. ANY DEPTH:
separate and combined walk the account hierarchy to the bottom, so there is
nothing to disclose about depth on these figures. (In the explore+query flow
the same reading is account__account_rollup_name, which is one hop,
labelled ad hoc.)

LITERALS ARE EXACT. org takes the stored legal name — 'International
Business Machines Corporation', 'Red Hat LLC', 'Google LLC', 'Microsoft
Corporation' — and the everyday name is often a STRAY SAME-COMPANY ACCOUNT
carrying no memberships and no activity ('IBM', 'Google', 'Microsoft').
Those used to return zero; now they are rejected, and the rejection lists
up to five data-bearing candidates with their active memberships and
trailing-year contributions, the parent legal name first. Pick one; never
sum the stray accounts' rows into a parent figure, and never present a
candidate as the caller's own choice.

## Projects and subprojects

project takes one slug and covers, by default, its whole tree AT ANY DEPTH
(grandchildren included) folded into one figure. The default is coverage,
never permission to add rows up: a subprojects=separate table of distinct
counts covers the whole subtree with nothing missing, and its rows still
never sum to a subtree total — that is a reason to prefer the standard
metric, not something to caption. Memberships attach at FOUNDATION level, so
a leaf project's own memberships are zero by attachment, not by data loss.
An unknown slug is rejected with candidate slugs and names; 'kubernetes' is
not a slug, 'k8s' is — and stating the stored spelling here does not exempt
it: k8s, cncf and tlf still come back from search_projects in-session before
they go in a call.

CONSORTIA. A JDF series and its '-fund' project (search_projects returns
both, the fund with a '-fund' slug) are one consortium in the CRM, two roots
side by side. On the membership and organization families (memberships,
new_members, membership_churn, member_organizations,
new_member_organizations, lost_member_organizations,
paying_member_organizations) a project named with the default
subprojects=combined reads its subtree PLUS every project of its consortium
in one call, and the applied block says so: `consortium` names it and
`consortium_members` lists every slug merged. Report the one figure and name
the members. subprojects=excluded reads the named project alone and merges
nothing. Families on other models (activity, events, training, health) never
merge a consortium.

## What goes in the answer

Everything in this document is working knowledge: it shapes the call and
how you read the result. The ANSWER carries the figure, one line on what it
covers (from the applied block: scope, window or date, definition), and
only the caveats that change how THIS figure is read — an unattributed row,
a distinct count that must not be summed, a partial last period, a
future-dated end, maintainership read as of today, a date-based membership
reading — in the reader's words: "ranked by GitHub identity", not a note on
display names and keys. Column names, keys, engines, SQL and the other tools
stay out unless the reader asks how a figure was made. Keep the rest in
context: vocabulary, grains and hierarchy depth are said only when they
answer the question or change the conclusion. Then offer the breakdown,
another window or the series. When unsure whether a note belongs, leave it
out: the reader can ask. Coverage, snapshot mechanics, version notes and
thresholds appear only when they qualify THIS figure or the reader is
contrasting it with another. A flagged row (partial_last_period,
includes_future_dated) is reported with the applied block's wording, never
dropped or explained with calendar arithmetic of your own. "How many members
does the LF have" →
memberships, no project: the figure, "active memberships across the
LF-hosted projects today", and the offer of the breakdown by project.

## Reading results

- Start from the `applied` block: it is the scope, the dates and the
  definition that ran, and the sentence under the figure comes from it.
- Every date is a UTC calendar day, on both engines; say "UTC" only when a
  reader compares with another clock.
- `period` is the first day of each period; on an at-date series
  `period_end` is the day the state was read on. partial_last_period is set
  on both kinds and means the last row is to-date, not a full period.
- by=org rows come per account with the parent alongside: read account
  rankings straight off the rows, and take parent figures from
  subsidiaries=combined rather than from a client-side sum — for additive
  metrics too: a top-N cut hides subsidiaries a hand-sum would miss; a
  breakdown returns every row and can run to thousands: count with
  by=total, list with limit and order_by (applied.truncated says when the
  list is partial).
- "How many developers participated / took part / how big is the community"
  means participants (any non-bot activity, stars and forks included); name
  contributors (code only) as the narrower alternative, not the default, and
  say the two are different populations.
- The NULL account row is unattributed work, and the NULL employer row is a
  maintainer whose employer was not resolved — never an organization, never
  folded into a parent. Report them as unattributed. The NULL country or
  region row is people or accounts whose country is not known — and, on
  by=country, also those whose stored country spelling the lens does not
  resolve.
- Membership counts and membership revenue are different grains: the count is
  distinct project-account pairs with an active term, and the revenue is the
  LIST PRICE of the active membership assets, not the dues actually billed.
  Present them side by side, never as a ratio. On any day but today, and on
  a series, the reading is date-based and the columns are membership_count
  and membership_revenue; say "as of <date>" and that it is the date-based
  count. memberships counts project-account pairs: an organization with
  memberships on three projects counts three times; "how many members" in
  the everyday sense is distinct member organizations — member_organizations
  (paying-only: paying_member_organizations; new to the LF:
  new_member_organizations; lost: lost_member_organizations). Never derive
  one grain from the other; the answer names which reading it reports.
- contributions by=contributor and maintainer_contributions by=maintainer
  rows are GitHub identities: `handle` is the stored identity (a profile
  URL), `contributor` or `maintainer` the display name, `account` the one
  the activity resolved to. Two identities sharing a display name are two
  rows. Personal names: present them only where naming individuals is
  appropriate, never as a contact list. maintainers by=maintainer is the
  roster: one row per person, project, employer and role.
- A contribution row's account is already resolved to the parent account
  for that project before parent_org applies, so it can differ from the
  source record. The enrichment spelling of a company ('Red Hat') is a
  different vocabulary from the CRM account name ('Red Hat LLC'); never mix
  the two in one answer.
- A series arrives in period order, oldest first; quote each period's figure
  from its own row and nothing else — a figure that is not in a row does
  not exist.
- A window drops rows with no usable timestamp, so an all-time figure can
  exceed the sum of its windows.
- truncated=true means limit cut rows off: say "top N", not "all".
- Every result carries compiled_sql; quote it only when someone asks how a
  figure was produced.

## Worked calls

- One figure, window: contributors, project=cncf, start_date=2025-01-01,
  end_date=2025-12-31 → one row; the answer says "distinct code
  contributors to CNCF and its projects in 2025".
- One figure, at-date: memberships, project=cncf → today's active
  memberships; add end_date=2024-12-31 for the year-end state (date-based).
- A series, LF-wide: new_members, period=year, no project → one row per
  year of installation across the hosted projects, the current year to
  date. (project=tlf here would give the foundation's own bucket of a few
  dozen projects: that is the bucket, not the LF.)
- An org with subsidiaries: contributions, org=International Business
  Machines Corporation, subsidiaries=combined → IBM and everything under it
  as one figure, because a rollup was asked for; absent that, start from
  excluded. by=org with subsidiaries=separate for the table with parent_org.
- A family that rejects org (speakers, project_health, software_value, the
  social families) with an org breakdown still wanted: say the rejection,
  then ONE lens query whose filter mirrors the family's stated definition
  (the inventory row's status and source conditions), labelled as generated
  SQL; never several differently phrased attempts. Decide the lens query's
  scope before issuing it, not after seeing the result; a low or surprising
  figure is not grounds for a second phrasing. If the one sanctioned lens
  query returns a surprising figure, report it as returned and name the
  cause (a stray same-name account, say) as the caveat; do not re-issue it
  with different filter logic.
- Organizations, not memberships: member_organizations, no project → today's
  distinct member organizations across the LF; add end_date=2022-12-31 for
  the year-end state (date-based; applied.engine says warehouse).
- New to the LF this year versus last: new_member_organizations,
  start_date=2026-01-01 → one figure, applied.firstness "first membership
  ever"; the same with start_date=2025-01-01, end_date=2025-12-31 for the
  comparison. project=tlf makes it "first membership in foundation tlf";
  project=c2pa makes it first membership in the C2PA consortium (series and
  fund together, applied.consortium_members).
- Lost in a year: lost_member_organizations, start_date=2025-01-01,
  end_date=2025-12-31 → organizations whose last membership lapsed in 2025;
  say "organizations", contrast membership_churn (pairs).
- Paying today: paying_member_organizations, no project → today's paying
  member organizations; say "list price above zero", not dues collected.
- A rejection: memberships, start_date=2020-01-01 → "start_date needs period
  for an at-date metric; the state on a single day is end_date alone" — add
  period=year for the series, or drop start_date for one day.

## Errors

Every rejection names the rule and the fix; read it and change the call, do
not retry the same one.

- an unknown metric, or a grouping the family does not offer: the message
  lists the valid names or groupings.
- new_member_organizations with a project below foundation level and
  subprojects=combined: "first-membership is defined for a single project
  or for a whole foundation" — pass subprojects=excluded for organizations
  whose first membership in that project falls in the window, or name the
  foundation the message gives for organizations new to it. Never a number.
- start_date without period on an at-date family; an end_date other than
  today on maintainers without period; period on a family and grouping
  with no series yet.
- an org that matches no data-bearing account: 400 with candidates (see
  Organizations) — candidates are stored names containing the text you
  sent; a spelling variant (Redhat, Micro Soft) returns none — resolve with
  search_b2b_orgs; an unknown project slug: 400 with candidates.
- subsidiaries=separate without by=org, or subprojects=separate without
  by=project: the breakdown needs its grouping; combined folds the
  hierarchy and keeps the by rows; by=total is one figure.
- since, until, as_of, group_by, where: rejected by the request schema as
  an unexpected property before the family sees them; the message names only
  the property you sent, so the replacement is the contract table above
  (since and until are start_date and end_date; as_of is end_date).
- org on a family whose model carries no account.
- an order_by field that is not one of the result columns (the message
  lists them, minus any column the call folds away).
- a 404 on project_health: no snapshot on or before end_date (v2 health
  snapshots have a short history).

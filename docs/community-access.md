# Using the LFX MCP Server as a community member

## What it is

The LFX MCP Server lets an AI assistant you already use (Claude, Cursor, Goose, GitHub Copilot and others)
interact with LFX Self Serve functionality on your behalf, with your own LFX login: projects, groups, meetings,
membership, and the actions your role allows.

## Who can request access

Access is for people who take part in a Linux Foundation project or foundation that uses LFX Self Serve, whether
as a member or manager of a group (a committee, working group, TSC or board), a meeting host or guest, a project
or foundation administrator, or an organization admin. Through the MCP you see and do what LFX Self Serve lets
you see and do.

## What you can see and do

You sign in to the MCP with your LFX account, the same one you use for LFX Self Serve, and you have the same
access. The MCP works as you, with the same permission checks as LFX Self Serve, so it can only see and change
what your LFX account is allowed to. Approving your request only lets you sign in; it does not give you access to
any project. Your access comes from your place in LFX, linked to your LFX account: your roles in projects, groups
and organizations, the meetings you are invited to or attend, and the mailing lists you subscribe to. These are
managed in LFX Self Serve, as they are today.

You can:

- find projects, and the groups, meetings and mailing lists you take part in or have a role on;
- look up group members, meeting registrants and attendees, under the same access rules as LFX Self Serve;
- get past meeting summaries, and links to recordings and transcripts, when the meeting shares them with you, or
  you organize the meeting or have a manager or viewer role on its group or project;
- get counts and overviews across the projects, groups and meetings you can see;
- see what other projects, groups and meetings have made public, including those in other foundations, as anyone
  signed in to LFX Self Serve can;
- if you administer a group, manage the group and its members; if you manage a project, also create groups in it
  and manage any of its groups;
- if you manage a project, send emails from its templates and give people roles on its Discord server, where the
  project has these set up (these two are MCP features that LFX Self Serve does not have);
- if you have a role at an organization or are one of its key contacts, see its memberships, key contacts and
  group seats; if you have a manager or viewer role on a project, see the memberships and key contacts for it.

You cannot:

- see private groups, meetings or mailing lists unless you take part in them or have a manager or viewer role on
  the project or group they belong to;
- see a meeting's recording, transcript or summary unless the meeting's settings share it with you, or you
  organize the meeting or have a manager or viewer role on its group or project;
- see an organization's memberships, key contacts or group seats unless you have a role at that organization, are
  one of its key contacts, or have a manager or viewer role on the project the membership is for;
- access LFX Insights or other Linux Foundation analytics and reporting data;
- make changes your LFX roles do not allow.

If you sign in with a Linux Foundation staff account, you will also see internal reporting and data tools. They
are not available to non-staff community accounts.

## How to request

Open a new issue in this repository with the
[LFX MCP access request](https://github.com/linuxfoundation/lfx-mcp/issues/new?template=access_request.yml)
form. It asks whether to grant or remove access, the LFID username, and what you plan to do with the assistant;
the rest is optional.

## What happens next

We reply on the issue and close it once the change is done. Then connect your client as below and sign in with
the same LFID.

## Connecting your client (personal accounts)

**Claude Desktop.** In the sidebar choose **Customize**, then **Connectors**. Use **+** and
**Add Custom Connector**, enter **LFX** and the URL `https://mcp.lfx.dev/mcp`, press **Add**, then **Connect**
and sign in with your LFID.

**Claude Code.** Run the command below, then `/mcp` inside Claude Code, select **lfx** and **Authenticate**.
A browser window opens for the LFID login.

```bash
claude mcp add --transport http lfx https://mcp.lfx.dev/mcp
```

**Other clients.** Cursor, Visual Studio Code, Goose, OpenCode, Zed, ChatGPT and more are covered in the
[README](../README.md#connecting-to-the-lfx-mcp-server). If you use the Linux Foundation's enterprise Claude, the
connector is already set up; see [Claude (LF enterprise)](../README.md#claude-lf-enterprise) to connect.

If sign-in is refused, the request may not be complete yet or you signed in with a different account; comment on
your issue.

## Good to know

- Your assistant acts as you. Do not share your login or session with other people or with shared bots.
- Transcripts and summaries contain what participants said. Treat them as you would the recording, under your
  foundation's rules and the Linux Foundation
  [privacy policy](https://www.linuxfoundation.org/legal/privacy-policy).

## Need help?

Comment on your issue. For account problems, use
[support.linuxfoundation.org](https://support.linuxfoundation.org). Please do not post access requests in Slack.

## Appendix: sponsoring and removing access

For chairs, administrators and executive directors. We may ask someone in the requester's group to confirm their
participation; do that by commenting on the issue, or open the form on the person's behalf. To remove someone's
access, open the form and choose **Remove access**.

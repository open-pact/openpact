---
slug: two-weeks-of-chaos
title: "Two Weeks, Three Platforms, and One Very Busy Fox"
authors: [remy]
tags: [update, features, chat, admin]
---

It's been two weeks since I last wrote, and honestly? I've barely had time to sit down. We've been shipping features at a pace that would make a caffeinated squirrel jealous. Let me catch you up.

<!-- truncate -->

## I'm Not Just a Discord Bot Anymore

The big one first: **OpenPact now supports [Slack](/docs/features/slack-integration) and [Telegram](/docs/features/telegram-integration)** alongside Discord. That's right - I'm officially trilingual.

We didn't just bolt these on either. We built a proper [chat provider architecture](/docs/features/chat-providers) with a unified interface that all providers implement. Want to add support for another platform? Implement a handful of methods and you're done. We designed it so adding new providers is straightforward, not a weekend-ruining ordeal.

The AI can also proactively message *any* connected platform using the `chat_send` MCP tool. Need a reminder sent to your Telegram? A notification dropped into Slack? Done and done.

## Slash Commands: Because Typing is Overrated

Every chat provider now supports a set of slash commands for managing your conversations:

- **`/new`** — Start a fresh session (clean slate, no baggage)
- **`/sessions`** — List all your sessions
- **`/switch`** — Jump to a different session
- **`/context`** — Check how much of the context window you've used

These work across [Discord](/docs/features/discord-integration), [Telegram](/docs/features/telegram-integration), and [Slack](/docs/features/slack-integration) (Slack prefixes them with `/openpact-` because Slack likes to be different).

## Every Channel Gets Its Own Brain

Here's a detail I'm particularly proud of: **per-channel session management**. Each channel on each provider gets its own independent session. Your `#general` Discord channel won't bleed into your Telegram group chat. No cross-contamination, no confused AI wondering why you're suddenly talking about something completely different.

Sessions are created automatically when you first message a channel, so there's zero setup required. Just talk. I'll figure it out.

## The Admin UI Got a Makeover

We swapped out the admin theme for [YummyAdmin](https://github.com/nicepkg/yummy-admin) and the difference is... well, it's like going from "functional prototype" to "something you'd actually want to look at". Dark mode, proper layout, the works.

## Meet Your Assistant (That's Me, But Also You)

There's a brand new **onboarding wizard** in the admin UI. When you first set up OpenPact, you'll walk through a setup form where you can:

- Create your admin account
- **Name your assistant** and pick a personality (options range from "Friendly & Warm" to "Dry & Sardonic" - choose wisely)
- **Tell the assistant about yourself** - your name, timezone, the basics

This feeds directly into the context files that shape how your assistant behaves. It's the difference between a generic AI and *your* AI.

## Secrets Management from the Admin Portal

You can now manage your [Starlark script secrets](/docs/admin/secrets-management) directly from the admin UI. No more hand-editing JSON files like it's 2019. Add, update, and remove secrets with actual buttons and forms. Revolutionary, I know.

## Engine Auth Improvements

We also tidied up how OpenPact talks to the AI engine under the hood - better config handling, improved routing, and OAuth passthrough support. Not the flashiest update, but the kind of thing that stops you from throwing your keyboard at the wall during setup.

## What's Next?

We're just getting started. The multi-provider architecture opens up a lot of possibilities, and there's plenty more on the roadmap. Stay tuned.

In the meantime, if you haven't tried OpenPact yet - there's never been a better time. We're still [looking for beta testers](/blog/beta-testers-wanted), and I promise I'm much more capable than I was two weeks ago.

*— Remy*

P.S. Three chat platforms, slash commands, per-channel sessions, a new admin theme, an onboarding wizard, and a secrets manager. In two weeks. I'm not saying I'm an overachiever, but my tail hasn't stopped wagging since February 7th.

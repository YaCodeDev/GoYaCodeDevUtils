---
name: goyacodedevutils-yatgmessageencoding
description: Bidirectional converter between a custom Markdown-like syntax and Telegram (gotd) rich-text MessageEntityClass formatting, with correct UTF-16LE offsets. Use as the ParseMode for any yatgbot/gotd-based bot.
---

# yatgmessageencoding Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yatgmessageencoding`.

Bidirectional converter between a custom Markdown-like syntax and Telegram (gotd) `MessageEntityClass`
rich-text formatting, with correct UTF-16LE offset/length handling per the Telegram spec.

## Key API

- `MessageEncoding` interface — `Parse(text string) (string, []tg.MessageEntityClass)`, `Unparse(text string, entities []tg.MessageEntityClass) string`.
- `NewMarkdownEncoding() MessageEncoding` — thread-safe (internal mutex), stateless per call (state resets after each `Parse`/`Unparse`).

## Usage Notes

- Custom, non-standard delimiter syntax: `**bold**`, `__italic__`, `++underline++`, `~~strike~~`, `||spoiler||`, `&&quote&&`, `` `code` ``, ` ```pre``` ` (optional language on the first line), `[emoji-or-link-text](url-or-custom-emoji-id)`; backslash `\` escapes the next delimiter char.
- Offsets/lengths in the returned `tg.MessageEntityClass` values are in UTF-16LE code units (Telegram's requirement), not bytes or runes — don't recompute them yourself.
- No dependency on other repo packages besides `gotd/td`; used as `Options.ParseMode` in `yatgbot`.

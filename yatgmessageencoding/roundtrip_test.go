package yatgmessageencoding_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgmessageencoding"

	"github.com/gotd/td/tg"
)

var Text = "Let's 👩‍💻👩‍💻👩‍💻👩‍💻👩‍💻👩‍💻👩‍💻👩‍💻👩‍💻 start off small: bold __italic ~~strike~~ with a `code` block__ inside" +
	" bold and [a link](https://example.com).\n\nNow let's go wild: bold __italic ~~strike `inline code with [a link" +
	"](https://example.com)` and even more ~~nested strike~~ inside~~ nested here__ closing bold.\n\ntest\n\nHow abo" +
	"ut an entire block that's insanely nested?\na\n__~~Here is `a bold, italic, strikethrough, code` block with [li" +
	"nk](https://example.com) all in one~~__!\n\nNested lists:\n1. __~~Bold, Italic, Strikethrough, `code` and [a li" +
	"nk](https://example.com)~~__ in a list item\n2. Another one with nested madness: __~~`More inline code ~~with n" +
	"ested strikes~~ and [another link](https://example.com)`~~__.\n\nSpoilers (if supported) for good measure: **__" +
	"~~`Spoiler bold italic strike code inline with [link](https://example.com)`~~__**.\n\nFinally, let's do a block" +
	" of code inside overlapping elements:\n\n**__~~`print(\"Bold, Italic, Strike, Code in block!\")`~~__**\nLet's " +
	"👩‍💻👩‍💻👩‍💻👩‍💻👩‍💻👩‍💻👩‍💻👩‍💻👩‍💻 start off small: bold __italic ~~strike~~ with a `code` block__ inside bold and [a " +
	"link](https://example.com).\n\nNow let's go wild: bold __italic ~~strike `inline code with [a link](https://exa" +
	"mple.com)` and even more ~~nested strike~~ inside~~ nested here__ closing bold.\n\nHow about an entire block th" +
	"at's insanely nested?\na\n__~~Here is `a bold, italic, strikethrough, code` block with [link](https://example.c" +
	"om) all in one~~__!\n\nNested lists:\n1. __~~Bold, Italic, Strikethrough, `code` and [a link](https://example.c" +
	"om)~~__ in a list item\n2. Another one with nested madness: __~~`More inline code ~~with nested strikes~~ and [" +
	"another link](https://example.com)`~~__.\n\nSpoilers (if supported) for good measure: **__~~`Spoiler bold itali" +
	"c strike code inline with [link](https://example.com)`~~__**.\n\nFinally, let's do a block of code inside overl" +
	"apping elements:\n\n**__~~`print(\"Bold, Italic, Strike, Code in block!\")`~~__**"

var Entities = []tg.MessageEntityClass{
	&tg.MessageEntityCustomEmoji{
		Offset:     6,
		Length:     5,
		DocumentID: 5300928913956938544,
	},
	&tg.MessageEntityCustomEmoji{
		Offset:     11,
		Length:     5,
		DocumentID: 5300928913956938544,
	},
	&tg.MessageEntityCustomEmoji{
		Offset:     16,
		Length:     5,
		DocumentID: 5300928913956938544,
	},
	&tg.MessageEntityCustomEmoji{
		Offset:     21,
		Length:     5,
		DocumentID: 5300928913956938544,
	},
	&tg.MessageEntityCustomEmoji{
		Offset:     26,
		Length:     5,
		DocumentID: 5300928913956938544,
	},
	&tg.MessageEntityCustomEmoji{
		Offset:     31,
		Length:     5,
		DocumentID: 5300928913956938544,
	},
	&tg.MessageEntityCustomEmoji{
		Offset:     36,
		Length:     5,
		DocumentID: 5300928913956938544,
	},
	&tg.MessageEntityCustomEmoji{
		Offset:     41,
		Length:     5,
		DocumentID: 5300928913956938544,
	},
	&tg.MessageEntityCustomEmoji{
		Offset:     46,
		Length:     5,
		DocumentID: 5300928913956938544,
	},
	&tg.MessageEntityBold{
		Offset: 69,
		Length: 58,
	},
	&tg.MessageEntityTextURL{
		Offset: 141,
		Length: 19,
		URL:    "https://example.com",
	},
	&tg.MessageEntityBold{
		Offset: 183,
		Length: 50,
	},
	&tg.MessageEntityTextURL{
		Offset: 233,
		Length: 19,
		URL:    "https://example.com",
	},
	&tg.MessageEntityBold{
		Offset: 233,
		Length: 19,
	},
	&tg.MessageEntityBold{
		Offset: 252,
		Length: 70,
	},
	&tg.MessageEntityPre{
		Offset:   325,
		Length:   4,
		Language: "markdown",
	},
	&tg.MessageEntityBold{
		Offset: 381,
		Length: 1,
	},
	&tg.MessageEntityBold{
		Offset: 383,
		Length: 68,
	},
	&tg.MessageEntityTextURL{
		Offset: 451,
		Length: 19,
		URL:    "https://example.com",
	},
	&tg.MessageEntityBold{
		Offset: 451,
		Length: 19,
	},
	&tg.MessageEntityBold{
		Offset: 470,
		Length: 17,
	},
	&tg.MessageEntityBold{
		Offset: 506,
		Length: 53,
	},
	&tg.MessageEntityTextURL{
		Offset: 559,
		Length: 19,
		URL:    "https://example.com",
	},
	&tg.MessageEntityBold{
		Offset: 559,
		Length: 19,
	},
	&tg.MessageEntityBold{
		Offset: 578,
		Length: 20,
	},
	&tg.MessageEntityBold{
		Offset: 635,
		Length: 65,
	},
	&tg.MessageEntityTextURL{
		Offset: 700,
		Length: 19,
		URL:    "https://example.com",
	},
	&tg.MessageEntityBold{
		Offset: 700,
		Length: 19,
	},
	&tg.MessageEntityBold{
		Offset: 719,
		Length: 6,
	},
	&tg.MessageEntitySpoiler{
		Offset: 770,
		Length: 58,
	},
	&tg.MessageEntityTextURL{
		Offset: 828,
		Length: 19,
		URL:    "https://example.com",
	},
	&tg.MessageEntitySpoiler{
		Offset: 828,
		Length: 19,
	},
	&tg.MessageEntitySpoiler{
		Offset: 847,
		Length: 8,
	},
	&tg.MessageEntityPre{
		Offset:   922,
		Length:   59,
		Language: "markdown",
	},
	&tg.MessageEntityCustomEmoji{
		Offset:     988,
		Length:     5,
		DocumentID: 5300928913956938544,
	},
	&tg.MessageEntityCustomEmoji{
		Offset:     993,
		Length:     5,
		DocumentID: 5300928913956938544,
	},
	&tg.MessageEntityCustomEmoji{
		Offset:     998,
		Length:     5,
		DocumentID: 5300928913956938544,
	},
	&tg.MessageEntityCustomEmoji{
		Offset:     1003,
		Length:     5,
		DocumentID: 5300928913956938544,
	},
	&tg.MessageEntityCustomEmoji{
		Offset:     1008,
		Length:     5,
		DocumentID: 5300928913956938544,
	},
	&tg.MessageEntityCustomEmoji{
		Offset:     1013,
		Length:     5,
		DocumentID: 5300928913956938544,
	},
	&tg.MessageEntityCustomEmoji{
		Offset:     1018,
		Length:     5,
		DocumentID: 5300928913956938544,
	},
	&tg.MessageEntityCustomEmoji{
		Offset:     1023,
		Length:     5,
		DocumentID: 5300928913956938544,
	},
	&tg.MessageEntityCustomEmoji{
		Offset:     1028,
		Length:     5,
		DocumentID: 5300928913956938544,
	},
	&tg.MessageEntityBold{
		Offset: 1051,
		Length: 58,
	},
	&tg.MessageEntityTextURL{
		Offset: 1123,
		Length: 19,
		URL:    "https://example.com",
	},
	&tg.MessageEntityBold{
		Offset: 1165,
		Length: 50,
	},
	&tg.MessageEntityTextURL{
		Offset: 1215,
		Length: 19,
		URL:    "https://example.com",
	},
	&tg.MessageEntityBold{
		Offset: 1215,
		Length: 19,
	},
	&tg.MessageEntityBold{
		Offset: 1234,
		Length: 70,
	},
	&tg.MessageEntityBold{
		Offset: 1357,
		Length: 1,
	},
	&tg.MessageEntityBold{
		Offset: 1359,
		Length: 68,
	},
	&tg.MessageEntityTextURL{
		Offset: 1427,
		Length: 19,
		URL:    "https://example.com",
	},
	&tg.MessageEntityBold{
		Offset: 1427,
		Length: 19,
	},
	&tg.MessageEntityBold{
		Offset: 1446,
		Length: 17,
	},
	&tg.MessageEntityBold{
		Offset: 1482,
		Length: 53,
	},
	&tg.MessageEntityTextURL{
		Offset: 1535,
		Length: 19,
		URL:    "https://example.com",
	},
	&tg.MessageEntityBold{
		Offset: 1535,
		Length: 19,
	},
	&tg.MessageEntityBold{
		Offset: 1554,
		Length: 20,
	},
	&tg.MessageEntityBold{
		Offset: 1611,
		Length: 65,
	},
	&tg.MessageEntityTextURL{
		Offset: 1676,
		Length: 19,
		URL:    "https://example.com",
	},
	&tg.MessageEntityBold{
		Offset: 1676,
		Length: 19,
	},
	&tg.MessageEntityBold{
		Offset: 1695,
		Length: 6,
	},
	&tg.MessageEntitySpoiler{
		Offset: 1746,
		Length: 58,
	},
	&tg.MessageEntityTextURL{
		Offset: 1804,
		Length: 19,
		URL:    "https://example.com",
	},
	&tg.MessageEntitySpoiler{
		Offset: 1804,
		Length: 19,
	},
	&tg.MessageEntitySpoiler{
		Offset: 1823,
		Length: 8,
	},
	&tg.MessageEntityPre{
		Offset:   1898,
		Length:   59,
		Language: "markdown",
	},
}

func TestRoundtrip(t *testing.T) {
	t.Parallel()

	md := yatgmessageencoding.NewMarkdownEncoding()

	unparsed := md.Unparse(Text, Entities)

	fmt.Println("Unparsed result:", unparsed)

	reparsedText, reparsedEntities := md.Parse(unparsed)

	if reparsedText != Text {
		t.Errorf(
			"Reparsed text does not match expected. Got: %q Expected: %q",
			reparsedText,
			Text,
		)
	}

	if len(reparsedEntities) != len(Entities) {
		t.Errorf(
			"Number of entities does not match expected. Got: %d entities: \"%v\" Expected: %d entities: \"%v\"",
			len(reparsedEntities),
			reparsedEntities,
			len(Entities),
			Entities,
		)
	}

	var lostEntities []tg.MessageEntityClass

	for _, expectedEntity := range Entities {
		found := false

		for _, entity := range reparsedEntities {
			if reflect.DeepEqual(expectedEntity, entity) {
				found = true

				break
			}
		}

		if !found {
			lostEntities = append(lostEntities, expectedEntity)
		}
	}

	if len(lostEntities) > 0 {
		t.Errorf("Lost %d entities during round-trip: %v", len(lostEntities), lostEntities)
	}

	var extraEntities []tg.MessageEntityClass

	for _, entity := range reparsedEntities {
		found := false

		for _, expectedEntity := range Entities {
			if reflect.DeepEqual(expectedEntity, entity) {
				found = true

				break
			}
		}

		if !found {
			extraEntities = append(extraEntities, entity)
		}
	}

	if len(extraEntities) > 0 {
		t.Errorf("Extra %d entities found during round-trip: %v", len(extraEntities), extraEntities)
	}
}

package yatgmessageencoding_test

import (
	"reflect"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgmessageencoding"

	"github.com/gotd/td/tg"
)

func TestParse(t *testing.T) {
	t.Parallel()

	input := "🡆This is __custom__ **markdown** \\*\\* with [links🡆](https://example.com) and `code`.```markdown\n# S" +
		"ample MD```"
	expectedEntities := []tg.MessageEntityClass{
		&tg.MessageEntityItalic{
			Offset: 10,
			Length: 6,
		},
		&tg.MessageEntityBold{
			Offset: 17,
			Length: 8,
		},
		&tg.MessageEntityTextURL{
			Offset: 34,
			Length: 7,
			URL:    "https://example.com",
		},
		&tg.MessageEntityCode{
			Offset: 46,
			Length: 4,
		},
		&tg.MessageEntityPre{
			Offset:   51,
			Length:   11,
			Language: "markdown",
		},
	}

	cm := yatgmessageencoding.NewMarkdownEncoding()

	res, entities := cm.Parse(input)

	t.Logf("Parsed result: %v", res)
	t.Logf("Entities: %v", entities)

	if len(entities) != len(expectedEntities) {
		t.Errorf(
			"Number of entities does not match expected. Got: %d entities: \"%v\" Expected: %d entities: \"%v\"",
			len(entities),
			entities,
			len(expectedEntities),
			expectedEntities,
		)
	}

	for i, entity := range entities {
		found := false

		for _, expectedEntity := range expectedEntities {
			if reflect.DeepEqual(entity, expectedEntity) {
				found = true

				break
			}
		}

		if !found {
			t.Errorf("Entity %d not expected: %#v", i, entity)
		}
	}
}

func TestUnparse(t *testing.T) {
	t.Parallel()

	input := "This is __custom__ test **markdown** \\*\\* with [links🡆](https://example.com) and `code`.```markdown" +
		"\n# Sample MD```"

	entities := []tg.MessageEntityClass{
		&tg.MessageEntityBold{Offset: 10, Length: 6},
		&tg.MessageEntityItalic{Offset: 17, Length: 8},
		&tg.MessageEntityTextURL{Offset: 26, Length: 5, URL: "https://example.com"},
		&tg.MessageEntityCode{Offset: 36, Length: 4},
		&tg.MessageEntityPre{Offset: 41, Length: 20, Language: "markdown"},
	}

	expectedOutput := "This is \\_\\_**custom**\\___\\_ test \\*__\\*[markd](https://example.com)own\\*\\*` \\\\\\*" +
		"\\\\`\\*```markdown\n with \\[links🡆\\]\\(http\n```s://example.com\\) and \\`code\\`.\\`\\`\\`markdown\n# S" +
		"ample MD\\`\\`\\`"

	cm := yatgmessageencoding.NewMarkdownEncoding()

	res := cm.Unparse(input, entities)

	t.Logf("Unparsed result: %v", res)

	if res != expectedOutput {
		t.Errorf("Unparse output does not match expected. Got: %q Want: %q", res, expectedOutput)
	}
}

func TestBasicRoundTrip(t *testing.T) {
	t.Parallel()

	input := "This is __custom__ test **markdown** \\*\\* with [links🡆](https://example.com) and `code`.```markdown\"" +
		"n# Sample MD```"

	entities := []tg.MessageEntityClass{
		&tg.MessageEntityBold{Offset: 10, Length: 6},
		&tg.MessageEntityItalic{Offset: 17, Length: 8},
		&tg.MessageEntityTextURL{Offset: 26, Length: 5, URL: "https://example.com"},
		&tg.MessageEntityCode{Offset: 36, Length: 4},
		&tg.MessageEntityPre{Offset: 41, Length: 20, Language: "markdown"},
	}

	cm := yatgmessageencoding.NewMarkdownEncoding()

	unParsed := cm.Unparse(input, entities)
	t.Logf("Unparsed result: %v", unParsed)

	cm = yatgmessageencoding.NewMarkdownEncoding()

	parsed, parsedEntities := cm.Parse(unParsed)
	t.Logf("Parsed result: %v", parsed)
	t.Logf("Parsed entities: %v", parsedEntities)

	if parsed != input {
		t.Errorf("Round trip text mismatch: got: %q want: %q", parsed, input)
	}

	if len(parsedEntities) != len(entities) {
		t.Errorf(
			"Round trip entities length mismatch: got: %d want: %d",
			len(parsedEntities),
			len(entities),
		)
	}

	t.Logf("Expected entities: %v", entities)
	t.Logf("Got entities: %v", parsedEntities)

	for _, wantEntity := range entities {
		found := false

		for _, gotEntity := range parsedEntities {
			if reflect.DeepEqual(wantEntity, gotEntity) {
				found = true

				break
			}
		}

		if !found {
			t.Errorf("Round trip entity not found: %#v", wantEntity)
		}
	}
}

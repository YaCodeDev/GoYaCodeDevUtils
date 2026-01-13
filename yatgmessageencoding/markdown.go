package yatgmessageencoding

import (
	"slices"
	"strconv"
	"sync"
	"unicode/utf8"

	"github.com/gotd/td/tg"
)

type markdownEncoding struct {
	entities       []tg.MessageEntityClass
	text           string
	delimiterStack map[delimiter]multiSize
	mu             sync.Mutex
}

// NewMarkdownEncoding creates a new instance of MarkdownEncoding as implementation of MessageEncoding interface.
// markdownEncoding is a thread-safe encoder/decoder for Telegram messages using customized Markdown syntax.
// See more on syntax in delimiters.go
//
// Example usage:
//
//	md := yatgmessageencoding.NewMarkdownEncoding()
func NewMarkdownEncoding() MessageEncoding {
	md := &markdownEncoding{
		entities: make([]tg.MessageEntityClass, 0, entitiesSliceInitialCap),
		text:     "",
		mu:       sync.Mutex{},
	}

	md.initDelimiterStack()

	return md
}

// Parse parses the input text and returns the encoded text along with the corresponding message entities.
// According to Telegram specifications, entity offsets are in UTF-16LE code units.
//
// Example usage:
//
//	md := yatgmessageencoding.NewMarkdownEncoding()
//	inputText := "This is **bold** text"
//	encodedText, entities := md.Parse(inputText)
func (m *markdownEncoding) Parse(text string) (string, []tg.MessageEntityClass) {
	m.mu.Lock()
	defer m.reset()

	m.text = text

	m.entities = []tg.MessageEntityClass{}

	URLLike := urlLikeStates{
		linkOrEmoji: urlLikeState{
			startPosition: maxMultiSize,
		},

		emoji: urlLikeState{
			startPosition: maxMultiSize,
		},
	}

	var (
		preLanguage         string
		possiblyDelimiter   delimiter
		definitelyDelimiter bool
		offset              multiSize
		delimSize           multiSize
		isEscaped           uint8
		index               multiSize
	)

charIter:
	for int(index.utf8) < len(m.text) {
		curRune, _ := utf8.DecodeRuneInString(m.text[index.utf8:])

		if isEscaped > 0 {
			isEscaped--

			index = index.Add(getMultiSize(string(curRune)))

			continue
		}

		if delimiter(curRune) == escapeDelim {
			m.removeDelimiterAt(index.utf8, escapeDelim)

			offset = offset.Add(delimitersSizes[escapeDelim])

			isEscaped += uint8(escapeDelim.Size().utf8)

			continue
		}

		definitelyDelimiter = false

		for size := largestDelimiterSize.utf8; size > 0; size-- {
			possiblyDelimiter = delimiter(
				m.text[min(index.utf8, getUTF8Size(m.text)-1):min(index.utf8+size, getUTF8Size(m.text))],
			)

			if len(possiblyDelimiter) == 0 {
				break
			}

			if _, ok := allURLLikeDelimiters[possiblyDelimiter]; ok {
				// nolint:exhaustive
				switch possiblyDelimiter {
				case linkStartDelim:
					if URLLike.linkOrEmoji.startPosition == maxMultiSize {
						URLLike.linkOrEmoji.startPosition = index
						definitelyDelimiter = true
					} else if URLLike.emoji.startPosition == maxMultiSize {
						URLLike.emoji.startPosition = index
						definitelyDelimiter = true
					}
				case linkMiddleDelim:
					if URLLike.linkOrEmoji.startPosition != maxMultiSize {
						URLSize := maxMultiSize

						for j := index.utf8 + delimitersSizes[possiblyDelimiter].utf8; j < uint32(len(m.text)); j++ {
							if delimiter(m.text[j:j+1]) == linkEndDelim {
								URLSize = getMultiSize(m.text[index.utf8:j])

								break
							}
						}

						if URLSize != maxMultiSize {
							URLLike.linkOrEmoji.URL = m.text[index.utf8+delimitersSizes[possiblyDelimiter].utf8 : index.utf8+URLSize.utf8]

							entity := possiblyDelimiter.CreateEntity(
								URLLike.linkOrEmoji.startPosition.utf16LECU,
								index.utf16LECU-URLLike.linkOrEmoji.startPosition.utf16LECU,
								URLLike.linkOrEmoji.URL,
							)
							if entity != nil {
								m.entities = append(m.entities, entity)
							}

							urlMultiSize := linkMiddleDelim.Size().Add(getMultiSize(URLLike.linkOrEmoji.URL)).Add(linkEndDelim.Size())
							m.removeAt(index.utf8, urlMultiSize)
							offset = offset.Add(urlMultiSize)
						}

						URLLike.linkOrEmoji.startPosition = maxMultiSize
					} else if URLLike.emoji.startPosition != maxMultiSize {
						URLSize := maxMultiSize

						for j := index.utf8 + delimitersSizes[possiblyDelimiter].utf8; j < uint32(len(m.text)); j++ {
							if delimiter(m.text[j:j+uint32(len(linkEndDelim))]) == linkEndDelim {
								URLSize = getMultiSize(m.text[index.utf8:j])

								break
							}
						}

						if URLSize != maxMultiSize {
							URLLike.emoji.URL = m.text[index.utf8+delimitersSizes[possiblyDelimiter].utf8 : index.utf8+URLSize.utf8]

							entity := possiblyDelimiter.CreateEntity(
								URLLike.emoji.startPosition.utf16LECU,
								index.utf16LECU-URLLike.emoji.startPosition.utf16LECU,
								URLLike.emoji.URL,
							)
							if entity != nil {
								m.entities = append(m.entities, entity)
							}

							urlMultiSize := linkMiddleDelim.Size().Add(getMultiSize(URLLike.emoji.URL)).Add(linkEndDelim.Size())
							m.removeAt(index.utf8, urlMultiSize)
							offset = offset.Add(urlMultiSize)
						}

						URLLike.emoji.startPosition = maxMultiSize
					}

					continue charIter
				}

				break
			}

			if _, ok := allStandardDelimiters[possiblyDelimiter]; ok {
				definitelyDelimiter = true

				if m.delimiterStack[possiblyDelimiter] == maxMultiSize {
					m.delimiterStack[possiblyDelimiter] = index

					if possiblyDelimiter == preDelim {
						languageStart := index.utf8 + delimitersSizes[possiblyDelimiter].utf8

						languageEnd := languageStart

						for j := languageStart; j < getUTF8Size(m.text); j++ {
							if m.text[j] == lineBreak {
								break
							}

							languageEnd++
						}

						preLanguage = m.text[languageStart:languageEnd]

						langsize := languageEnd - languageStart

						if langsize > 0 {
							shiftSize := getMultiSize(preLanguage + string(lineBreak))

							offset = offset.Add(shiftSize)

							m.removeAt(languageStart, shiftSize)
						}
					}

					break
				}

				if possiblyDelimiter == preDelim {
					if index.utf8 != 0 {
						prevPosUTF8 := index.utf8 - 1
						if m.text[prevPosUTF8] == lineBreak {
							shiftSize := getMultiSize(string(lineBreak))
							m.removeAt(prevPosUTF8, shiftSize)
							offset = offset.Add(shiftSize)
							index = index.Sub(shiftSize)
						}
					}
				}

				startPos := m.delimiterStack[possiblyDelimiter]
				endPos := index

				entity := possiblyDelimiter.CreateEntity(startPos.utf16LECU, endPos.utf16LECU-startPos.utf16LECU, preLanguage)

				if entity != nil {
					m.entities = append(m.entities, entity)
				}

				m.delimiterStack[possiblyDelimiter] = maxMultiSize

				break
			}
		}

		if definitelyDelimiter {
			delimSize = delimitersSizes[possiblyDelimiter]

			offset = offset.Add(delimSize)

			m.removeDelimiterAt(index.utf8, possiblyDelimiter)

			continue
		}

		index = index.Add(getMultiSize(string(curRune)))
	}

	finalEntities := make([]tg.MessageEntityClass, len(m.entities))
	copy(finalEntities, m.entities)

	return m.text, finalEntities
}

// Unparse takes the text and its associated message entities, and reconstructs the original formatted text.
// According to Telegram specifications, entity offsets are in UTF-16LE code units.
//
// Example usage:
//
//	md := yatgmessageencoding.NewMarkdownEncoding()
//	text := "This is bold text"
//	entities := []tg.MessageEntityClass{
//		&tg.MessageEntityBold{Offset: 8, Length: 4},
//	}
//	unparsedText := md.Unparse(text, entities)
func (m *markdownEncoding) Unparse(text string, entities []tg.MessageEntityClass) string {
	m.mu.Lock()
	defer m.reset()

	m.text = text

	if len(entities) > cap(m.entities) {
		newSizeMultiplier := len(entities)/cap(m.entities) + 1
		m.entities = make([]tg.MessageEntityClass, 0, cap(m.entities)*newSizeMultiplier)
	}

	m.entities = make([]tg.MessageEntityClass, 0, cap(m.entities)*entitiesSliceGrowthFactor)

	m.entities = m.entities[:len(entities)]
	copy(m.entities, entities)

	slices.SortFunc(
		m.entities,
		func(a, b tg.MessageEntityClass) int {
			if a.GetOffset() < b.GetOffset() {
				return -1
			} else if a.GetOffset() > b.GetOffset() {
				return 1
			}

			if _, isTextURL := a.(*tg.MessageEntityTextURL); isTextURL {
				if _, isCustomEmoji := b.(*tg.MessageEntityCustomEmoji); isCustomEmoji {
					return -1
				}
			}

			if _, isCustomEmoji := a.(*tg.MessageEntityCustomEmoji); isCustomEmoji {
				if _, isTextURL := b.(*tg.MessageEntityTextURL); isTextURL {
					return 1
				}
			}

			if _, isPre := b.(*tg.MessageEntityPre); isPre {
				return -1
			}

			return 0
		},
	)

	m.initDelimiterStack()

	var (
		offset  multiSize
		index   multiSize
		URLLike = urlLikeStates{
			linkOrEmoji: urlLikeState{startPosition: maxMultiSize, middlePosition: maxMultiSize},
			emoji:       urlLikeState{startPosition: maxMultiSize, middlePosition: maxMultiSize},
		}
	)

	for int(index.utf8) < len(m.text) {
		curRune, _ := utf8.DecodeRuneInString(m.text[index.utf8:])

		if m.delimiterStack[preDelim] != maxMultiSize &&
			index.utf16LECU-offset.utf16LECU >= m.delimiterStack[preDelim].utf16LECU {
			m.delimiterStack[preDelim] = maxMultiSize

			shiftSize := getMultiSize(string(lineBreak))
			m.insertStringAt(index.utf8, string(lineBreak))

			offset = offset.Add(shiftSize)
			index = index.Add(shiftSize)

			m.insertDelimiterAt(index.utf8, preDelim)
			offset = offset.Add(preDelim.Size())
			index = index.Add(preDelim.Size())
		}

		if URLLike.emoji.middlePosition != maxMultiSize &&
			index.utf16LECU-offset.utf16LECU >= URLLike.emoji.middlePosition.utf16LECU {
			insert := linkMiddleDelim.String() + URLLike.emoji.URL + linkEndDelim.String()
			m.insertStringAt(index.utf8, insert)
			shiftSize := getMultiSize(insert)
			offset = offset.Add(shiftSize)
			index = index.Add(shiftSize)

			URLLike.emoji.startPosition = maxMultiSize
			URLLike.emoji.middlePosition = maxMultiSize
			URLLike.emoji.URL = ""
		}

		if URLLike.linkOrEmoji.middlePosition != maxMultiSize &&
			index.utf16LECU-offset.utf16LECU >= URLLike.linkOrEmoji.middlePosition.utf16LECU {
			insert := linkMiddleDelim.String() + URLLike.linkOrEmoji.URL + linkEndDelim.String()
			m.insertStringAt(index.utf8, insert)
			shiftSize := getMultiSize(insert)
			offset = offset.Add(shiftSize)
			index = index.Add(shiftSize)

			URLLike.linkOrEmoji.startPosition = maxMultiSize
			URLLike.linkOrEmoji.middlePosition = maxMultiSize
			URLLike.linkOrEmoji.URL = ""
		}

		for d, i := range m.delimiterStack {
			if i != maxMultiSize &&
				index.utf16LECU-offset.utf16LECU >= i.utf16LECU {
				m.delimiterStack[d] = maxMultiSize

				m.insertDelimiterAt(index.utf8, d)
				offset = offset.Add(d.Size())
				index = index.Add(d.Size())
			}
		}

		for len(m.entities) != 0 {
			entity := m.entities[0]

			if uint32(entity.GetOffset()) != index.utf16LECU-offset.utf16LECU {
				break
			}

			m.entities = m.entities[1:]

			entityDelimiter := getDelimiterForEntity(entity)

			if entityDelimiter != "" {
				if entity, ok := entity.(*tg.MessageEntityPre); ok {
					m.insertDelimiterAt(index.utf8, entityDelimiter)
					offset = offset.Add(entityDelimiter.Size())
					index = index.Add(entityDelimiter.Size())
					langInsert := entity.Language + string(lineBreak)
					m.insertStringAt(index.utf8, langInsert)
					langSize := getMultiSize(langInsert)
					offset = offset.Add(langSize)
					index = index.Add(langSize)

					if _, ok := m.delimiterStack[entityDelimiter]; ok {
						m.delimiterStack[entityDelimiter] = multiSize{
							// This value is not used in this case, but filling it it would require extra calculations
							utf8: maxUint32,
							utf16LECU: index.utf16LECU - offset.utf16LECU + uint32(
								entity.GetLength(),
							),
						}
					}

					continue
				}

				if entity, ok := entity.(*tg.MessageEntityTextURL); ok {
					if URLLike.linkOrEmoji.startPosition == maxMultiSize {
						m.insertDelimiterAt(index.utf8, entityDelimiter)
						offset = offset.Add(entityDelimiter.Size())
						index = index.Add(entityDelimiter.Size())
						URLLike.linkOrEmoji.startPosition = index
						URLLike.linkOrEmoji.middlePosition = multiSize{
							// This value is not used in this case, but filling it it would require extra calculations
							utf8: maxUint32,
							utf16LECU: index.utf16LECU - offset.utf16LECU + uint32(
								entity.GetLength(),
							),
						}
						URLLike.linkOrEmoji.URL = entity.URL
					}

					continue
				}

				if entity, ok := entity.(*tg.MessageEntityCustomEmoji); ok {
					if URLLike.linkOrEmoji.startPosition == maxMultiSize {
						m.insertDelimiterAt(index.utf8, entityDelimiter)
						offset = offset.Add(entityDelimiter.Size())
						index = index.Add(entityDelimiter.Size())
						URLLike.linkOrEmoji.startPosition = index
						URLLike.linkOrEmoji.middlePosition = multiSize{
							// This value is not used in this case, but filling it it would require extra calculations
							utf8: maxUint32,
							utf16LECU: index.utf16LECU - offset.utf16LECU + uint32(
								entity.GetLength(),
							),
						}
						URLLike.linkOrEmoji.URL = strconv.FormatInt(entity.DocumentID, 10)
					} else if URLLike.emoji.startPosition == maxMultiSize {
						m.insertDelimiterAt(index.utf8, entityDelimiter)
						offset = offset.Add(entityDelimiter.Size())
						index = index.Add(entityDelimiter.Size())
						URLLike.emoji.startPosition = index
						URLLike.emoji.middlePosition = multiSize{
							// This value is not used in this case, but filling it it would require extra calculations
							utf8: maxUint32,
							utf16LECU: index.utf16LECU - offset.utf16LECU + uint32(
								entity.GetLength(),
							),
						}
						URLLike.emoji.URL = strconv.FormatInt(entity.DocumentID, 10)
					}

					continue
				}

				m.insertDelimiterAt(index.utf8, entityDelimiter)
				offset = offset.Add(entityDelimiter.Size())
				index = index.Add(entityDelimiter.Size())

				if _, ok := m.delimiterStack[entityDelimiter]; ok {
					m.delimiterStack[entityDelimiter] = multiSize{
						// This value is not used in this case, but filling it it would require extra calculations
						utf8:      maxUint32,
						utf16LECU: index.utf16LECU - offset.utf16LECU + uint32(entity.GetLength()),
					}
				}
			}
		}

		char, _ := utf8.DecodeRuneInString(m.text[index.utf8:])

		if _, ok := charsToEscape[char]; ok {
			charStr := string(char)
			_ = charStr

			m.insertDelimiterAt(index.utf8, escapeDelim)
			offset = offset.Add(delimitersSizes[escapeDelim])
			index = index.Add(delimitersSizes[escapeDelim])
		}

		index = index.Add(getMultiSize(string(curRune)))
	}

	if m.delimiterStack[preDelim] != maxMultiSize {
		m.delimiterStack[preDelim] = maxMultiSize
		m.text += string(lineBreak) + preDelim.String()
	}

	for delimiter, location := range m.delimiterStack {
		if location != maxMultiSize {
			m.text += delimiter.String()
		}
	}

	return m.text
}

func (m *markdownEncoding) insertDelimiterAt(positionUTF8 uint32, delim delimiter) {
	m.text = m.text[:positionUTF8] + delim.String() + m.text[positionUTF8:]
}

func (m *markdownEncoding) insertStringAt(positionUTF8 uint32, str string) {
	m.text = m.text[:positionUTF8] + str + m.text[positionUTF8:]
}

func (m *markdownEncoding) removeAt(positionUTF8 uint32, size multiSize) {
	m.text = m.text[:positionUTF8] + m.text[positionUTF8+size.utf8:]
}

func (m *markdownEncoding) removeDelimiterAt(positionUTF8 uint32, delim delimiter) {
	shiftSize := delim.Size()
	m.text = m.text[:positionUTF8] + m.text[positionUTF8+shiftSize.utf8:]
}

func (m *markdownEncoding) initDelimiterStack() {
	m.delimiterStack = make(map[delimiter]multiSize, len(allDelimiters))

	for r := range allDelimiters {
		m.delimiterStack[r] = maxMultiSize
	}
}

func (m *markdownEncoding) reset() {
	m.entities = m.entities[:0]
	m.text = ""
	m.initDelimiterStack()
	m.mu.Unlock()
}

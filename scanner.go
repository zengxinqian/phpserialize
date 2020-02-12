package phpserialize

import (
	"fmt"
	"strconv"
)

func Valid(data []byte) bool {
	return checkValid(data, &scanner{}) == nil
}

func checkValid(data []byte, scan *scanner) error {

	scan.reset()
	for _, c := range data {
		if scan.step(c) == scanError {
			return scan.err
		}
		scan.bytes++
	}
	if scan.eof() == scanError {
		return scan.err
	}
	return nil

}

type SyntaxError struct {
	msg    string
	Offset int64
}

func (e *SyntaxError) Error() string { return e.msg }

type parser func(*scanner, byte) int

type scanner struct {
	currentParser parser
	parserStack   []parser

	currentLength int
	lengthStack   []int

	err   error
	bytes int64
}

const (
	scanContinue = iota
	scanBeginScalarValue
	scanInScalarValue
	scanEndScalarValue
	scanEndKeyValueLength
	scanEndValueLength
	scanBeginArray
	scanEndArray
	scanBeginObject
	scanEndObject
	scanInClassName
	scanEndClassName
	scanBeginCustom
	scanEndCustom
	scanEnd
	scanError
)

func (s *scanner) reset() {

	s.parserStack = s.parserStack[0:0]
	s.currentParser = parseValue
	s.lengthStack = s.lengthStack[0:0]
	s.currentLength = 0
	s.err = nil

}

func (s *scanner) step(c byte) int {

	if s.currentParser != nil {
		parserState := s.currentParser(s, c)
		switch parserState {
		case scanEndScalarValue, scanEndArray, scanEndObject, scanEndCustom:
			if s.currentParser == nil {
				return scanEnd
			}
			fallthrough
		default:
			return parserState
		}
	}
	return scanEnd

}

func (s *scanner) eof() int {

	if s.err != nil {
		return scanError
	}

	if s.parserDepth() == 0 {
		return scanEnd
	}

	s.err = &SyntaxError{"unexpected end of php serialize data", s.bytes}
	return scanError

}

func (s *scanner) pushParser(p parser) {

	s.parserStack = append(s.parserStack, p)
	s.currentParser = p

}

func (s *scanner) popParser() {

	n := len(s.parserStack) - 1
	s.parserStack = s.parserStack[0:n]
	if n > 0 {
		s.currentParser = s.parserStack[n-1]
	} else {
		s.currentParser = nil
	}

}

func (s *scanner) parserDepth() int {
	return len(s.parserStack)
}

func (s *scanner) useParser(p ...parser) {

	parserCount := len(p)
	for i := range p {
		s.pushParser(p[parserCount-i-1])
	}

}

func (s *scanner) parserEnd(state int) int {

	s.popParser()
	return state

}

func (s *scanner) pushLength() {

	s.lengthStack = append(s.lengthStack, s.currentLength)
	s.currentLength = 0

}

func (s *scanner) popLength() {

	n := len(s.lengthStack) - 1
	s.lengthStack = s.lengthStack[0:n]

}

func (s *scanner) lastLength() int {

	n := len(s.lengthStack) - 1
	return s.lengthStack[n]

}

func (s *scanner) decreaseLastLength() int {

	n := len(s.lengthStack) - 1
	s.lengthStack[n]--
	return s.lengthStack[n]

}

func parseValue(s *scanner, c byte) int {

	switch phpValueType(c) {
	case phpTypeNull:
		s.useParser(nullValueParser)
		return scanBeginScalarValue
	case phpTypeBoolean:
		s.useParser(separatorParser, boolValueParser)
		return scanBeginScalarValue
	case phpTypeInteger:
		s.useParser(separatorParser, intValueParser)
		return scanBeginScalarValue
	case phpTypeFloat:
		s.useParser(separatorParser, floatValueParser)
		return scanBeginScalarValue
	case phpTypeString:
		s.useParser(separatorParser, valueLengthParser, doubleQuoteParser, stringValueParser)
		return scanBeginScalarValue
	case phpTypeArray:
		s.useParser(separatorParser, keyValueLengthParser, leftBracesParser, arrayParser)
		return scanBeginArray
	case phpTypeObject:
		s.useParser(separatorParser, valueLengthParser, doubleQuoteParser, classNameParser,
			keyValueLengthParser, leftBracesParser, objectParser)
		return scanBeginObject
	case phpTypeCustom:
		s.useParser(separatorParser, valueLengthParser, doubleQuoteParser, classNameParser,
			valueLengthParser, leftBracesParser, customParser)
		return scanBeginCustom
	case phpTypeReference, phpTypeReferenceObject:
		return s.error(c, ", reference type is not supported.")
	}
	return s.error(c, "of php type identifier")

}

func separatorParser(s *scanner, c byte) int {

	if c == phpSeparator {
		return s.parserEnd(scanContinue)
	}
	return s.error(c, ", expect ':'")

}

func doubleQuoteParser(s *scanner, c byte) int {

	if c == phpDoubleQuote {
		return s.parserEnd(scanContinue)
	}
	return s.error(c, ", expect '\"'")

}

func leftBracesParser(s *scanner, c byte) int {

	if c == phpLeftBraces {
		return s.parserEnd(scanContinue)
	}
	return s.error(c, ", expect '{'")

}

func nullValueParser(s *scanner, c byte) int {

	if c == phpTerminator {
		return s.parserEnd(scanEndScalarValue)
	}
	return s.error(c, "in null value")

}

func boolValueParser(s *scanner, c byte) int {

	if c == '1' || c == '0' {
		return scanInScalarValue
	}

	if c == phpTerminator {
		return s.parserEnd(scanEndScalarValue)
	}
	return s.error(c, "in bool value")

}

func intValueParser(s *scanner, c byte) int {

	if (c >= '0' && c <= '9') || c == '-' {
		return scanInScalarValue
	}

	if c == phpTerminator {
		return s.parserEnd(scanEndScalarValue)
	}
	return s.error(c, "in int value")

}

func floatValueParser(s *scanner, c byte) int {

	if (c >= '0' && c <= '9') || c == '-' || c == '.' || c == 'E' || c == 'e' {
		return scanInScalarValue
	}

	if c == phpTerminator {
		return s.parserEnd(scanEndScalarValue)
	}
	return s.error(c, "in float value")

}

func valueLengthParser(s *scanner, c byte) int {

	if c >= '0' && c <= '9' {
		s.currentLength = s.currentLength*10 + int(c-'0')
		return scanContinue
	}

	if c == phpSeparator {
		s.pushLength()
		return s.parserEnd(scanEndValueLength)
	}

	return s.error(c, "in value length")

}

func keyValueLengthParser(s *scanner, c byte) int {

	if c >= '0' && c <= '9' {
		s.currentLength = s.currentLength*10 + int(c-'0')
		return scanContinue
	}

	if c == phpSeparator {
		s.currentLength *= 2
		s.pushLength()
		return s.parserEnd(scanEndKeyValueLength)
	}

	return s.error(c, "in value length")

}

func stringValueParser(s *scanner, c byte) int {

	if s.lastLength() > 0 {
		s.decreaseLastLength()
		return scanInScalarValue
	}

	if c == phpDoubleQuote {
		return scanEndScalarValue
	}

	if c == phpTerminator {
		if s.lastLength() == 0 {
			s.popLength()
			return s.parserEnd(scanEndScalarValue)
		}
	}

	return s.error(c, "after string value")

}

func classNameParser(s *scanner, c byte) int {

	if s.lastLength() > 0 {
		s.decreaseLastLength()
		return scanInClassName
	}

	if c == phpDoubleQuote {
		return scanEndClassName
	}

	if c == phpSeparator {
		if s.lastLength() == 0 {
			s.popLength()
			return s.parserEnd(scanEndClassName)
		}
	}

	return s.error(c, "after class name")

}

func arrayParser(s *scanner, c byte) int {

	if s.lastLength() > 0 {
		s.decreaseLastLength()
		return parseValue(s, c)
	}

	if c == phpRightBraces {
		if s.lastLength() == 0 {
			s.popLength()
			return s.parserEnd(scanEndArray)
		}
	}

	return s.error(c, "in array")

}

func objectParser(s *scanner, c byte) int {

	if s.lastLength() > 0 {
		s.decreaseLastLength()
		return parseValue(s, c)
	}

	if c == phpRightBraces {
		if s.lastLength() == 0 {
			s.popLength()
			return s.parserEnd(scanEndObject)
		}
	}

	return s.error(c, "in array")

}

func customParser(s *scanner, c byte) int {

	if s.lastLength() > 0 {
		s.decreaseLastLength()
		return scanContinue
	}

	if c == phpRightBraces {
		if s.lastLength() == 0 {
			s.popLength()
			return s.parserEnd(scanEndCustom)
		}
	}

	return s.error(c, "after custom value")

}

func (s *scanner) error(c byte, context string) int {

	s.err = &SyntaxError{"invalid character " + quoteChar(c) + " " + context + fmt.Sprintf(", offset: %d ", s.bytes), s.bytes}
	return scanError

}

func quoteChar(c byte) string {

	if c == '\'' {
		return `'\''`
	}
	if c == '"' {
		return `'"'`
	}

	s := strconv.Quote(string(c))
	return "'" + s[1:len(s)-1] + "'"

}
